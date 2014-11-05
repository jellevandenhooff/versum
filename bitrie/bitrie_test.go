package bitrie

import (
	"certcomp/comp"
	"certcomp/sha"
	"fmt"
	"testing"
)

const stressN = 10000

func TestBitrieStress(t *testing.T) {
	trie := Nil
	keys := make([]Bits, 0)

	for i := 0; i < stressN; i++ {
		keys = append(keys, MakeBits(sha.Sum([]byte(fmt.Sprint(i)))))
	}

	for i := 0; i < stressN; i++ {
		trie = trie.Set(keys[i], i, comp.NilC)
	}

	for i := 0; i < stressN; i++ {
		value, found := trie.Get(keys[i], comp.NilC)
		if !found || value != i {
			t.Fatalf("missing %d", i)
		}
	}

	for i := 0; i < stressN; i += 2 {
		trie = trie.Delete(keys[i], comp.NilC)
	}

	for i := 0; i < stressN; i++ {
		value, found := trie.Get(keys[i], comp.NilC)
		if i%2 == 0 {
			if found {
				t.Fatalf("unexpected %d", i)
			}
		} else {
			if !found || value != i {
				t.Fatalf("missing %d", i)
			}
		}
	}
}

func TestBitrieSimple(t *testing.T) {
	trie := Nil

	a := MakeBits(sha.Sum([]byte("a")))
	b := MakeBits(sha.Sum([]byte("b")))
	c := MakeBits(sha.Sum([]byte("c")))
	d := MakeBits(sha.Sum([]byte("d")))

	trie = trie.Set(a, "a", comp.NilC)
	trie = trie.Set(b, "b", comp.NilC)
	trie = trie.Set(c, "c", comp.NilC)
	trie = trie.Set(d, "d", comp.NilC)

	trie = trie.Delete(b, comp.NilC)

	if value, found := trie.Get(a, comp.NilC); value != "a" || !found {
		t.Fatalf("no a")
	}

	if _, found := trie.Get(b, comp.NilC); found {
		t.Fatalf("got b")
	}

	if value, found := trie.Get(c, comp.NilC); value != "c" || !found {
		t.Fatalf("no c")
	}

	if value, found := trie.Get(d, comp.NilC); value != "d" || !found {
		t.Fatalf("no d")
	}
}
