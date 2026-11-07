package types

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"testing"
)

func TestEncodeSessionPayload(t *testing.T) {
	var actionKey [32]byte
	successor := &SessionSuccessor{}
	for i := range actionKey {
		actionKey[i] = byte(i + 1)
		successor.PredecessorID[i] = byte(i + 33)
		successor.ExpectedCommitment[i] = byte(i + 65)
	}
	const sequence = uint64(0x0102030405060708)
	for _, tc := range []struct {
		name      string
		successor *SessionSuccessor
		header    []byte
		overhead  int
	}{
		{"grant", nil, []byte{0x71, 5, 1, 3}, 52},
		{"successor", successor, []byte{0x71, 5, 2, 5}, 124},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, size := range []int{1, 257, 1024 - tc.overhead} {
				t.Run(fmt.Sprint(size), func(t *testing.T) {
					grant := make([]byte, size)
					for i := range grant {
						grant[i] = byte(i)
					}
					payload, err := EncodeSessionPayload(grant, sequence, actionKey, tc.successor)
					if err != nil {
						t.Fatal(err)
					}
					if len(payload) != tc.overhead+size || !bytes.Equal(payload[:4], tc.header) {
						t.Fatalf("length/header: got %d/%x", len(payload), payload[:4])
					}
					if err := ValidateSessionPayload(payload); err != nil {
						t.Fatalf("validator rejected encoded payload: %v", err)
					}
					if binary.BigEndian.Uint32(payload[4:8]) != uint32(size) || !bytes.Equal(payload[8:8+size], grant) {
						t.Fatal("grant length or bytes differ")
					}
					if !bytes.Equal(payload[8+size:16+size], []byte{1, 2, 3, 4, 5, 6, 7, 8}) {
						t.Fatal("expiry sequence is not big endian")
					}
					offset := 16 + size
					keys := [][32]byte{actionKey}
					if tc.successor != nil {
						keys = append(keys, successor.PredecessorID, successor.ExpectedCommitment)
					}
					for _, key := range keys {
						if !bytes.Equal(payload[offset:offset+4], []byte{0, 0, 0, 32}) || !bytes.Equal(payload[offset+4:offset+36], key[:]) {
							t.Fatalf("key length or bytes differ at %d", offset)
						}
						offset += 36
					}
					if offset != len(payload) {
						t.Fatal("unexpected trailing bytes")
					}
					grant[0] ^= 0xff
					if payload[8] != 0 {
						t.Fatal("payload aliases grant input")
					}
				})
			}
			for _, size := range []int{0, 1025 - tc.overhead, 1025} {
				payload, err := EncodeSessionPayload(make([]byte, size), sequence, actionKey, tc.successor)
				if !errors.Is(err, ErrNonCanonical) || payload != nil {
					t.Fatalf("grant length %d: got %x, %v", size, payload, err)
				}
			}
		})
	}
}

func TestEncodeCapabilityAndRevokePayload(t *testing.T) {
	t.Run("capability", func(t *testing.T) {
		for _, size := range []int{1, 257, 1016} {
			grant := bytes.Repeat([]byte{0xa5}, size)
			payload, err := EncodeCapabilityPayload(grant)
			if err != nil {
				t.Fatal(err)
			}
			if len(payload) != 8+size || !bytes.Equal(payload[:4], []byte{0x71, 8, 1, 1}) {
				t.Fatalf("length/header: got %d/%x", len(payload), payload[:4])
			}
			if binary.BigEndian.Uint32(payload[4:8]) != uint32(size) || !bytes.Equal(payload[8:], grant) {
				t.Fatal("grant length or bytes differ")
			}
			grant[0] = 0
			if payload[8] != 0xa5 {
				t.Fatal("payload aliases grant input")
			}
		}
		for _, grant := range [][]byte{nil, {}, make([]byte, 1017), make([]byte, 1025)} {
			payload, err := EncodeCapabilityPayload(grant)
			if !errors.Is(err, ErrNonCanonical) || payload != nil {
				t.Fatalf("grant length %d: got %x, %v", len(grant), payload, err)
			}
		}
	})
	t.Run("revoke", func(t *testing.T) {
		var grantID [32]byte
		for i := range grantID {
			grantID[i] = byte(i + 1)
		}
		for reason := uint8(1); reason <= 5; reason++ {
			for _, sequence := range []uint64{0, 0x0102030405060708, ^uint64(0)} {
				payload := EncodeRevokePayload(grantID, reason, sequence)
				if len(payload) != 45 || !bytes.Equal(payload[:4], []byte{0x71, 6, 0, 3}) {
					t.Fatalf("length/header: got %d/%x", len(payload), payload[:4])
				}
				if !bytes.Equal(payload[4:36], grantID[:]) || payload[36] != reason {
					t.Fatal("grant id or reason differs")
				}
				var wantSequence [8]byte
				for i := range wantSequence {
					wantSequence[i] = byte(sequence >> (56 - 8*i))
				}
				if !bytes.Equal(payload[37:], wantSequence[:]) {
					t.Fatal("sequence is not big endian")
				}
				if err := ValidateRevokePayload(payload); err != nil {
					t.Fatalf("reason %d rejected: %v", reason, err)
				}
			}
		}
		for _, reason := range []uint8{0, 6, 255} {
			payload := EncodeRevokePayload(grantID, reason, 1)
			if payload[36] != reason || !errors.Is(ValidateRevokePayload(payload), ErrAuthScope) {
				t.Fatalf("invalid reason %d was changed or accepted", reason)
			}
		}
	})
}
