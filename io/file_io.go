package io

import "os"

type FileManager struct {
	fd *os.File
}

func newFileManager(filepath string) (Manager, error) {
	fd, err := os.OpenFile(filepath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}
	return &FileManager{fd}, nil
}

func (fm *FileManager) Read(buffer []byte, offset int64) (int, error) {
	return fm.fd.ReadAt(buffer, offset)
}

func (fm *FileManager) Write(content []byte) (int, error) {
	return fm.fd.Write(content)
}

func (fm *FileManager) Sync() error {
	return fm.fd.Sync()
}

func (fm *FileManager) Close() error {
	return fm.fd.Close()
}
