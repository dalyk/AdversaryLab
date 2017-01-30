package storage

import (
	"fmt"

	"github.com/Workiva/go-datastructures/trie/ctrie"
)

type SequenceMap struct {
	store   *Store		// store with /dataset1-incoming-offsets-sequence/index /source/ files
	ctrie   *ctrie.Ctrie	// hash tree that stores sequences; key is the sequence, value is the Record{index, sequence}.
	bytemap *Countmap	// struct including countmap file pointer, best rule, and rule update channel
}

func NewSequenceMap(name string, updates chan *RuleCandidate) (*SequenceMap, error) {
	// OpenStore will create files like
	// store/dataset1-incoming-offsets-sequence/index
	// store/dataset1-incoming-offsets-sequence/source
	store, err := OpenStore(name + "-sequence")
	if err != nil {
		return nil, err
	}
	// Ctrie is a concurrent, lock-free hash trie. By default, keys are hashed
	// using FNV-1a unless a HashFactory is provided to New.
	// In computer science, a hash tree (or hash trie) is a persistent data structure
	// that can be used to implement sets and maps, intended to replace hash tables in
	// purely functional programming. In its basic form, a hash tree stores the hashes
	// of its keys, regarded as strings of bits, in a trie, with the actual keys and
	// (optional) values stored at the trie's "final" nodes. (from Wikipedia)
	var ctrie *ctrie.Ctrie = ctrie.New(nil)
	var bytemap *Countmap
	bytemap, err = NewCountmap(name+"-sequence", updates)
	if err != nil {
		return nil, err
	}

	// Puts sequences already in the store into the ctrie. Confused about 0 index.
	store.BlockingFromIndexDo(0, func(record *Record) {
		// Insert adds the key-value pair to the Ctrie, replacing the existing value if
		// the key already exists.
		ctrie.Insert(record.Data, record)
	})

	return &SequenceMap{store: store, ctrie: ctrie, bytemap: bytemap}, nil
}

// The sequence contains the offset (first two bytes) and the subpayload (beginning at the offset and of
// variable length).
func (self *SequenceMap) Increment(allowBlock bool, sequence []byte) {
	//	fmt.Println("Incrementing", len(sequence), sequence)
	if val, ok := self.ctrie.Lookup(sequence); ok {
		// If the sequence has already been encountered, increment the number of times it has
		// been seen for allow/block.
		record := val.(*Record)
		self.bytemap.IncrementCount(record.Index, allowBlock)
	} else {
		// If the sequence hasn't been encountered before, add it to the store (both source and index
		// files). The index file is needed because the offset/subsequence combo may have variable length.
		// The purpose of the store is to associate an offset/subsequence combo with an index that is then
		// used in the countmap file to determine how many times this offset/subsequence combo has been
		// seen for allow/block.
		index := self.store.Add(sequence)
		if index == -1 {
			fmt.Println("Error adding sequence to store", len(sequence), sequence)
			return
		}
		//		fmt.Println("Added sequence", self.store.Path, len(sequence), "got index", index)
		record, err := self.store.GetRecord(index)	// Attempt to retrieve the just-added sequence
		if err != nil {
			fmt.Println("Error adding record")
			return
		}
		if len(record.Data) == 0 {
			fmt.Println("Error, added sequence now has 0 length")
			return
		}
		if record.Index != index {
			fmt.Println("Error, record has incorrect index")
			return
		}

		self.bytemap.IncrementCount(index, allowBlock)
		self.ctrie.Insert(sequence, record)
	}
}

// not used
func (self *SequenceMap) ProcessBytes(allowBlock bool, sequence []byte) {
	for length := 1; length <= len(sequence); length++ {
		for offset := 0; offset+length <= len(sequence); offset++ {
			self.Increment(allowBlock, sequence[offset:offset+length])
		}
	}

	self.bytemap.Save()
}
