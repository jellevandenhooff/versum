package verified

import (
	"certcomp/ads"
	"certcomp/comp"
	"certcomp/seqhash"
	"certcomp/sha"
	"encoding/binary"
	"math/rand"
)

type EntryType int8

const (
	FunctionEntry EntryType = iota
	FunctionExit
)

type LogTree interface {
	ads.ADS
	Count() int32
	Index(idx int32, c comp.C) *LogEntry
	Flatten() []*LogEntry
}

type LogEntry struct {
	ads.Base

	Type          EntryType
	ArgsOrResults []interface{}

	// entry:
	Func interface{}

	// exit:
	Length int32
	//Internal *seqhash.Hash
}

func (l *LogEntry) Count() int32 {
	return 1
}

func (l *LogEntry) Index(idx int32, c comp.C) *LogEntry {
	c.Use(l)

	if idx != 0 {
		panic(idx)
	}
	return l
}

func (l *LogEntry) CombineWith(other seqhash.Hashable, c comp.C) seqhash.Hashable {
	return CombineTree(l, other.(LogTree), c)
}

func (l *LogEntry) Flatten() []*LogEntry {
	return []*LogEntry{l}
}

type LogTreeNode struct {
	ads.Base

	Num         int32
	Left, Right LogTree
}

func CombineTree(left, right LogTree, c comp.C) *LogTreeNode {
	c.Use(left, right)
	return &LogTreeNode{
		Num:   left.Count() + right.Count(),
		Left:  left,
		Right: right,
	}
}

func (l *LogTreeNode) Count() int32 {
	return l.Num
}

func (l *LogTreeNode) Index(idx int32, c comp.C) *LogEntry {
	c.Use(l)

	c.Use(l.Left)
	if idx < l.Left.Count() {
		return l.Left.Index(idx, c)
	}
	return l.Right.Index(idx-l.Left.Count(), c)
}

func (l *LogTreeNode) CombineWith(other seqhash.Hashable, c comp.C) seqhash.Hashable {
	return CombineTree(l, other.(LogTree), c)
}

func (l *LogTreeNode) Flatten() []*LogEntry {
	return append(l.Left.Flatten(), l.Right.Flatten()...)
}

func (l *LogTreeNode) ComputeHash() sha.Hash {
	var buffer [68]byte
	binary.LittleEndian.PutUint32(buffer[0:4], uint32(l.Num))
	copy(buffer[4:36], ads.Hash(l.Left).Bytes())
	copy(buffer[36:68], ads.Hash(l.Right).Bytes())
	return sha.Sum(buffer[:])
}

type LogTreap struct {
	ads.Base

	Value  *LogEntry
	Num    int32
	Merged *seqhash.Hash

	Priority    int64
	Left, Right *LogTreap
}

func NewLogTreap(value *LogEntry) *LogTreap {
	priority := rand.Int63()

	return &LogTreap{Value: value, Num: 1, Merged: nil, Priority: priority, Left: nil, Right: nil}
}

func (t *LogTreap) UpdateLeft(newLeft *LogTreap, c comp.C) *LogTreap {
	num := newLeft.Count(c) + 1 + t.Right.Count(c)
	return &LogTreap{
		Value:    t.Value,
		Num:      num,
		Merged:   nil,
		Priority: t.Priority,
		Left:     newLeft,
		Right:    t.Right,
	}
}

func (t *LogTreap) UpdateRight(newRight *LogTreap, c comp.C) *LogTreap {
	num := t.Left.Count(c) + 1 + newRight.Count(c)
	return &LogTreap{
		Value:    t.Value,
		Num:      num,
		Merged:   nil,
		Priority: t.Priority,
		Left:     t.Left,
		Right:    newRight,
	}
}

// TODO: make sure seqhash is only encoded when merged != nil....
func (t *LogTreap) SeqHash(c comp.C) *seqhash.Hash {
	if t == nil {
		return new(seqhash.Hash)
	}

	c.Use(t)
	if t.Merged == nil {
		t.Merged = t.computeSeqHash(c)
	}

	return t.Merged
}

func (t *LogTreap) computeSeqHash(c comp.C) *seqhash.Hash {
	return seqhash.Merge(t.Left.SeqHash(c),
		seqhash.Merge(seqhash.New(t.Value), t.Right.SeqHash(c), c), c)
}

func (t *LogTreap) Count(c comp.C) int32 {
	if t == nil {
		return 0
	}

	c.Use(t)

	return t.Num
}

func (t *LogTreap) Slice(start, end int32, c comp.C) *seqhash.Hash {
	if t == nil {
		return new(seqhash.Hash)
	}

	c.Use(t)

	if start <= 0 && end >= t.Num {
		return t.SeqHash(c)
	}

	leftCount := t.Left.Count(c)

	h := new(seqhash.Hash)
	if start < leftCount {
		h = seqhash.Merge(h, t.Left.Slice(start, end, c), c)
	}
	if start <= leftCount && end >= leftCount+1 {
		h = seqhash.Merge(h, seqhash.New(t.Value), c)
	}
	if end > leftCount+1 {
		h = seqhash.Merge(h, t.Right.Slice(start-leftCount-1, end-leftCount-1, c), c)
	}
	return h
}

func CombineTreap(left, right *LogTreap, c comp.C) *LogTreap {
	if left == nil {
		return right
	} else if right == nil {
		return left
	}

	c.Use(left, right)

	if left.Priority > right.Priority {
		return left.UpdateRight(CombineTreap(left.Right, right, c), c)
	} else {
		return right.UpdateLeft(CombineTreap(left, right.Left, c), c)
	}
}
