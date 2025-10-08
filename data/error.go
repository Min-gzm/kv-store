package data

import "errors"

var (
	ErrInvalidCRC error = errors.New("invalid crc")
)
