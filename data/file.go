package data

import "kv_store/io"

type File struct {
	ID          string
	WriteOffset int64
	IOManager   io.Manager
}
