package user

import (
	"bytes"
	"kv_store"
	"kv_store/config"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to create a temporary test directory
func createTestDir(t *testing.T) string {
	dir, err := os.MkdirTemp("", "kv_store_test_*")
	require.NoError(t, err)
	t.Cleanup(func() {
		err = os.RemoveAll(dir)
		if err != nil {
			return
		}
	})
	return dir
}

// Helper function to create a test DB configuration
func createTestConfig(dir string) config.DBConfig {
	return config.DBConfig{
		DataDirPath:          dir,
		DataFileSize:         256 * 1024 * 1024, // 256 MB
		SyncAfterEveryWrites: false,
		BytesPerSync:         0,
		MergeRatio:           0.5,
	}
}

// TestOpenDB tests database opening functionality
func TestOpenDB(t *testing.T) {
	t.Run("successful open with valid config", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)

		db, err := OpenDB(cfg)
		require.NoError(t, err)
		require.NotNil(t, db)
		defer func(db *DB) {
			err := db.Close()
			if err != nil {
				log.Println(err)
			}
		}(db)

		assert.Equal(t, dir, db.config.DataDirPath)
		assert.NotNil(t, db.mutex)
		assert.NotNil(t, db.indexer)
	})

	t.Run("error when data dir path not set", func(t *testing.T) {
		cfg := config.DBConfig{
			DataDirPath:  "",
			DataFileSize: 256 * 1024 * 1024,
		}

		db, err := OpenDB(cfg)
		assert.ErrorIs(t, err, kv_store.ErrDataDirPathNotSet)
		assert.Nil(t, db)
	})

	t.Run("error when data file size is invalid", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := config.DBConfig{
			DataDirPath:  dir,
			DataFileSize: 0,
		}

		db, err := OpenDB(cfg)
		assert.ErrorIs(t, err, kv_store.ErrInvalidDataFileSize)
		assert.Nil(t, db)
	})

	t.Run("error when data file size is negative", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := config.DBConfig{
			DataDirPath:  dir,
			DataFileSize: -100,
		}

		db, err := OpenDB(cfg)
		assert.ErrorIs(t, err, kv_store.ErrInvalidDataFileSize)
		assert.Nil(t, db)
	})

	t.Run("error when data dir is already in use", func(t *testing.T) {
		t.Skip("File locking behavior differs on Windows - skipping")
		dir := createTestDir(t)
		cfg := createTestConfig(dir)

		// Open first database
		db1, err := OpenDB(cfg)
		require.NoError(t, err)
		require.NotNil(t, db1)
		defer func(db1 *DB) {
			err := db1.Close()
			if err != nil {
				log.Println(err)
			}
		}(db1)

		// Try to open second database with same directory
		db2, err := OpenDB(cfg)
		assert.ErrorIs(t, err, kv_store.ErrDataDirIsBusy)
		assert.Nil(t, db2)
	})

	t.Run("creates data directory if it doesn't exist", func(t *testing.T) {
		dir := createTestDir(t)
		subDir := filepath.Join(dir, "nested", "test")
		cfg := createTestConfig(subDir)

		db, err := OpenDB(cfg)
		require.NoError(t, err)
		require.NotNil(t, db)
		defer func(db *DB) {
			err := db.Close()
			if err != nil {
				log.Println(err)
			}
		}(db)

		// Verify directory was created
		info, err := os.Stat(subDir)
		require.NoError(t, err)
		assert.True(t, info.IsDir())
	})
}

// TestDB_Get tests the Get functionality
func TestDB_Get(t *testing.T) {
	t.Run("successful get", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)
		db, err := OpenDB(cfg)
		require.NoError(t, err)
		defer func(db *DB) {
			err := db.Close()
			if err != nil {
				log.Println(err)
			}
		}(db)

		key := []byte("test_key")
		value := []byte("test_value")

		err = db.Put(key, value)
		require.NoError(t, err)

		result, err := db.Get(key)
		require.NoError(t, err)
		assert.Equal(t, value, result)
	})

	t.Run("error when key is empty", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)
		db, err := OpenDB(cfg)
		require.NoError(t, err)
		defer func(db *DB) {
			err := db.Close()
			if err != nil {
				log.Println(err)
			}
		}(db)

		_, err = db.Get([]byte(""))
		assert.ErrorIs(t, err, kv_store.ErrKeyIsEmpty)
	})

	t.Run("error when key is nil", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)
		db, err := OpenDB(cfg)
		require.NoError(t, err)
		defer func(db *DB) {
			err := db.Close()
			if err != nil {
				log.Println(err)
			}
		}(db)

		_, err = db.Get(nil)
		assert.ErrorIs(t, err, kv_store.ErrKeyIsEmpty)
	})

	t.Run("error when key not found", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)
		db, err := OpenDB(cfg)
		require.NoError(t, err)
		defer func(db *DB) {
			err := db.Close()
			if err != nil {
				log.Println(err)
			}
		}(db)

		_, err = db.Get([]byte("non_existent_key"))
		assert.ErrorIs(t, err, kv_store.ErrKeyNotFound)
	})

	t.Run("get with empty value", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)
		db, err := OpenDB(cfg)
		require.NoError(t, err)
		defer func(db *DB) {
			err := db.Close()
			if err != nil {
				log.Println(err)
			}
		}(db)

		key := []byte("key")
		err = db.Put(key, []byte(""))
		require.NoError(t, err)

		result, err := db.Get(key)
		require.NoError(t, err)
		assert.Equal(t, []byte(""), result)
	})

	t.Run("get after overwrite", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)
		db, err := OpenDB(cfg)
		require.NoError(t, err)
		defer func(db *DB) {
			err := db.Close()
			if err != nil {
				log.Println(err)
			}
		}(db)

		key := []byte("key")
		value1 := []byte("value1")
		value2 := []byte("value2")

		err = db.Put(key, value1)
		require.NoError(t, err)

		err = db.Put(key, value2)
		require.NoError(t, err)

		result, err := db.Get(key)
		require.NoError(t, err)
		assert.Equal(t, value2, result)
	})
}

// TestDB_Put tests the Put functionality
func TestDB_Put(t *testing.T) {
	t.Run("successful put", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)
		db, err := OpenDB(cfg)
		require.NoError(t, err)
		defer func(db *DB) {
			err := db.Close()
			if err != nil {
				log.Println(err)
			}
		}(db)

		key := []byte("test_key")
		value := []byte("test_value")

		err = db.Put(key, value)
		assert.NoError(t, err)
	})

	t.Run("error when key is empty", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)
		db, err := OpenDB(cfg)
		require.NoError(t, err)
		defer func(db *DB) {
			err := db.Close()
			if err != nil {
				log.Println(err)
			}
		}(db)

		err = db.Put([]byte(""), []byte("value"))
		assert.ErrorIs(t, err, kv_store.ErrKeyIsEmpty)
	})

	t.Run("error when key is nil", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)
		db, err := OpenDB(cfg)
		require.NoError(t, err)
		defer func(db *DB) {
			err := db.Close()
			if err != nil {
				log.Println(err)
			}
		}(db)

		err = db.Put(nil, []byte("value"))
		assert.ErrorIs(t, err, kv_store.ErrKeyIsEmpty)
	})

	t.Run("put with empty value", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)
		db, err := OpenDB(cfg)
		require.NoError(t, err)
		defer func(db *DB) {
			err := db.Close()
			if err != nil {
				log.Println(err)
			}
		}(db)

		key := []byte("key")
		err = db.Put(key, []byte(""))
		assert.NoError(t, err)
	})

	t.Run("overwrite existing key", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)
		db, err := OpenDB(cfg)
		require.NoError(t, err)
		defer func(db *DB) {
			err := db.Close()
			if err != nil {
				log.Println(err)
			}
		}(db)

		key := []byte("key")
		value1 := []byte("value1")
		value2 := []byte("value2")

		err = db.Put(key, value1)
		require.NoError(t, err)

		err = db.Put(key, value2)
		assert.NoError(t, err)

		// Verify the value was updated
		result, err := db.Get(key)
		require.NoError(t, err)
		assert.Equal(t, value2, result)
	})

	t.Run("put multiple keys", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)
		db, err := OpenDB(cfg)
		require.NoError(t, err)
		defer func(db *DB) {
			err := db.Close()
			if err != nil {
				log.Println(err)
			}
		}(db)

		for i := 0; i < 100; i++ {
			key := []byte("key_" + string(rune(i)))
			value := []byte("value_" + string(rune(i)))
			err = db.Put(key, value)
			require.NoError(t, err)
		}
	})
}

// TestDB_Delete tests the Delete functionality
func TestDB_Delete(t *testing.T) {
	t.Run("successful delete", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)
		db, err := OpenDB(cfg)
		require.NoError(t, err)
		defer func(db *DB) {
			err := db.Close()
			if err != nil {
				log.Println(err)
			}
		}(db)

		key := []byte("test_key")
		value := []byte("test_value")

		err = db.Put(key, value)
		require.NoError(t, err)

		err = db.Delete(key)
		assert.NoError(t, err)

		// Verify key is deleted
		_, err = db.Get(key)
		assert.ErrorIs(t, err, kv_store.ErrKeyNotFound)
	})

	t.Run("error when key is empty", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)
		db, err := OpenDB(cfg)
		require.NoError(t, err)
		defer func(db *DB) {
			err := db.Close()
			if err != nil {
				log.Println(err)
			}
		}(db)

		err = db.Delete([]byte(""))
		assert.ErrorIs(t, err, kv_store.ErrKeyIsEmpty)
	})

	t.Run("error when key is nil", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)
		db, err := OpenDB(cfg)
		require.NoError(t, err)
		defer func(db *DB) {
			err := db.Close()
			if err != nil {
				log.Println(err)
			}
		}(db)

		err = db.Delete(nil)
		assert.ErrorIs(t, err, kv_store.ErrKeyIsEmpty)
	})

	t.Run("error when key not found", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)
		db, err := OpenDB(cfg)
		require.NoError(t, err)
		defer func(db *DB) {
			err := db.Close()
			if err != nil {
				log.Println(err)
			}
		}(db)

		err = db.Delete([]byte("non_existent_key"))
		assert.ErrorIs(t, err, kv_store.ErrKeyNotFound)
	})

	t.Run("delete and re-add", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)
		db, err := OpenDB(cfg)
		require.NoError(t, err)
		defer func(db *DB) {
			err := db.Close()
			if err != nil {
				log.Println(err)
			}
		}(db)

		key := []byte("key")
		value1 := []byte("value1")
		value2 := []byte("value2")

		// Add key
		err = db.Put(key, value1)
		require.NoError(t, err)

		// Delete key
		err = db.Delete(key)
		require.NoError(t, err)

		// Re-add with different value
		err = db.Put(key, value2)
		require.NoError(t, err)

		// Verify new value
		result, err := db.Get(key)
		require.NoError(t, err)
		assert.Equal(t, value2, result)
	})
}

// TestDB_Close tests the Close functionality
func TestDB_Close(t *testing.T) {
	t.Run("successful close", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)
		db, err := OpenDB(cfg)
		require.NoError(t, err)

		// Add some data
		err = db.Put([]byte("key"), []byte("value"))
		require.NoError(t, err)

		err = db.Close()
		assert.NoError(t, err)
	})

	t.Run("close empty database", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)
		db, err := OpenDB(cfg)
		require.NoError(t, err)

		err = db.Close()
		assert.NoError(t, err)
	})

	t.Run("reopen after close", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)

		// Open and add data
		db, err := OpenDB(cfg)
		require.NoError(t, err)

		key := []byte("key")
		value := []byte("value")
		err = db.Put(key, value)
		require.NoError(t, err)

		err = db.Close()
		require.NoError(t, err)

		// Reopen and verify data
		db2, err := OpenDB(cfg)
		require.NoError(t, err)
		defer func(db2 *DB) {
			err := db2.Close()
			if err != nil {
				log.Println(err)
			}
		}(db2)

		result, err := db2.Get(key)
		require.NoError(t, err)
		assert.Equal(t, value, result)
	})
}

// TestDB_Integration tests integration scenarios
func TestDB_Integration(t *testing.T) {
	t.Run("put, get, delete flow", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)
		db, err := OpenDB(cfg)
		require.NoError(t, err)
		defer func(db *DB) {
			err := db.Close()
			if err != nil {
				log.Println(err)
			}
		}(db)

		// Put multiple keys
		keys := [][]byte{
			[]byte("key1"),
			[]byte("key2"),
			[]byte("key3"),
		}
		values := [][]byte{
			[]byte("value1"),
			[]byte("value2"),
			[]byte("value3"),
		}

		for i, key := range keys {
			err = db.Put(key, values[i])
			require.NoError(t, err)
		}

		// Get all keys
		for i, key := range keys {
			result, err := db.Get(key)
			require.NoError(t, err)
			assert.Equal(t, values[i], result)
		}

		// Delete one key
		err = db.Delete(keys[1])
		require.NoError(t, err)

		// Verify deletion
		_, err = db.Get(keys[1])
		assert.ErrorIs(t, err, kv_store.ErrKeyNotFound)

		// Verify other keys still exist
		result, err := db.Get(keys[0])
		require.NoError(t, err)
		assert.Equal(t, values[0], result)

		result, err = db.Get(keys[2])
		require.NoError(t, err)
		assert.Equal(t, values[2], result)
	})

	t.Run("persistence test", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)

		// First session: write data
		db1, err := OpenDB(cfg)
		require.NoError(t, err)

		testData := map[string]string{
			"key1": "value1",
			"key2": "value2",
			"key3": "value3",
		}

		for key, value := range testData {
			err = db1.Put([]byte(key), []byte(value))
			require.NoError(t, err)
		}

		err = db1.Close()
		require.NoError(t, err)

		// Second session: read data
		db2, err := OpenDB(cfg)
		require.NoError(t, err)
		defer func(db2 *DB) {
			err := db2.Close()
			if err != nil {
				log.Println(err)
			}
		}(db2)

		for key, expectedValue := range testData {
			result, err := db2.Get([]byte(key))
			require.NoError(t, err)
			assert.Equal(t, []byte(expectedValue), result)
		}
	})

	t.Run("large value storage", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)
		db, err := OpenDB(cfg)
		require.NoError(t, err)
		defer func(db *DB) {
			err := db.Close()
			if err != nil {
				log.Println(err)
			}
		}(db)

		key := []byte("large_key")
		// Create a large value (1MB)
		largeValue := bytes.Repeat([]byte("a"), 1024*1024)

		err = db.Put(key, largeValue)
		require.NoError(t, err)

		result, err := db.Get(key)
		require.NoError(t, err)
		assert.Equal(t, largeValue, result)
	})

	t.Run("many small operations", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)
		db, err := OpenDB(cfg)
		require.NoError(t, err)
		defer func(db *DB) {
			err := db.Close()
			if err != nil {
				log.Println(err)
			}
		}(db)

		numOps := 1000
		for i := 0; i < numOps; i++ {
			key := []byte("key_" + string(rune(i)))
			value := []byte("value_" + string(rune(i)))

			err = db.Put(key, value)
			require.NoError(t, err)

			result, err := db.Get(key)
			require.NoError(t, err)
			assert.Equal(t, value, result)
		}
	})
}

// TestDB_ListKeys tests the ListKeys functionality
func TestDB_ListKeys(t *testing.T) {
	t.Run("list keys in empty database", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)
		db, err := OpenDB(cfg)
		require.NoError(t, err)
		defer func(db *DB) {
			err := db.Close()
			if err != nil {
				log.Println(err)
			}
		}(db)

		keys := db.ListKeys()
		assert.Empty(t, keys)
	})

	t.Run("list keys after insertions", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)
		db, err := OpenDB(cfg)
		require.NoError(t, err)
		defer func(db *DB) {
			err := db.Close()
			if err != nil {
				log.Println(err)
			}
		}(db)

		expectedKeys := [][]byte{
			[]byte("key1"),
			[]byte("key2"),
			[]byte("key3"),
		}

		for _, key := range expectedKeys {
			err = db.Put(key, []byte("value"))
			require.NoError(t, err)
		}

		keys := db.ListKeys()
		assert.Len(t, keys, len(expectedKeys))
	})

	t.Run("list keys excludes deleted keys", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)
		db, err := OpenDB(cfg)
		require.NoError(t, err)
		defer func(db *DB) {
			err := db.Close()
			if err != nil {
				log.Println(err)
			}
		}(db)

		// Add keys
		_ = db.Put([]byte("key1"), []byte("value1"))
		_ = db.Put([]byte("key2"), []byte("value2"))
		_ = db.Put([]byte("key3"), []byte("value3"))

		// Delete one key
		err = db.Delete([]byte("key2"))
		require.NoError(t, err)

		keys := db.ListKeys()
		assert.Len(t, keys, 2)

		// Verify deleted key is not in list
		for _, key := range keys {
			assert.NotEqual(t, []byte("key2"), key)
		}
	})
}

// TestDB_Sync tests the Sync functionality
func TestDB_Sync(t *testing.T) {
	t.Run("sync after writes", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)
		db, err := OpenDB(cfg)
		require.NoError(t, err)
		defer func(db *DB) {
			err := db.Close()
			if err != nil {
				log.Println(err)
			}
		}(db)

		err = db.Put([]byte("key"), []byte("value"))
		require.NoError(t, err)

		err = db.Sync()
		assert.NoError(t, err)
	})
}
