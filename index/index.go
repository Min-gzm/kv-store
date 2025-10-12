package index

import (
	"encoding/binary"
	"fmt"
	"kv_store"
)

const maxIndexHeaderSize = 30

type Index struct {
	FileID string
	Offset int64
	Size   int
}

type Header struct {
	FileIDSize int64
	Offset     int64
	Size       int
}

// EncodeIndex header(id size + offset + size) + id byte
func EncodeIndex(index *Index) []byte {
	fileIDByte := []byte(index.FileID)
	idLen := len(fileIDByte)

	buffer := make([]byte, maxIndexHeaderSize+idLen)

	i := 0
	i += binary.PutVarint(buffer[i:], int64(idLen))
	i += binary.PutVarint(buffer[i:], index.Offset)
	i += binary.PutVarint(buffer[i:], int64(index.Size))

	copy(buffer[i:], fileIDByte)
	i += idLen

	return buffer[:i]
}

func decodeIndexHeader(b []byte) (*Header, int, error) {
	i := 0

	idSize, n := binary.Varint(b[i:])
	if n <= 0 {
		return nil, 0, fmt.Errorf("invalid fileID size")
	}
	i += n

	offset, n := binary.Varint(b[i:])
	if n <= 0 {
		return nil, 0, fmt.Errorf("invalid offset")
	}
	i += n

	size, n := binary.Varint(b[i:])
	if n <= 0 {
		return nil, 0, fmt.Errorf("invalid size")
	}
	i += n

	return &Header{
		FileIDSize: idSize,
		Offset:     offset,
		Size:       int(size),
	}, i, nil
}

func DecodeIndex(b []byte) (*Index, error) {
	header, n, err := decodeIndexHeader(b)
	if err != nil {
		return nil, err
	}

	if n+int(header.FileIDSize) > len(b) {
		return nil, kv_store.ErrInvalidByteSize
	}

	fileIDByte := b[n : n+int(header.FileIDSize)]

	return &Index{
		FileID: string(fileIDByte),
		Offset: header.Offset,
		Size:   header.Size,
	}, nil
}
