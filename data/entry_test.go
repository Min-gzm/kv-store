package data

import (
	"github.com/stretchr/testify/assert"
	"kv_store/transaction"
	"testing"
)

func TestEntry(t *testing.T) {
	t.Run("Test the EncodeEntry func", func(t *testing.T) {
		entry := &Entry{
			Key:   []byte("key1"),
			Value: []byte("value1"),
			Type:  EntryNormal,
			Seq:   transaction.NonTxnSeq,
		}

		encoded, size := EncodeEntry(entry)
		assert.NotEqual(t, 0, size)
		assert.NotNil(t, encoded)

	})

	t.Run("Test the DecodeEntryHeader func", func(t *testing.T) {
		entry := &Entry{
			Key:   []byte("key1"),
			Value: []byte("value1"),
			Type:  EntryNormal,
			Seq:   transaction.NonTxnSeq,
		}
		encoded, size := EncodeEntry(entry)
		assert.NotEqual(t, 0, size)
		assert.NotNil(t, encoded)

		header, size := DecodeEntryHeader(encoded)
		assert.NotNil(t, header)
		assert.True(t, size > 0)
		assert.Equal(t, header.KeySize, uint32(len(entry.Key)))
		assert.Equal(t, header.ValueSize, uint32(len(entry.Value)))
	})

	t.Run("Test the DecodeEntry func", func(t *testing.T) {
		entry := &Entry{
			Key:   []byte("key1"),
			Value: []byte("value1"),
			Type:  EntryNormal,
			Seq:   transaction.NonTxnSeq,
		}
		encoded, _ := EncodeEntry(entry)

		decoded, err := DecodeEntry(encoded)
		assert.Nil(t, err)
		assert.Equal(t, entry, decoded)
	})
}
