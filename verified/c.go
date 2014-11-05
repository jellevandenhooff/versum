package verified

import (
	"certcomp/ads"
	"certcomp/comp"
	//"certcomp/seqhash"
	"errors"
	"fmt"
	"runtime/debug"
)

type VerifyC struct {
}

func (c *VerifyC) Use(values ...ads.ADS) {
	for _, value := range values {
		value.AssertTransparent()
	}
}

func (c *VerifyC) Call(f interface{}, args ...interface{}) []interface{} {
	return comp.Call(f, append(args, c))
}

type TrackC struct {
	Outer comp.C
	Used  map[ads.ADS]bool
}

func (c *TrackC) Use(values ...ads.ADS) {
	c.Outer.Use(values...)
	for _, value := range values {
		c.Used[value] = true
	}
}

func (c *TrackC) Call(f interface{}, args ...interface{}) []interface{} {
	return comp.Call(f, append(args, c))
}

func NewTrackC(outer comp.C) *TrackC {
	return &TrackC{
		Outer: outer,
		Used:  make(map[ads.ADS]bool),
	}
}

type cacheInfo struct {
}

type ProofC struct {
	Outer comp.C
	Stack []*LogTreap

	ToCache         int8
	CachedLog       *LogTreap
	CachedExitEntry *LogEntry
	Caching         bool
}

func (c *ProofC) Use(values ...ads.ADS) {
	c.Outer.Use(values...)
}

func (c *ProofC) Call(f interface{}, args ...interface{}) []interface{} {
	entryEntry := &LogEntry{
		Type:          FunctionEntry,
		Func:          f,
		ArgsOrResults: args,
	}

	var exitEntry *LogEntry
	var log *LogTreap

	cacheFunc := ads.GetFuncId(f) == c.ToCache

	if cacheFunc && c.Caching {
		exitEntry = c.CachedExitEntry
		log = c.CachedLog

	} else {
		c.Stack = append(c.Stack, NewLogTreap(entryEntry))

		if cacheFunc {
			c.Caching = true
		}

		ret := comp.Call(f, append(args, c))

		top := len(c.Stack) - 1
		exitEntry = &LogEntry{
			Type:   FunctionExit,
			Length: c.Stack[top].Count(c.Outer),
			//Internal:      c.Stack[top].SeqHash(c.Outer),
			ArgsOrResults: ret,
		}

		log = CombineTreap(c.Stack[top], NewLogTreap(exitEntry), c.Outer)

		if cacheFunc {
			c.Caching = false
			c.CachedLog = log
			c.CachedExitEntry = exitEntry
		}

		c.Stack = c.Stack[:top]
	}

	top := len(c.Stack) - 1
	if top >= 0 {
		c.Stack[top] = CombineTreap(c.Stack[top], log, c.Outer)
	}

	return exitEntry.ArgsOrResults
}

type ResolveC struct {
	Outer    comp.C
	Expected []*LogEntry
	Idx      int32
	Length   int32
	//Internal *seqhash.Hash
}

func (c *ResolveC) Use(values ...ads.ADS) {
	c.Outer.Use(values...)
}

func (c *ResolveC) Call(f interface{}, args ...interface{}) []interface{} {
	entryEntry := LogEntry{
		Type:          FunctionEntry,
		Func:          f,
		ArgsOrResults: args,
	}

	if c.Idx == int32(len(c.Expected)) {
		panic(&entryEntry)
	}

	c.Outer.Use(c.Expected[c.Idx])
	if !ads.Equals(&entryEntry, c.Expected[c.Idx]) {
		panic(errors.New("bad entry"))
	}
	c.Idx++

	c.Outer.Use(c.Expected[c.Idx])
	ret := c.Expected[c.Idx].ArgsOrResults
	c.Length += c.Expected[c.Idx].Length + 1
	//c.Internal = seqhash.Merge(c.Internal,
	//seqhash.Merge(c.Expected[c.Idx].Internal, seqhash.New(c.Expected[c.Idx]), c.Outer), c.Outer)

	c.Idx++

	return ret
}

func (c *ResolveC) Resolve() (next *LogEntry, err error) {
	done := false

	defer func() {
		if done {
			return
		}

		result := recover()

		if entry, ok := result.(*LogEntry); ok {
			next = entry
			err = nil
			return
		}

		var ok bool
		err, ok = result.(error)
		if !ok {
			err = errors.New("paniced: " + fmt.Sprint(result) + "\n" + string(debug.Stack()))
		} else {
			err = errors.New("errored: " + err.Error())
		}
	}()

	entry := c.Expected[0]
	c.Idx = 1
	c.Length = 1
	//c.Internal = seqhash.New(entry)

	ret := comp.Call(entry.Func, append(entry.ArgsOrResults, c))

	next = &LogEntry{
		Type:          FunctionExit,
		ArgsOrResults: ret,
		Length:        c.Length,
		//Internal:      c.Internal,
	}
	err = nil

	done = true
	return
}

func NewProofC() *ProofC {
	return &ProofC{
		Outer: &TrackC{
			Outer: &VerifyC{},
			Used:  make(map[ads.ADS]bool),
		},
		Stack: []*LogTreap{nil},
	}
}
