package storage

import (
	"github.com/orcaman/concurrent-map"
)

// A concurrent map that has dataset names (i.e. dataset1-incoming) as keys and
// pointers to Stores as values.
type StoreCache struct {
	cmap.ConcurrentMap
}

func NewStoreCache() *StoreCache {
	return &StoreCache{cmap.New()}
}

func (self *StoreCache) Get(name string) *Store {
	val, ok := self.ConcurrentMap.Get(name)
	if ok {
		return val.(*Store)
	} else {
		return nil
	}
}

func (self *StoreCache) Put(name string, store *Store) {
	self.ConcurrentMap.Set(name, store)
}
