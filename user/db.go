package user

import (
	"github.com/gofrs/flock"
	"github.com/google/uuid"
	"io"
	"kv_store"
	"kv_store/config"
	"kv_store/data"
	"kv_store/index"
	myio "kv_store/io"
	"kv_store/transaction"
	"log"
	"os"
	"path"
	"sort"
	"strings"
	"sync"
)

const (
	fileLockFileName = "file-lock.lock"
)

type DB struct {
	config       config.DBConfig
	mutex        *sync.RWMutex
	activeFile   *data.File
	oldFiles     map[string]*data.File
	indexer      index.Indexer
	seqNum       uint64 // start from 1 and increment for each transaction
	isMerging    bool
	fileLock     *flock.Flock
	writtenBytes int64
	reclaimSize  int64
}

func OpenDB(config config.DBConfig) (*DB, error) {
	if config.DataDirPath == "" {
		return nil, kv_store.ErrDataDirPathNotSet
	}

	if config.DataFileSize <= 0 {
		return nil, kv_store.ErrInvalidDataFileSize
	}

	if config.MergeRatio < 0 && config.MergeRatio > 1 {
		return nil, kv_store.ErrInvalidMergeRatio
	}

	// check if data directory exists, if not create it
	if err := os.MkdirAll(config.DataDirPath, 0755); err != nil {
		return nil, err
	}

	// check if other process is using the data dir
	fileLock := flock.New(path.Join(config.DataDirPath, fileLockFileName))
	_, err := fileLock.TryLock()
	if err != nil {
		return nil, kv_store.ErrDataDirIsBusy
	}

	// Create DB instance
	instance := &DB{
		config:       config,
		mutex:        new(sync.RWMutex),
		oldFiles:     make(map[string]*data.File),
		indexer:      index.NewBTreeIndexer(2),
		fileLock:     fileLock,
		writtenBytes: 0,
		reclaimSize:  0,
	}

	// load merged files
	if err = instance.loadMergeFiles(); err != nil {
		return nil, err
	}

	// load hint file
	if err = instance.loadIndexesFromHintFile(); err != nil {
		return nil, err
	}

	// Load existing data files
	if err = instance.loadDataFiles(); err != nil {
		return nil, err
	}

	return instance, nil
}

func (db *DB) loadAllDataFilesFromDesk(ioType myio.FileManagerType) ([]*data.File, error) {
	var files []*data.File
	dir, err := os.ReadDir(db.config.DataDirPath)
	if err != nil {
		return files, err
	}

	// filter .data files and get their modification times for sorting
	for _, entry := range dir {
		if !entry.IsDir() && len(entry.Name()) > 5 && entry.Name()[len(entry.Name())-5:] == data.FileSuffix {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			fileID := strings.TrimSuffix(entry.Name(), data.FileSuffix)
			dataFile, err := data.LoadDataFile(db.config.DataDirPath, fileID, info.ModTime().Unix(), ioType)

			files = append(files, dataFile)
		}
	}

	// Sort files by modification time, oldest first
	sort.Slice(files, func(i, j int) bool {
		return files[i].Timestamp < files[j].Timestamp
	})

	return files, nil
}

func (db *DB) loadDataFiles() error {
	files, err := db.loadAllDataFilesFromDesk(myio.Mmap)
	defer func() {
		err = db.resetFileType()
		if err != nil {
			log.Printf("Error resetting file type: %v\n", err)
		}
	}()
	if err != nil {
		return err
	}

	hasMerged := false
	firstNonMergedFileID := ""
	// check if there is a merge finished file
	mergeFinishedFilePath := path.Join(db.config.DataDirPath, data.MergeFinishedFileName)
	if _, err := os.Stat(mergeFinishedFilePath); err == nil {
		firstNonMergedFileID, err = db.getFirstNonMergedFileID(db.config.DataDirPath)
		if err != nil {
			return err
		}
		hasMerged = true
	}

	// remove the merged files
	// because their indexes are loaded in the hint file
	if hasMerged {
		for i, file := range files {
			if file.ID == firstNonMergedFileID {
				files = files[i:]
				break
			}
		}
	}

	updateIndex := func(entry *data.Entry, indexOfEntry *index.Index) {
		var ok = true
		var oldIndex *index.Index
		if entry.Type == data.EntryNormal {
			ok, oldIndex = db.indexer.Put(entry.Key, indexOfEntry)
		} else if entry.Type == data.EntryDeleted {
			oldIndex = db.indexer.Delete(entry.Key)
		}
		if !ok {
			panic("failed to update index during loading")
		}
		if oldIndex != nil {
			db.reclaimSize += int64(oldIndex.Size)
		}
	}

	type entryWithIndex struct {
		entry *data.Entry
		index *index.Index
	}
	entryWithIndexes := make(map[uint64][]*entryWithIndex)
	var currentSeq = uint64(0)
	// load data files from oldest to newest
	for i, dataFile := range files {
		// set up the active and old files
		if i == len(files)-1 {
			db.activeFile = dataFile
		} else {
			db.oldFiles[dataFile.ID] = dataFile
		}

		// load index from data file
		var offset int64 = 0
		for {
			entry, size, err := dataFile.Read(offset)
			if err != nil {
				if err == io.EOF {
					break
				}
				return err
			}

			indexOfEntry := &index.Index{
				FileID: dataFile.ID,
				Offset: offset,
				Size:   size,
			}
			if entry.Seq == transaction.NonTxnSeq { // non-transactional entry
				updateIndex(entry, indexOfEntry)
			} else {
				if entry.Type == data.EntryTxFinished {
					for _, ewi := range entryWithIndexes[entry.Seq] {
						updateIndex(ewi.entry, ewi.index)
					}
					delete(entryWithIndexes, entry.Seq)
				} else {
					entryWithIndexes[entry.Seq] = append(entryWithIndexes[entry.Seq], &entryWithIndex{
						entry: entry,
						index: indexOfEntry,
					})
				}
			}
			if entry.Seq > currentSeq {
				currentSeq = entry.Seq
			}

			offset += int64(size)
		}

		// update the seq number in the db
		db.seqNum = currentSeq

		// set the write offset for the current active file
		if i == len(files)-1 {
			dataFile.WriteOffset = offset
		}
	}

	return nil
}

func (db *DB) resetFileType() error {
	// db is empty
	if db.activeFile == nil {
		return nil
	}

	if err := db.activeFile.ChangeIOType(myio.General); err != nil {
		return err
	}

	for _, file := range db.oldFiles {
		if err := file.ChangeIOType(myio.General); err != nil {
			return err
		}
	}

	return nil
}

func (db *DB) Put(key []byte, value []byte) error {
	if len(key) == 0 {
		return kv_store.ErrKeyIsEmpty
	}

	entry := &data.Entry{
		Key:   key,
		Value: value,
		Type:  data.EntryNormal,
		Seq:   transaction.NonTxnSeq,
	}

	indexOfEntry, err := db.appendEntry(entry)
	if err != nil {
		return err
	}

	ok, oldIndex := db.indexer.Put(key, indexOfEntry)
	if !ok {
		return kv_store.ErrIndexUpdateFailed
	}
	if oldIndex != nil {
		db.reclaimSize += int64(oldIndex.Size)
	}

	return nil
}

func (db *DB) Get(key []byte) ([]byte, error) {
	db.mutex.RLock()
	defer db.mutex.RUnlock()

	if len(key) == 0 {
		return nil, kv_store.ErrKeyIsEmpty
	}

	// get index
	indexOfEntry := db.indexer.Get(key)
	if indexOfEntry == nil {
		return nil, kv_store.ErrKeyNotFound
	}

	return db.readEntryValueByIndex(indexOfEntry)
}

func (db *DB) readEntryValueByIndex(indexOfEntry *index.Index) ([]byte, error) {
	var dataFile *data.File
	// find the data file
	if db.activeFile != nil && indexOfEntry.FileID == db.activeFile.ID {
		dataFile = db.activeFile
	} else {
		dataFile = db.oldFiles[indexOfEntry.FileID]
	}

	if dataFile == nil {
		return nil, kv_store.ErrFileNotFound
	}

	// read data from data file
	entry, _, err := dataFile.Read(indexOfEntry.Offset)
	if err != nil {
		return nil, err
	}
	return entry.Value, nil
}

func (db *DB) Delete(key []byte) error {
	if len(key) == 0 {
		return kv_store.ErrKeyIsEmpty
	}

	// check if key exists
	existingIndex := db.indexer.Get(key)
	if existingIndex == nil {
		return kv_store.ErrKeyNotFound
	}

	// create a delete entry
	entry := &data.Entry{
		Key:   key,
		Value: []byte{},
		Type:  data.EntryDeleted,
		Seq:   transaction.NonTxnSeq,
	}
	_, err := db.appendEntry(entry)
	if err != nil {
		return err
	}

	oldIndex := db.indexer.Delete(key)
	if oldIndex != nil {
		db.reclaimSize += int64(oldIndex.Size)
	}
	return nil
}

func (db *DB) appendEntryWithoutMutex(entry *data.Entry) (*index.Index, error) {
	// check if the active file exists
	if db.activeFile == nil {
		if err := db.setupActiveFile(); err != nil {
			return nil, err
		}
	}

	encodedEntry, size := data.EncodeEntry(entry)
	// create a new active file
	if db.activeFile.WriteOffset+int64(size) > db.config.DataFileSize {
		if err := db.activeFile.Sync(); err != nil {
			return nil, err
		}

		// update the writtenBytes
		db.writtenBytes = 0

		db.oldFiles[db.activeFile.ID] = db.activeFile
		if err := db.setupActiveFile(); err != nil {
			return nil, err
		}
	}
	if db.config.SyncAfterEveryWrites {
		if err := db.activeFile.Sync(); err != nil {
			return nil, err
		}
		db.writtenBytes = 0
	} else {
		db.writtenBytes += int64(size)
		if db.writtenBytes > db.config.BytesPerSync {
			if err := db.activeFile.Sync(); err != nil {
				return nil, err
			}
		}
	}

	indexOfEntry := &index.Index{
		FileID: db.activeFile.ID,
		Offset: db.activeFile.WriteOffset,
		Size:   size,
	}

	err := db.activeFile.Write(encodedEntry)
	if err != nil {
		return nil, err
	}

	return indexOfEntry, nil
}

func (db *DB) appendEntry(entry *data.Entry) (*index.Index, error) {
	db.mutex.Lock()
	defer db.mutex.Unlock()
	return db.appendEntryWithoutMutex(entry)
}

func (db *DB) setupActiveFile() error {
	fileID := uuid.New().String()
	dataFile, err := data.NewDataFile(db.config.DataDirPath, fileID)
	if err != nil {
		return err
	}
	db.activeFile = dataFile

	return nil
}

func (db *DB) ListKeys() [][]byte {
	it := db.indexer.Iterator(false)
	keys := make([][]byte, it.Size())
	it.Rewind()
	for i := 0; it.Valid(); it.Next() {
		keys[i] = it.Key()
		i++
	}
	return keys
}

func (db *DB) Fold(f func(key []byte, value []byte) bool) {
	it := db.indexer.Iterator(false)
	it.Rewind()
	for ; it.Valid(); it.Next() {
		key := it.Key()
		value, err := db.Get(key)
		if err != nil {
			continue
		}
		if !f(key, value) {
			break
		}
	}
}

func (db *DB) Sync() error {
	return db.activeFile.Sync()
}

func (db *DB) Close() error {
	defer func() {
		// unlock the dir
		err := db.fileLock.Unlock()
		if err != nil {
			panic(err)
		}
	}()

	db.mutex.Lock()
	defer db.mutex.Unlock()

	if db.activeFile == nil {
		return nil
	}

	log.Printf("Syncing Active File ID: %s", db.activeFile.ID)

	if err := db.Sync(); err != nil {
		return err
	}

	// close active file
	if err := db.activeFile.Close(); err != nil {
		return err
	}

	// close old files
	for _, file := range db.oldFiles {
		if err := file.Close(); err != nil {
			return err
		}
	}

	return nil
}
