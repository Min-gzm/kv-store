package kv_store

import (
	"fmt"
	"github.com/google/uuid"
	"kv_store/config"
	"kv_store/data"
	"kv_store/index"
	"kv_store/io"
	"sync"
)

type DB struct {
	config     config.DBConfig
	mutex      sync.Mutex
	activeFile *data.File
	oldFiles   map[string]*data.File
	indexer    index.Indexer
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
	if indexOfEntry.FileId == db.activeFile.ID {
		dataFile = db.activeFile
	} else {
		dataFile = db.oldFiles[indexOfEntry.FileId]
	}

	if dataFile == nil {
		return nil, ErrFileNotFound
	}

	buffer := make([]byte, 0, indexOfEntry.Size)
	// read data from data file
	_, err := db.activeFile.IOManager.Read(buffer, indexOfEntry.Offset)
	if err != nil {
		return nil, err
	}

	entry, err := data.DecodeEntry(buffer)
	if err != nil {
		return nil, err
	}
	if entry.Type != data.EntryDeleted {
		return nil, ErrKeyNotFound
	}

	return entry.Value, nil
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
		if err := db.activeFile.IOManager.Sync(); err != nil {
			return nil, err
		}
		db.oldFiles[db.activeFile.ID] = db.activeFile
		if err := db.setupActiveFile(); err != nil {
			return nil, err
		}
	}

	if db.config.SyncAfterEveryWrites {
		if err := db.activeFile.IOManager.Sync(); err != nil {
			return nil, err
		}
	}

	indexOfEntry := &index.Index{
		FileId: db.activeFile.ID,
		Offset: db.activeFile.WriteOffset,
		Size:   size,
	}

	size, err := db.activeFile.IOManager.Write(encodedEntry)
	if err != nil {
		return nil, err
	}
	db.activeFile.WriteOffset += int64(size)

	return indexOfEntry, nil
}

func (db *DB) setupActiveFile() error {
	fileID := uuid.New().String()
	fileName := fmt.Sprintf("%s/%d.data", db.config.DataDirPath, fileID)
	fileManager, err := io.NewFileManager(fileName)
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
