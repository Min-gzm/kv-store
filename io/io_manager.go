package io

type FileManagerType = int

const (
	General FileManagerType = iota
	Mmap
)

type Manager interface {
	Read(buffer []byte, offset int64) (int, error)
	Write(content []byte) (int, error)
	Sync() error
	Close() error
}

func NewIOManager(filepath string, t FileManagerType) (Manager, error) {
	if t == General {
		return newFileManager(filepath)
	} else {
		return NewMmapFileManager(filepath)
	}
}
