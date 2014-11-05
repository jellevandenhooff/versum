package bitrie

import (
	"certcomp/ads"
	"certcomp/comp"
	"certcomp/seqhash"
	"certcomp/sha"
	"encoding/binary"
	"github.com/davecgh/go-spew/spew"
)

type Tuple struct {
	ads.Base

	A, B ads.ADS
}

func (t *Tuple) CombineWith(other seqhash.Hashable, c comp.C) seqhash.Hashable {
	return &Tuple{A: t, B: other}
}

type Bitrie interface {
	// ads.ADS
	seqhash.Hashable

	Get(b Bits, c comp.C) (ads.ADS, bool)
	Delete(b Bits, c comp.C) Bitrie
	Set(b Bits, value ads.ADS, c comp.C) Bitrie
	prepend(b Bits) Bitrie

	CollectChildren() []ads.ADS
}

type BitrieNil struct {
	ads.Base
}

func (n *BitrieNil) CombineWith(other seqhash.Hashable, c comp.C) seqhash.Hashable {
	return &Tuple{A: n, B: other}
}

func (n *BitrieNil) Get(b Bits, c comp.C) (ads.ADS, bool) {
	return nil, false
}

func (n *BitrieNil) Set(b Bits, value ads.ADS, c comp.C) Bitrie {
	if value == nil {
		return n
	}

	return &BitrieLeaf{
		Bits:  b,
		Value: value,
	}
}

func (n *BitrieNil) Delete(b Bits, c comp.C) Bitrie {
	return n
}

func (n *BitrieNil) prepend(b Bits) Bitrie {
	return n
}

func (n *BitrieNil) ComputeHash() sha.Hash {
	return sha.Sum([]byte{})
}

func (n *BitrieNil) CollectChildren() []ads.ADS {
	return []ads.ADS{}
}

var Nil = Bitrie(&BitrieNil{})

type BitrieNode struct {
	ads.Base
	Bits        Bits
	Left, Right Bitrie
}

func (n *BitrieNode) CombineWith(other seqhash.Hashable, c comp.C) seqhash.Hashable {
	return &Tuple{A: n, B: other}
}

func (n *BitrieNode) Get(b Bits, c comp.C) (ads.ADS, bool) {
	c.Use(n)

	if SplitPoint(n.Bits, b) < n.Bits.Length {
		return nil, false
	}

	tail := b.Cut(n.Bits.Length+1, b.Length)

	if b.Get(n.Bits.Length) == 0 {
		return n.Left.Get(tail, c)
	} else {
		return n.Right.Get(tail, c)
	}
}

func (n *BitrieNode) Set(b Bits, value ads.ADS, c comp.C) Bitrie {
	c.Use(n)

	s := SplitPoint(n.Bits, b)

	if s < n.Bits.Length {
		var left, right Bitrie
		left = &BitrieLeaf{
			Bits:  b.Cut(s+1, b.Length),
			Value: value,
		}

		right = &BitrieNode{
			Bits:  n.Bits.Cut(s+1, n.Bits.Length),
			Left:  n.Left,
			Right: n.Right,
		}

		if b.Get(s) != 0 {
			left, right = right, left
		}

		return &BitrieNode{
			Bits:  n.Bits.Cut(0, s),
			Left:  left,
			Right: right,
		}
	} else {
		tail := b.Cut(n.Bits.Length+1, b.Length)

		if b.Get(n.Bits.Length) == 0 {
			newLeft := n.Left.Set(tail, value, c)
			return &BitrieNode{
				Bits:  n.Bits,
				Left:  newLeft,
				Right: n.Right,
			}
		} else {
			newRight := n.Right.Set(tail, value, c)
			return &BitrieNode{
				Bits:  n.Bits,
				Left:  n.Left,
				Right: newRight,
			}
		}
	}
}

func (n *BitrieNode) Delete(b Bits, c comp.C) Bitrie {
	c.Use(n)

	if SplitPoint(n.Bits, b) < n.Bits.Length {
		return n
	}

	tail := b.Cut(n.Bits.Length+1, b.Length)
	if b.Get(n.Bits.Length) == 0 {
		newLeft := n.Left.Delete(tail, c)

		if _, isNil := newLeft.(*BitrieNil); isNil {
			c.Use(n.Right)
			return n.Right.prepend(n.Bits.Append(1))
		} else {
			return &BitrieNode{
				Bits:  n.Bits,
				Left:  newLeft,
				Right: n.Right,
			}
		}
	} else {
		newRight := n.Right.Delete(tail, c)

		if _, isNil := newRight.(*BitrieNil); isNil {
			c.Use(n.Left)
			return n.Left.prepend(n.Bits.Append(0))
		} else {
			return &BitrieNode{
				Bits:  n.Bits,
				Left:  n.Left,
				Right: newRight,
			}
		}
	}
}

func (n *BitrieNode) prepend(b Bits) Bitrie {
	return &BitrieNode{
		Bits:  b.Cat(n.Bits),
		Left:  n.Left,
		Right: n.Right,
	}
}

func (n *BitrieNode) ComputeHash() sha.Hash {
	var buffer [96]byte
	n.Bits.Canonicalize(buffer[0:32])
	copy(buffer[32:64], ads.Hash(n.Left).Bytes())
	copy(buffer[64:96], ads.Hash(n.Right).Bytes())
	return sha.Sum(buffer[:])
}

func (n *BitrieNode) Encode(e *ads.Encoder) {
	var buffer [40]byte
	copy(buffer[0:32], n.Bits.Bits[0:32])
	binary.LittleEndian.PutUint32(buffer[32:36], uint32(n.Bits.Start))
	binary.LittleEndian.PutUint32(buffer[36:40], uint32(n.Bits.Length))
	e.Write(buffer[0:40])
	e.Encode(&n.Left)
	e.Encode(&n.Right)
}

func (n *BitrieNode) Decode(d *ads.Decoder) {
	var buffer [40]byte
	d.Read(buffer[:])
	n.Bits.Bits = buffer[0:32]
	n.Bits.Start = int32(binary.LittleEndian.Uint32(buffer[32:36]))
	n.Bits.Length = int32(binary.LittleEndian.Uint32(buffer[36:40]))
	d.Decode(&n.Left)
	d.Decode(&n.Right)
}

func (n *BitrieNode) CollectChildren() []ads.ADS {
	return []ads.ADS{n.Left, n.Right}
}

type BitrieLeaf struct {
	ads.Base
	Bits  Bits
	Value ads.ADS
}

func (l *BitrieLeaf) CombineWith(other seqhash.Hashable, c comp.C) seqhash.Hashable {
	return &Tuple{A: l, B: other}
}

func (l *BitrieLeaf) Get(b Bits, c comp.C) (ads.ADS, bool) {
	c.Use(l)

	s := SplitPoint(l.Bits, b)

	if s == b.Length {
		return l.Value, true
	}
	return nil, false
}

func (l *BitrieLeaf) Set(b Bits, value ads.ADS, c comp.C) Bitrie {
	c.Use(l)

	s := SplitPoint(l.Bits, b)

	if s == b.Length {
		return &BitrieLeaf{
			Bits:  b,
			Value: value,
		}
	}

	left := &BitrieLeaf{
		Bits:  b.Cut(s+1, b.Length),
		Value: value,
	}
	right := &BitrieLeaf{
		Bits:  l.Bits.Cut(s+1, l.Bits.Length),
		Value: l.Value,
	}

	if b.Get(s) != 0 {
		left, right = right, left
	}

	return &BitrieNode{
		Bits:  b.Cut(0, s),
		Left:  left,
		Right: right,
	}
}

func (l *BitrieLeaf) Delete(b Bits, c comp.C) Bitrie {
	c.Use(l)

	if SplitPoint(l.Bits, b) < l.Bits.Length {
		return l
	}

	return Nil
}

func (l *BitrieLeaf) prepend(b Bits) Bitrie {
	return &BitrieLeaf{
		Bits:  b.Cat(l.Bits),
		Value: l.Value,
	}
}

func (l *BitrieLeaf) ComputeHash() sha.Hash {
	var buffer [64]byte
	l.Bits.Canonicalize(buffer[0:32])
	if l.Value == nil {
		spew.Dump(l)
	}
	copy(buffer[32:64], ads.Hash(l.Value).Bytes())
	return sha.Sum(buffer[:])
}

func (n *BitrieLeaf) CollectChildren() []ads.ADS {
	return []ads.ADS{n.Value}
}

func (n *BitrieLeaf) Encode(e *ads.Encoder) {
	var buffer [40]byte
	copy(buffer[0:32], n.Bits.Bits[0:32])
	binary.LittleEndian.PutUint32(buffer[32:36], uint32(n.Bits.Start))
	binary.LittleEndian.PutUint32(buffer[36:40], uint32(n.Bits.Length))
	e.Write(buffer[0:40])
	e.Encode(&n.Value)
}

func (n *BitrieLeaf) Decode(d *ads.Decoder) {
	var buffer [40]byte
	d.Read(buffer[:])
	n.Bits.Bits = buffer[0:32]
	n.Bits.Start = int32(binary.LittleEndian.Uint16(buffer[32:36]))
	n.Bits.Length = int32(binary.LittleEndian.Uint16(buffer[36:40]))
	d.Decode(&n.Value)
}
