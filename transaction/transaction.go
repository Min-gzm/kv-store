package transaction

const (
	TxnFinishedKey = "txn_finished"
	NonTxnSeq      = 0
)

type Transaction interface {
	Put(key []byte, value []byte) error
	Delete(key []byte) error
	Commit() error
}
