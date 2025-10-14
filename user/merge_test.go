package user

import (
	"bytes"
	"kv_store"
	"log"
	"os"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// cleanupMergeDir removes any existing merge directory to prevent test interference
func cleanupMergeDir(t *testing.T, db *DB) {
	mergeDir := db.getMergeDir()
	if _, err := os.Stat(mergeDir); err == nil {
		err = os.RemoveAll(mergeDir)
		if err != nil {
			t.Logf("Warning: failed to cleanup merge dir: %v", err)
		}
	}
}

// TestDB_Merge tests the Merge functionality
func TestDB_Merge(t *testing.T) {
	t.Run("error when database is empty", func(t *testing.T) {
		dir := createTestDir(t)
		log.Printf("Testing dir: %s", dir)
		cfg := createTestConfig(dir)
		db, err := OpenDB(cfg)
		log.Printf("Merged dir: %s", db.getMergeDir())
		require.NoError(t, err)
		defer closeDB(t, db)

		err = db.Merge()
		assert.ErrorIs(t, err, kv_store.ErrDatabaseIsEmpty)
	})

	t.Run("error when merge is already in progress", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)
		db, err := OpenDB(cfg)
		require.NoError(t, err)
		defer closeDB(t, db)

		// Add some data
		err = db.Put([]byte("key1"), []byte("value1"))
		require.NoError(t, err)

		// Manually set isMerging flag
		db.mutex.Lock()
		db.isMerging = true
		db.mutex.Unlock()

		err = db.Merge()
		assert.ErrorIs(t, err, kv_store.ErrMergeInProgress)

		// Clean up
		db.mutex.Lock()
		db.isMerging = false
		db.mutex.Unlock()
	})

	t.Run("merge with no reclaim space skips merge", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)
		cfg.MergeRatio = 0.5
		db, err := OpenDB(cfg)
		require.NoError(t, err)
		defer closeDB(t, db)

		// Add some data without any deletions or updates
		// This means reclaimSize will be 0, and merge should skip
		err = db.Put([]byte("key1"), []byte("value1"))
		require.NoError(t, err)
		err = db.Put([]byte("key2"), []byte("value2"))
		require.NoError(t, err)

		// Verify reclaimSize is 0
		assert.Equal(t, int64(0), db.reclaimSize)

		err = db.Merge()
		assert.NoError(t, err)
	})

	t.Run("merge skips when ratio not met", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)
		cfg.DataFileSize = 10240 // Large file size to ensure we stay in one file
		cfg.MergeRatio = 0.9     // Very high ratio - won't be met
		db, err := OpenDB(cfg)
		require.NoError(t, err)
		defer closeDB(t, db)

		// Add keys
		for i := 0; i < 10; i++ {
			key := []byte("key_" + strconv.Itoa(i))
			value := []byte("value_" + strconv.Itoa(i))
			err = db.Put(key, value)
			require.NoError(t, err)
		}

		// Update one key to create some reclaim space
		err = db.Put([]byte("key_0"), []byte("new_value"))
		require.NoError(t, err)

		// Verify we have some reclaim space
		assert.Greater(t, db.reclaimSize, int64(0))

		// Merge should skip because ratio is too high
		err = db.Merge()
		assert.NoError(t, err)
	})
}

// TestDB_StartEndMerge tests merge state management
func TestDB_StartEndMerge(t *testing.T) {
	t.Run("start merge sets flag", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)
		db, err := OpenDB(cfg)
		require.NoError(t, err)
		defer closeDB(t, db)

		// Add data first
		err = db.Put([]byte("key1"), []byte("value1"))
		require.NoError(t, err)

		err = db.startMerge()
		require.NoError(t, err)

		assert.True(t, db.isMerging)

		db.endMerge()
	})

	t.Run("end merge clears flag", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)
		db, err := OpenDB(cfg)
		require.NoError(t, err)
		defer closeDB(t, db)

		// Add data first
		err = db.Put([]byte("key1"), []byte("value1"))
		require.NoError(t, err)

		err = db.startMerge()
		require.NoError(t, err)
		db.endMerge()

		assert.False(t, db.isMerging)
	})

	t.Run("cannot start merge twice", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)
		db, err := OpenDB(cfg)
		require.NoError(t, err)
		defer closeDB(t, db)

		// Add data first
		err = db.Put([]byte("key1"), []byte("value1"))
		require.NoError(t, err)

		err = db.startMerge()
		require.NoError(t, err)

		err = db.startMerge()
		assert.ErrorIs(t, err, kv_store.ErrMergeInProgress)

		db.endMerge()
	})
}

// TestDB_GetMergeDir tests merge directory path generation
func TestDB_GetMergeDir(t *testing.T) {
	t.Run("merge directory path is not empty", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)
		db, err := OpenDB(cfg)
		require.NoError(t, err)
		defer closeDB(t, db)

		mergeDir := db.getMergeDir()
		assert.NotEmpty(t, mergeDir)
		assert.Contains(t, mergeDir, "merge")
	})
}

// TestDB_GetSortedOldFiles tests old files sorting
func TestDB_GetSortedOldFiles(t *testing.T) {
	t.Run("sorted old files with multiple files", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)
		cfg.DataFileSize = 512 // Small size to create multiple files
		db, err := OpenDB(cfg)
		require.NoError(t, err)
		defer closeDB(t, db)

		// Add enough data to create multiple files
		for i := 0; i < 30; i++ {
			key := []byte("key_" + string(rune(i)))
			value := bytes.Repeat([]byte("v"), 100)
			err = db.Put(key, value)
			require.NoError(t, err)
		}

		oldFiles, activeFileID := db.getSortedOldFiles()

		// Should have some old files
		if len(oldFiles) > 1 {
			// Verify files are sorted by timestamp
			for i := 1; i < len(oldFiles); i++ {
				assert.LessOrEqual(t, oldFiles[i-1].Timestamp, oldFiles[i].Timestamp)
			}
		}

		// Active file ID should not be empty
		assert.NotEmpty(t, activeFileID)
	})

	t.Run("empty old files when single active file", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)
		cfg.DataFileSize = 10240 // Large size to keep everything in one file
		db, err := OpenDB(cfg)
		require.NoError(t, err)
		defer closeDB(t, db)

		// Add small amount of data
		err = db.Put([]byte("key1"), []byte("value1"))
		require.NoError(t, err)

		oldFiles, activeFileID := db.getSortedOldFiles()

		// Should have no old files
		assert.Empty(t, oldFiles)

		// Active file ID should not be empty
		assert.NotEmpty(t, activeFileID)
	})
}

// TestMergeRatioCalculation tests the reclaim size tracking
func TestMergeRatioCalculation(t *testing.T) {
	t.Run("reclaim size increases with updates", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)
		db, err := OpenDB(cfg)
		require.NoError(t, err)
		defer closeDB(t, db)

		// Initial insert - no reclaim
		err = db.Put([]byte("key1"), []byte("value1"))
		require.NoError(t, err)
		assert.Equal(t, int64(0), db.reclaimSize)

		// Update - should increase reclaim
		err = db.Put([]byte("key1"), []byte("value2"))
		require.NoError(t, err)
		assert.Greater(t, db.reclaimSize, int64(0))
	})

	t.Run("reclaim size increases with deletes", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)
		db, err := OpenDB(cfg)
		require.NoError(t, err)
		defer closeDB(t, db)

		// Insert
		err = db.Put([]byte("key1"), []byte("value1"))
		require.NoError(t, err)
		initialReclaim := db.reclaimSize

		// Delete - should increase reclaim
		err = db.Delete([]byte("key1"))
		require.NoError(t, err)
		assert.Greater(t, db.reclaimSize, initialReclaim)
	})

	t.Run("reclaim size tracked across multiple operations", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)
		db, err := OpenDB(cfg)
		require.NoError(t, err)
		defer closeDB(t, db)

		// Add keys
		for i := 0; i < 5; i++ {
			key := []byte("key_" + string(rune(i)))
			value := []byte("value_" + string(rune(i)))
			err = db.Put(key, value)
			require.NoError(t, err)
		}
		assert.Equal(t, int64(0), db.reclaimSize)

		// Update some keys
		for i := 0; i < 3; i++ {
			key := []byte("key_" + string(rune(i)))
			value := []byte("updated_" + string(rune(i)))
			err = db.Put(key, value)
			require.NoError(t, err)
		}

		reclaimAfterUpdates := db.reclaimSize
		assert.Greater(t, reclaimAfterUpdates, int64(0))

		// Delete some keys
		for i := 3; i < 5; i++ {
			key := []byte("key_" + string(rune(i)))
			err = db.Delete(key)
			require.NoError(t, err)
		}

		// Reclaim should be even higher after deletes
		assert.Greater(t, db.reclaimSize, reclaimAfterUpdates)
	})
}

// TestMergeIntegration tests merge integration scenarios
func TestMergeIntegration(t *testing.T) {
	t.Run("full merge process with data validation", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)
		cfg.DataFileSize = 1024 // Small size to force multiple files
		cfg.MergeRatio = 0.0    // Always merge when there's reclaim space
		db, err := OpenDB(cfg)
		require.NoError(t, err)
		cleanupMergeDir(t, db) // Clean up any stale merge directories

		// Create test data
		testData := make(map[string]string)
		for i := 0; i < 50; i++ {
			key := "key_" + string(rune(i))
			value := "value_" + string(rune(i))
			testData[key] = value
			err = db.Put([]byte(key), bytes.Repeat([]byte(value), 10))
			require.NoError(t, err)
		}

		// Update half of the keys
		for i := 0; i < 25; i++ {
			key := "key_" + string(rune(i))
			value := "updated_" + string(rune(i))
			testData[key] = value
			err = db.Put([]byte(key), bytes.Repeat([]byte(value), 10))
			require.NoError(t, err)
		}

		// Delete some keys
		for i := 25; i < 35; i++ {
			key := "key_" + string(rune(i))
			delete(testData, key)
			err = db.Delete([]byte(key))
			require.NoError(t, err)
		}

		// Verify we have multiple old files and reclaim space
		oldFilesBefore := len(db.oldFiles)
		reclaimBefore := db.reclaimSize
		assert.Greater(t, oldFilesBefore, 0, "should have old files")
		assert.Greater(t, reclaimBefore, int64(0), "should have reclaim space")

		// Perform merge
		err = db.Merge()
		require.NoError(t, err)

		// Verify all expected data is still accessible
		for key, expectedValue := range testData {
			value, err := db.Get([]byte(key))
			require.NoError(t, err, "failed to get key: %s", key)
			assert.Equal(t, bytes.Repeat([]byte(expectedValue), 10), value, "value mismatch for key: %s", key)
		}

		// Verify deleted keys are gone
		for i := 25; i < 35; i++ {
			key := "key_" + string(rune(i))
			_, err = db.Get([]byte(key))
			assert.ErrorIs(t, err, kv_store.ErrKeyNotFound, "deleted key should not exist: %s", key)
		}

		// Verify active file still works
		err = db.Put([]byte("new_key"), []byte("new_value"))
		require.NoError(t, err)
		value, err := db.Get([]byte("new_key"))
		require.NoError(t, err)
		assert.Equal(t, []byte("new_value"), value)

		closeDB(t, db)
	})

	t.Run("merge and reload database with data validation", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)
		cfg.DataFileSize = 1024
		cfg.MergeRatio = 0.0

		// First session: create, update, delete, and merge
		db1, err := OpenDB(cfg)
		require.NoError(t, err)
		cleanupMergeDir(t, db1) // Clean up any stale merge directories

		testData := make(map[string]string)
		// Add initial data
		for i := 0; i < 40; i++ {
			key := "key_" + strconv.Itoa(i)
			value := "value_" + strconv.Itoa(i)
			testData[key] = value
			err = db1.Put([]byte(key), bytes.Repeat([]byte(value), 15))
			require.NoError(t, err)
		}

		// Update some keys
		for i := 0; i < 15; i++ {
			key := "key_" + strconv.Itoa(i)
			value := "updated_" + strconv.Itoa(i)
			testData[key] = value
			err = db1.Put([]byte(key), bytes.Repeat([]byte(value), 15))
			require.NoError(t, err)
		}

		// Delete some keys
		for i := 30; i < 40; i++ {
			key := "key_" + strconv.Itoa(i)
			delete(testData, key)
			err = db1.Delete([]byte(key))
			require.NoError(t, err)
		}

		// Perform merge
		err = db1.Merge()
		require.NoError(t, err)

		// Close database
		err = db1.Close()
		require.NoError(t, err)

		// Second session: reopen and verify
		db2, err := OpenDB(cfg)
		require.NoError(t, err)
		defer closeDB(t, db2)

		// Verify all data is correct
		for key, expectedValue := range testData {
			value, err := db2.Get([]byte(key))
			require.NoError(t, err, "failed to get key after reload: %s", key)
			assert.Equal(t, expectedValue, string(value), "value mismatch after reload for key: %s", key)
		}

		// Verify deleted keys are still deleted
		for i := 30; i < 40; i++ {
			key := "key_" + string(rune(i))
			_, err = db2.Get([]byte(key))
			assert.ErrorIs(t, err, kv_store.ErrKeyNotFound, "deleted key should not exist after reload: %s", key)
		}

		// Verify we can still write and read
		err = db2.Put([]byte("after_reload"), []byte("test_value"))
		require.NoError(t, err)
		value, err := db2.Get([]byte("after_reload"))
		require.NoError(t, err)
		assert.Equal(t, []byte("test_value"), value)
	})

	t.Run("multiple merges with continuous operations", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)
		cfg.DataFileSize = 2048
		cfg.MergeRatio = 0.0
		db, err := OpenDB(cfg)
		require.NoError(t, err)
		defer closeDB(t, db)
		cleanupMergeDir(t, db) // Clean up any stale merge directories

		testData := make(map[string]string)

		// Round 1: Initial data
		for i := 0; i < 20; i++ {
			key := "key_" + string(rune(i))
			value := "value1_" + string(rune(i))
			testData[key] = value
			err = db.Put([]byte(key), bytes.Repeat([]byte(value), 20))
			require.NoError(t, err)
		}

		// First merge
		err = db.Merge()
		require.NoError(t, err)

		// Verify data after first merge
		for key, expectedValue := range testData {
			value, err := db.Get([]byte(key))
			require.NoError(t, err)
			assert.Equal(t, bytes.Repeat([]byte(expectedValue), 20), value)
		}

		// Round 2: Update and add more data
		for i := 0; i < 10; i++ {
			key := "key_" + string(rune(i))
			value := "value2_" + string(rune(i))
			testData[key] = value
			err = db.Put([]byte(key), bytes.Repeat([]byte(value), 20))
			require.NoError(t, err)
		}
		for i := 20; i < 30; i++ {
			key := "key_" + string(rune(i))
			value := "value2_" + string(rune(i))
			testData[key] = value
			err = db.Put([]byte(key), bytes.Repeat([]byte(value), 20))
			require.NoError(t, err)
		}

		// Second merge
		err = db.Merge()
		require.NoError(t, err)

		// Verify all data is correct
		for key, expectedValue := range testData {
			value, err := db.Get([]byte(key))
			require.NoError(t, err)
			assert.Equal(t, bytes.Repeat([]byte(expectedValue), 20), value)
		}

		// Round 3: Delete some keys
		for i := 15; i < 25; i++ {
			key := "key_" + string(rune(i))
			delete(testData, key)
			err = db.Delete([]byte(key))
			require.NoError(t, err)
		}

		// Third merge
		err = db.Merge()
		require.NoError(t, err)

		// Final verification
		for key, expectedValue := range testData {
			value, err := db.Get([]byte(key))
			require.NoError(t, err)
			assert.Equal(t, bytes.Repeat([]byte(expectedValue), 20), value)
		}
	})

	t.Run("merge with active file operations during merge", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)
		cfg.DataFileSize = 1024
		cfg.MergeRatio = 0.0
		db, err := OpenDB(cfg)
		require.NoError(t, err)
		defer closeDB(t, db)
		cleanupMergeDir(t, db) // Clean up any stale merge directories

		// Create data in old files
		for i := 0; i < 30; i++ {
			key := []byte("old_key_" + string(rune(i)))
			value := bytes.Repeat([]byte("v"), 50)
			err = db.Put(key, value)
			require.NoError(t, err)
		}

		// Update to create reclaim space
		for i := 0; i < 15; i++ {
			key := []byte("old_key_" + string(rune(i)))
			value := bytes.Repeat([]byte("u"), 50)
			err = db.Put(key, value)
			require.NoError(t, err)
		}

		// Perform merge
		err = db.Merge()
		require.NoError(t, err)

		// Add new data to active file after merge
		for i := 0; i < 10; i++ {
			key := []byte("new_key_" + string(rune(i)))
			value := []byte("new_value_" + string(rune(i)))
			err = db.Put(key, value)
			require.NoError(t, err)
		}

		// Verify old data is accessible
		for i := 0; i < 30; i++ {
			key := []byte("old_key_" + string(rune(i)))
			value, err := db.Get(key)
			require.NoError(t, err)
			if i < 15 {
				assert.Equal(t, bytes.Repeat([]byte("u"), 50), value)
			} else {
				assert.Equal(t, bytes.Repeat([]byte("v"), 50), value)
			}
		}

		// Verify new data is accessible
		for i := 0; i < 10; i++ {
			key := []byte("new_key_" + string(rune(i)))
			value, err := db.Get(key)
			require.NoError(t, err)
			assert.Equal(t, []byte("new_value_"+string(rune(i))), value)
		}
	})
}

// TestMergeEndToEnd tests complete end-to-end merge scenarios
func TestMergeEndToEnd(t *testing.T) {
	t.Run("end to end merge with complex data operations", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)
		cfg.DataFileSize = 2048
		cfg.MergeRatio = 0.0

		// Session 1: Create initial database
		db1, err := OpenDB(cfg)
		require.NoError(t, err)
		cleanupMergeDir(t, db1) // Clean up any stale merge directories

		// Add 100 keys with varying sizes
		expectedData := make(map[string][]byte)
		for i := 0; i < 100; i++ {
			key := "key_" + string(rune(i%50)) + "_" + string(rune(i/50))
			value := bytes.Repeat([]byte{byte(i)}, (i%20)+10)
			expectedData[key] = value
			err = db1.Put([]byte(key), value)
			require.NoError(t, err)
		}

		// Update 40 keys
		for i := 0; i < 40; i++ {
			key := "key_" + string(rune(i%50)) + "_" + string(rune(i/50))
			value := bytes.Repeat([]byte{byte(i + 100)}, (i%15)+20)
			expectedData[key] = value
			err = db1.Put([]byte(key), value)
			require.NoError(t, err)
		}

		// Delete 20 keys
		for i := 80; i < 100; i++ {
			key := "key_" + string(rune(i%50)) + "_" + string(rune(i/50))
			delete(expectedData, key)
			err = db1.Delete([]byte(key))
			require.NoError(t, err)
		}

		initialOldFiles := len(db1.oldFiles)
		initialReclaimSize := db1.reclaimSize

		// Perform merge
		err = db1.Merge()
		require.NoError(t, err)

		// Verify data integrity immediately after merge
		for key, expectedValue := range expectedData {
			value, err := db1.Get([]byte(key))
			require.NoError(t, err, "failed to get key after merge: %s", key)
			assert.Equal(t, expectedValue, value, "value mismatch after merge for key: %s", key)
		}

		// Verify active file is working
		assert.NotNil(t, db1.activeFile, "active file should exist")
		activeFileIDBefore := db1.activeFile.ID

		// Add more data to active file
		err = db1.Put([]byte("after_merge_1"), []byte("test1"))
		require.NoError(t, err)
		err = db1.Put([]byte("after_merge_2"), []byte("test2"))
		require.NoError(t, err)

		// Verify active file ID hasn't changed
		assert.Equal(t, activeFileIDBefore, db1.activeFile.ID, "active file should remain the same")

		// Close database
		err = db1.Close()
		require.NoError(t, err)

		// Session 2: Reopen and verify
		db2, err := OpenDB(cfg)
		require.NoError(t, err)
		defer closeDB(t, db2)

		// Verify all original data
		for key, expectedValue := range expectedData {
			value, err := db2.Get([]byte(key))
			require.NoError(t, err, "failed to get key after reopen: %s", key)
			assert.Equal(t, expectedValue, value, "value mismatch after reopen for key: %s", key)
		}

		// Verify data added after merge
		value, err := db2.Get([]byte("after_merge_1"))
		require.NoError(t, err)
		assert.Equal(t, []byte("test1"), value)

		value, err = db2.Get([]byte("after_merge_2"))
		require.NoError(t, err)
		assert.Equal(t, []byte("test2"), value)

		// Verify deleted keys are still deleted
		for i := 80; i < 100; i++ {
			key := "key_" + string(rune(i%50)) + "_" + string(rune(i/50))
			_, err = db2.Get([]byte(key))
			assert.ErrorIs(t, err, kv_store.ErrKeyNotFound, "deleted key should not exist: %s", key)
		}

		// Verify database state
		assert.NotNil(t, db2.activeFile, "active file should exist after reopen")
		t.Logf("Initial old files: %d, initial reclaim: %d", initialOldFiles, initialReclaimSize)
		t.Logf("After reopen old files: %d, reclaim: %d", len(db2.oldFiles), db2.reclaimSize)
	})

	t.Run("merge preserves key ordering and ListKeys functionality", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)
		cfg.DataFileSize = 1024
		cfg.MergeRatio = 0.0
		db, err := OpenDB(cfg)
		require.NoError(t, err)
		defer closeDB(t, db)
		cleanupMergeDir(t, db) // Clean up any stale merge directories

		// Add keys in non-sorted order
		keys := []string{"zebra", "apple", "mango", "banana", "grape"}
		for _, key := range keys {
			err = db.Put([]byte(key), []byte("value_"+key))
			require.NoError(t, err)
		}

		// Update some keys to create reclaim space
		err = db.Put([]byte("apple"), bytes.Repeat([]byte("updated"), 20))
		require.NoError(t, err)
		err = db.Put([]byte("mango"), bytes.Repeat([]byte("updated"), 20))
		require.NoError(t, err)

		// Perform merge
		err = db.Merge()
		require.NoError(t, err)

		// Verify ListKeys returns all keys
		allKeys := db.ListKeys()
		assert.Len(t, allKeys, len(keys), "should have all keys")

		// Verify keys are sorted
		for i := 1; i < len(allKeys); i++ {
			assert.LessOrEqual(t, string(allKeys[i-1]), string(allKeys[i]), "keys should be sorted")
		}

		// Verify all values are accessible
		for _, key := range keys {
			value, err := db.Get([]byte(key))
			require.NoError(t, err)
			if key == "apple" || key == "mango" {
				assert.Equal(t, bytes.Repeat([]byte("updated"), 20), value)
			} else {
				assert.Equal(t, []byte("value_"+key), value)
			}
		}
	})

	t.Run("merge handles large values correctly", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)
		cfg.DataFileSize = 10240 // Larger file size for large values
		cfg.MergeRatio = 0.0
		db, err := OpenDB(cfg)
		require.NoError(t, err)
		defer closeDB(t, db)
		cleanupMergeDir(t, db) // Clean up any stale merge directories

		// Add keys with large values
		largeValue1 := bytes.Repeat([]byte("a"), 5000)
		largeValue2 := bytes.Repeat([]byte("b"), 5000)

		err = db.Put([]byte("large1"), largeValue1)
		require.NoError(t, err)
		err = db.Put([]byte("large2"), largeValue2)
		require.NoError(t, err)
		err = db.Put([]byte("small"), []byte("small_value"))
		require.NoError(t, err)

		// Update one large value to create reclaim space
		largeValue1Updated := bytes.Repeat([]byte("A"), 5000)
		err = db.Put([]byte("large1"), largeValue1Updated)
		require.NoError(t, err)

		// Perform merge
		err = db.Merge()
		require.NoError(t, err)

		// Verify large values
		value, err := db.Get([]byte("large1"))
		require.NoError(t, err)
		assert.Equal(t, largeValue1Updated, value)

		value, err = db.Get([]byte("large2"))
		require.NoError(t, err)
		assert.Equal(t, largeValue2, value)

		value, err = db.Get([]byte("small"))
		require.NoError(t, err)
		assert.Equal(t, []byte("small_value"), value)
	})

	t.Run("active file remains accessible after merge", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)
		cfg.DataFileSize = 1024
		cfg.MergeRatio = 0.0
		db, err := OpenDB(cfg)
		require.NoError(t, err)
		defer closeDB(t, db)
		cleanupMergeDir(t, db) // Clean up any stale merge directories

		// Fill old files
		for i := 0; i < 30; i++ {
			key := []byte("old_" + string(rune(i)))
			value := bytes.Repeat([]byte("v"), 60)
			err = db.Put(key, value)
			require.NoError(t, err)
		}

		// Create reclaim space
		for i := 0; i < 15; i++ {
			key := []byte("old_" + string(rune(i)))
			value := bytes.Repeat([]byte("u"), 60)
			err = db.Put(key, value)
			require.NoError(t, err)
		}

		// Store active file ID before merge
		activeFileIDBeforeMerge := db.activeFile.ID
		oldFilesCountBefore := len(db.oldFiles)

		// Perform merge
		err = db.Merge()
		require.NoError(t, err)

		// Verify active file
		assert.NotNil(t, db.activeFile, "active file should exist after merge")
		activeFileIDAfterMerge := db.activeFile.ID
		t.Logf("Active file before merge: %s", activeFileIDBeforeMerge)
		t.Logf("Active file after merge: %s", activeFileIDAfterMerge)
		t.Logf("Old files before: %d, after: %d", oldFilesCountBefore, len(db.oldFiles))

		// Write to active file
		for i := 0; i < 5; i++ {
			key := []byte("new_" + string(rune(i)))
			value := []byte("new_value_" + string(rune(i)))
			err = db.Put(key, value)
			require.NoError(t, err)
		}

		// Verify all data
		for i := 0; i < 30; i++ {
			key := []byte("old_" + string(rune(i)))
			value, err := db.Get(key)
			require.NoError(t, err)
			if i < 15 {
				assert.Equal(t, bytes.Repeat([]byte("u"), 60), value)
			} else {
				assert.Equal(t, bytes.Repeat([]byte("v"), 60), value)
			}
		}

		for i := 0; i < 5; i++ {
			key := []byte("new_" + string(rune(i)))
			value, err := db.Get(key)
			require.NoError(t, err)
			assert.Equal(t, []byte("new_value_"+string(rune(i))), value)
		}
	})
}
