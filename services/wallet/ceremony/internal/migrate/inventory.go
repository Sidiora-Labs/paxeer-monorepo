package migrate

import (
	"context"
	"crypto/ecdsa"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ethereum/go-ethereum/crypto"

	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/ceremony/internal/attestor"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/ceremony/internal/envelope"
)

type InventoryRequest struct {
	KeyID      string   `json:"key_id"`
	Epoch      uint64   `json:"epoch"`
	Curve      string   `json:"curve"`
	PublicKey  string   `json:"public_key"`
	Owner      string   `json:"owner"`
	Operations []string `json:"operations"`
}

type InventoryRequests struct {
	Requests []InventoryRequest `json:"inventory_requests"`
}

func (p *Plan) WriteInventoryRequests(ctx context.Context, db *sql.DB, masterKey []byte, path string) (int, error) {
	if !filepath.IsAbs(path) {
		return 0, errors.New("migrate: inventory requests path must be absolute")
	}
	if p == nil {
		return 0, errors.New("migrate: inventory requests require a plan")
	}
	document := InventoryRequests{Requests: make([]InventoryRequest, 0, 2*len(p.Deliverable))}
	for _, w := range p.Deliverable {
		requests, err := walletInventoryRequests(ctx, db, masterKey, w)
		if err != nil {
			return 0, &WalletError{WalletID: w.ID, Stage: "inventory requests", Err: err}
		}
		document.Requests = append(document.Requests, requests...)
	}
	raw, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return 0, err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".ceremony-inventory-*")
	if err != nil {
		return 0, fmt.Errorf("migrate: create inventory requests: %w", err)
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(append(raw, '\n'))
	}
	if err == nil {
		err = f.Sync()
	}
	err = errors.Join(err, f.Close())
	if err != nil {
		return 0, fmt.Errorf("migrate: write inventory requests: %w", err)
	}
	if err = os.Rename(f.Name(), path); err != nil {
		return 0, fmt.Errorf("migrate: publish inventory requests: %w", err)
	}
	return len(document.Requests), nil
}

func walletInventoryRequests(ctx context.Context, db *sql.DB, masterKey []byte, w Wallet) ([]InventoryRequest, error) {
	original, err := loadOriginalIdentity(ctx, db, w)
	if err != nil {
		return nil, err
	}
	if original.External {
		return nil, ErrExternalIdentity
	}
	key, err := envelope.OpenEnvelope(w.EncryptedPrivateKey, masterKey, w.Address)
	if err != nil {
		return nil, err
	}
	scalar, err := key.Scalar()
	key.Zero()
	if err != nil {
		return nil, err
	}
	defer wipeInt(scalar)
	secret := scalar.FillBytes(make([]byte, envelope.PrivateKeyLen))
	defer zero(secret)
	x, y := crypto.S256().ScalarBaseMult(secret)
	public := hex.EncodeToString(crypto.FromECDSAPub(&ecdsa.PublicKey{Curve: crypto.S256(), X: x, Y: y}))
	return []InventoryRequest{
		{KeyID: SecpKeyID(w.ID), Epoch: 0, Curve: attestor.CurveSecp256k1, PublicKey: public, Owner: w.UserID, Operations: []string{"import", "refresh", "sign"}},
		{KeyID: original.KeyID, Epoch: 0, Curve: attestor.CurveEd25519, PublicKey: original.PublicKey, Owner: original.Owner, Operations: []string{"sign", "refresh"}},
	}, nil
}
