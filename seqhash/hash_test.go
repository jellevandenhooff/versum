package seqhash

import (
	"certcomp/ads"
	"certcomp/comp"
	"math/rand"
	"reflect"
	"testing"
)

type X struct {
	ads.Base

	I    int32
	A, B *X
}

func (x *X) CombineWith(other Hashable, c comp.C) Hashable {
	o := other.(*X)

	return &X{
		A: x,
		B: o,
	}
}

func randomMerge(sequence []*X) *Hash {
	res := make([]*Hash, len(sequence))
	for i, x := range sequence {
		res[i] = New(x)
	}

	for len(res) > 1 {
		idx := rand.Intn(len(res) - 1)

		res[idx] = Merge(res[idx], res[idx+1], comp.NilC)
		copy(res[idx+1:], res[idx+2:])
		res = res[:len(res)-1]
	}

	return res[0]
}

const (
	randomRuns   = 1000
	randomLength = 100
)

func TestMerge_Random(t *testing.T) {
	ads.RegisterType(0, &X{})

	rand.Seed(1)

	for run := 0; run < randomRuns; run++ {

		sequence := make([]*X, 0)
		for i := 0; i < randomLength; i++ {
			sequence = append(sequence, &X{I: rand.Int31()})
		}

		a := randomMerge(sequence)
		b := randomMerge(sequence)

		if !reflect.DeepEqual(a, b) {
			t.Fatalf("%v != %v\n", a, b)
		}
	}
}
