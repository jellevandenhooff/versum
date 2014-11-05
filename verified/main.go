package verified

import (
	"certcomp/ads"
	"certcomp/comp"
	"certcomp/seqhash"
	"fmt"

	"github.com/davecgh/go-spew/spew"
)

type Tmp struct {
	ads.Base
	Log []LogEntry
}

func yayGo() {
	spew.Dump(nil)
}

func fib(i int64, c comp.C) int64 {
	if i <= 1 {
		return 1
	}

	return Fib(i-2, c) + Fib(i-1, c)
}

func Fib(i int64, c comp.C) int64 {
	return c.Call(fib, i)[0].(int64)
}

func Resolve(lt LogTree, c comp.C) (*LogEntry, error) {
	c.Use(lt)
	pos := lt.Count() - 1

	expected := make([]*LogEntry, 0)

	for {
		if pos == -1 {
			return nil, nil
		}

		entry := lt.Index(pos, c)
		expected = append(expected, entry)

		c.Use(entry)
		if entry.Type == FunctionEntry {
			break
		} else {
			pos -= entry.Length
			entryEntry := lt.Index(pos, c)
			expected = append(expected, entryEntry)
			pos--
		}
	}

	n := len(expected)
	for i := 0; i < n/2; i++ {
		expected[i], expected[n-1-i] = expected[n-1-i], expected[i]
	}

	resolveC := ResolveC{
		Outer:    c,
		Expected: expected,
	}

	return resolveC.Resolve()
}

func verifiedMain() {
	ads.RegisterType(0, &LogTreap{})
	ads.RegisterType(1, &LogEntry{})
	ads.RegisterType(2, &Tmp{})
	ads.RegisterType(4, int64(0))
	ads.RegisterType(5, &LogTreeNode{})
	ads.RegisterType(6, &seqhash.Hash{})

	ads.RegisterFunc(0, fib)

	c := NewProofC()
	fmt.Printf("fib(5) = %d\n", Fib(5, c))

	/*
		log := c.stack[0]

		seqHash := log.Slice(0, 10000)
		c = NewProofC()
		next, err := Resolve(seqHash.Finish(c).(LogTree), c)
		spew.Dump(next, err)

		buffer := new(bytes.Buffer)
		encoder := ads.Encoder{
			Writer:      buffer,
			Transparent: c.used,
		}
		encoder.Encode(&seqHash)

		fmt.Printf("proof length: %v\n", buffer.Len())

		var newSeqHash *seqhash.Hash
		decoder := ads.Decoder{
			Reader: buffer,
		}
		decoder.Decode(&newSeqHash)

		next, err = Resolve(newSeqHash.Finish(comp.NilC).(LogTree), comp.NilC)
		spew.Dump(next, err)
		// spew.Dump(newSeqHash)

		fmt.Printf("%s\n%s\n", ads.Hash(seqHash), ads.Hash(newSeqHash))
	*/

	for i := int32(1); i <= c.Stack[0].Count(comp.NilC); i++ {
		seqHash := c.Stack[0].Slice(0, i, comp.NilC)

		fmt.Printf("commitment %v\n", ads.Hash(seqHash))
		fmt.Printf("printing prefix of length %d\n", i)

		logTree := seqHash.Finish(comp.NilC).(LogTree)

		for j, entry := range logTree.Flatten() {
			switch entry.Type {
			case FunctionEntry:
				fmt.Printf("%d: enter fib(%d)\n", j, entry.ArgsOrResults[0])
			case FunctionExit:
				idx := int32(j) - entry.Length
				entryEntry := logTree.Index(idx, comp.NilC)
				fmt.Printf("%d: exit fib(%d)@%d -> %d\n", j, entryEntry.ArgsOrResults[0], idx, entry.ArgsOrResults[0])
			}
		}

		//spew.Dump("last:", logTree.Flatten()[i-1])

		next, err := Resolve(logTree, comp.NilC)

		if err != nil {
			panic(err)
		} else {
			if next != nil {
				expected := seqhash.Merge(seqHash, seqhash.New(next), comp.NilC)
				fmt.Printf("predicted  %v\n", ads.Hash(expected))
			}

			//spew.Dump("next:", next)
		}
	}

	/*
		var t *Treap

		for i := 0; i < 10000; i++ {
			c := NewProofC()
			t = t.Insert(KeyType(i), ValueType(i*3), c)
		}

		c := NewProofC()
		fmt.Printf("found %v\n", *t.Find(20, c))
		t.CachingCount(c)

		tmp := &Tmp{Log: c.log}

		fmt.Printf("%v\n", Hash(tmp))

		buffer := new(bytes.Buffer)
		encoder := Encoder{
			Writer:      buffer,
			transparent: map[Value]bool{tmp: true},
		}

		encoder.Encode(&tmp)

		var newTmp *Tmp
		decoder := Decoder{
			Reader: buffer,
		}
		decoder.Decode(&newTmp)

		spew.Dump(newTmp)
	*/

	/*
		buffer := new(bytes.Buffer)
		encoder := Encoder{
			Writer:      buffer,
			transparent: c.used,
		}

		encoder.Encode(&t)

		fmt.Printf("proof size: %d\n", buffer.Len())

		var newT *Treap
		decoder := Decoder{
			Reader: buffer,
		}
		decoder.Decode(&newT)

		// spew.Dump(t)
		// spew.Dump(newT)

		c = NewC()
		fmt.Printf("found %v\n", *newT.Find(20, c))

		fmt.Printf("%+v\n", Hash(t))
		fmt.Printf("%+v\n", Hash(newT))
	*/

	/*
		le := new(LogEntry)

		le.Func = (*Treap).cachingCount
		le.ArgsOrResults = []interface{}{t}

		buffer := new(bytes.Buffer)
		encoder := Encoder{
			Writer:      buffer,
			transparent: map[Value]bool{le: true},
		}

		encoder.Encode(&le)

		fmt.Printf("proof size: %d\n", buffer.Len())

		var newLE *LogEntry
		decoder := Decoder{
			Reader: buffer,
		}
		decoder.Decode(&newLE)

		fmt.Printf("hash of     le: %v\n", Hash(le))
		fmt.Printf("hash of new le: %v\n", Hash(newLE))
	*/
}
