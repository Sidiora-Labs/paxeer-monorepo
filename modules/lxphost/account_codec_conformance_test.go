//go:build cgo

package lxphost

import (
	"crypto/ed25519"
	"encoding/binary"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
)

func codecAccountConsent(t *testing.T, activity Activity) []byte {
	t.Helper()
	payloadHash, err := codec.DomainHash(codec.DomainPayloadHash, activity.Payload)
	if err != nil {
		t.Fatal(err)
	}
	encoded := binary.BigEndian.AppendUint16(nil, activity.ProtocolVersion)
	encoded = binary.BigEndian.AppendUint16(encoded, 0x1001)
	encoded = append(encoded, 12)
	field := func(tag byte, value []byte) {
		encoded = append(encoded, tag)
		encoded = append(encoded, value...)
	}
	bytesField := func(tag byte, value []byte) {
		field(tag, binary.BigEndian.AppendUint32(nil, uint32(len(value))))
		encoded = append(encoded, value...)
	}
	field(1, binary.BigEndian.AppendUint16(nil, activity.ProtocolVersion))
	field(2, binary.BigEndian.AppendUint32(nil, testNetworkID))
	field(3, binary.BigEndian.AppendUint32(nil, activity.Type))
	bytesField(4, activity.ActorDID)
	bytesField(5, activity.AuthorityKey[:])
	field(6, binary.BigEndian.AppendUint64(nil, activity.AccountSequence))
	field(7, binary.BigEndian.AppendUint64(nil, activity.NotBefore))
	encoded = binary.BigEndian.AppendUint64(encoded, activity.NotAfter)
	bytesField(8, activity.IdempotencyKey[:])
	field(9, activity.FeeLimit[:])
	bytesField(10, payloadHash[:])
	bytesField(11, activity.Payload)
	bytesField(12, activity.Signature)
	return encoded
}

func codecAccountHostRows(t *testing.T) (map[[32]byte]accountRow, [2]testActor, testAsset) {
	t.Helper()
	const blockTime = int64(1700000000000)
	chain := newTestChain(t)
	host := chain.host(t)
	actors := [2]testActor{newTestActor(t, 91), newTestActor(t, 92)}
	sponsor := newTestActor(t, 93)
	seedProgramsFeeRecord(t, chain, sponsor.id)
	sequence := uint64(41)
	run := func(actor testActor, activity Activity) {
		t.Helper()
		receipt, err := host.Execute(chain.context(1, blockTime, sequence), activity, actor.authority())
		if err != nil {
			t.Fatalf("execute type %#x at sequence %d: %v", activity.Type, sequence, err)
		}
		if receipt.ResultCode != Code(0) {
			t.Fatalf("execute type %#x at sequence %d: %v", activity.Type, sequence, receipt.ResultCode)
		}
		sequence++
	}
	run(sponsor, sponsor.register(t, host, 0, blockTime))
	occupancyAsset := host.ProgramsOccupancyAsset()
	if occupancyAsset == [32]byte{} {
		t.Fatal("onboarding requires a nonzero occupancy asset")
	}
	for i, actor := range actors {
		_, mainID, err := MainAccountID(actor.public)
		if err != nil {
			t.Fatal(err)
		}
		wrapper := sponsor.activity(t, host, registerType, uint64(i+1), blockTime, nil)
		payload := append([]byte{0x71, 1, 3, 5}, sponsor.id[:]...)
		payload = append(payload, occupancyAsset[:]...)
		payload = append(payload, mainID[:]...)
		payload = append(payload, wrapper.IdempotencyKey[:]...)
		payload = binary.BigEndian.AppendUint64(payload, wrapper.NotAfter)
		consent := actor.activity(t, host, registerType, 0, blockTime, payload)
		consent.IdempotencyKey = wrapper.IdempotencyKey
		hash, err := host.SigningHash(consent)
		if err != nil {
			t.Fatal(err)
		}
		consent.Signature = ed25519.Sign(actor.private, hash[:])
		encoded := codecAccountConsent(t, consent)
		wrapperPayload := binary.BigEndian.AppendUint32([]byte{0x71, 1, 2, 1}, uint32(len(encoded)))
		run(sponsor, sponsor.activity(t, host, registerType, uint64(i+1), blockTime, append(wrapperPayload, encoded...)))
	}
	asset := newTestAsset(t, actors[0], 0x63)
	run(actors[0], actors[0].registerAsset(t, host, 0, blockTime, asset, 0x63, 1000))
	for i, actor := range actors {
		accountSequence := uint64(0)
		if i == 0 {
			accountSequence = 1
		}
		run(actor, actor.openAssetAccount(t, host, accountSequence, blockTime, asset))
		_, accountID, err := AssetAccountID(actor.public, asset.id)
		if err != nil {
			t.Fatal(err)
		}
		target := asset
		target.account = accountID
		run(actors[0], actors[0].mint(t, host, uint64(2+i), blockTime, target, uint64(100*(i+1))))
	}
	chain.commit()
	rows := accountRows(t, chain)
	if len(rows) != 5 {
		t.Fatalf("got %d account rows, want two main, two asset and one issuance account", len(rows))
	}
	if rows[asset.issuance].balance != 700 {
		t.Fatalf("issuance balance = %d, want 700 after both mints", rows[asset.issuance].balance)
	}
	for i, actor := range actors {
		_, id, err := AssetAccountID(actor.public, asset.id)
		if err != nil {
			t.Fatal(err)
		}
		if row, ok := rows[id]; !ok || row.balance != uint64(100*(i+1)) {
			t.Fatalf("asset account %x: present=%t balance=%d, want %d", id, ok, row.balance, 100*(i+1))
		}
	}
	return rows, actors, asset
}

func TestCodecAccountIDsMatchHostRows(t *testing.T) {
	rows, _, _ := codecAccountHostRows(t)
	for id, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			derived, err := codec.DeriveAccountID([]byte(row.name))
			if err != nil {
				t.Fatalf("derive host account ID: %v", err)
			}
			if derived != id {
				t.Fatalf("codec ID = %x, host row key ID = %x", derived, id)
			}
		})
	}
}

func TestCodecAccountKindOfHostNames(t *testing.T) {
	rows, actors, asset := codecAccountHostRows(t)
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			kind, ok := codec.AccountKind([]byte(row.name))
			if !ok {
				t.Fatal("codec rejected kernel-written account name")
			}
			if kind != row.kind {
				t.Fatalf("codec kind = %d, host kind = %d", kind, row.kind)
			}
		})
	}
	checkName := func(name string, id [32]byte, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		row, ok := rows[id]
		if !ok || row.name != name {
			t.Fatalf("accountmap name %q ID %x: present=%t row name=%q", name, id, ok, row.name)
		}
	}
	for _, actor := range actors {
		checkName(MainAccountID(actor.public))
		checkName(AssetAccountID(actor.public, asset.id))
	}
	checkName(IssuanceAccountID(asset.id))
}
