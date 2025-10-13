package user

import (
	"bytes"
	"kv_store"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDB_Merge tests the Merge functionality
func TestDB_Merge(t *testing.T) {
	t.Run("error when database is empty", func(t *testing.T) {
		dir := createTestDir(t)
		cfg := createTestConfig(dir)
		db, err := OpenDB(cfg)
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
			key := []byte("key_" + string(rune(i)))
			value := []byte("value_" + string(rune(i)))
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
