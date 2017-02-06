package storage

import (
	"encoding/binary"
	"fmt"
	"os"
)

type Countmap struct {
	bytemap *os.File		// Pointer to the countmap file
	Best    *RuleCandidate		// initially nil
	Updates chan *RuleCandidate	// Channel for best rule candidate updates
}

// description of countmap file:
// header is composed of: 16 unused bytes, 8 bytes for total # blocked sequences seen,
// 8 bytes for total # of allowed sequences seen.
// For each index, have block count followed by allow count.

func NewCountmap(name string, updates chan *RuleCandidate) (*Countmap, error) {
	// Creates a file like store/dataset1-incoming-offsets-sequence/countmap
	bytemap, err := os.OpenFile("store/"+name+"/countmap", os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		fmt.Println("Error opening countmap file", err)
		return nil, err
	}
	// stat, err2 := bytemap.Stat()
	// if err2 != nil {
	// 	fmt.Println("Error getting size of countmap file", err2)
	// 	return nil, err2
	// }
	// if stat.Size() == 0 {
	// 	zeros := make([]byte, int64Size)
	// 	bytemap.WriteAt(zeros, (1500*256*256+1)*int64Size)
	// }

	return &Countmap{bytemap: bytemap, Best: nil, Updates: updates}, nil
}

// index refers to a particular sequence in the source file.  Update both the number
// of times this sequence has been seen for allow/block and the total number of
// allow/block sequences now observed.
func (self *Countmap) IncrementCount(index int64, allowBlock bool) {
	value := self.GetCount(index, allowBlock)
	value++
	self.PutCount(index, allowBlock, value)

	self.IncrementTotal(allowBlock)

	self.keepBest(index)
}

// Get number of times this sequence (referred to by its index) has been seen for allow/block.
func (self *Countmap) GetCount(index int64, allowBlock bool) int64 {
	// FIXME - check file length
	offset := self.getOffset(index, allowBlock)
	return self.getInt64(offset)
}

// Set number of times this sequence (referred to by its index) has been seen for allow/block.
func (self *Countmap) PutCount(index int64, allowBlock bool, count int64) {
	offset := self.getOffset(index, allowBlock)
	self.putInt64(offset, count)
}

// Increment number of allow/block sequences seen.
func (self *Countmap) IncrementTotal(allowBlock bool) {
	value := self.GetTotal(allowBlock)
	value++
	self.PutTotal(allowBlock, value)
}

// not used
func (self *Countmap) GetIndex(allowBlock bool) int64 {
	self.bytemap.Sync()
	offset := self.getHeaderOffset(indexHeaderOffset, allowBlock)
	return self.getInt64(offset)
}

// not used
func (self *Countmap) PutIndex(index int64, allowBlock bool) {
	offset := self.getHeaderOffset(indexHeaderOffset, allowBlock)
	self.putInt64(offset, index)
	self.bytemap.Sync()
}

// Get total number of allow/block subsequences seen.
func (self *Countmap) GetTotal(allowBlock bool) int64 {
	self.bytemap.Sync()
	offset := self.getHeaderOffset(totalHeaderOffset, allowBlock)
	return self.getInt64(offset)
}

// Set total number of allow/block subsequences seen.
func (self *Countmap) PutTotal(allowBlock bool, total int64) {
	offset := self.getHeaderOffset(totalHeaderOffset, allowBlock)
	self.putInt64(offset, total)
	self.bytemap.Sync()
}

func (self *Countmap) Save() {
	self.bytemap.Sync()
}

// index refers to a specific offset/subsequence combo, refers to where it is recorded in the store file.
func (self *Countmap) candidate(index int64) *RuleCandidate {
	ac := self.GetCount(index, true)	// # of times this offset/subsequence combo has been seen for accept packets
	at := self.GetTotal(true)		// # of offset/subsequence combos (not necessarily distinct) seen for accept packets
	bc := self.GetCount(index, false)	// # of times this offset/subsequence combo has been seen for block packets
	bt := self.GetTotal(false)		// # of offset/subsequence combos (not necessarily distinct) seen for block packets

	return &RuleCandidate{Index: index, AllowCount: ac, AllowTotal: at, BlockCount: bc, BlockTotal: bt}
}

// index is the index of the last seen offset/subsequence pairing. If this is a better rule than the
// current better rule, updates the best rule field and pushes the rule candidate to the updates channel.
func (self *Countmap) keepBest(index int64) {
	c := self.candidate(index)
	if c.Score() == 0 {
		return
	}

	if self.Best == nil {	// Originally, no best rule is available, so use the first generated rule.
		self.Best = c
		if Debug {
			fmt.Println("First best rule.", self.Best, self.Best.rawScore())
		} else {
			fmt.Print("@")
		}
		self.Updates <- self.Best
	} else {
		if c.BetterThan(self.Best) {
			self.Best = c
			if Debug {
				fmt.Println("New best rule!", self.Best, self.Best.rawScore())
			} else {
				fmt.Print("*")
			}
			self.Updates <- self.Best
		}
	}
}

// header is composed of: 16 unused bytes, 8 bytes for total # blocked sequences seen,
// 8 bytes for total # of allowed sequences seen.
func (self *Countmap) getHeaderOffset(headerIndex int64, allowBlock bool) int64 {
	offset := headerIndex * cellsize
	if allowBlock {
		offset = offset + int64Size
	}

	return offset
}

// For each index, have block count followed by allow count.
func (self *Countmap) getOffset(index int64, allowBlock bool) int64 {
	offset := headerSize + (index * cellsize)
	if allowBlock {
		offset = offset + int64Size
	}

	return offset
}

// Get the value (int64) at the given offset in the countmap file.
func (self *Countmap) getInt64(offset int64) int64 {
	// FIXME - check file length
	buff := make([]byte, int64Size)
	self.bytemap.Seek(offset, 0)
	self.bytemap.Read(buff)
	value, _ := binary.Varint(buff)
	return value
}

// Write the provided value and the provided offset to the countmap file.
func (self *Countmap) putInt64(offset int64, value int64) {
	buff := make([]byte, int64Size)
	binary.PutVarint(buff, value)
	self.bytemap.Seek(offset, 0)
	self.bytemap.Write(buff)
}
