package types

import "encoding/binary"

const (
	ModuleName   = "layerxchains"
	StoreKey     = ModuleName
	RouterKey    = ModuleName
	QuerierRoute = ModuleName
)

var (
	ParamsKey               = []byte{0x01}
	DomainKeyPrefix         = []byte{0x02}
	NameIndexPrefix         = []byte{0x03}
	NextIDKey               = []byte{0x04}
	DomainTxPrefix          = []byte{0x06}
	DomainGasUsedPrefix     = []byte{0x07}
	DomainNextBaseFeePrefix = []byte{0x08}
)

func DomainKey(id uint32) []byte { return IDKey(DomainKeyPrefix, id) }
func NameIndexKey(name string) []byte {
	return append(append([]byte{}, NameIndexPrefix...), []byte(name)...)
}
func IDKey(prefix []byte, id uint32) []byte {
	key := make([]byte, len(prefix)+4)
	copy(key, prefix)
	binary.BigEndian.PutUint32(key[len(prefix):], id)
	return key
}
func HeightKey(prefix []byte, id uint32, height uint64) []byte {
	key := append(IDKey(prefix, id), make([]byte, 8)...)
	binary.BigEndian.PutUint64(key[len(prefix)+4:], height)
	return key
}
