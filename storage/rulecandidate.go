package storage

import "math"

// A rule candidate is represented by an index, which represents an offset/subsequence combo.
// The ideal candidate has BlockCount close to BlockTotal and AllowCount far from AllowTotal if
// this rule candidate is going to be used for blocking data.  Want high bc/bt - ac/at.
// The ideal candidate has AllowCount close to AllowTotal and BlockCount far from BlockTotal if
// this rule candidate is going to be used for allowing data.  Want high ac/at - bc/bt.

type RuleCandidate struct {
	Index      int64
	AllowCount int64
	AllowTotal int64
	BlockCount int64
	BlockTotal int64
}

func (self *RuleCandidate) BetterThan(other *RuleCandidate) bool {
	return self.Score() > other.Score()
}

func (self *RuleCandidate) Score() float64 {
	return math.Abs(self.rawScore())
}

// Return true if the rule should be used for allowing.  Return false if the rule
// should be used for blocking.
func (self *RuleCandidate) RequireForbid() bool {
	return self.rawScore() > 0
}

func (self *RuleCandidate) rawScore() float64 {
	// If haven't seen more than 3 subsequences yet, return 0.  (Thought this
	// was supposed to be 3 overall packets, not subsequences?)
	if self.AllowTotal < 3 || self.BlockTotal < 3 {
		return 0
	}

	allow := float64(self.AllowCount) / float64(self.AllowTotal)
	block := float64(self.BlockCount) / float64(self.BlockTotal)
	return allow - block
}
