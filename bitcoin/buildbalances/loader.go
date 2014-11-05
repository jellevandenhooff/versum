// BASED ON https://github.com/conformal/btcd/blob/master/util/addblock/import.go

// Copyright (c) 2013-2014 Conformal Systems LLC.
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"github.com/conformal/btcutil"
	"github.com/conformal/btcwire"
	"io"
)

// blockImporter houses information about an ongoing import from a block data
// file to the block database.
type Loader struct {
	r io.Reader
}

// readBlock reads the next block from the input file.
func (bi *Loader) readBlock() (*btcutil.Block, error) {
	// The block file format is:
	//  <network> <block length> <serialized block>
	var net uint32
	err := binary.Read(bi.r, binary.LittleEndian, &net)
	if err != nil {
		if err != io.EOF {
			return nil, err
		}

		// No block and no error means there are no more blocks to read.
		return nil, nil
	}

	// Read the block length and ensure it is sane.
	var blockLen uint32
	if err := binary.Read(bi.r, binary.LittleEndian, &blockLen); err != nil {
		return nil, err
	}
	if blockLen > btcwire.MaxBlockPayload {
		return nil, fmt.Errorf("block payload of %d bytes is larger "+
			"than the max allowed %d bytes", blockLen,
			btcwire.MaxBlockPayload)
	}

	serializedBlock := make([]byte, blockLen)
	if _, err := io.ReadFull(bi.r, serializedBlock); err != nil {
		return nil, err
	}

	block, err := btcutil.NewBlockFromBytes(serializedBlock)
	if err != nil {
		return nil, err
	}

	return block, nil
}

func NewLoader(r io.Reader) *Loader {
	return &Loader{
		r: bufio.NewReaderSize(r, 1024*1024),
	}
}
