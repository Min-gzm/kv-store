package user

import (
	"kv_store"
	"kv_store/config"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMutexTransaction_Put tests the Put functionality
func TestMutexTransaction_Put(t *testing.T) {
	t.Run("successful put single key", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)
		db, err := OpenDB(cfg)
		require.NoError(t, err)
		defer closeDB(t, db)

		txnConfig := &config.TransactionConfig{
			MaxBatchSize: 100,
			SyncOnCommit: false,
		}
		txn := db.NewMutexTransaction(txnConfig)

		err = txn.Put([]byte("key1"), []byte("value1"))
		assert.NoError(t, err)
	})

	t.Run("put multiple keys", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)
		db, err := OpenDB(cfg)
		require.NoError(t, err)
		defer closeDB(t, db)

		txnConfig := &config.TransactionConfig{
			MaxBatchSize: 100,
			SyncOnCommit: false,
		}
		txn := db.NewMutexTransaction(txnConfig)

		for i := 0; i < 10; i++ {
			key := []byte("key_" + string(rune(i)))
			value := []byte("value_" + string(rune(i)))
			err = txn.Put(key, value)
			require.NoError(t, err)
		}
	})

	t.Run("error when key is empty", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)
		db, err := OpenDB(cfg)
		require.NoError(t, err)
		defer closeDB(t, db)

		txnConfig := &config.TransactionConfig{
			MaxBatchSize: 100,
			SyncOnCommit: false,
		}
		txn := db.NewMutexTransaction(txnConfig)

		err = txn.Put([]byte(""), []byte("value"))
		assert.ErrorIs(t, err, kv_store.ErrKeyIsEmpty)
	})

	t.Run("error when key is nil", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)
		db, err := OpenDB(cfg)
		require.NoError(t, err)
		defer closeDB(t, db)

		txnConfig := &config.TransactionConfig{
			MaxBatchSize: 100,
			SyncOnCommit: false,
		}
		txn := db.NewMutexTransaction(txnConfig)

		err = txn.Put(nil, []byte("value"))
		assert.ErrorIs(t, err, kv_store.ErrKeyIsEmpty)
	})

	t.Run("put with empty value", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)
		db, err := OpenDB(cfg)
		require.NoError(t, err)
		defer closeDB(t, db)

		txnConfig := &config.TransactionConfig{
			MaxBatchSize: 100,
			SyncOnCommit: false,
		}
		txn := db.NewMutexTransaction(txnConfig)

		err = txn.Put([]byte("key"), []byte(""))
		assert.NoError(t, err)
	})
}

// TestMutexTransaction_Delete tests the Delete functionality
func TestMutexTransaction_Delete(t *testing.T) {
	t.Run("successful delete", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)
		db, err := OpenDB(cfg)
		require.NoError(t, err)
		defer closeDB(t, db)

		txnConfig := &config.TransactionConfig{
			MaxBatchSize: 100,
			SyncOnCommit: false,
		}
		txn := db.NewMutexTransaction(txnConfig)

		err = txn.Delete([]byte("key1"))
		assert.NoError(t, err)
	})

	t.Run("error when key is empty", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)
		db, err := OpenDB(cfg)
		require.NoError(t, err)
		defer closeDB(t, db)

		txnConfig := &config.TransactionConfig{
			MaxBatchSize: 100,
			SyncOnCommit: false,
		}
		txn := db.NewMutexTransaction(txnConfig)

		err = txn.Delete([]byte(""))
		assert.ErrorIs(t, err, kv_store.ErrKeyIsEmpty)
	})

	t.Run("error when key is nil", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)
		db, err := OpenDB(cfg)
		require.NoError(t, err)
		defer closeDB(t, db)

		txnConfig := &config.TransactionConfig{
			MaxBatchSize: 100,
			SyncOnCommit: false,
		}
		txn := db.NewMutexTransaction(txnConfig)

		err = txn.Delete(nil)
		assert.ErrorIs(t, err, kv_store.ErrKeyIsEmpty)
	})
}

// TestMutexTransaction_Commit tests the Commit functionality
func TestMutexTransaction_Commit(t *testing.T) {
	t.Run("commit empty transaction", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)
		db, err := OpenDB(cfg)
		require.NoError(t, err)
		defer closeDB(t, db)

		txnConfig := &config.TransactionConfig{
			MaxBatchSize: 100,
			SyncOnCommit: false,
		}
		txn := db.NewMutexTransaction(txnConfig)

		err = txn.Commit()
		assert.NoError(t, err)
	})

	t.Run("commit single put", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)
		db, err := OpenDB(cfg)
		require.NoError(t, err)
		defer closeDB(t, db)

		txnConfig := &config.TransactionConfig{
			MaxBatchSize: 100,
			SyncOnCommit: false,
		}
		txn := db.NewMutexTransaction(txnConfig)

		err = txn.Put([]byte("key1"), []byte("value1"))
		require.NoError(t, err)

		err = txn.Commit()
		assert.NoError(t, err)

		// Verify the key was written
		result, err := db.Get([]byte("key1"))
		require.NoError(t, err)
		assert.Equal(t, []byte("value1"), result)
	})

	t.Run("commit multiple puts", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)
		db, err := OpenDB(cfg)
		require.NoError(t, err)
		defer closeDB(t, db)

		txnConfig := &config.TransactionConfig{
			MaxBatchSize: 100,
			SyncOnCommit: false,
		}
		txn := db.NewMutexTransaction(txnConfig)

		expectedData := make(map[string]string)
		for i := 0; i < 10; i++ {
			key := "key_" + string(rune(i))
			value := "value_" + string(rune(i))
			expectedData[key] = value
			err = txn.Put([]byte(key), []byte(value))
			require.NoError(t, err)
		}

		err = txn.Commit()
		assert.NoError(t, err)

		// Verify all keys were written
		for key, expectedValue := range expectedData {
			result, err := db.Get([]byte(key))
			require.NoError(t, err)
			assert.Equal(t, []byte(expectedValue), result)
		}
	})

	t.Run("commit with delete", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)
		db, err := OpenDB(cfg)
		require.NoError(t, err)
		defer closeDB(t, db)

		// First add a key directly to the database
		err = db.Put([]byte("key1"), []byte("value1"))
		require.NoError(t, err)

		// Now delete it in a transaction
		txnConfig := &config.TransactionConfig{
			MaxBatchSize: 100,
			SyncOnCommit: false,
		}
		txn := db.NewMutexTransaction(txnConfig)

		err = txn.Delete([]byte("key1"))
		require.NoError(t, err)

		err = txn.Commit()
		assert.NoError(t, err)

		// Verify the key was deleted
		_, err = db.Get([]byte("key1"))
		assert.ErrorIs(t, err, kv_store.ErrKeyNotFound)
	})

	t.Run("commit with puts and deletes", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)
		db, err := OpenDB(cfg)
		require.NoError(t, err)
		defer closeDB(t, db)

		// Add some keys directly
		err = db.Put([]byte("key1"), []byte("value1"))
		require.NoError(t, err)
		err = db.Put([]byte("key2"), []byte("value2"))
		require.NoError(t, err)

		// Transaction: add new keys and delete one existing key
		txnConfig := &config.TransactionConfig{
			MaxBatchSize: 100,
			SyncOnCommit: false,
		}
		txn := db.NewMutexTransaction(txnConfig)

		err = txn.Put([]byte("key3"), []byte("value3"))
		require.NoError(t, err)
		err = txn.Put([]byte("key4"), []byte("value4"))
		require.NoError(t, err)
		err = txn.Delete([]byte("key1"))
		require.NoError(t, err)

		err = txn.Commit()
		assert.NoError(t, err)

		// Verify results
		_, err = db.Get([]byte("key1"))
		assert.ErrorIs(t, err, kv_store.ErrKeyNotFound) // Deleted

		result, err := db.Get([]byte("key2"))
		require.NoError(t, err)
		assert.Equal(t, []byte("value2"), result) // Still exists

		result, err = db.Get([]byte("key3"))
		require.NoError(t, err)
		assert.Equal(t, []byte("value3"), result) // New key

		result, err = db.Get([]byte("key4"))
		require.NoError(t, err)
		assert.Equal(t, []byte("value4"), result) // New key
	})

	t.Run("error when exceeding max batch size", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)
		db, err := OpenDB(cfg)
		require.NoError(t, err)
		defer closeDB(t, db)

		txnConfig := &config.TransactionConfig{
			MaxBatchSize: 5,
			SyncOnCommit: false,
		}
		txn := db.NewMutexTransaction(txnConfig)

		// Add more than max batch size
		for i := 0; i < 10; i++ {
			key := []byte("key_" + string(rune(i)))
			value := []byte("value_" + string(rune(i)))
			err = txn.Put(key, value)
			require.NoError(t, err)
		}

		err = txn.Commit()
		assert.ErrorIs(t, err, kv_store.ErrExceededMaxBatchSize)
	})

	t.Run("commit with sync on commit enabled", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)
		db, err := OpenDB(cfg)
		require.NoError(t, err)
		defer closeDB(t, db)

		txnConfig := &config.TransactionConfig{
			MaxBatchSize: 100,
			SyncOnCommit: true,
		}
		txn := db.NewMutexTransaction(txnConfig)

		err = txn.Put([]byte("key1"), []byte("value1"))
		require.NoError(t, err)

		err = txn.Commit()
		assert.NoError(t, err)

		// Verify the key was written
		result, err := db.Get([]byte("key1"))
		require.NoError(t, err)
		assert.Equal(t, []byte("value1"), result)
	})

	t.Run("commit clears pending writes", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)
		db, err := OpenDB(cfg)
		require.NoError(t, err)
		defer closeDB(t, db)

		txnConfig := &config.TransactionConfig{
			MaxBatchSize: 100,
			SyncOnCommit: false,
		}
		txn := db.NewMutexTransaction(txnConfig)

		// First transaction
		err = txn.Put([]byte("key1"), []byte("value1"))
		require.NoError(t, err)
		err = txn.Commit()
		require.NoError(t, err)

		// Second transaction with same txn object
		err = txn.Put([]byte("key2"), []byte("value2"))
		require.NoError(t, err)
		err = txn.Commit()
		assert.NoError(t, err)

		// Verify both keys exist
		result, err := db.Get([]byte("key1"))
		require.NoError(t, err)
		assert.Equal(t, []byte("value1"), result)

		result, err = db.Get([]byte("key2"))
		require.NoError(t, err)
		assert.Equal(t, []byte("value2"), result)
	})

	t.Run("overwrite key in transaction", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)
		db, err := OpenDB(cfg)
		require.NoError(t, err)
		defer closeDB(t, db)

		// Add a key directly
		err = db.Put([]byte("key1"), []byte("value1"))
		require.NoError(t, err)

		// Overwrite in transaction
		txnConfig := &config.TransactionConfig{
			MaxBatchSize: 100,
			SyncOnCommit: false,
		}
		txn := db.NewMutexTransaction(txnConfig)

		err = txn.Put([]byte("key1"), []byte("value2"))
		require.NoError(t, err)

		err = txn.Commit()
		assert.NoError(t, err)

		// Verify the value was updated
		result, err := db.Get([]byte("key1"))
		require.NoError(t, err)
		assert.Equal(t, []byte("value2"), result)
	})
}

// TestMutexTransaction_Integration tests integration scenarios
func TestMutexTransaction_Integration(t *testing.T) {
	t.Run("multiple transactions sequentially", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)
		db, err := OpenDB(cfg)
		require.NoError(t, err)
		defer closeDB(t, db)

		txnConfig := &config.TransactionConfig{
			MaxBatchSize: 100,
			SyncOnCommit: false,
		}

		// First transaction
		txn1 := db.NewMutexTransaction(txnConfig)
		err = txn1.Put([]byte("key1"), []byte("value1"))
		require.NoError(t, err)
		err = txn1.Put([]byte("key2"), []byte("value2"))
		require.NoError(t, err)
		err = txn1.Commit()
		require.NoError(t, err)

		// Second transaction
		txn2 := db.NewMutexTransaction(txnConfig)
		err = txn2.Put([]byte("key3"), []byte("value3"))
		require.NoError(t, err)
		err = txn2.Delete([]byte("key1"))
		require.NoError(t, err)
		err = txn2.Commit()
		require.NoError(t, err)

		// Verify final state
		_, err = db.Get([]byte("key1"))
		assert.ErrorIs(t, err, kv_store.ErrKeyNotFound)

		result, err := db.Get([]byte("key2"))
		require.NoError(t, err)
		assert.Equal(t, []byte("value2"), result)

		result, err = db.Get([]byte("key3"))
		require.NoError(t, err)
		assert.Equal(t, []byte("value3"), result)
	})

	t.Run("transaction persistence after close and reopen", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)

		// First session: write data in transaction
		db1, err := OpenDB(cfg)
		require.NoError(t, err)

		txnConfig := &config.TransactionConfig{
			MaxBatchSize: 100,
			SyncOnCommit: true, // Ensure sync for persistence
		}
		txn := db1.NewMutexTransaction(txnConfig)

		err = txn.Put([]byte("key1"), []byte("value1"))
		require.NoError(t, err)
		err = txn.Put([]byte("key2"), []byte("value2"))
		require.NoError(t, err)
		err = txn.Commit()
		require.NoError(t, err)

		err = db1.Close()
		require.NoError(t, err)

		// Second session: verify data persisted
		db2, err := OpenDB(cfg)
		require.NoError(t, err)
		defer closeDB(t, db2)

		result, err := db2.Get([]byte("key1"))
		require.NoError(t, err)
		assert.Equal(t, []byte("value1"), result)

		result, err = db2.Get([]byte("key2"))
		require.NoError(t, err)
		assert.Equal(t, []byte("value2"), result)
	})

	t.Run("large transaction within limits", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)
		db, err := OpenDB(cfg)
		require.NoError(t, err)
		defer closeDB(t, db)

		txnConfig := &config.TransactionConfig{
			MaxBatchSize: 1000,
			SyncOnCommit: false,
		}
		txn := db.NewMutexTransaction(txnConfig)

		// Add 500 keys
		for i := 0; i < 500; i++ {
			key := []byte("key_" + string(rune(i)))
			value := []byte("value_" + string(rune(i)))
			err = txn.Put(key, value)
			require.NoError(t, err)
		}

		err = txn.Commit()
		assert.NoError(t, err)

		// Verify some random keys
		result, err := db.Get([]byte("key_" + string(rune(0))))
		require.NoError(t, err)
		assert.Equal(t, []byte("value_"+string(rune(0))), result)

		result, err = db.Get([]byte("key_" + string(rune(499))))
		require.NoError(t, err)
		assert.Equal(t, []byte("value_"+string(rune(499))), result)
	})
}

// Helper function to close database with error logging
func closeDB(t *testing.T, db *DB) {
	err := db.Close()
	if err != nil {
		t.Logf("Error closing database: %v", err)
	}
}
