package sha

import (
	"crypto/sha256"
	"encoding/hex"
)

type Hash [32]byte

const Bits = 256

func Sum(data []byte) Hash {
	hash := Hash(sha256.Sum256(data))
	return hash
}

func (h Hash) Bytes() []byte {
	return h[:]
}

func (h Hash) Bit(idx uint) int {
	byte := int(h[idx/8])
	return (byte >> (idx % 8)) & 1
}

func (h Hash) String() string {
	return hex.EncodeToString(h.Bytes())
}
