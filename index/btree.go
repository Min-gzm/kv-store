package index

import (
	"bytes"
	"github.com/google/btree"
	"kv_store/iterator"
	"sort"
	"sync"
)

type Item struct {
	key   []byte
	index *Index
}

func (item *Item) Less(than btree.Item) bool {
	return bytes.Compare(item.key, than.(*Item).key) < 0
}

type bTree struct {
	tree  *btree.BTree
	mutex sync.RWMutex
}

func newBTree(degree int) *bTree {
	return &bTree{
		tree: btree.New(degree),
	}
}

func (bt *bTree) Put(key []byte, index *Index) bool {
	bt.mutex.Lock()
	defer bt.mutex.Unlock()

	item := &Item{
		key:   key,
		index: index,
	}
	bt.tree.ReplaceOrInsert(item)
	return true
}

func (bt *bTree) Get(key []byte) *Index {
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

func (bt *bTree) Delete(key []byte) bool {
	bt.mutex.Lock()
	defer bt.mutex.Unlock()

	searchItem := &Item{key: key}
	oldItem := bt.tree.Delete(searchItem)

	return oldItem != nil
}

func (bt *bTree) Iterator(reverse bool) iterator.Iterator[*Index] {
	it := newBTreeIterator(bt.tree, reverse)
	return it
}

type bTreeIterator struct {
	currentIndex int
	reverse      bool
	values       []*Item
}

func newBTreeIterator(bt *btree.BTree, reverse bool) *bTreeIterator {
	bTreeIterator := &bTreeIterator{
		currentIndex: 0,
		reverse:      reverse,
		values:       make([]*Item, bt.Len()),
	}

	saveValues := func(item btree.Item) bool {
		if it, ok := item.(*Item); ok {
			bTreeIterator.values = append(bTreeIterator.values, it)
		}
		return true
	}
	if reverse {
		bt.Descend(saveValues)
	} else {
		bt.Ascend(saveValues)
	}

	return bTreeIterator
}

func (bti *bTreeIterator) Rewind() {
	bti.currentIndex = 0
}

func (bti *bTreeIterator) Seek(key []byte) {
	if bti.reverse {
		// descending order, find the first position where key <= input key
		bti.currentIndex = sort.Search(len(bti.values), func(i int) bool {
			return bytes.Compare(bti.values[i].key, key) <= 0
		})
	} else {
		bti.currentIndex = sort.Search(len(bti.values), func(i int) bool {
			return bytes.Compare(bti.values[i].key, key) >= 0
		})
	}
}

func (bti *bTreeIterator) Next() {
	bti.currentIndex++
}

func (bti *bTreeIterator) Valid() bool {
	return bti.currentIndex >= 0 && bti.currentIndex < len(bti.values)
}

func (bti *bTreeIterator) Key() []byte {
	return bti.values[bti.currentIndex].key
}

func (bti *bTreeIterator) Value() *Index {
	return bti.values[bti.currentIndex].index
}

func (bti *bTreeIterator) Close() {
	bti.values = nil
}

func (bti *bTreeIterator) Size() int {
	return len(bti.values)
}
