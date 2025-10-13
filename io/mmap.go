package io

import (
	"golang.org/x/exp/mmap"
)

// MmapManager The mmap is only for reading
type MmapManager struct {
	reader *mmap.ReaderAt // memory-mapped read-only access
}

func NewMmapFileManager(filepath string) (Manager, error) {
	reader, err := mmap.Open(filepath)
	if err != nil {
		return nil, err
	}

	return &MmapManager{
		reader: reader,
	}, nil
}

func (m *MmapManager) Read(buffer []byte, offset int64) (int, error) {
	// Use memory-mapped read for better performance
	return m.reader.ReadAt(buffer, offset)
}

func (m *MmapManager) Write(content []byte) (int, error) {
	panic("not implement")
}

func (m *MmapManager) Sync() error {
	return nil
}

func (m *MmapManager) Close() error {
	if m.reader == nil {
		return nil
	}
	return m.reader.Close()
}
