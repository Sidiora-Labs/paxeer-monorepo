package lxwire_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/lxwire"
	policylx "github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/policy/lx"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
)

type repair241Vector struct {
	name   string
	kind   uint8
	direct bool
}

func repair241Vectors() []repair241Vector {
	id := strings.Repeat("0123456789abcdef", 4)
	vectors := []repair241Vector{
		{"agent:did:layerx:alice:main", 1, false},
		{"agent:did:layerx:alice:budget:b1", 2, false},
		{"agent:did:layerx:alice:escrow:e1", 3, false},
		{"agent:did:layerx:alice:stream:s1", 4, false},
		{"agent:did:layerx:alice:margin:m1", 5, false},
		{"agent:did:layerx:alice:asset:" + id, 14, false},
		{"system:liquidity:btc-usd", 6, false},
		{"system:funding:btc-usd:long", 7, false},
		{"system:funding:btc-usd:short", 8, false},
		{"system:insurance", 9, false},
		{"system:fees", 10, false},
		{"system:paxeer-reserve", 11, false},
		{"system:paxeer-withdrawals", 12, false},
		{"module:asset:value:" + id, 13, true},
		{"module:" + strings.Repeat("a", 31) + ":value:" + id, 13, true},
		{"module:perps-2:value:" + strings.Repeat("0", 64), 13, true},
		{"asset:" + id + ":issuance", 13, false},
		{"agent:did:layerx:alice:stream:nested:stream:s1", 4, false},
		{"agent:did:layerx:alice:budget:nested:budget:b1", 2, false},
		{"agent:did:layerx:alice:escrow:nested:escrow:e1", 3, false},
		{"agent:did:layerx:alice:margin:nested:margin:m1", 5, false},
		{"agent:did:asset:x:asset:" + id, 14, false},
		{"agent:" + strings.Repeat("a", 256) + ":main", 1, false},
		{"agent:" + strings.Repeat("a", 501) + ":main", 1, false},
	}
	for _, asset := range []string{"pax", "sid", "usdc", "usdl"} {
		vectors = append(vectors, repair241Vector{"system:paxeer-reserve:" + asset, 11, false})
	}
	for _, name := range []string{
		"", ":system:fees", "system:fees:", "system::fees", "system:Fees", "system:fees\x00", "system:other",
		"agent::main", "agent:alice:stream:", "agent:alice:stream:s1:extra", "agent:alice:unknown:s1",
		"agent:alice:asset:" + id[:63], "agent:alice:asset:" + id + "0", "agent:alice:asset:" + strings.ToUpper(id),
		"agent:" + strings.Repeat("a", 502) + ":main", "agent:álîce:main",
		"system:liquidity:", "system:liquidity:a:b", "system:funding::long", "system:funding:a:b:long",
		"system:funding:btc:LONG", "system:funding:btc:long:x", "system:funding:btc:sideways",
		"system:paxeer-reserve:other", "system:paxeer-reserve:pax:extra",
		"module::value:" + id, "module:a_b:value:" + id, "module:a.b:value:" + id,
		"module:a:b:value:" + id, "module:" + strings.Repeat("a", 32) + ":value:" + id,
		"module:asset:value:" + id[:63], "module:asset:value:" + id + "0", "module:asset:value:" + strings.ToUpper(id),
		"module:asset:values:" + id, "asset:" + strings.ToUpper(id) + ":issuance", "asset:" + id + ":issuance:x",
	} {
		vectors = append(vectors, repair241Vector{name: name})
	}
	return vectors
}

// The adapter only frames calls to the actual chain parser and identifier producer.
const repair241NativeAdapter = `
#include "layerx/lxp_ledger.h"
#include <stdio.h>
int main(void) {
    uint8_t size[4], name[1024], result[34];
    while (fread(size, 1U, 4U, stdin) == 4U) {
        size_t n = ((size_t)size[0] << 24U) | ((size_t)size[1] << 16U) |
                   ((size_t)size[2] << 8U) | size[3];
        lx_account_name parsed;
        if (n > sizeof(name) || fread(name, 1U, n, stdin) != n) return 2;
        for (size_t i = 0; i < sizeof(result); ++i) result[i] = 0;
        if (lx_account_name_parse(name, n, &parsed) == LXP_OK) {
            result[0] = 1;
            result[1] = (uint8_t)parsed.kind;
            if (lx_account_id_from_string(name, n, result + 2) != LXP_OK) return 3;
        } else if (lx_account_id_from_string(name, n, result + 2) == LXP_OK) return 4;
        if (fwrite(result, 1U, sizeof(result), stdout) != sizeof(result)) return 5;
    }
    return ferror(stdin) ? 6 : 0;
}
`

func repair241NativeResults(t *testing.T, vectors []repair241Vector) []byte {
	t.Helper()
	root, err := filepath.Abs("../../../../..")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	source, executable := filepath.Join(dir, "accounts.c"), filepath.Join(dir, "accounts")
	if err := os.WriteFile(source, []byte(repair241NativeAdapter), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("cc", "-std=c11", "-Wall", "-Wextra", "-Werror", "-ffunction-sections", "-fdata-sections", "-Wl,--gc-sections",
		"-I"+filepath.Join(root, "include"), source, filepath.Join(root, "src/ledger/lx_account_id.c"),
		filepath.Join(root, "src/crypto/lxp_hash.c"), filepath.Join(root, "src/crypto/lxp_ct.c"), "-o", executable)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("native producer compile: %v\n%s", err, output)
	}
	var input []byte
	for _, v := range vectors {
		input = binary.BigEndian.AppendUint32(input, uint32(len(v.name)))
		input = append(input, v.name...)
	}
	cmd = exec.Command(executable)
	cmd.Stdin = bytes.NewReader(input)
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("native producer: %v", err)
	}
	if len(output) != len(vectors)*34 {
		t.Fatalf("native producer emitted %d bytes for %d vectors", len(output), len(vectors))
	}
	return output
}

func repair241ExpectedID(t *testing.T, v repair241Vector) [32]byte {
	t.Helper()
	var id [32]byte
	if v.direct {
		if _, err := hex.Decode(id[:], []byte(v.name[len(v.name)-64:])); err != nil {
			t.Fatal(err)
		}
		return id
	}
	preimage := append([]byte("LX:ACCOUNT:v1"), binary.BigEndian.AppendUint32(nil, uint32(len(v.name)))...)
	return sha256.Sum256(append(preimage, v.name...))
}

func TestRepair_241_AccountNames(t *testing.T) {
	vectors := repair241Vectors()
	native := repair241NativeResults(t, vectors)
	for i, v := range vectors {
		t.Run(v.name, func(t *testing.T) {
			result := native[i*34 : (i+1)*34]
			kind, ok := codec.AccountKind([]byte(v.name))
			chainID, chainErr := codec.DeriveAccountID([]byte(v.name))
			wireID, wireErr := lxwire.AccountID([]byte(v.name))
			if v.kind == 0 {
				if result[0] != 0 || ok || kind != 0 || !errors.Is(chainErr, codec.ErrAccountIdentity) || !errors.Is(wireErr, lxwire.ErrAccountName) || chainID != ([32]byte{}) || wireID != ([32]byte{}) {
					t.Fatalf("invalid name accepted: native=%x codec=%d/%v/%v wire=%x/%v", result, kind, ok, chainErr, wireID, wireErr)
				}
				return
			}
			want := repair241ExpectedID(t, v)
			if result[0] != 1 || result[1] != v.kind || !bytes.Equal(result[2:], want[:]) || !ok || kind != v.kind || chainErr != nil || wireErr != nil || chainID != want || wireID != want {
				t.Fatalf("identifier disagreement: want=%d/%x native=%x codec=%d/%x/%v wire=%x/%v", v.kind, want, result, kind, chainID, chainErr, wireID, wireErr)
			}
		})
	}
}

func TestRepair_241_ProducerTransportPolicy(t *testing.T) {
	vectors := repair241Vectors()
	native := repair241NativeResults(t, vectors)
	key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{7}, ed25519.SeedSize))
	var publicKey [32]byte
	copy(publicKey[:], key.Public().(ed25519.PublicKey))
	from, err := codec.DeriveAccountID([]byte(lxwire.MainAccountName(lxwire.DIDFromKey(publicKey))))
	if err != nil {
		t.Fatal(err)
	}
	registry, err := lxwire.NewRegistry(policylx.OpAssetTransfer)
	if err != nil {
		t.Fatal(err)
	}
	for i, v := range vectors {
		if v.kind == 0 {
			continue
		}
		t.Run(v.name, func(t *testing.T) {
			var to [32]byte
			copy(to[:], native[i*34+2:(i+1)*34])
			send := &lxwire.Send{From: from, To: to, Amount: lxwire.Uint128{Lo: 17}, SourceSequence: 3,
				IdempotencyKey: [32]byte{1}, ExpiresAt: 2000, AuthorizationKind: lxwire.OwnerAuthorization,
				Controller: from, PublicKey: publicKey, NetworkID: 125, ProtocolVersion: lxwire.StateCommitmentProtocolVersion}
			digest, err := send.AuthorizationDigest()
			if err != nil {
				t.Fatal(err)
			}
			copy(send.Signature[:], ed25519.Sign(key, digest[:]))
			payload, err := send.Encode()
			if err != nil {
				t.Fatal(err)
			}
			activity := &lxwire.Activity{ProtocolVersion: send.ProtocolVersion, NetworkID: send.NetworkID,
				Type: policylx.OpAssetTransfer, ActorDID: []byte(lxwire.DIDFromKey(publicKey)), Authority: publicKey[:],
				AccountSequence: 3, NotBefore: 1000, NotAfter: 2000, IdempotencyKey: send.IdempotencyKey,
				Payload: payload, PayloadHash: lxwire.PayloadHash(payload)}
			envelope, err := lxwire.EncodeUnsignedActivity(activity)
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := lxwire.DecodeUnsignedActivity(envelope, registry)
			if err != nil {
				t.Fatal(err)
			}
			effect, err := policylx.DecodeEffect(decoded)
			if err != nil {
				t.Fatal(err)
			}
			recipient, err := lxwire.AccountID([]byte(v.name))
			if err != nil {
				t.Fatal(err)
			}
			disclosure := policylx.Disclosure{Account: policylx.ID(from), Module: "asset", Operation: 5,
				Amounts: []policylx.Amount{{Amount: big.NewInt(17)}}, Destinations: []policylx.ID{recipient},
				Sequence: 3, NotBefore: 1000, NotAfter: 2000}
			raw, err := json.Marshal(disclosure)
			if err != nil {
				t.Fatal(err)
			}
			var transported policylx.Disclosure
			if err := json.Unmarshal(raw, &transported); err != nil {
				t.Fatal(err)
			}
			if len(effect.Destinations) != 1 || effect.Destinations[0] != policylx.ID(to) || effect.Legs[0].To != policylx.ID(to) {
				t.Fatalf("policy lost native recipient: %+v", effect)
			}
			if err := policylx.MatchDisclosure(decoded, effect, &transported); err != nil {
				t.Fatalf("canonical recipient refused: %v", err)
			}
			transported.Destinations[0][0] ^= 1
			if err := policylx.MatchDisclosure(decoded, effect, &transported); err == nil {
				t.Fatal("policy accepted changed identifier bytes")
			}
			decoded.Payload[len(decoded.Payload)-40] ^= 1
			if _, err := policylx.DecodeEffect(decoded); err == nil {
				t.Fatal("policy accepted tampered signed send")
			}
		})
	}
}
