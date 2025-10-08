package index

type Indexer interface {
	Put(key []byte, index *Index) bool
	Get(key []byte) *Index
	Delete(key []byte) bool
	Iterator(reverse bool) Iterator
}

func NewBTreeIndexer(degree int) Indexer {
	return newBTree(degree)
}
