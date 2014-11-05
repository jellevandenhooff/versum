package transactions

import (
	"certcomp/ads"
	"certcomp/bitcoin/core"
	"certcomp/bitrie"
	"certcomp/comp"
	"certcomp/sha"
	"github.com/conformal/btcwire"
)

type TxnChain struct {
	ads.Base

	Next *TxnChain
	Txn  *core.Transaction
}

func ProcessOutputImpl(t *core.Transaction, output *btcwire.TxOut, txns bitrie.Bitrie, c comp.C) bitrie.Bitrie {
	hash := sha.Sum(output.PkScript)

	loc := bitrie.MakeBits(hash)

	x, found := txns.Get(loc, c)
	var tc *TxnChain
	if found {
		tc = x.(*TxnChain)
	} else {
		tc = nil
	}

	tc = &TxnChain{
		Next: tc,
		Txn:  t,
	}
	return txns.Set(loc, tc, c)
}

func ProcessOutput(t *core.Transaction, output *btcwire.TxOut, txns bitrie.Bitrie, c comp.C) bitrie.Bitrie {
	return c.Call(ProcessOutputImpl, t, output, txns)[0].(bitrie.Bitrie)
}

func ProcessTxnImpl(txn *core.Transaction, txns bitrie.Bitrie, c comp.C) bitrie.Bitrie {
	c.Use(txn)

	for _, output := range txn.MsgTx.TxOut {
		txns = ProcessOutput(txn, output, txns, c)
	}

	return txns
}

func ProcessTxn(txn *core.Transaction, txns bitrie.Bitrie, c comp.C) bitrie.Bitrie {
	return c.Call(ProcessTxnImpl, txn, txns)[0].(bitrie.Bitrie)
}

func ProcessBlock(block *core.Block, txns bitrie.Bitrie, c comp.C) bitrie.Bitrie {
	c.Use(block)

	for _, txn := range block.Transactions {
		txns = ProcessTxn(txn.(*core.Transaction), txns, c)
	}

	return txns
}

func CalculateTxnsImpl(block *core.Block, c comp.C) bitrie.Bitrie {
	if block == nil {
		return bitrie.Nil
	}

	c.Use(block)
	txns := CalculateTxns(block.Previous, c)
	txns = ProcessBlock(block, txns, c)

	return txns
}

func CalculateTxns(block *core.Block, c comp.C) bitrie.Bitrie {
	return c.Call(CalculateTxnsImpl, block)[0].(bitrie.Bitrie)
}

func RegisterTypes() {
	ads.RegisterType(12, &TxnChain{})
	ads.RegisterType(13, &btcwire.TxOut{})

	ads.RegisterFunc(4, CalculateTxnsImpl)
	ads.RegisterFunc(5, ProcessTxnImpl)
	ads.RegisterFunc(6, ProcessOutputImpl)
}
