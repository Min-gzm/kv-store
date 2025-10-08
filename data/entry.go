package data

import (
	"encoding/binary"
	"encoding/json"
)

type EntryType byte

const (
	EntryNormal EntryType = iota
	EntryDeleted
)

type Entry struct {
	Key   []byte
	Value []byte
	Type  EntryType
}

type EntryHeader struct {
	CRC       uint32    // 4byte
	Type      EntryType // 1byte
	KeySize   uint32    // max 5byte
	ValueSize uint32    // max 5byte
}

// MaxEntryHeaderSize crc: 4byte
// type: 1byte
// keySize: max 5byte
// valueSize: max 5byte
const MaxEntryHeaderSize = 4 + 1 + binary.MaxVarintLen32*2

func EncodeEntry(entry *Entry) ([]byte, int) {
	result, err := json.Marshal(entry)
	if err != nil {
		return nil, 0
	}
	return result, len(result)
}

func DecodeEntry(data []byte) (*Entry, error) {
	entry := new(Entry)
	err := json.Unmarshal(data, entry)
	if err != nil {
		return nil, err
	}
	return entry, nil
}

func DecodeEntryHeader(data []byte) (*EntryHeader, int) {
	return nil, 0
}

func verifyHeaderCRC(header *EntryHeader, headerBuffer []byte) bool {

	return true
}
