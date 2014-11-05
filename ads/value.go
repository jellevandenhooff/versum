package ads

import (
	"certcomp/sha"
	"fmt"
)

// authenticated data structure

type ADS interface {
	AssertTransparent()
	IsOpaque() bool
	CachedHash() *sha.Hash
	SetCachedHash(sha.Hash)
	MakeOpaque()
	MakeTransparent()

	GetInfo() *Info
}

type Info struct {
	Prev, Next *Info
	Value      ADS
	Token      int64
}

func (i *Info) String() string {
	return fmt.Sprintf("Token: %d", i.Token)
}

// For convenient struct initializaton, the default Base is marked as
// having data but no cached hash.
type Base struct {
	cachedHash    sha.Hash
	hasCachedHash bool
	isOpaque      bool
	info          Info
}

func (bv *Base) AssertTransparent() {
	if bv.isOpaque {
		panic(bv)
	}
}

func (bv *Base) IsOpaque() bool {
	return bv.isOpaque
}

func (bv *Base) MakeTransparent() {
	bv.isOpaque = false
}

func (bv *Base) SetCachedHash(hash sha.Hash) {
	bv.cachedHash = hash
	bv.hasCachedHash = true
}

func (bv *Base) CachedHash() *sha.Hash {
	if bv.hasCachedHash {
		return &bv.cachedHash
	} else {
		return nil
	}
}

func (bv *Base) MakeOpaque() {
	bv.isOpaque = true
}

func (bv *Base) GetInfo() *Info {
	return &bv.info
}

func GetInfo(ads ADS) *Info {
	info := ads.GetInfo()
	info.Value = ads
	return info
}
