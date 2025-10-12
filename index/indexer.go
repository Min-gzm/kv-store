package index

import "kv_store/iterator"

type Indexer interface {
	Put(key []byte, index *Index) (bool, *Index)
	Get(key []byte) *Index
	Delete(key []byte) *Index
	Iterator(reverse bool) iterator.Iterator[*Index]
}

func NewBTreeIndexer(degree int) Indexer {
	return newBTree(degree)
}
