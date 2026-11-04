package keyring

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	tmcrypto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/input"
	"github.com/tendermint/crypto/bcrypt"
)

const (
	keyhashVersion = 1
	// Match the default armored-export cost; never accept unbounded backup work.
	keyhashCost    = 12
	maxKeyhashCost = 16
)

type passphraseVerifier struct {
	Version int    `json:"version"`
	KDF     string `json:"kdf"`
	Hash    []byte `json:"hash"`
}

func decodeKeyhash(data []byte) ([]byte, int, bool, error) {
	hash, legacy := data, true
	if len(data) != 0 && data[0] == '{' {
		var verifier passphraseVerifier
		if err := json.Unmarshal(data, &verifier); err != nil || verifier.Version != keyhashVersion || verifier.KDF != "bcrypt" {
			return nil, 0, false, fmt.Errorf("unsupported keyring verifier format")
		}
		hash, legacy = verifier.Hash, false
	}
	cost, err := bcrypt.Cost(hash)
	if err != nil || cost < bcrypt.MinCost || cost > maxKeyhashCost {
		return nil, 0, false, fmt.Errorf("invalid keyring verifier cost (allowed %d-%d)", bcrypt.MinCost, maxKeyhashCost)
	}
	return hash, cost, legacy, nil
}

// Rename only after a complete synced write, retaining the old verifier on error.
func storeKeyhash(path, pass string, cost int) error {
	hash, err := bcrypt.GenerateFromPassword(tmcrypto.CRandBytes(16), []byte(pass), cost)
	if err != nil {
		return fmt.Errorf("failed to derive keyring verifier")
	}
	data, err := json.Marshal(passphraseVerifier{Version: keyhashVersion, KDF: "bcrypt", Hash: hash})
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".keyhash-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(f.Name(), path); err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func newRealPrompt(dir string, buf io.Reader) func(string) (string, error) {
	reader := bufio.NewReader(buf)
	return func(_ string) (string, error) {
		path := filepath.Join(dir, "keyhash")
		data, err := os.ReadFile(path)
		stored := err == nil
		if err != nil && !os.IsNotExist(err) {
			return "", fmt.Errorf("failed to read keyring verifier: %w", err)
		}
		var hash []byte
		var cost int
		var legacy bool
		if stored {
			hash, cost, legacy, err = decodeKeyhash(data)
			if err != nil {
				return "", err
			}
		}
		for attempt := 0; attempt < maxPassphraseEntryAttempts; attempt++ {
			pass, err := input.GetPassword("Enter keyring passphrase:", reader)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				continue
			}
			if stored {
				if bcrypt.CompareHashAndPassword(hash, []byte(pass)) != nil {
					fmt.Fprintln(os.Stderr, "incorrect passphrase")
					continue
				}
				if legacy || cost < keyhashCost {
					if cost < keyhashCost {
						cost = keyhashCost
					}
					if err := storeKeyhash(path, pass, cost); err != nil {
						return "", fmt.Errorf("failed to migrate keyring verifier: %w", err)
					}
				}
				return pass, nil
			}
			confirm, err := input.GetPassword("Re-enter keyring passphrase:", reader)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				continue
			}
			if pass != confirm {
				fmt.Fprintln(os.Stderr, "passphrase do not match")
				continue
			}
			if err := storeKeyhash(path, pass, keyhashCost); err != nil {
				return "", err
			}
			return pass, nil
		}
		return "", fmt.Errorf("too many failed passphrase attempts")
	}
}
