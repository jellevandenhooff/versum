package namereg

import (
	"bytes"
	"certcomp/ads"
	"certcomp/bitcoin/core"
	"certcomp/bitrie"
	"certcomp/comp"
	"certcomp/sha"
	"github.com/conformal/btcscript"
	"github.com/conformal/btcwire"
)

type Claim struct {
	ads.Base

	Key []byte
}

func GetKey(txns bitrie.Bitrie, outpoint btcwire.OutPoint, c comp.C) []byte {
	ads, _ := txns.Get(bitrie.MakeBits(sha.Hash(outpoint.Hash)), c)
	return ads.(*core.Transaction).MsgTx.TxOut[outpoint.Index].PkScript
}

var tag = []byte{0, 1, 2, 3, 4, 5, 6, 7}

func ProcessTxnImpl(txn *core.Transaction, txns, regs bitrie.Bitrie, c comp.C) (bitrie.Bitrie, bitrie.Bitrie) {
	c.Use(txn)

	txns = txns.Set(bitrie.MakeBits(txn.ComputeHash()), txn, c)

	if len(txn.MsgTx.TxOut) != 1 {
		return txns, regs
	}

	script := txn.MsgTx.TxOut[0].PkScript
	if len(script) < 1 || script[0] != btcscript.OP_RETURN {
		return txns, regs
	}

	data := script[1:]
	if len(data) != 40 {
		return txns, regs
	}

	if !bytes.Equal(data[0:8], tag) {
		return txns, regs
	}

	loc := bitrie.MakeBits(sha.Sum(data[8:40]))

	ads, found := regs.Get(loc, c)
	claim := ads.(*Claim)

	// two types of txns: register and transfer
	if len(txn.MsgTx.TxIn) == 1 {
		if found {
			return txns, regs
		}

		key := GetKey(txns, txn.MsgTx.TxIn[0].PreviousOutpoint, c)
		regs = regs.Set(loc, &Claim{Key: key}, c)
	}

	if len(txn.MsgTx.TxIn) == 2 {
		if !found {
			return txns, regs
		}

		from := GetKey(txns, txn.MsgTx.TxIn[0].PreviousOutpoint, c)

		if !bytes.Equal(from, claim.Key) {
			return txns, regs
		}

		to := GetKey(txns, txn.MsgTx.TxIn[1].PreviousOutpoint, c)
		regs = regs.Set(loc, &Claim{Key: to}, c)
	}

	return txns, regs
}

func ProcessTxn(txn *core.Transaction, txns, regs bitrie.Bitrie, c comp.C) (bitrie.Bitrie, bitrie.Bitrie) {
	res := c.Call(ProcessTxnImpl, txn, txns, regs)
	return res[0].(bitrie.Bitrie), res[1].(bitrie.Bitrie)
}

func ProcessBlock(block *core.Block, txns, regs bitrie.Bitrie, c comp.C) (bitrie.Bitrie, bitrie.Bitrie) {
	c.Use(block)

	for _, txn := range block.Transactions {
		txns, regs = ProcessTxn(txn.(*core.Transaction), txns, regs, c)
	}

	return txns, regs
}

func CalculateRegsImpl(block *core.Block, c comp.C) (bitrie.Bitrie, bitrie.Bitrie) {
	if block == nil {
		return bitrie.Nil, bitrie.Nil
	}

	c.Use(block)
	txns, regs := CalculateRegs(block.Previous, c)
	txns, regs = ProcessBlock(block, txns, regs, c)
	return txns, regs
}

func CalculateRegs(block *core.Block, c comp.C) (bitrie.Bitrie, bitrie.Bitrie) {
	res := c.Call(CalculateRegsImpl, block)
	return res[0].(bitrie.Bitrie), res[1].(bitrie.Bitrie)
}

func RegisterTypes() {
	ads.RegisterType(14, &Claim{})

	ads.RegisterFunc(7, CalculateRegsImpl)
	ads.RegisterFunc(8, ProcessTxnImpl)
}
