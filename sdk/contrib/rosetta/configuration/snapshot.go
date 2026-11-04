package configuration

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/bech32"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"google.golang.org/protobuf/encoding/protowire"
)

const maxArchiveSize = 64 << 20

func readArchive(file string) ([]byte, error) {
	r, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	b, err := io.ReadAll(io.LimitReader(r, maxArchiveSize+1))
	if err != nil {
		return nil, err
	}
	if len(b) > maxArchiveSize {
		return nil, errors.New("fixture archive is too large")
	}
	return b, nil
}

func checkArchive(b []byte, f Fixture) error {
	sum := sha256.Sum256(b)
	if hex.EncodeToString(sum[:]) != f.ArchiveSHA256 {
		return errors.New("archive does not match explicit fixture identity")
	}
	return nil
}

func unpackArchive(b []byte, dest string) error {
	gz, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	total := int64(0)
	const prefix = "root/.simapp/"
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		name := strings.TrimSuffix(h.Name, "/")
		if name == "root" || name == "root/.simapp" {
			if h.Typeflag != tar.TypeDir {
				return errors.New("invalid archive root")
			}
			continue
		}
		if !strings.HasPrefix(h.Name, prefix) || path.Clean(h.Name) != name {
			return errors.New("unsafe archive path")
		}
		rel := strings.TrimPrefix(name, prefix)
		if rel == "" || strings.HasPrefix(rel, "../") || path.IsAbs(rel) {
			return errors.New("unsafe archive path")
		}
		target := filepath.Join(dest, filepath.FromSlash(rel))
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0700); err != nil {
				return err
			}
		case tar.TypeReg:
			total += h.Size
			if h.Size < 0 || total > maxArchiveSize {
				return errors.New("expanded fixture is too large")
			}
			if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
				return err
			}
			file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
			if err != nil {
				return err
			}
			_, copyErr := io.CopyN(file, tr, h.Size)
			closeErr := file.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		default:
			return errors.New("unsupported archive entry")
		}
	}
}

// SnapshotMetadata reads the latest committed bank root, rather than genesis
// allocations or obsolete IAVL leaves still present in the snapshot's LevelDB.
func SnapshotMetadata(archive []byte, identity Fixture) (Fixture, error) {
	if err := identity.Validate(); err != nil {
		return Fixture{}, err
	}
	if err := checkArchive(archive, identity); err != nil {
		return Fixture{}, err
	}
	dir, err := os.MkdirTemp("", "rosetta-snapshot-")
	if err != nil {
		return Fixture{}, err
	}
	defer os.RemoveAll(dir)
	if err := unpackArchive(archive, dir); err != nil {
		return Fixture{}, err
	}
	return metadataFromHome(dir, identity)
}

func metadataFromHome(home string, identity Fixture) (Fixture, error) {
	var g struct {
		ChainID  string `json:"chain_id"`
		AppState struct {
			Staking struct {
				Params struct {
					BondDenom string `json:"bond_denom"`
				} `json:"params"`
			} `json:"staking"`
			Bank struct {
				Balances []struct {
					Address string `json:"address"`
				} `json:"balances"`
			} `json:"bank"`
		} `json:"app_state"`
	}
	b, err := os.ReadFile(filepath.Join(home, "config/genesis.json"))
	if err != nil {
		return Fixture{}, err
	}
	if err = json.Unmarshal(b, &g); err != nil {
		return Fixture{}, err
	}
	if g.ChainID != identity.ChainID || g.AppState.Staking.Params.BondDenom != identity.Currency.Symbol {
		return Fixture{}, errors.New("genesis disagrees with fixture currency or chain")
	}
	found := false
	for _, balance := range g.AppState.Bank.Balances {
		found = found || balance.Address == identity.Address
	}
	if !found {
		return Fixture{}, errors.New("fixture account absent from genesis")
	}
	db, err := leveldb.OpenFile(filepath.Join(home, "data/application.db"), &opt.Options{ReadOnly: true})
	if err != nil {
		return Fixture{}, err
	}
	defer db.Close()
	latest, err := db.Get([]byte("s/latest"), nil)
	if err != nil {
		return Fixture{}, err
	}
	if len(latest) < 2 || latest[0] != 8 {
		return Fixture{}, errors.New("unsupported snapshot height encoding")
	}
	height, n := protowire.ConsumeVarint(latest[1:])
	if n < 0 || n+1 != len(latest) || height < 1 || height > 1<<63-1 {
		return Fixture{}, errors.New("invalid snapshot height")
	}
	rootKey := append([]byte("s/k:bank/r"), make([]byte, 8)...)
	binary.BigEndian.PutUint64(rootKey[len(rootKey)-8:], height)
	root, err := db.Get(rootKey, nil)
	if err != nil {
		return Fixture{}, err
	}
	_, raw, err := bech32.DecodeAndConvert(identity.Address)
	if err != nil {
		return Fixture{}, err
	}
	bankKey := append([]byte{2, byte(len(raw))}, raw...)
	bankKey = append(bankKey, identity.Currency.Symbol...)
	value, err := iavlLookup(db, root, bankKey)
	if err != nil {
		return Fixture{}, err
	}
	denom, amount, err := decodeCoin(value)
	if err != nil {
		return Fixture{}, err
	}
	if denom != identity.Currency.Symbol || !positiveInteger(amount) {
		return Fixture{}, errors.New("invalid bank coin")
	}
	identity.SnapshotHeight = int64(height)
	identity.Balance = amount
	return identity, nil
}

func readSized(r *bytes.Reader) ([]byte, error) {
	n, err := binary.ReadUvarint(r)
	if err != nil {
		return nil, err
	}
	if n > uint64(r.Len()) {
		return nil, io.ErrUnexpectedEOF
	}
	b := make([]byte, int(n))
	_, err = io.ReadFull(r, b)
	return b, err
}

func iavlLookup(db *leveldb.DB, hash, key []byte) ([]byte, error) {
	for depth := 0; depth < 64; depth++ {
		if len(hash) != 32 {
			return nil, errors.New("invalid IAVL hash")
		}
		b, err := db.Get(append([]byte("s/k:bank/n"), hash...), nil)
		if err != nil {
			return nil, err
		}
		r := bytes.NewReader(b)
		height, err := binary.ReadVarint(r)
		if err != nil || height < 0 || height > 63 {
			return nil, errors.New("invalid IAVL node height")
		}
		size, err := binary.ReadVarint(r)
		if err != nil || size < 1 {
			return nil, errors.New("invalid IAVL node size")
		}
		version, err := binary.ReadVarint(r)
		if err != nil || version < 1 {
			return nil, errors.New("invalid IAVL node version")
		}
		nodeKey, err := readSized(r)
		if err != nil {
			return nil, err
		}
		if height == 0 {
			value, err := readSized(r)
			if err != nil {
				return nil, err
			}
			if r.Len() != 0 {
				return nil, errors.New("trailing IAVL leaf data")
			}
			if !bytes.Equal(key, nodeKey) {
				return nil, errors.New("bank balance absent")
			}
			return value, nil
		}
		left, err := readSized(r)
		if err != nil {
			return nil, err
		}
		right, err := readSized(r)
		if err != nil {
			return nil, err
		}
		if r.Len() != 0 {
			return nil, errors.New("trailing IAVL branch data")
		}
		if bytes.Compare(key, nodeKey) < 0 {
			hash = left
		} else {
			hash = right
		}
	}
	return nil, errors.New("IAVL depth exceeded")
}

func decodeCoin(b []byte) (string, string, error) {
	values := make(map[protowire.Number]string)
	for len(b) > 0 {
		field, typ, n := protowire.ConsumeTag(b)
		if n < 0 || typ != protowire.BytesType || (field != 1 && field != 2) {
			return "", "", errors.New("invalid coin encoding")
		}
		b = b[n:]
		value, n := protowire.ConsumeBytes(b)
		if n < 0 {
			return "", "", errors.New("truncated coin")
		}
		b = b[n:]
		if _, ok := values[field]; ok {
			return "", "", errors.New("duplicate coin field")
		}
		values[field] = string(value)
	}
	if values[1] == "" || values[2] == "" {
		return "", "", errors.New("incomplete coin")
	}
	return values[1], values[2], nil
}

// RestoreFixture reproduces the pinned snapshot without generating a new key or
// erasing an existing node. Both the archive and its derived consumers are emitted.
func RestoreFixture(configDir, archiveFile, home, output string) error {
	identity, err := LoadFixture(configDir)
	if err != nil {
		return err
	}
	archive, err := readArchive(archiveFile)
	if err != nil {
		return err
	}
	f, err := SnapshotMetadata(archive, identity)
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(home)
	if err == nil && len(entries) > 0 {
		return errors.New("refusing to replace nonempty node home")
	}
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(home), 0700); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(filepath.Dir(home), ".rosetta-restore-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	if err := unpackArchive(archive, stage); err != nil {
		return err
	}
	if err := os.Rename(stage, home); err != nil {
		return fmt.Errorf("install snapshot: %w", err)
	}
	if err := os.WriteFile(output, archive, 0600); err != nil {
		return err
	}
	return WriteMetadata(configDir, f)
}
