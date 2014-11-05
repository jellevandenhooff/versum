package main

import (
	"certcomp/ads"
	"certcomp/bitcoin/core"
	"certcomp/bitcoin/transactions"
	"certcomp/bitrie"
	"certcomp/comp"
	"certcomp/verified"
	"flag"
	"fmt"
	//"github.com/davecgh/go-spew/spew"
	"log"
	"path/filepath"
)

var BaseDbPath = flag.String("DbPath", "/x/4/jelle/db", "Where to store data.")

var treapToken = flag.Int64("token", 0, "token from builder")

func BitrieSize(balances bitrie.Bitrie, c comp.C) int {
	c.Use(balances)

	if _, ok := balances.(*bitrie.BitrieLeaf); ok {
		return 1
	}

	if node, ok := balances.(*bitrie.BitrieNode); ok {
		return BitrieSize(node.Left, c) + BitrieSize(node.Right, c)
	}

	log.Panic("wut!")
	return 0
}

func main() {
	core.RegisterTypes()
	transactions.RegisterTypes()

	flag.Parse()

	db := core.ContinueDB(filepath.Join(*BaseDbPath, "transactions"), *treapToken)

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
	transactions := lastReturn.ArgsOrResults[0].(bitrie.Bitrie)

	c.Use(transactions)
	fmt.Println(BitrieSize(transactions, c))

	for i := 0; i < 100; i++ {
		fmt.Println(core.RandomKey(bitrie.Bits{}, transactions, c))
	}
}
