package main

import (
	"certcomp/ads"
	"certcomp/bitcoin/core"
	"certcomp/bitcoin/transactions"
	"certcomp/bitrie"
	"certcomp/comp"
	"certcomp/seqhash"
	"certcomp/verified"
	"flag"
	"log"
	"os"
	"path/filepath"
	"time"
)

var BootstrapPath = flag.String("BootstrapPath", "/x/4/jelle/bootstrap.dat", "Location of bootstrap.dat")
var BaseDbPath = flag.String("DbPath", "/x/4/jelle/db", "Where to store data.")

func main() {
	flag.Parse()

	var mode = flag.Arg(0)

	var f func(b *core.Block, c comp.C) bitrie.Bitrie

	if mode == "balances" {
		f = core.CalculateBalancesImpl
	} else if mode == "transactions" {
		f = transactions.CalculateTxnsImpl
	} else {
		log.Fatalf("Usage: buildbalances [balances|transactions]")
	}

	core.RegisterTypes()
	transactions.RegisterTypes()

	db := core.CreateDB(filepath.Join(*BaseDbPath, mode))
	pagingC := core.NewPagingC(db)

	file, err := os.Open(*BootstrapPath)
	if err != nil {
		log.Fatalf("couldn't open %v: %v\n", *BootstrapPath, err)
	}
	loader := NewLoader(file)

	processed := int64(0)
	processedNow := int64(0)
	startNow := time.Now()

	start := time.Now()
	last := time.Now()

	log.Println(ads.GetFuncId(f))

	c := &verified.ProofC{
		Outer:   pagingC,
		Stack:   []*verified.LogTreap{nil},
		ToCache: ads.GetFuncId(f),
	}

	var lastBlock *core.Block

	c.Stack[0] = nil

	c.Call(f, (*core.Block)(nil))

	for i := 0; ; i++ {
		b, err := loader.readBlock()
		if err != nil {
			log.Fatalf("couldn't read block: %v\n", err)
		}
		if b == nil {
			break
		}

		block := core.MakeBlock(b, lastBlock)

		// balances = ProcessBlock(block, balances, c)
		c.Stack[0] = nil
		c.Call(f, block)

		//fmt.Printf("block: %v\n", i)
		//dump(bitrie.Bits{}, balances)

		// logtreap = verified.CombineTreap(logtreap, c.Stack[0], c)
		logtreap := c.Stack[0]
		// markUsed(&c.Log[0])

		pagingC.Unload()

		if i%100 == 0 {
			// before we page logtreap, we must compute all seqhashes, or they'll be stored empty...
			logtreap.SeqHash(c)
			pagingC.MarkUsed(logtreap, true)

			// dump(bitrie.Bits{}, balances)
			//fmt.Printf("%v: %v\n", i, doCount(balances, c))
		}

		if i%1000 == 0 {
			token := pagingC.Store(ads.GetInfo(logtreap))
			log.Printf("after %d: %d\n", i, token)
			if err := db.BufferedWriter.Flush(); err != nil {
				log.Panic(err)
			}
		}

		bytes, _ := b.Bytes()
		processed += int64(len(bytes))
		processedNow += int64(len(bytes))

		now := time.Now()

		if now.Sub(last) > time.Second {
			last = now

			log.Printf("block: %d\n", i)

			nowSecs := int64(now.Sub(startNow) / time.Second)
			secs := int64(now.Sub(start) / time.Second)

			ops := int64(logtreap.Count(c)) / 2

			log.Printf("count: %d\n", pagingC.Count)
			log.Printf("processed % 8.2f MB, % 5.2f MB/sec\n", float64(processed)/1000/1000, float64(processed/secs)/1000/1000)
			log.Printf("procesnow % 8.2f MB, % 5.2f MB/sec\n", float64(processedNow)/1000/1000, float64(processedNow/nowSecs)/1000/1000)
			log.Printf("merges    % 8.3fe6, % 5.3fe6 per sec\n", float64(seqhash.Calls)/1000/1000, float64(seqhash.Calls/secs)/1000/1000)
			log.Printf("ops       % 8.3fe6, % 5.3fe6 per sec\n", float64(ops)/1000/1000, float64(ops/secs)/1000/1000)
			log.Printf("uses      % 8.3fe6, % 5.3fe6 per sec\n", float64(comp.Uses)/1000/1000, float64(comp.Uses/secs)/1000/1000)
			log.Printf("loads     % 8.3fe6, % 5.3fe6 per sec\n", float64(pagingC.Loads)/1000/1000, float64(pagingC.Loads/secs)/1000/1000)
			log.Printf("unloads   % 8.3fe6, % 5.3fe6 per sec\n", float64(pagingC.Unloads)/1000/1000, float64(pagingC.Unloads/secs)/1000/1000)
			log.Printf("loadtime %d unloadtime %d loaddisktime %d total %d", pagingC.LoadTime/time.Second, pagingC.UnloadTime/time.Second, pagingC.LoadDiskTime/time.Second, secs)

			if nowSecs > 5 {
				startNow = now
				processedNow = 0
			}
		}
	}

	logtreap := c.Stack[0]
	token := pagingC.Store(ads.GetInfo(logtreap))
	log.Printf("final: %d\n", token)
	if err := db.BufferedWriter.Flush(); err != nil {
		log.Panic(err)
	}
}
