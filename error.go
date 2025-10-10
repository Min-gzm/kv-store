package kv_store

import "errors"

var (
	ErrKeyIsEmpty           error = errors.New("key is empty")
	ErrKeyNotFound          error = errors.New("key not found")
	ErrIndexUpdateFailed    error = errors.New("index update failed")
	ErrFileNotFound         error = errors.New("file not found")
	ErrDataDirPathNotSet    error = errors.New("data dir path not set")
	ErrInvalidDataFileSize  error = errors.New("invalid data file size")
	ErrInvalidEntryType     error = errors.New("invalid entry type")
	ErrExceededMaxBatchSize error = errors.New("exceeded max batch size")
)
