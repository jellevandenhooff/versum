package seqhash

import (
	"certcomp/ads"
	"certcomp/comp"
	"certcomp/sha"
)

type Hashable interface {
	ads.ADS

	CombineWith(other Hashable, c comp.C) Hashable
}

const (
	unknown int = iota
	mergeLeft
	mergeRight
	leftFringe
	rightFringe
)

type round struct {
	leftFringe, center, rightFringe []Hashable
}

func doRound(elems []Hashable, volatileLeft, volatileRight bool, c comp.C) round {
	N := len(elems)
	kind := make([]int, N)

	left := 0
	right := N - 1

	hashes := make([]sha.Hash, N)
	for i := 0; i < N; i++ {
		hashes[i] = ads.Hash(elems[i])
	}

	for idx := uint(0); ; idx++ {
		if idx > 0 && idx%sha.Bits == 0 {
			for i := 0; i < N; i++ {
				hashes[i] = sha.Sum(hashes[i].Bytes())
			}
		}

		done := true

		if volatileLeft {
			if left < N && kind[left] == unknown && hashes[left].Bit(idx%sha.Bits) == 0 {
				kind[left] = leftFringe
				left++
			}

			if left < N && kind[left] == unknown {
				done = false
			}
		}

		if volatileRight {
			if right >= 0 && kind[right] == unknown && hashes[right].Bit(idx%sha.Bits) == 1 {
				kind[right] = rightFringe
				right--
			}

			if right >= 0 && kind[right] == unknown {
				done = false
			}
		}

		for j := 0; j < N-1; j++ {
			if kind[j] == unknown && kind[j+1] == unknown {
				if hashes[j].Bit(idx%sha.Bits) == 1 && hashes[j+1].Bit(idx%sha.Bits) == 0 {
					kind[j] = mergeLeft
					kind[j+1] = mergeRight
				} else {
					done = false
				}
			}
		}

		if done {
			break
		}
	}

	var r round
	for i := 0; i < N; i++ {
		switch kind[i] {
		case unknown:
			r.center = append(r.center, elems[i])
		case mergeLeft:
			r.center = append(r.center, elems[i].CombineWith(elems[i+1], c))
			i++
		case leftFringe:
			r.leftFringe = append(r.leftFringe, elems[i])
		case rightFringe:
			r.rightFringe = append(r.rightFringe, elems[i])
		}
	}
	return r
}

type Hash struct {
	ads.Base

	Height       int8
	LeftFringes  [][]Hashable
	Top          []Hashable
	RightFringes [][]Hashable
}

func (h *Hash) Empty() bool {
	return h == nil || (h.Height == 0 && len(h.Top) == 0)
}

func New(elem Hashable) *Hash {
	return &Hash{
		Height: 0,
		Top:    []Hashable{elem},
	}
}

var Calls int64

func Merge(left, right *Hash, c comp.C) *Hash {
	if left != nil {
		c.Use(left)
	}

	if right != nil {
		c.Use(right)
	}

	if left.Empty() {
		return right
	}

	if right.Empty() {
		return left
	}

	Calls++

	merged := Hash{}

	elems := make([]Hashable, 0)

	for {
		if merged.Height < left.Height {
			elems = append(left.RightFringes[merged.Height], elems...)
		} else if merged.Height == left.Height {
			elems = append(left.Top, elems...)
		}

		if merged.Height < right.Height {
			elems = append(elems, right.LeftFringes[merged.Height]...)
		} else if merged.Height == right.Height {
			elems = append(elems, right.Top...)
		}

		if merged.Height >= left.Height && merged.Height >= right.Height && len(elems) == 0 {
			break
		}

		round := doRound(elems, merged.Height >= left.Height, merged.Height >= right.Height, c)
		elems = round.center

		if merged.Height < left.Height {
			merged.LeftFringes = append(merged.LeftFringes, left.LeftFringes[merged.Height])
		} else {
			merged.LeftFringes = append(merged.LeftFringes, round.leftFringe)
		}

		if merged.Height < right.Height {
			merged.RightFringes = append(merged.RightFringes, right.RightFringes[merged.Height])
		} else {
			merged.RightFringes = append(merged.RightFringes, round.rightFringe)
		}

		merged.Height++
	}

	merged.Height--
	merged.Top = append(merged.LeftFringes[merged.Height], merged.RightFringes[merged.Height]...)
	merged.LeftFringes = merged.LeftFringes[:merged.Height]
	merged.RightFringes = merged.RightFringes[:merged.Height]

	return &merged
}

func (h *Hash) Finish(c comp.C) Hashable {
	c.Use(h)

	if h.Empty() {
		return nil
	}

	left := make([]Hashable, 0)
	right := make([]Hashable, 0)

	for i := int8(0); i < h.Height; i++ {
		left = append(left, h.LeftFringes[i]...)
		right = append(h.RightFringes[i], right...)

		left = doRound(left, false, false, c).center
		right = doRound(right, false, false, c).center
	}

	elems := append(append(left, h.Top...), right...)
	for len(elems) > 1 {
		elems = doRound(elems, false, false, c).center
	}

	return elems[0]
}
