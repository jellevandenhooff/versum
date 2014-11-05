package main

import (
	"certcomp/ads"
	"certcomp/bitcoin/core"
	"certcomp/bitrie"
	"certcomp/comp"
	"certcomp/seqhash"
	"certcomp/verified"
	"flag"
	"fmt"
	//"github.com/davecgh/go-spew/spew"
	"math/rand"
	"path/filepath"
	"sort"
)

var BaseDbPath = flag.String("DbPath", "/x/4/jelle/db", "Where to store data.")

var treapToken = flag.Int64("token", 0, "token from builder")

func computeSize(value ads.ADS, trackc *verified.TrackC) int {
	buffer := ads.GetFromPool()
	defer ads.ReturnToPool(buffer)

	buffer.Reset()
	encoder := &ads.Encoder{
		Writer:      buffer,
		Transparent: trackc.Used,
	}
	encoder.Encode(&value)

	return buffer.Len()
}

func commitmentToBalances(h *seqhash.Hash, c comp.C) int {
	trackc := verified.NewTrackC(c)

	t := h.Finish(trackc).(verified.LogTree)
	trackc.Use(t)

	lastReturn := t.Index(t.Count()-1, trackc)
	trackc.Use(lastReturn)

	_ = lastReturn.ArgsOrResults[0].(bitrie.Bitrie)

	return computeSize(h, trackc)
}

func randomBalance(balances bitrie.Bitrie, c comp.C) int {
	key := core.RandomKey(bitrie.Bits{}, balances, c)

	trackc := verified.NewTrackC(c)
	info, found := balances.Get(key, trackc)
	if !found {
		panic(key)
	}

	oi := info.(*core.OutpointInfo)
	trackc.Use(oi)

	return computeSize(balances, trackc)
}

func nextstep(h *seqhash.Hash, c comp.C) int {
	trackc := verified.NewTrackC(c)
	_, _ = verified.Resolve(h.Finish(trackc).(verified.LogTree), trackc)

	return computeSize(h, trackc)
}

func main() {
	core.RegisterTypes()

	flag.Parse()

	db := core.ContinueDB(filepath.Join(*BaseDbPath, "balances"), *treapToken)

	logtreap := new(verified.LogTreap)
	logtreap.MakeOpaque()
	ads.GetInfo(logtreap).Token = *treapToken

	pagingC := core.NewPagingC(db)
	pagingC.Load(ads.GetInfo(logtreap))

	c := pagingC

	c.Use(logtreap)
	length := logtreap.Count(c)
	hash := logtreap.Slice(0, length, c)

	// this hash is what we commit to....
	c.Use(hash)
	tree := hash.Finish(c).(verified.LogTree)

	c.Use(tree)
	lastReturn := tree.Index(length-1, c)

	c.Use(lastReturn)
	balances := lastReturn.ArgsOrResults[0].(bitrie.Bitrie)

	c.Use(balances)
	// core.Dump(bitrie.Bits{}, balances, c)

	fmt.Println(logtreap.Count(c))
	fmt.Println(commitmentToBalances(hash, c))
	n := 1000
	sizes := make([]int, 0)
	for i := 0; i < n; i++ {
		sizes = append(sizes, randomBalance(balances, c))
	}
	sort.Ints(sizes)
	fmt.Println(sizes)

	sizes = make([]int, 0)
	for i := 0; i < n; i++ {
		j := rand.Intn(int(logtreap.Count(c) - 1))
		hash := logtreap.Slice(0, int32(j+1), c)
		sizes = append(sizes, nextstep(hash, c))
	}

	for i := 0; i < 20; i++ {
		j := rand.Intn(int(logtreap.Count(c) - 5000 - 1))
		for k := j; k < j+5000; k++ {
			hash := logtreap.Slice(0, int32(k+1), c)
			sizes = append(sizes, nextstep(hash, c))
		}
	}
	sort.Ints(sizes)
	fmt.Println(sizes)
}
