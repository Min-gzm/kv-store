package data

import (
	"io"
	"kv_store/index"
	myio "kv_store/io"
	"kv_store/transaction"
	"path/filepath"
	"time"
)

const (
	FileSuffix            = ".data"
	HintFileName          = "hint.h"
	MergeFinishedFileName = "merge_finished"
)

type File struct {
	ID          string
	WriteOffset int64
	ioManager   myio.Manager
	Timestamp   int64
	Filepath    string
}

func NewDataFile(dir, id string) (*File, error) {
	path := filepath.Join(dir, id+FileSuffix)
	return newFile(id, path, time.Now().UnixMilli(), myio.General)
}

func LoadDataFile(dir, id string, timestamp int64, ioType myio.FileManagerType) (*File, error) {
	path := filepath.Join(dir, id+FileSuffix)
	return newFile(id, path, timestamp, ioType)
}

func newFile(id, path string, timestamp int64, ioType myio.FileManagerType) (*File, error) {
	manager, err := myio.NewIOManager(path, ioType)
	if err != nil {
		return nil, err
	}
	return &File{
		ID:          id,
		WriteOffset: 0,
		ioManager:   manager,
		Timestamp:   timestamp,
		Filepath:    path,
	}, nil
}

func NewHintFile(dir string) (*File, error) {
	path := filepath.Join(dir, HintFileName)
	return newFile(HintFileName, path, time.Now().UnixMilli(), myio.General)
}

func LoadHintFile(dir string) (*File, error) {
	path := filepath.Join(dir, HintFileName)
	return newFile(HintFileName, path, time.Now().UnixMilli(), myio.Mmap)
}

func NewMergeFinishedFile(dir string) (*File, error) {
	path := filepath.Join(dir, MergeFinishedFileName)
	return newFile(MergeFinishedFileName, path, time.Now().UnixMilli(), myio.General)
}

func (f *File) readNBytes(n int, offset int64) ([]byte, error) {
	buffer := make([]byte, n)
	_, err := f.ioManager.Read(buffer, offset)
	return buffer, err
}

func (f *File) Read(offset int64) (*Entry, int, error) {
	// read a max entry header size to get the header
	headerBuffer, err := f.readNBytes(MaxEntryHeaderSize, offset)
	if err != nil && err != io.EOF {
		return nil, 0, err
	}
	header, headerSize := DecodeEntryHeader(headerBuffer)
	// reach the end of the file
	if header == nil {
		return nil, 0, io.EOF
	}
	// reach the end of the file
	if header.CRC == 0 && header.KeySize == 0 && header.ValueSize == 0 {
		return nil, 0, io.EOF
	}

	entrySize := uint32(headerSize) + header.KeySize + header.ValueSize
	entryBuffer := make([]byte, entrySize)
	_, err = f.ioManager.Read(entryBuffer, offset)
	if err != nil {
		return nil, 0, err
	}
	entry, err := DecodeEntry(entryBuffer)

	return entry, int(entrySize), nil
}

func (f *File) Write(b []byte) error {
	size, err := f.ioManager.Write(b)
	if err != nil {
		return err
	}
	f.WriteOffset += int64(size)
	return nil
}

func (f *File) WriteHintFile(key []byte, i *index.Index) error {
	encodedIndex := index.EncodeIndex(i)
	entry := &Entry{
		Key:   key,
		Value: encodedIndex,
		Type:  EntryNormal,
		Seq:   transaction.NonTxnSeq,
	}
	encodedEntry, _ := EncodeEntry(entry)
	if err := f.Write(encodedEntry); err != nil {
		return err
	}
	return nil
}

func (f *File) Sync() error {
	return f.ioManager.Sync()
}

func (f *File) Close() error {
	// sync before close
	_ = f.ioManager.Sync()
	return f.ioManager.Close()
}

func (f *File) ChangeIOType(managerType myio.FileManagerType) error {
	err := f.Close()
	if err != nil {
		return err
	}
	manager, err := myio.NewIOManager(f.Filepath, managerType)
	if err != nil {
		return err
	}
	f.ioManager = manager
	return nil
}
