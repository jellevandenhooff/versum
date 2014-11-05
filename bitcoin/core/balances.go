package core

import (
	"bytes"
	"certcomp/ads"
	"certcomp/bitrie"
	"certcomp/comp"
	"certcomp/seqhash"
	"certcomp/sha"
	"certcomp/verified"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"github.com/conformal/btcutil"
	"github.com/conformal/btcwire"
	"log"
	"math/rand"
	"time"
)

type Transaction struct {
	ads.Base

	MsgTx btcwire.MsgTx
}

func (t *Transaction) Encode(e *ads.Encoder) {
	t.MsgTx.BtcEncode(e, btcwire.ProtocolVersion)
}

func (t *Transaction) Decode(d *ads.Decoder) {
	t.MsgTx.BtcDecode(d, btcwire.ProtocolVersion)
}

func (t *Transaction) ComputeHash() sha.Hash {
	hash, _ := t.MsgTx.TxSha()
	return sha.Hash(hash)
}

func (t *Transaction) CollectChildren() []ads.ADS {
	return []ads.ADS{}
}

type Block struct {
	ads.Base

	Header btcwire.BlockHeader

	Previous     *Block
	Transactions []ads.ADS
}

func (b *Block) Encode(e *ads.Encoder) {
	msgBlock := btcwire.MsgBlock{
		Header: b.Header,
	}
	msgBlock.BtcEncode(e, btcwire.ProtocolVersion)

	e.Encode(&b.Previous)
	e.Encode(&b.Transactions)
}

func (b *Block) Decode(d *ads.Decoder) {
	msgBlock := btcwire.MsgBlock{
		Header: b.Header,
	}
	msgBlock.BtcDecode(d, btcwire.ProtocolVersion)

	d.Decode(&b.Previous)
	d.Decode(&b.Transactions)

	// check that msgBlock has zero transactions and that the transaction merkle tree hash is correct
}

func (b *Block) ComputeHash() sha.Hash {
	hash, _ := b.Header.BlockSha()
	return sha.Hash(hash)
}

func (b *Block) CollectChildren() []ads.ADS {
	return b.Transactions
}

func MakeBlock(block *btcutil.Block, previous *Block) *Block {
	transactions := make([]ads.ADS, 0)
	for _, transaction := range block.Transactions() {
		t := &Transaction{
			MsgTx: *transaction.MsgTx(),
		}
		t.SetCachedHash(sha.Hash(*transaction.Sha()))

		transactions = append(transactions, t)
	}

	b := &Block{
		Header:       block.MsgBlock().Header,
		Previous:     previous,
		Transactions: transactions,
	}
	hash, _ := block.Sha()
	b.SetCachedHash(sha.Hash(*hash))

	return b
}

type OutpointInfo struct {
	ads.Base

	Count []int8
}

func (o *OutpointInfo) Empty() bool {
	for _, count := range o.Count {
		if count != 0 {
			return false
		}
	}
	return true
}

func (o *OutpointInfo) Add(numOutputs int) *OutpointInfo {
	count := make([]int8, numOutputs)
	for i := 0; i < numOutputs; i++ {
		if i < len(o.Count) {
			count[i] = o.Count[i] + 1
		} else {
			count[i] = 1
		}
	}
	return &OutpointInfo{
		Count: count,
	}
}

func (o *OutpointInfo) Spend(idx int) *OutpointInfo {
	n := idx + 1
	if len(o.Count) > n {
		n = len(o.Count)
	}

	count := make([]int8, n)
	copy(count, o.Count)
	count[idx]--

	return &OutpointInfo{
		Count: count,
	}
}

func ProcessOutpointImpl(outpoint btcwire.OutPoint, balances bitrie.Bitrie, c comp.C) bitrie.Bitrie {
	loc := bitrie.MakeBits(sha.Hash(outpoint.Hash))

	x, found := balances.Get(loc, c)
	var oi *OutpointInfo
	if found {
		oi = x.(*OutpointInfo)
	} else {
		oi = &OutpointInfo{}
	}

	c.Use(oi)
	oi = oi.Spend(int(outpoint.Index))

	if oi.Empty() {
		return balances.Delete(loc, c)
	} else {
		return balances.Set(loc, oi, c)
	}
}

func ProcessOutpoint(outpoint btcwire.OutPoint, balances bitrie.Bitrie, c comp.C) bitrie.Bitrie {
	return c.Call(ProcessOutpointImpl, outpoint, balances)[0].(bitrie.Bitrie)
}

func ProcessTransactionImpl(transaction *Transaction, balances bitrie.Bitrie, c comp.C) bitrie.Bitrie {
	c.Use(transaction, balances)

	for _, input := range transaction.MsgTx.TxIn {
		if (input.PreviousOutpoint.Hash == btcwire.ShaHash{}) {
			continue
		}

		balances = ProcessOutpoint(input.PreviousOutpoint, balances, c)
	}

	loc := bitrie.MakeBits(ads.Hash(transaction))
	x, found := balances.Get(loc, c)
	var oi *OutpointInfo
	if found {
		oi = x.(*OutpointInfo)
	} else {
		oi = &OutpointInfo{}
	}

	c.Use(oi)

	oi = oi.Add(len(transaction.MsgTx.TxOut))
	return balances.Set(loc, oi, c)
}

func ProcessTransaction(transaction *Transaction, balances bitrie.Bitrie, c comp.C) bitrie.Bitrie {
	return c.Call(ProcessTransactionImpl, transaction, balances)[0].(bitrie.Bitrie)
}

func ProcessBlock(block *Block, balances bitrie.Bitrie, c comp.C) bitrie.Bitrie {
	c.Use(block)

	for _, transaction := range block.Transactions {
		balances = ProcessTransaction(transaction.(*Transaction), balances, c)
	}

	return balances
}

func CalculateBalancesImpl(block *Block, c comp.C) bitrie.Bitrie {
	if block == nil {
		return bitrie.Nil
	}

	c.Use(block)
	balances := CalculateBalances(block.Previous, c)
	balances = ProcessBlock(block, balances, c)

	return balances
}

func CalculateBalances(block *Block, c comp.C) bitrie.Bitrie {
	return c.Call(CalculateBalancesImpl, block)[0].(bitrie.Bitrie)
}

type PagingC struct {
	DB                                 *DB
	LoadDiskTime, LoadTime, UnloadTime time.Duration
	Head, Tail                         *ads.Info
	Count                              int
	Loads, Unloads                     int64
}

func NewPagingC(db *DB) *PagingC {
	head, tail := &ads.Info{}, &ads.Info{}
	head.Next = tail
	tail.Prev = head
	return &PagingC{
		DB:   db,
		Head: head,
		Tail: tail,
	}
}

func (c *PagingC) remove(info *ads.Info) {
	a, b := info.Prev, info.Next

	a.Next = b
	b.Prev = a

	info.Next = nil
	info.Prev = nil

	c.Count--
}

func (c *PagingC) MarkUsed(value ads.ADS, include bool) {
	if value.IsOpaque() {
		return
	}

	info := ads.GetInfo(value)

	if info.Next != nil {
		a, b := info.Prev, info.Next
		a.Next = b
		b.Prev = a
	} else {
		if !include {
			return
		} else {
			for _, child := range ads.CollectChildren(value) {
				c.MarkUsed(child, true)
			}
		}
		c.Count++
	}

	a, b := c.Tail.Prev, c.Tail
	a.Next = info
	info.Prev = a
	info.Next = b
	b.Prev = info
}

func (c *PagingC) Use(values ...ads.ADS) {
	for _, value := range values {
		comp.Uses++

		if value.IsOpaque() {
			c.Loads++

			info := ads.GetInfo(value)
			c.Load(info)
		}

		c.MarkUsed(value, false)
	}
}

func (c *PagingC) Call(f interface{}, args ...interface{}) []interface{} {
	return comp.Call(f, append(args, c))
}

func (c *PagingC) Load(info *ads.Info) {
	begin := time.Now()

	data := c.DB.Read(info.Token)

	c.LoadDiskTime += time.Now().Sub(begin)

	begin = time.Now()
	decoder := ads.Decoder{
		Reader: bytes.NewBuffer(data),
	}

	decoder.Decode(&info.Value)
	info.Value.MakeTransparent()

	for _, root := range ads.CollectChildren(info.Value) {
		var buffer [8]byte
		if n, err := decoder.Read(buffer[:]); n != 8 || err != nil {
			log.Panic(err)
		}

		info := root.GetInfo()
		info.Token = int64(binary.LittleEndian.Uint64(buffer[:]))
	}

	c.LoadTime += time.Now().Sub(begin)
}

func (c *PagingC) Store(info *ads.Info) int64 {
	if info.Token != 0 {
		return info.Token
	}

	buffer := ads.GetFromPool()
	defer ads.ReturnToPool(buffer)

	ads.Hash(info.Value)

	buffer.Reset()
	e := ads.Encoder{
		Writer:      buffer,
		Transparent: map[ads.ADS]bool{info.Value: true},
	}
	e.Encode(&info.Value)

	for _, root := range ads.CollectChildren(info.Value) {
		info := ads.GetInfo(root)
		c.Store(info)

		var buffer [8]byte
		binary.LittleEndian.PutUint64(buffer[:], uint64(info.Token))
		if n, err := e.Write(buffer[:]); n != 8 || err != nil {
			log.Panic(err)
		}
	}

	info.Token = c.DB.Write(buffer.Bytes())
	return info.Token
}

const workingSet = 1000 * 1000

func (c *PagingC) Unload() {
	begin := time.Now()
	for c.Count > workingSet {
		info := c.Head.Next
		c.Store(info)

		c.Unloads++
		ads.MakeOpaque(info.Value)

		c.remove(info)
	}
	c.UnloadTime += time.Now().Sub(begin)
}

func Dump(prefix bitrie.Bits, balances bitrie.Bitrie, c comp.C) {
	if leaf, ok := balances.(*bitrie.BitrieLeaf); ok {
		c.Use(leaf)
		c.Use(leaf.Value)
		fmt.Printf("%v: %v\n", hex.EncodeToString(prefix.Cat(leaf.Bits).Bits), leaf.Value.(*OutpointInfo).Count)
	}

	if node, ok := balances.(*bitrie.BitrieNode); ok {
		c.Use(node)
		Dump(prefix.Cat(node.Bits).Append(0), node.Left, c)
		Dump(prefix.Cat(node.Bits).Append(1), node.Right, c)
	}
}

func RandomKey(prefix bitrie.Bits, balances bitrie.Bitrie, c comp.C) bitrie.Bits {
	if leaf, ok := balances.(*bitrie.BitrieLeaf); ok {
		c.Use(leaf)
		return prefix.Cat(leaf.Bits)
	}

	if node, ok := balances.(*bitrie.BitrieNode); ok {
		c.Use(node)
		if rand.Intn(2) == 0 {
			return RandomKey(prefix.Cat(node.Bits).Append(0), node.Left, c)
		} else {
			return RandomKey(prefix.Cat(node.Bits).Append(1), node.Right, c)
		}
	}

	panic(balances)
}

func RegisterTypes() {
	ads.RegisterType(0, &bitrie.BitrieLeaf{})
	ads.RegisterType(1, &bitrie.BitrieNode{})
	ads.RegisterType(2, &bitrie.BitrieNil{})
	ads.RegisterType(3, &Transaction{})
	ads.RegisterType(4, &bitrie.Tuple{})
	ads.RegisterType(5, &seqhash.Hash{})
	ads.RegisterType(6, &Block{})
	ads.RegisterType(7, &OutpointInfo{})
	ads.RegisterType(8, &verified.LogEntry{})
	ads.RegisterType(9, &verified.LogTreeNode{})
	ads.RegisterType(10, &verified.LogTreap{})
	ads.RegisterType(11, btcwire.OutPoint{})

	ads.RegisterFunc(0, ProcessBlock)
	ads.RegisterFunc(1, ProcessTransactionImpl)
	ads.RegisterFunc(2, CalculateBalancesImpl)
	ads.RegisterFunc(3, ProcessOutpointImpl)
}
