package io

type Manager interface {
	Read(buffer []byte, offset int64) (int, error)
	Write(content []byte) (int, error)
	Sync() error
	Close() error
}
