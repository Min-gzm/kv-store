package kv_store

import "errors"

var (
	ErrKeyIsEmpty                error = errors.New("key is empty")
	ErrKeyNotFound               error = errors.New("key not found")
	ErrIndexUpdateFailed         error = errors.New("index update failed")
	ErrFileNotFound              error = errors.New("file not found")
	ErrDataDirPathNotSet         error = errors.New("data dir path not set")
	ErrInvalidDataFileSize       error = errors.New("invalid data file size")
	ErrInvalidEntryType          error = errors.New("invalid entry type")
	ErrExceededMaxBatchSize      error = errors.New("exceeded max batch size")
	ErrMergeInProgress           error = errors.New("merge in progress")
	ErrDatabaseIsEmpty           error = errors.New("database is empty")
	ErrInvalidByteSize                 = errors.New("invalid byte size")
	ErrMergeIsNotFinished              = errors.New("merge is not finished")
	ErrLastNonMergedFileNotFound       = errors.New("last non merged file not found")
)
