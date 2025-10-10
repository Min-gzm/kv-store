package user

import (
	"kv_store"
	"kv_store/config"
	"kv_store/data"
	"kv_store/index"
	"sync"
	"sync/atomic"
)

const (
	txnFinishedKey = "txn_finished"
	nonTxnSeq      = 0
)

type MutexTransaction struct {
	mutex         *sync.RWMutex
	db            *DB
	config        *config.TransactionConfig
	pendingWrites []*data.Entry
}

func (db *DB) NewMutexTransaction(config *config.TransactionConfig) *MutexTransaction {
	return &MutexTransaction{
		mutex:         &sync.RWMutex{},
		db:            db,
		config:        config,
		pendingWrites: make([]*data.Entry, 0, config.MaxBatchSize),
	}
}

func (t *MutexTransaction) Put(key []byte, value []byte) error {
	if len(key) == 0 {
		return kv_store.ErrKeyIsEmpty
	}

	entry := &data.Entry{
		Key:   key,
		Value: value,
		Type:  data.EntryNormal,
	}

	t.mutex.Lock()
	defer t.mutex.Unlock()
	t.pendingWrites = append(t.pendingWrites, entry)

	return nil
}

func (t *MutexTransaction) Delete(key []byte) error {
	if len(key) == 0 {
		return kv_store.ErrKeyIsEmpty
	}

	entry := &data.Entry{
		Key:  key,
		Type: data.EntryDeleted,
	}

	t.mutex.Lock()
	defer t.mutex.Unlock()
	t.pendingWrites = append(t.pendingWrites, entry)

	return nil
}

func (t *MutexTransaction) Commit() error {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	if len(t.pendingWrites) == 0 {
		return nil
	}
	if len(t.pendingWrites) > t.config.MaxBatchSize {
		return kv_store.ErrExceededMaxBatchSize
	}

	seqNum := atomic.AddUint64(&t.db.seqNum, 1)
	indexes := make([]*index.Index, 0, len(t.pendingWrites))
	// update the index and write entries to the data file
	for _, entry := range t.pendingWrites {
		entryWithSeq := &data.Entry{
			Key:   entry.Key,
			Value: entry.Value,
			Type:  entry.Type,
			Seq:   seqNum,
		}
		indexOfEntry, err := t.db.appendEntryWithoutMutex(entryWithSeq)
		if err != nil {
			return err
		}
		indexes = append(indexes, indexOfEntry)

		if entryWithSeq.Type == data.EntryNormal {
			t.db.indexer.Put(entry.Key, indexOfEntry)
		} else if entryWithSeq.Type == data.EntryDeleted {
			t.db.indexer.Delete(entry.Key)
		} else {
			return kv_store.ErrInvalidEntryType
		}
	}

	// append a finished entry to indicate the transaction is complete
	finishedEntry := &data.Entry{
		Key:  []byte(txnFinishedKey),
		Type: data.EntryTxFinished,
		Seq:  seqNum,
	}
	_, err := t.db.appendEntryWithoutMutex(finishedEntry)
	if err != nil {
		return err
	}

	// sync the data file based on the config
	if t.config.SyncOnCommit {
		if err := t.db.activeFile.Sync(); err != nil {
			return err
		}
	}

	// update the indexer
	for i, entry := range t.pendingWrites {
		if entry.Type == data.EntryNormal {
			if ok := t.db.indexer.Put(entry.Key, indexes[i]); !ok {
				return kv_store.ErrIndexUpdateFailed
			}
		} else if entry.Type == data.EntryDeleted {
			if ok := t.db.indexer.Delete(entry.Key); !ok {
				return kv_store.ErrIndexUpdateFailed
			}
		} else {
			return kv_store.ErrInvalidEntryType
		}
	}

	// clear the pending writes
	t.pendingWrites = make([]*data.Entry, 0)
	return nil
}
