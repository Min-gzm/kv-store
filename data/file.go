package data

import (
	"io"
	myio "kv_store/io"
)

const (
	FileSuffix = ".data"
)

type File struct {
	ID          string
	WriteOffset int64
	IOManager   myio.Manager
}

func NewFile(dir, id string) (*File, error) {
	manager, err := myio.NewFileManager(dir + "/" + id + FileSuffix)
	if err != nil {
		return nil, err
	}
	return &File{
		ID:          id,
		WriteOffset: 0,
		IOManager:   manager,
	}, nil
}

func (f *File) readNBytes(n int, offset int64) ([]byte, error) {
	buffer := make([]byte, n)
	_, err := f.IOManager.Read(buffer, offset)
	return buffer, err
}

func (f *File) Read(offset int64) (*Entry, int, error) {
	// read a max entry header size to get the header
	headerBuffer, err := f.readNBytes(MaxEntryHeaderSize, offset)
	if err != nil {
		return nil, 0, err
	}
	header, headerSize := DecodeEntryHeader(headerBuffer)
	// reach the end of the file
	if header == nil {
		return nil, 0, io.EOF
	}
	// read the end of the file
	if header.CRC == 0 && header.KeySize == 0 && header.ValueSize == 0 {
		return nil, 0, io.EOF
	}

	entrySize := uint32(headerSize) + header.KeySize + header.ValueSize

	// read key and value based on the header info
	keyAndValueBuffer, err := f.readNBytes(int(header.KeySize+header.ValueSize), offset+int64(headerSize))
	if err != nil {
		return nil, 0, err
	}

	// build the entry
	entry := &Entry{
		Type:  header.Type,
		Key:   keyAndValueBuffer[:header.KeySize],
		Value: keyAndValueBuffer[header.KeySize:],
	}

	// verify the crc
	if !verifyHeaderCRC(header, headerBuffer[:headerSize]) {
		return nil, 0, ErrInvalidCRC
	}
	return entry, int(entrySize), nil
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
