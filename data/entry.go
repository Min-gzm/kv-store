package data

import (
	"encoding/binary"
	"hash/crc32"
)

type EntryType = byte

const (
	EntryNormal EntryType = iota
	EntryDeleted
	EntryTxFinished
)

type Entry struct {
	Key   []byte
	Value []byte
	Type  EntryType
	Seq   uint64
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
	headerBuffer := make([]byte, MaxEntryHeaderSize)

	// write header type
	index := 4
	headerBuffer[index] = entry.Type

	// write key size and value size
	index++
	index += binary.PutUvarint(headerBuffer[index:], uint64(len(entry.Key)))
	index += binary.PutUvarint(headerBuffer[index:], uint64(len(entry.Value)))

	headerSize := index
	entrySize := headerSize + len(entry.Key) + len(entry.Value)
	entryBuffer := make([]byte, entrySize)

	// copy the header
	copy(entryBuffer[:headerSize], headerBuffer[:headerSize])
	index = headerSize
	// copy the key and value
	copy(entryBuffer[index:], entry.Key)
	index += len(entry.Key)
	copy(entryBuffer[index:], entry.Value)

	// get crc
	crc := crc32.ChecksumIEEE(entryBuffer[4:])
	binary.LittleEndian.PutUint32(entryBuffer[:4], crc)

	return entryBuffer, entrySize
}

func DecodeEntry(buffer []byte) (*Entry, error) {
	if len(buffer) < 4 {
		return nil, nil
	}

	header, index := DecodeEntryHeader(buffer)
	headerSize := index

	// read key and value from buffer
	key := buffer[index : index+int(header.KeySize)]
	index += int(header.KeySize)
	value := buffer[index : index+int(header.ValueSize)]
	entry := &Entry{
		Type:  header.Type,
		Key:   key,
		Value: value,
	}

	// verify the crc
	if !verifyHeaderCRC(entry, buffer[0:headerSize], header.CRC) {
		return nil, ErrInvalidCRC
	}

	return entry, nil
}

func DecodeEntryHeader(buffer []byte) (*EntryHeader, int) {
	if len(buffer) < 4 {
		return nil, 0
	}

	index := 4
	header := &EntryHeader{
		CRC:  binary.LittleEndian.Uint32(buffer[:4]),
		Type: buffer[index],
	}
	index++

	// read key size and value size
	keySize, n := binary.Uvarint(buffer[index:])
	header.KeySize = uint32(keySize)
	index += n
	valueSize, n := binary.Uvarint(buffer[index:])
	header.ValueSize = uint32(valueSize)
	index += n

	return header, index
}

func verifyHeaderCRC(entry *Entry, header []byte, expectedCRC uint32) bool {
	// calculate the crc
	crc := crc32.ChecksumIEEE(header[4:])
	crc = crc32.Update(crc, crc32.IEEETable, entry.Key)
	crc = crc32.Update(crc, crc32.IEEETable, entry.Value)
	if crc != expectedCRC {
		return false
	}

	return true
}
