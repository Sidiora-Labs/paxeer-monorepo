package keyring

import (
	"encoding/hex"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/99designs/keyring"
	bip39 "github.com/cosmos/go-bip39"
	"github.com/pkg/errors"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/legacy"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/hd"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/sr25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/ledger"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
)

// Backend options for Keyring
const (
	BackendFile    = "file"
	BackendOS      = "os"
	BackendKWallet = "kwallet"
	BackendPass    = "pass"
	BackendTest    = "test"
	BackendMemory  = "memory"
)

const (
	keyringFileDirName = "keyring-file"
	keyringTestDirName = "keyring-test"
	passKeyringPrefix  = "keyring-%s"
)

var (
	_                          Keyring = &keystore{}
	maxPassphraseEntryAttempts         = 3
)

// Keyring exposes operations over a backend supported by github.com/99designs/keyring.
type Keyring interface {
	// List all keys.
	List() ([]Info, error)

	// Supported signing algorithms for Keyring and Ledger respectively.
	SupportedAlgorithms() (SigningAlgoList, SigningAlgoList)

	// Key and KeyByAddress return keys by uid and address respectively.
	Key(uid string) (Info, error)
	KeyByAddress(address sdk.Address) (Info, error)

	// Delete and DeleteByAddress remove keys from the keyring.
	Delete(uid string) error
	DeleteByAddress(address sdk.Address) error

	// NewMnemonic generates a new mnemonic, derives a hierarchical deterministic key from it, and
	// persists the key to storage. Returns the generated mnemonic and the key Info.
	// It returns an error if it fails to generate a key for the given algo type, or if
	// another key is already stored under the same name or address.
	//
	// A passphrase set to the empty string will set the passphrase to the DefaultBIP39Passphrase value.
	NewMnemonic(uid string, language Language, hdPath, bip39Passphrase string, algo SignatureAlgo) (Info, string, error)

	// NewAccount converts a mnemonic to a private key and BIP-39 HD Path and persists it.
	// It fails if a different record exists with the same name or address.
	// An identical retry returns the existing account without replacing it.
	NewAccount(uid, mnemonic, bip39Passphrase, hdPath string, algo SignatureAlgo) (Info, error)

	// SaveLedgerKey retrieves a public key reference from a Ledger device and persists it.
	SaveLedgerKey(uid string, algo SignatureAlgo, hrp string, coinType, account, index uint32) (Info, error)

	// SavePubKey stores a public key and returns the persisted Info structure.
	SavePubKey(uid string, pubkey types.PubKey, algo hd.PubKeyType) (Info, error)

	// SaveMultisig stores and returns a new multsig (offline) key reference.
	SaveMultisig(uid string, pubkey types.PubKey) (Info, error)

	Signer

	Importer
	Exporter
}

// UnsafeKeyring exposes unsafe operations such as unsafe unarmored export in
// addition to those that are made available by the Keyring interface.
type UnsafeKeyring interface {
	Keyring
	UnsafeExporter
}

// Signer is implemented by key stores that want to provide signing capabilities.
type Signer interface {
	// Sign sign byte messages with a user key.
	Sign(uid string, msg []byte) ([]byte, types.PubKey, error)

	// SignByAddress sign byte messages with a user key providing the address.
	SignByAddress(address sdk.Address, msg []byte) ([]byte, types.PubKey, error)
}

// Importer is implemented by key stores that support import of public and private keys.
type Importer interface {
	// ImportPrivKey imports ASCII armored passphrase-encrypted private keys.
	ImportPrivKey(uid, armor, passphrase string) error
}

// Exporter is implemented by key stores that support export of public and private keys.
type Exporter interface {

	// ExportPrivKeyArmor returns a private key in ASCII armored format.
	// It returns an error if the key does not exist or a wrong encryption passphrase is supplied.
	ExportPrivKeyArmor(uid, encryptPassphrase string) (armor string, err error)
	ExportPrivKeyArmorByAddress(address sdk.Address, encryptPassphrase string) (armor string, err error)
}

// UnsafeExporter is implemented by key stores that support unsafe export
// of private keys' material.
type UnsafeExporter interface {
	// UnsafeExportPrivKeyHex returns a private key in unarmored hex format
	UnsafeExportPrivKeyHex(uid string) (string, error)
}

// Option overrides keyring configuration options.
type Option func(options *Options)

// Options define the options of the Keyring.
type Options struct {
	// supported signing algorithms for keyring
	SupportedAlgos SigningAlgoList
	// supported signing algorithms for Ledger
	SupportedAlgosLedger SigningAlgoList
}

// NewInMemory creates a transient keyring useful for testing
// purposes and on-the-fly key generation.
// Keyring options can be applied to the new instance.
func NewInMemory(opts ...Option) Keyring {
	return NewInMemoryWithKeyring(keyring.NewArrayKeyring(nil), opts...)
}

// NewInMemoryWithKeyring returns an in memory keyring using the specified keyring.Keyring
// as the backing keyring.
func NewInMemoryWithKeyring(kr keyring.Keyring, opts ...Option) Keyring {
	return newKeystore(kr, opts...)
}

// New creates a new instance of a keyring.
// Keyring options can be applied when generating the new instance.
// Available backends are "os", "file", "kwallet", "memory", "pass", "test".
func New(
	appName, backend, rootDir string, userInput io.Reader, opts ...Option,
) (Keyring, error) {
	var (
		db  keyring.Keyring
		err error
	)

	switch backend {
	case BackendMemory:
		return NewInMemory(opts...), err
	case BackendTest:
		db, err = keyring.Open(newTestBackendKeyringConfig(appName, rootDir))
	case BackendFile:
		db, err = keyring.Open(newFileBackendKeyringConfig(appName, rootDir, userInput))
	case BackendOS:
		db, err = keyring.Open(newOSBackendKeyringConfig(appName, rootDir, userInput))
	case BackendKWallet:
		db, err = keyring.Open(newKWalletBackendKeyringConfig(appName, rootDir, userInput))
	case BackendPass:
		db, err = keyring.Open(newPassBackendKeyringConfig(appName, rootDir, userInput))
	default:
		return nil, fmt.Errorf("unknown keyring backend %v", backend)
	}

	if err != nil {
		return nil, err
	}

	ks := newKeystore(db, opts...)
	if err := ks.recoverOperation(); err != nil {
		return nil, err
	}
	return ks, nil
}

type keystore struct {
	db      keyring.Keyring
	options Options
	mu      *sync.Mutex
}

func newKeystore(kr keyring.Keyring, opts ...Option) keystore {
	// Default options for keybase
	options := Options{
		SupportedAlgos:       SigningAlgoList{hd.Sr25519, hd.Secp256k1},
		SupportedAlgosLedger: SigningAlgoList{hd.Secp256k1},
	}

	for _, optionFn := range opts {
		optionFn(&options)
	}

	ledgerAlgos := make(SigningAlgoList, 0, len(options.SupportedAlgosLedger))
	for _, algo := range options.SupportedAlgosLedger {
		if algo.Name() == hd.Secp256k1Type {
			ledgerAlgos = append(ledgerAlgos, algo)
		}
	}
	options.SupportedAlgosLedger = ledgerAlgos

	return keystore{db: kr, options: options, mu: new(sync.Mutex)}
}

func (ks keystore) ExportPrivKeyArmor(uid, encryptPassphrase string) (armor string, err error) {
	priv, err := ks.ExportPrivateKeyObject(uid)
	if err != nil {
		return "", err
	}

	info, err := ks.Key(uid)
	if err != nil {
		return "", err
	}

	return crypto.EncryptArmorPrivKey(priv, encryptPassphrase, string(info.GetAlgo())), nil
}

// ExportPrivateKeyObject exports an armored private key object.
func (ks keystore) ExportPrivateKeyObject(uid string) ([]byte, error) {
	info, err := ks.Key(uid)
	if err != nil {
		return nil, err
	}

	var priv []byte

	switch linfo := info.(type) {
	case LocalInfo:
		if linfo.PrivKeyArmor == "" {
			return nil, fmt.Errorf("private key not available")
		}

		if linfo.Algo == hd.Sr25519Type {
			typedPriv := &sr25519.PrivKey{}
			if err := typedPriv.UnmarshalJSON([]byte(linfo.PrivKeyArmor)); err != nil {
				return nil, err
			}
			priv = []byte(linfo.PrivKeyArmor)
		} else {
			privKeys, err := legacy.PrivKeyFromBytes([]byte(linfo.PrivKeyArmor))
			if err != nil {
				return nil, err
			}
			priv = privKeys.Bytes()
		}

	case ledgerInfo, offlineInfo, multiInfo:
		return nil, errors.New("only works on local private keys")
	}

	return priv, nil
}

func (ks keystore) ExportPrivKeyArmorByAddress(address sdk.Address, encryptPassphrase string) (armor string, err error) {
	byAddress, err := ks.KeyByAddress(address)
	if err != nil {
		return "", err
	}

	return ks.ExportPrivKeyArmor(byAddress.GetName(), encryptPassphrase)
}

func (ks keystore) ImportPrivKey(uid, armor, passphrase string) error {
	if _, err := ks.Key(uid); err == nil {
		return fmt.Errorf("cannot overwrite key: %s", uid)
	}

	privKeyBytes, algo, err := crypto.UnarmorDecryptPrivKey(armor, passphrase)
	if err != nil {
		return errors.Wrap(err, "failed to decrypt private key")
	}

	var privKey types.PrivKey
	switch hd.PubKeyType(algo) {
	case hd.Sr25519Type:
		if !ks.options.SupportedAlgos.Contains(hd.Sr25519) {
			return ErrUnsupportedSigningAlgo
		}
		typedKey := &sr25519.PrivKey{}
		if err := typedKey.UnmarshalJSON(privKeyBytes); err != nil {
			return err
		}
		if len(typedKey.Bytes()) != sr25519.PrivKeySize {
			return fmt.Errorf("invalid sr25519 private key size")
		}
		privKey = typedKey
	case hd.Secp256k1Type:
		if !ks.options.SupportedAlgos.Contains(hd.Secp256k1) {
			return ErrUnsupportedSigningAlgo
		}
		secpKey := &secp256k1.PrivKey{}
		if err := secpKey.UnmarshalAmino(privKeyBytes); err != nil {
			return err
		}
		privKey = secpKey
	default:
		return fmt.Errorf("%w: armor algorithm %q", ErrUnsupportedSigningAlgo, algo)
	}

	_, err = ks.writeLocalKey(uid, privKey, hd.PubKeyType(algo))
	if err != nil {
		return err
	}

	return nil
}

func (ks keystore) Sign(uid string, msg []byte) ([]byte, types.PubKey, error) {
	info, err := ks.Key(uid)
	if err != nil {
		return nil, nil, err
	}

	var priv types.PrivKey

	switch i := info.(type) {
	case LocalInfo:
		if i.PrivKeyArmor == "" {
			return nil, nil, fmt.Errorf("private key not available")
		}

		if i.Algo == hd.Sr25519Type {
			typedPriv := &sr25519.PrivKey{}
			if err := typedPriv.UnmarshalJSON([]byte(i.PrivKeyArmor)); err != nil {
				return nil, nil, err
			}
			priv = typedPriv
		} else {
			priv, err = legacy.PrivKeyFromBytes([]byte(i.PrivKeyArmor))
			if err != nil {
				return nil, nil, err
			}
		}

	case ledgerInfo:
		return SignWithLedger(info, msg)

	case offlineInfo, multiInfo:
		return nil, info.GetPubKey(), errors.New("cannot sign with offline keys")
	}

	sig, err := priv.Sign(msg)
	if err != nil {
		return nil, nil, err
	}

	return sig, priv.PubKey(), nil
}

func (ks keystore) SignByAddress(address sdk.Address, msg []byte) ([]byte, types.PubKey, error) {
	key, err := ks.KeyByAddress(address)
	if err != nil {
		return nil, nil, err
	}

	return ks.Sign(key.GetName(), msg)
}

func (ks keystore) SaveLedgerKey(uid string, algo SignatureAlgo, hrp string, coinType, account, index uint32) (Info, error) {
	if algo.Name() != hd.Secp256k1Type || !ks.options.SupportedAlgosLedger.Contains(algo) {
		return nil, fmt.Errorf(
			"%w: signature algo %s is not defined in the keyring options",
			ErrUnsupportedSigningAlgo, algo.Name(),
		)
	}

	hdPath := hd.NewFundraiserParams(account, coinType, index)

	priv, _, err := ledger.NewPrivKeySecp256k1(*hdPath, hrp)
	if err != nil {
		return nil, fmt.Errorf("failed to generate ledger key: %w", err)
	}

	return ks.writeLedgerKey(uid, priv.PubKey(), *hdPath, algo.Name())
}

func (ks keystore) writeLedgerKey(name string, pub types.PubKey, path hd.BIP44Params, algo hd.PubKeyType) (Info, error) {
	info := newLedgerInfo(name, pub, path, algo)
	if err := ks.writeInfo(info); err != nil {
		return nil, err
	}

	return info, nil
}

func (ks keystore) SaveMultisig(uid string, pubkey types.PubKey) (Info, error) {
	return ks.writeMultisigKey(uid, pubkey)
}

func (ks keystore) SavePubKey(uid string, pubkey types.PubKey, algo hd.PubKeyType) (Info, error) {
	actual, err := publicKeyAlgorithm(pubkey)
	if err != nil {
		return nil, err
	}
	if algo != "" && algo != actual {
		return nil, fmt.Errorf("%w: public key type %s does not match %s", ErrUnsupportedSigningAlgo, actual, algo)
	}
	return ks.writeOfflineKey(uid, pubkey, actual)
}

func (ks keystore) DeleteByAddress(address sdk.Address) error {
	info, err := ks.KeyByAddress(address)
	if err != nil {
		return err
	}

	err = ks.Delete(info.GetName())
	if err != nil {
		return err
	}

	return nil
}

func (ks keystore) Delete(uid string) error {
	ks.mu.Lock()
	defer ks.mu.Unlock()
	recovered, err := ks.recoverOperationLocked()
	if err != nil {
		return err
	}
	if recovered != nil && recovered.Operation == "delete" && recovered.NameKey == infoKey(uid) {
		return nil
	}
	info, err := ks.key(infoKey(uid))
	if err != nil {
		return err
	}
	return ks.persistOperation("delete", info)
}

func (ks keystore) KeyByAddress(address sdk.Address) (Info, error) {
	ks.mu.Lock()
	defer ks.mu.Unlock()
	if _, err := ks.recoverOperationLocked(); err != nil {
		return nil, err
	}
	ik, err := ks.db.Get(addrHexKeyAsString(address))
	if err != nil {
		return nil, wrapKeyNotFound(err, fmt.Sprint("key with address", address, "not found"))
	}

	if len(ik.Data) == 0 {
		return nil, sdkerrors.Wrap(sdkerrors.ErrKeyNotFound, fmt.Sprint("key with address", address, "not found"))
	}
	return ks.key(string(ik.Data))
}

func wrapKeyNotFound(err error, msg string) error {
	if err == keyring.ErrKeyNotFound {
		return sdkerrors.Wrap(sdkerrors.ErrKeyNotFound, msg)
	}
	return err
}

func (ks keystore) List() ([]Info, error) {
	ks.mu.Lock()
	defer ks.mu.Unlock()
	if _, err := ks.recoverOperationLocked(); err != nil {
		return nil, err
	}
	res := []Info{}
	keys, err := ks.db.Keys()
	if err != nil {
		return nil, err
	}
	sort.Strings(keys)
	corruption := &CorruptionError{}
	for _, key := range keys {
		if !strings.HasSuffix(key, "."+infoSuffix) {
			continue
		}
		info, err := ks.key(key)
		if err != nil {
			corruption.Entries = append(corruption.Entries, &EntryError{Entry: key, Err: err})
			continue
		}
		res = append(res, info)
	}
	if len(corruption.Entries) != 0 {
		return res, corruption
	}
	return res, nil
}

func (ks keystore) NewMnemonic(uid string, language Language, hdPath, bip39Passphrase string, algo SignatureAlgo) (Info, string, error) {
	if language != English {
		return nil, "", ErrUnsupportedLanguage
	}

	if !ks.isSupportedSigningAlgo(algo) {
		return nil, "", ErrUnsupportedSigningAlgo
	}

	// Default number of words (24): This generates a mnemonic directly from the
	// number of words by reading system entropy.
	entropy, err := bip39.NewEntropy(defaultEntropySize)
	if err != nil {
		return nil, "", err
	}

	mnemonic, err := bip39.NewMnemonic(entropy)
	if err != nil {
		return nil, "", err
	}

	if bip39Passphrase == "" {
		bip39Passphrase = DefaultBIP39Passphrase
	}

	info, err := ks.NewAccount(uid, mnemonic, bip39Passphrase, hdPath, algo)
	if err != nil {
		return nil, "", err
	}

	return info, mnemonic, nil
}

func (ks keystore) NewAccount(name string, mnemonic string, bip39Passphrase string, hdPath string, algo SignatureAlgo) (Info, error) {
	if !ks.isSupportedSigningAlgo(algo) {
		return nil, ErrUnsupportedSigningAlgo
	}

	// create master key and derive first key for keyring
	derivedPriv, err := algo.Derive()(mnemonic, bip39Passphrase, hdPath)
	if err != nil {
		return nil, err
	}

	privKey := algo.Generate()(derivedPriv)

	return ks.writeLocalKey(name, privKey, algo.Name())
}

func (ks keystore) isSupportedSigningAlgo(algo SignatureAlgo) bool {
	return ks.options.SupportedAlgos.Contains(algo)
}

func (ks keystore) key(infoKey string) (Info, error) {
	bs, err := ks.db.Get(infoKey)
	if err != nil {
		return nil, wrapKeyNotFound(err, infoKey)
	}
	if len(bs.Data) == 0 {
		return nil, sdkerrors.Wrap(sdkerrors.ErrKeyNotFound, infoKey)
	}
	info, err := unmarshalInfo(bs.Data)
	if err != nil {
		return nil, &EntryError{Entry: infoKey, Err: errors.New("invalid encoded key information")}
	}
	if err := validateInfo(info); err != nil {
		return nil, &EntryError{Entry: infoKey, Err: err}
	}
	return info, nil
}

func (ks keystore) Key(uid string) (Info, error) {
	ks.mu.Lock()
	defer ks.mu.Unlock()
	if _, err := ks.recoverOperationLocked(); err != nil {
		return nil, err
	}
	return ks.key(infoKey(uid))
}

// SupportedAlgorithms returns the keystore Options' supported signing algorithm.
// for the keyring and Ledger.
func (ks keystore) SupportedAlgorithms() (SigningAlgoList, SigningAlgoList) {
	return ks.options.SupportedAlgos, ks.options.SupportedAlgosLedger
}

// SignWithLedger signs a binary message with the ledger device referenced by an Info object
// and returns the signed bytes and the public key. It returns an error if the device could
// not be queried or it returned an error.
func SignWithLedger(info Info, msg []byte) (sig []byte, pub types.PubKey, err error) {
	switch info.(type) {
	case *ledgerInfo, ledgerInfo:
	default:
		return nil, nil, errors.New("not a ledger object")
	}

	path, err := info.GetPath()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get BIP44 path from ledger info: %w", err)
	}

	// Use single-connection signing to avoid race conditions from multiple device open/close cycles
	sig, pub, err = ledger.SignWithPath(*path, msg)
	if err != nil {
		return nil, nil, fmt.Errorf("ledger signing failed: %w", err)
	}

	// Validate that the public key from the device matches the cached key in the keyring
	if !pub.Equals(info.GetPubKey()) {
		return nil, nil, errors.New("the public key from the Ledger device does not match the cached key in the keyring")
	}

	return sig, pub, nil
}

func newOSBackendKeyringConfig(appName, dir string, buf io.Reader) keyring.Config {
	return keyring.Config{
		ServiceName:              appName,
		FileDir:                  dir,
		KeychainTrustApplication: true,
		FilePasswordFunc:         newRealPrompt(dir, buf),
	}
}

func newTestBackendKeyringConfig(appName, dir string) keyring.Config {
	return keyring.Config{
		AllowedBackends: []keyring.BackendType{keyring.FileBackend},
		ServiceName:     appName,
		FileDir:         filepath.Join(dir, keyringTestDirName),
		FilePasswordFunc: func(_ string) (string, error) {
			return "test", nil
		},
	}
}

func newKWalletBackendKeyringConfig(appName, _ string, _ io.Reader) keyring.Config {
	return keyring.Config{
		AllowedBackends: []keyring.BackendType{keyring.KWalletBackend},
		ServiceName:     "kdewallet",
		KWalletAppID:    appName,
		KWalletFolder:   "",
	}
}

func newPassBackendKeyringConfig(appName, _ string, _ io.Reader) keyring.Config {
	prefix := fmt.Sprintf(passKeyringPrefix, appName)

	return keyring.Config{
		AllowedBackends: []keyring.BackendType{keyring.PassBackend},
		ServiceName:     appName,
		PassPrefix:      prefix,
	}
}

func newFileBackendKeyringConfig(name, dir string, buf io.Reader) keyring.Config {
	fileDir := filepath.Join(dir, keyringFileDirName)

	return keyring.Config{
		AllowedBackends:  []keyring.BackendType{keyring.FileBackend},
		ServiceName:      name,
		FileDir:          fileDir,
		FilePasswordFunc: newRealPrompt(fileDir, buf),
	}
}

func (ks keystore) writeLocalKey(name string, priv types.PrivKey, algo hd.PubKeyType) (Info, error) {
	// encrypt private key using keyring
	pub := priv.PubKey()
	var info Info
	if algo == hd.Sr25519Type {
		typedPriv := priv.(*sr25519.PrivKey)
		jsonBytes, err := typedPriv.MarshalJSON()
		if err != nil {
			return nil, err
		}
		info = newLocalInfo(name, pub, string(jsonBytes), algo)
	} else {
		info = newLocalInfo(name, pub, string(legacy.Cdc.MustMarshal(priv)), algo)
	}
	if err := ks.writeInfo(info); err != nil {
		return nil, err
	}

	return info, nil
}

func (ks keystore) writeInfo(info Info) error {
	if err := validateInfo(info); err != nil {
		return err
	}
	ks.mu.Lock()
	defer ks.mu.Unlock()
	recovered, err := ks.recoverOperationLocked()
	if err != nil {
		return err
	}
	// An interrupted create may be retried only with the identical complete record.
	if recovered != nil && recovered.Operation == "create" && recovered.NameKey == infoKey(info.GetName()) && string(recovered.Data) == string(marshalInfo(info)) {
		return nil
	}
	exists, err := ks.existsInDb(info)
	if err != nil {
		return err
	}
	if exists {
		saved, nameErr := ks.db.Get(infoKey(info.GetName()))
		index, addressErr := ks.db.Get(addrHexKeyAsString(info.GetAddress()))
		if nameErr == nil && addressErr == nil && string(saved.Data) == string(marshalInfo(info)) && string(index.Data) == infoKey(info.GetName()) {
			return nil
		}
		return errors.New("public key already exists in keybase")
	}
	return ks.persistOperation("create", info)
}

// existsInDb returns true if key is in DB. Error is returned only when we have error
// different thant ErrKeyNotFound
func (ks keystore) existsInDb(info Info) (bool, error) {
	if _, err := ks.db.Get(addrHexKeyAsString(info.GetAddress())); err == nil {
		return true, nil // address lookup succeeds - info exists
	} else if err != keyring.ErrKeyNotFound {
		return false, err // received unexpected error - returns error
	}

	if _, err := ks.db.Get(infoKey(info.GetName())); err == nil {
		return true, nil // uid lookup succeeds - info exists
	} else if err != keyring.ErrKeyNotFound {
		return false, err // received unexpected error - returns
	}

	// both lookups failed, info does not exist
	return false, nil
}

func (ks keystore) writeOfflineKey(name string, pub types.PubKey, algo hd.PubKeyType) (Info, error) {
	info := newOfflineInfo(name, pub, algo)
	err := ks.writeInfo(info)
	if err != nil {
		return nil, err
	}

	return info, nil
}

func (ks keystore) writeMultisigKey(name string, pub types.PubKey) (Info, error) {
	info, err := NewMultiInfo(name, pub)
	if err != nil {
		return nil, err
	}
	if err = ks.writeInfo(info); err != nil {
		return nil, err
	}

	return info, nil
}

type unsafeKeystore struct {
	keystore
}

// NewUnsafe returns a new keyring that provides support for unsafe operations.
func NewUnsafe(kr Keyring) UnsafeKeyring {
	// The type assertion is against the only keystore
	// implementation that is currently provided.
	ks := kr.(keystore)

	return unsafeKeystore{ks}
}

// UnsafeExportPrivKeyHex exports private keys in unarmored hexadecimal format.
func (ks unsafeKeystore) UnsafeExportPrivKeyHex(uid string) (privkey string, err error) {
	priv, err := ks.ExportPrivateKeyObject(uid)
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(priv), nil
}

func addrHexKeyAsString(address sdk.Address) string {
	return fmt.Sprintf("%s.%s", hex.EncodeToString(address.Bytes()), addressSuffix)
}
