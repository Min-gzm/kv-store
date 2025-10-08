package data

import "encoding/json"

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
