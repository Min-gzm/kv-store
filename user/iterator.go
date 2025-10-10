package user

import (
	"kv_store/config"
	"kv_store/index"
	"kv_store/iterator"
)

type Iterator struct {
	indexerIterator iterator.Iterator[*index.Index]
	db              *DB
	config          *config.IteratorConfig
}

func (db *DB) NewIterator(config *config.IteratorConfig) *Iterator {
	db.mutex.RLock()
	defer db.mutex.RUnlock()

	return &Iterator{
		indexerIterator: db.indexer.Iterator(config.Reverse),
		db:              db,
		config:          config,
	}
}

func (it *Iterator) Rewind() {
	it.indexerIterator.Rewind()
	it.skipToNext()
}

func (it *Iterator) Seek(key []byte) {
	it.indexerIterator.Seek(key)
}

func (it *Iterator) Next() {
	it.indexerIterator.Next()
	it.skipToNext()
}

func (it *Iterator) Valid() bool {
	return it.indexerIterator.Valid()
}

func (it *Iterator) Key() []byte {
	return it.indexerIterator.Key()
}

func (it *Iterator) Value() []byte {
	it.db.mutex.RLock()
	defer it.db.mutex.RUnlock()

	indexOfEntry := it.indexerIterator.Value()
	value, err := it.db.readEntryValueByIndex(indexOfEntry)
	if err != nil {
		return nil
	}
	return value
}

func (it *Iterator) Close() {
	it.indexerIterator.Close()
}

func (it *Iterator) skipToNext() {
	if it.config == nil || len(it.config.Prefix) == 0 {
		return
	}

	for ; it.Valid(); it.Next() {
		key := it.Key()
		if len(key) >= len(it.config.Prefix) && string(key[:len(it.config.Prefix)]) == string(it.config.Prefix) {
			return
		}
	}
}
