package kv_store

import "errors"

var (
	ErrKeyIsEmpty          error = errors.New("key is empty")
	ErrKeyNotFound         error = errors.New("key not found")
	ErrDatabaseNotOpen     error = errors.New("database not open")
	ErrDatabaseClosed      error = errors.New("database closed")
	ErrIndexUpdateFailed   error = errors.New("index update failed")
	ErrFileNotFound        error = errors.New("file not found")
	ErrDataDirPathNotSet   error = errors.New("data dir path not set")
	ErrInvalidDataFileSize error = errors.New("invalid data file size")
)
