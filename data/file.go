package data

import (
	"kv_store/io"
)

type File struct {
	ID          string
	WriteOffset int64
	IOManager   io.Manager
}

func NewFile(id string) (*File, error) {
	manager, err := io.NewFileManager(id)
	if err != nil {
		return nil, err
	}
	return &File{
		ID:        id,
		IOManager: manager,
	}, nil
}

func (f *File) Read(offset int64) (*Entry, int, error) {
	// TODO: optimize the buffer size
	buffer := make([]byte, 0, 200)
	size, err := f.IOManager.Read(buffer, offset)
	if err != nil {
		return nil, 0, err
	}
	entry, err := DecodeEntry(buffer)
	if err != nil {
		return nil, 0, err
	}
	return entry, size, nil
}

func (f *File) Write(b []byte) error {
	size, err := f.IOManager.Write(b)
	if err != nil {
		return err
	}
	f.WriteOffset += int64(size)
	return nil
}

func (f *File) Sync() error {
	return f.IOManager.Sync()
}

func (f *File) Close() error {
	return f.IOManager.Close()
}
