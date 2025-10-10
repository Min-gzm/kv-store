package transaction

type Transaction interface {
	Put(key []byte, value []byte) error
	Delete(key []byte) error
	Commit() error
}
