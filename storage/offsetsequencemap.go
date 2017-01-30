package storage

import (
	"bytes"
	"encoding/binary"
)

type OffsetSequenceMap struct {
	*SequenceMap
}

func NewOffsetSequenceMap(name string, updates chan *RuleCandidate) (*OffsetSequenceMap, error) {
	result, err := NewSequenceMap(name+"-offsets", updates)
	if err != nil {
		return nil, err
	}

	return &OffsetSequenceMap{SequenceMap: result}, nil
}

// offset currently always 0 0.  Increment the number of times this offset/subsequence combo has been
// seen for allow/block as well as the total number of allow/block subsequences now seen.
func (self *OffsetSequenceMap) Increment(allowBlock bool, offset int16, bs []byte) {
	buff := new(bytes.Buffer)
	// Use little endian (smallest byte first) so that when do buff.Bytes(), comes back in correct order.
	binary.Write(buff, binary.LittleEndian, offset)
	binary.Write(buff, binary.LittleEndian, bs)

	// This sequence will be two bytes of the offset and the rest is the subsequence (variable length, starting
	// from the offset).
	sequence := buff.Bytes()

	self.SequenceMap.Increment(allowBlock, sequence)
}

// Process a new training packet. The input sequence is the full training packet payload.
func (self *OffsetSequenceMap) ProcessBytes(allowBlock bool, sequence []byte) {
	var length int16
	var offset int16	// currently always 0
	// Cycle through the byte sequences that always start at the beginning
	for length = 1; length <= int16(len(sequence)); length++ {
		// for offset = 0; offset+length <= int16(len(sequence)); offset++ {
		// 	self.Increment(allowBlock, offset, sequence[int(offset):int(offset+length)])
		// }

		offset = 0
		self.Increment(allowBlock, offset, sequence[int(offset):int(offset+length)])
	}

	self.bytemap.Save()	// commit the countmap file to disk
}
