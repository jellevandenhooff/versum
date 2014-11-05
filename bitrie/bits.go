package bitrie

import (
	"certcomp/sha"
)

type Bits struct {
	Length int32
	Bits   []byte
	Start  int32
}

var zero = make([]byte, 32)

func (b Bits) Canonicalize(target []byte) {
	copy(target, b.Bits)
	copy(target[:b.Start/8], zero)

	target[b.Start/8] &= ^((1 << uint(b.Start%8)) - 1)

	end := b.Start + b.Length

	if end < 256 {
		target[end/8] &= (1 << uint(end%8)) - 1
	}

	copy(target[(end+7)/8:32], zero)
}

func MakeBits(hash sha.Hash) Bits {
	return Bits{
		Length: 256,
		Bits:   hash.Bytes(),
	}
}

func (b Bits) Get(a int32) int {
	a += b.Start
	return int((b.Bits[a/8] >> uint(a%8)) & 1)
}

func (b Bits) Set(a int32, v int) {
	a += b.Start
	if v == 1 {
		b.Bits[a/8] |= 1 << uint(a%8)
	} else {
		b.Bits[a/8] &= ^(1 << uint(a%8))
	}
}

func (b Bits) String() string {
	s := ""
	for i := int32(0); i < b.Length; i++ {
		if b.Get(i) == 0 {
			s += "0"
		} else {
			s += "1"
		}
	}
	return s
}

func (b Bits) Cut(x, y int32) Bits {
	r := Bits{
		Length: y - x,
		Start:  b.Start + x,
		Bits:   b.Bits,
	}

	return r
}

func (b Bits) Append(x int) Bits {
	r := Bits{
		Length: b.Length + 1,
		Start:  b.Start,
		Bits:   make([]byte, 32),
	}

	copy(r.Bits, b.Bits)
	r.Set(b.Length, x)

	return r
}

func (b Bits) Cat(o Bits) Bits {
	r := Bits{
		Length: b.Length + o.Length,
		Start:  b.Start,
		Bits:   make([]byte, 32),
	}

	copy(r.Bits, b.Bits)

	for i := b.Length; i < r.Length; i++ {
		r.Set(i, o.Get(i-b.Length))
	}

	return r
}

func SplitPoint(a, b Bits) int32 {
	l := a.Length
	if b.Length < l {
		l = b.Length
	}

	for i := int32(0); i < l; i++ {
		if a.Get(i) != b.Get(i) {
			return i
		}
	}

	return l
}
