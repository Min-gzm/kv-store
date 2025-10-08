package kv_store

import (
	"fmt"
	"github.com/google/uuid"
	"io"
	"kv_store/config"
	"kv_store/data"
	"kv_store/index"
	myio "kv_store/io"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

type DB struct {
	config     config.DBConfig
	mutex      *sync.RWMutex
	activeFile *data.File
	oldFiles   map[string]*data.File
	indexer    index.Indexer
}

func (db *DB) Start(config config.DBConfig) (*DB, error) {
	if config.DataDirPath == "" {
		return nil, ErrDataDirPathNotSet
	}

	if config.DataFileSize <= 0 {
		return nil, ErrInvalidDataFileSize
	}

	// check if data directory exists, if not create it
	if err := os.MkdirAll(config.DataDirPath, 0755); err != nil {
		return nil, err
	}

	// Create DB instance
	instance := &DB{
		config:   config,
		mutex:    new(sync.RWMutex),
		oldFiles: make(map[string]*data.File),
		indexer:  index.NewBTree(2),
	}

	// Load existing data files
	if err := db.loadDataFiles(); err != nil {
		return nil, err
	}

	return instance, nil
}

type fileWithTime struct {
	ID      string
	ModTime time.Time
}

func (db *DB) loadDataFiles() error {
	dir, err := os.ReadDir(db.config.DataDirPath)
	if err != nil {
		return err
	}

	var files []fileWithTime
	// filter .data files and get their modification times for sorting
	for _, entry := range dir {
		if !entry.IsDir() && len(entry.Name()) > 5 && entry.Name()[len(entry.Name())-5:] == data.FileSuffix {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			fileID := strings.TrimSuffix(entry.Name(), data.FileSuffix)
			files = append(files, fileWithTime{
				ID:      fileID,
				ModTime: info.ModTime(),
			})
		}
	}

	// Sort files by modification time, oldest first
	sort.Slice(files, func(i, j int) bool {
		return files[i].ModTime.Before(files[j].ModTime)
	})

	// load data files from oldest to newest
	for i := 0; i < len(files); i++ {
		dataFile, err := data.NewFile(db.config.DataDirPath, files[i].ID)
		if err != nil {
			continue
		}

		if i == len(files)-1 {
			db.activeFile = dataFile
		} else {
			db.oldFiles[files[i].ID] = dataFile
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

			if entry.Type == data.EntryNormal {
				db.indexer.Put(entry.Key, indexOfEntry)
			} else if entry.Type == data.EntryDeleted {
				db.indexer.Delete(entry.Key)
			}

			offset += int64(size)
		}

		// set the write offset for the current active file
		if i == len(files)-1 {
			dataFile.WriteOffset = offset
		}
	}

	return nil
}

func (db *DB) Put(key []byte, value []byte) error {
	if len(key) == 0 {
		return ErrKeyIsEmpty
	}

	entry := &data.Entry{
		Key:   key,
		Value: value,
		Type:  data.EntryNormal,
	}

	indexOfEntry, err := db.appendEntry(entry)
	if err != nil {
		return err
	}

	if ok := db.indexer.Put(key, indexOfEntry); !ok {
		return ErrIndexUpdateFailed
	}

	return nil
}

func (db *DB) Get(key []byte) ([]byte, error) {
	db.mutex.RLock()
	defer db.mutex.RUnlock()

	if len(key) == 0 {
		return nil, ErrKeyIsEmpty
	}

	// get index
	indexOfEntry := db.indexer.Get(key)
	if indexOfEntry == nil {
		return nil, ErrKeyNotFound
	}

	var dataFile *data.File
	// find the data file
	if indexOfEntry.FileID == db.activeFile.ID {
		dataFile = db.activeFile
	} else {
		dataFile = db.oldFiles[indexOfEntry.FileID]
	}

	if dataFile == nil {
		return nil, ErrFileNotFound
	}

	// read data from data file
	entry, _, err := db.activeFile.Read(indexOfEntry.Offset)
	if err != nil {
		return nil, err
	}

	return entry.Value, nil
}

func (db *DB) Delete(key []byte) error {
	if len(key) == 0 {
		return ErrKeyIsEmpty
	}

	// check if key exists
	existingIndex := db.indexer.Get(key)
	if existingIndex == nil {
		return ErrKeyNotFound
	}

	// create a delete entry
	entry := &data.Entry{
		Key:   key,
		Value: []byte{},
		Type:  data.EntryDeleted,
	}
	_, err := db.appendEntry(entry)
	if err != nil {
		return err
	}

	if ok := db.indexer.Delete(key); !ok {
		return ErrIndexUpdateFailed
	}
	return nil
}

func (db *DB) appendEntry(entry *data.Entry) (*index.Index, error) {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	// check if the active file exists
	if db.activeFile == nil {
		if err := db.setupActiveFile(); err != nil {
			return nil, err
		}
	}

	encodedEntry, size := data.EncodeEntry(entry)
	if db.activeFile.WriteOffset+int64(size) > db.config.DataFileSize {
		if err := db.activeFile.Sync(); err != nil {
			return nil, err
		}
		db.oldFiles[db.activeFile.ID] = db.activeFile
		if err := db.setupActiveFile(); err != nil {
			return nil, err
		}
	}

	if db.config.SyncAfterEveryWrites {
		if err := db.activeFile.Sync(); err != nil {
			return nil, err
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

func (db *DB) setupActiveFile() error {
	fileID := uuid.New().String()
	fileName := fmt.Sprintf("%s/%s%s", db.config.DataDirPath, fileID, data.FileSuffix)
	fileManager, err := myio.NewIOManager(fileName)
	if err != nil {
		return err
	}
	dataFile := &data.File{
		ID:          fileID,
		WriteOffset: 0,
		IOManager:   fileManager,
	}
	db.activeFile = dataFile

	return nil
}
