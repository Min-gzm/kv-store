package index

type Indexer interface {
	Put(key []byte, index *Index) bool
	Get(key []byte) *Index
	Delete(key []byte) bool
}
