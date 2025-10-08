package index

import (
	"bytes"
	"github.com/google/btree"
	"sync"
)

type Item struct {
	key   []byte
	index *Index
}

func (item *Item) Less(than btree.Item) bool {
	return bytes.Compare(item.key, than.(*Item).key) < 0
}

type BTree struct {
	tree  *btree.BTree
	mutex sync.RWMutex
}

func NewBTree(degree int) *BTree {
	return &BTree{
		tree: btree.New(degree),
	}
}

func (bt *BTree) Put(key []byte, index *Index) bool {
	bt.mutex.Lock()
	defer bt.mutex.Unlock()

	item := &Item{
		key:   key,
		index: index,
	}
	bt.tree.ReplaceOrInsert(item)
	return true
}

func (bt *BTree) Get(key []byte) *Index {
	searchItem := &Item{
		key:   key,
		index: nil,
	}
	item := bt.tree.Get(searchItem)
	if item == nil {
		return nil
	}
	return item.(*Item).index
}

func (bt *BTree) Delete(key []byte) bool {
	bt.mutex.Lock()
	defer bt.mutex.Unlock()

	searchItem := &Item{key: key}
	oldItem := bt.tree.Delete(searchItem)

	return oldItem != nil
}
