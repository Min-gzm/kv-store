package user

import (
	"io"
	"kv_store"
	"kv_store/data"
	"kv_store/index"
	"kv_store/transaction"
	"kv_store/utils"
	"os"
	"path"
	"path/filepath"
	"sort"
	"time"
)

const (
	mergeDirName     = "merge"
	mergeFinishedKey = "merge_finished"
)

func (db *DB) Merge() error {
	err := db.startMerge()
	if err != nil {
		return err
	}
	defer db.endMerge()

	// check if the reclaim size reach the merge ratio
	dataSize, err := utils.GetDirSize(db.config.DataDirPath)
	if float64(dataSize/db.reclaimSize) < db.config.MergeRatio {
		return nil
	}

	// read all old files from db
	oldFiles, firstNonMergedFileID := db.getSortedOldFiles()

	mergeDir := db.getMergeDir()
	// check if the path already exists
	// if not, create it, if yes, remove it and recreate
	err = os.RemoveAll(mergeDir)
	if err != nil {
		return err
	}
	err = os.MkdirAll(mergeDir, os.ModePerm)
	if err != nil {
		return err
	}

	mergeConfig := db.config
	mergeConfig.SyncAfterEveryWrites = false
	mergeConfig.DataDirPath = mergeDir

	// create new db instance to merge
	mergeDB, err := OpenDB(mergeConfig)
	if err != nil {
		return err
	}
	hintFile, err := data.NewHintFile(mergeConfig.DataDirPath)
	if err != nil {
		return err
	}

	// handle merging process for each file from old to new
	for _, file := range oldFiles {
		var offset int64 = 0
		for {
			entry, size, err := file.Read(offset)
			if err != nil {
				if err == io.EOF {
					break
				}
				return err
			}

			// compare the index from old file with the index from current db
			currentIndex := db.indexer.Get(entry.Key)
			if currentIndex != nil &&
				currentIndex.FileID == file.ID &&
				currentIndex.Offset == offset {
				// remove the index seq
				entry.Seq = transaction.NonTxnSeq
				// this entry is valid, add it in the merge db
				newIndex, err := mergeDB.appendEntry(entry)
				if err != nil {
					return err
				}

				// write this new index into the hint file
				if err = hintFile.WriteHintFile(entry.Key, newIndex); err != nil {
					return err
				}
			}

			offset += int64(size)
		}
	}

	// sync the hint file
	if err = hintFile.Sync(); err != nil {
		return err
	}
	// sync the db
	if err = mergeDB.Sync(); err != nil {
		return err
	}

	// add a merge finished file to indicate the merging process is finished or not
	finishedFile, err := data.NewMergeFinishedFile(mergeConfig.DataDirPath)
	if err != nil {
		return err
	}
	mergeFinishedEntry := &data.Entry{
		Key:   []byte(mergeFinishedKey),
		Value: []byte(firstNonMergedFileID),
		Type:  data.EntryNormal,
		Seq:   transaction.NonTxnSeq,
	}
	encodedEntry, _ := data.EncodeEntry(mergeFinishedEntry)
	if err = finishedFile.Write(encodedEntry); err != nil {
		return err
	}
	if err = finishedFile.Sync(); err != nil {
		return err
	}

	return nil
}

func (db *DB) getMergeDir() string {
	rootDir := path.Dir(db.config.DataDirPath)
	dbDir := path.Dir(db.config.DataDirPath)
	margePath := filepath.Join(rootDir, dbDir+mergeDirName)
	return margePath
}

func (db *DB) getSortedOldFiles() ([]*data.File, string) {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	files := make([]*data.File, 0)
	for _, file := range db.oldFiles {
		files = append(files, file)
	}

	// sort the files in ascending order by time
	sort.Slice(files, func(i, j int) bool {
		return files[i].Timestamp < files[j].Timestamp
	})

	return files, db.activeFile.ID
}

func (db *DB) startMerge() error {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	if db.activeFile == nil {
		return kv_store.ErrDatabaseIsEmpty
	}

	if db.isMerging {
		return kv_store.ErrMergeInProgress
	}

	db.isMerging = true
	return nil
}

func (db *DB) endMerge() {
	db.mutex.Lock()
	defer db.mutex.Unlock()
	db.isMerging = false
}

func (db *DB) loadMergeFiles() error {
	mergeDir := db.getMergeDir()
	if _, err := os.Stat(mergeDir); os.IsNotExist(err) {
		return nil
	}
	defer func() {
		if err := os.RemoveAll(mergeDir); err != nil {
			return
		}
	}()

	files, err := os.ReadDir(mergeDir)
	if err != nil {
		return err
	}

	// check if there is a merge finished file
	var isFinished = false
	for _, file := range files {
		if file.Name() == data.MergeFinishedFileName {
			isFinished = true
			break
		}
	}
	if !isFinished {
		return kv_store.ErrMergeIsNotFinished
	}

	firstNonMergeFileID, err := db.getFirstNonMergedFileID(mergeDir)
	if err != nil {
		return err
	}
	firstNonMergedFile := db.oldFiles[firstNonMergeFileID]
	if firstNonMergedFile == nil {
		return kv_store.ErrLastNonMergedFileNotFound
	}
	maxTimestamp := db.oldFiles[firstNonMergeFileID].Timestamp

	// replace data files in the db with data files in the merge data dir
	// get data files in the db
	originalDataFiles, err := os.ReadDir(db.config.DataDirPath)
	// remove previous data files, remove files before the first non merged file
	for _, dataFile := range originalDataFiles {
		if dataFile.Name() == firstNonMergeFileID {
			break
		}
		dataFilePath := path.Join(db.config.DataDirPath, dataFile.Name())
		err = os.Remove(dataFilePath)
	}

	// put the merge file into the db data dir
	// in order to maintain the order of the merged files and the non-merged files
	// we need to modify the mod time of the merged files manually
	// to make sure that the mod time them is less than the timestamp of the first non-merged file
	for _, file := range files {
		// move the file into the db data dir
		oldPath := path.Join(mergeDir, file.Name())
		newPath := path.Join(db.config.DataDirPath, file.Name())
		err = os.Rename(oldPath, newPath)
		if err != nil {
			return err
		}

		// edit the mod time
		t := time.Unix(maxTimestamp-100, 0)
		err = os.Chtimes(newPath, t, t)
		if err != nil {
			return err
		}
	}

	return nil
}

func (db *DB) getFirstNonMergedFileID(dir string) (string, error) {
	file, err := data.NewMergeFinishedFile(dir)
	if err != nil {
		return "", err
	}
	entry, _, err := file.Read(0)
	if err != nil {
		return "", err
	}

	return string(entry.Value), nil
}

func (db *DB) loadIndexesFromHintFile() error {
	dir := db.getMergeDir()
	hintFilePath := path.Join(dir, data.HintFileName)
	if _, err := os.Stat(hintFilePath); os.IsNotExist(err) {
		return nil
	}

	hintFile, err := data.LoadHintFile(dir)
	if err != nil {
		return err
	}

	var offset int64 = 0
	for {
		entry, size, err := hintFile.Read(offset)
		if err != nil {
			if err == io.EOF {
				break
			} else {
				return err
			}
		}
		decodedIndex, err := index.DecodeIndex(entry.Value)
		if err != nil {
			return err
		}
		db.indexer.Put(entry.Key, decodedIndex)
		offset += int64(size)
	}

	return nil
}
