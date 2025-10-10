package iterator

type Iterator[T interface{}] interface {
	Rewind()
	Seek(key []byte)
	Next()
	Valid() bool
	Key() []byte
	Value() T
	Close()
	Size() int
}
