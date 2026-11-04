package crypto

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strconv"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto"
	"github.com/tendermint/crypto/bcrypt"
	"golang.org/x/crypto/nacl/secretbox"
	"golang.org/x/crypto/openpgp/armor" //nolint:staticcheck // SA1019: not worth fixing

	cosmoscrypto "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/utils"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
)

const (
	blockTypePrivKey = "TENDERMINT PRIVATE KEY"
	blockTypeKeyInfo = "TENDERMINT KEY INFO"
	blockTypePubKey  = "TENDERMINT PUBLIC KEY"

	defaultAlgo = "secp256k1"

	headerVersion = "version"
	headerType    = "type"
)

func EncodeArmor(blockType string, headers map[string]string, data []byte) string {
	buf := new(bytes.Buffer)
	w, err := armor.Encode(buf, blockType, headers)
	if err != nil {
		panic(fmt.Errorf("could not encode ascii armor: %s", err))
	}
	_, err = w.Write(data)
	if err != nil {
		panic(fmt.Errorf("could not encode ascii armor: %s", err))
	}
	err = w.Close()
	if err != nil {
		panic(fmt.Errorf("could not encode ascii armor: %s", err))
	}
	return buf.String()
}

func DecodeArmor(armorStr string) (blockType string, headers map[string]string, data []byte, err error) {
	buf := bytes.NewBufferString(armorStr)
	block, err := armor.Decode(buf)
	if err != nil {
		return "", nil, nil, err
	}
	data, err = io.ReadAll(block.Body)
	if err != nil {
		return "", nil, nil, err
	}
	return block.Type, block.Header, data, nil
}

const nonceLen = 24
const secretLen = 32

// ErrCiphertextDecryptionFailed identifies a failed secretbox authentication.
var ErrCiphertextDecryptionFailed = errors.New("ciphertext decryption failed")

const (
	privKeyArmorVersion = "1"
	legacyArmorCost     = 12
	// Bound work requested by untrusted imports, well below bcrypt's maximum of 31.
	maxArmorCost = 16
)

// secret must be 32 bytes long. Use something like Sha256(Bcrypt(passphrase))
// The ciphertext is (secretbox.Overhead + 24) bytes longer than the plaintext.
func EncryptSymmetric(plaintext []byte, secret []byte) (ciphertext []byte) {
	if len(secret) != secretLen {
		panic(fmt.Sprintf("Secret must be 32 bytes long, got len %v", len(secret)))
	}
	nonce := crypto.CRandBytes(nonceLen)
	nonceArr := [nonceLen]byte{}
	copy(nonceArr[:], nonce)
	secretArr := [secretLen]byte{}
	copy(secretArr[:], secret)
	ciphertext = make([]byte, nonceLen+secretbox.Overhead+len(plaintext))
	copy(ciphertext, nonce)
	secretbox.Seal(ciphertext[nonceLen:nonceLen], plaintext, &nonceArr, &secretArr)
	return ciphertext
}

// secret must be 32 bytes long. Use something like Sha256(Bcrypt(passphrase))
// The ciphertext is (secretbox.Overhead + 24) bytes longer than the plaintext.
func DecryptSymmetric(ciphertext []byte, secret []byte) (plaintext []byte, err error) {
	if len(secret) != secretLen {
		panic(fmt.Sprintf("Secret must be 32 bytes long, got len %v", len(secret)))
	}
	if len(ciphertext) < secretbox.Overhead+nonceLen {
		return nil, errors.New("ciphertext is too short")
	}
	nonce := ciphertext[:nonceLen]
	nonceArr := [nonceLen]byte{}
	copy(nonceArr[:], nonce)
	secretArr := [secretLen]byte{}
	copy(secretArr[:], secret)
	plaintext = make([]byte, len(ciphertext)-nonceLen-secretbox.Overhead)
	_, ok := secretbox.Open(plaintext[:0], ciphertext[nonceLen:], &nonceArr, &secretArr)
	if !ok {
		return nil, ErrCiphertextDecryptionFailed
	}
	return plaintext, nil
}

// BcryptSecurityParameter is the cost for new armored exports (default 12).
// Costs below bcrypt.MinCost retain bcrypt's normalization to DefaultCost.
// Imports use the envelope's cost, or cost 12 for legacy unversioned exports.
// Configure this before use; concurrent mutation is unsupported.
// See bcrypt_readme.md for the separate file-keyring verifier threat model.
var BcryptSecurityParameter = 12

//-----------------------------------------------------------------
// encrypt/decrypt with armor

// Encrypt and armor the private key.
func EncryptArmorPrivKey(privKeyBytes []byte, passphrase string, algo string) string {
	cost := BcryptSecurityParameter
	if cost < bcrypt.MinCost {
		cost = bcrypt.DefaultCost
	}
	if cost > maxArmorCost {
		panic(fmt.Errorf("armor bcrypt cost must be between %d and %d", bcrypt.MinCost, maxArmorCost))
	}
	saltBytes, encBytes := encryptPrivKey(privKeyBytes, passphrase, cost)
	header := map[string]string{
		headerVersion: privKeyArmorVersion,
		"kdf":         "bcrypt",
		"cost":        strconv.Itoa(cost),
		"salt":        fmt.Sprintf("%X", saltBytes),
	}

	if algo != "" {
		header[headerType] = algo
	}

	armorStr := EncodeArmor(blockTypePrivKey, header, encBytes)

	return armorStr
}

// encrypt the given privKey with the passphrase using a randomly
// generated salt and the xsalsa20 cipher. returns the salt and the
// encrypted priv key.
func encryptPrivKey(privKeyBytes []byte, passphrase string, cost int) (saltBytes []byte, encBytes []byte) {
	saltBytes = crypto.CRandBytes(16)
	key, err := bcrypt.GenerateFromPassword(saltBytes, []byte(passphrase), cost)

	if err != nil {
		panic(sdkerrors.Wrap(err, "error generating bcrypt key from passphrase"))
	}

	key = cosmoscrypto.Sha256(key) // get 32 bytes

	return saltBytes, EncryptSymmetric(privKeyBytes, key)
}

// UnarmorDecryptPrivKey returns the privkey byte slice, a string of the algo type, and an error
func UnarmorDecryptPrivKey(armorStr string, passphrase string) (privKey []byte, algo string, err error) {
	blockType, header, encBytes, err := DecodeArmor(armorStr)
	if err != nil {
		return privKey, "", err
	}

	if blockType != blockTypePrivKey {
		return privKey, "", fmt.Errorf("unrecognized armor type: %v", blockType)
	}

	if header["kdf"] != "bcrypt" {
		return privKey, "", fmt.Errorf("unrecognized KDF type: %v", header["kdf"])
	}

	if header["salt"] == "" {
		return privKey, "", fmt.Errorf("missing salt bytes")
	}

	saltBytes, err := hex.DecodeString(header["salt"])
	if err != nil {
		return privKey, "", fmt.Errorf("error decoding salt: %v", err.Error())
	}

	if len(saltBytes) != 16 {
		return nil, "", fmt.Errorf("armor salt must be 16 bytes")
	}

	cost := legacyArmorCost
	switch header[headerVersion] {
	case "":
		if _, exists := header["cost"]; exists {
			return nil, "", fmt.Errorf("unversioned armor must not specify a cost")
		}
	case privKeyArmorVersion:
		cost, err = strconv.Atoi(header["cost"])
		if err != nil || cost < bcrypt.MinCost || cost > maxArmorCost {
			return nil, "", fmt.Errorf("armor bcrypt cost must be between %d and %d", bcrypt.MinCost, maxArmorCost)
		}
	default:
		return nil, "", fmt.Errorf("unrecognized armor version: %s", header[headerVersion])
	}

	privKey, err = decryptPrivKey(saltBytes, encBytes, passphrase, cost)

	if header[headerType] == "" {
		header[headerType] = defaultAlgo
	}

	return privKey, header[headerType], err
}

func decryptPrivKey(saltBytes []byte, encBytes []byte, passphrase string, cost int) (privKey []byte, err error) {
	key, err := bcrypt.GenerateFromPassword(saltBytes, []byte(passphrase), cost)
	if err != nil {
		return privKey, sdkerrors.Wrap(err, "error generating bcrypt key from passphrase")
	}

	key = cosmoscrypto.Sha256(key) // Get 32 bytes

	privKeyBytes, err := DecryptSymmetric(encBytes, key)
	if errors.Is(err, ErrCiphertextDecryptionFailed) {
		return privKey, sdkerrors.Wrap(sdkerrors.ErrWrongPassword, "armor authentication failed")
	} else if err != nil {
		return privKey, err
	}

	return privKeyBytes, nil
}
