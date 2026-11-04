package types

import (
	"encoding/binary"
	"fmt"
)

// AddressSerializedSize is the on-disk size of a serialized Address in bytes.
// Layout: index(4) | offset(4) | shardID(1) | valueSize(4)
const AddressSerializedSize = 13

// Address describes the location of a value on disk.
//
// An Address identifies the segment (Index), the byte offset of the first referenced byte within
// its shard's value file (Offset), the shard within that segment (ShardID), and the exact number of
// referenced bytes (ValueSize). V3 value files store raw bytes with no per-value length prefix.
// A primary address spans the whole value; a secondary address may span any subrange of those bytes.
type Address struct {
	// index is the segment index that owns the value. Combined with the shardID, it identifies the value file
	// that contains the value's bytes.
	index uint32
	// offset is the byte position of the first referenced byte within the shard's value file.
	offset uint32
	// shardID is the index of the shard within the segment that holds the value. Encoded as a single byte,
	// which caps the maximum sharding factor at 255 (see litt.MaxShardingFactor).
	shardID uint8
	// valueSize is the exact span in bytes starting at offset, with no length prefix.
	valueSize uint32
}

// NewAddress creates a new Address.
func NewAddress(index uint32, offset uint32, shardID uint8, valueSize uint32) Address {
	return Address{
		index:     index,
		offset:    offset,
		shardID:   shardID,
		valueSize: valueSize,
	}
}

// DeserializeAddress converts a byte slice to an Address. The slice must be exactly AddressSerializedSize bytes.
func DeserializeAddress(bytes []byte) (Address, error) {
	if len(bytes) != AddressSerializedSize {
		return Address{}, fmt.Errorf("invalid address length: %d", len(bytes))
	}
	return Address{
		index:     binary.BigEndian.Uint32(bytes[0:4]),
		offset:    binary.BigEndian.Uint32(bytes[4:8]),
		shardID:   bytes[8],
		valueSize: binary.BigEndian.Uint32(bytes[9:13]),
	}, nil
}

// Index returns the segment index of the value.
func (a Address) Index() uint32 {
	return a.index
}

// Offset returns the byte offset of the first referenced byte within its shard's value file.
func (a Address) Offset() uint32 {
	return a.offset
}

// ShardID returns the shard within the segment that owns the value.
func (a Address) ShardID() uint8 {
	return a.shardID
}

// ValueSize returns the exact number of referenced bytes starting at Offset, with no length prefix.
func (a Address) ValueSize() uint32 {
	return a.valueSize
}

// String returns a string representation of the address.
func (a Address) String() string {
	return fmt.Sprintf("(%d:%d@%d, %d)", a.index, a.offset, a.shardID, a.valueSize)
}

// Serialize converts the address to a byte slice of length AddressSerializedSize.
func (a Address) Serialize() []byte {
	bytes := make([]byte, AddressSerializedSize)
	binary.BigEndian.PutUint32(bytes[0:4], a.index)
	binary.BigEndian.PutUint32(bytes[4:8], a.offset)
	bytes[8] = a.shardID
	binary.BigEndian.PutUint32(bytes[9:13], a.valueSize)
	return bytes
}
