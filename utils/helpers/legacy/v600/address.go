package v600

import (
	"errors"
	"math/big"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/utils/helpers"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

func GetAddresses(V *big.Int, R *big.Int, S *big.Int, data common.Hash) (common.Address, sdk.AccAddress, cryptotypes.PubKey, error) {
	return GetAddressesForContext(sdk.Context{}, V, R, S, data)
}

// GetAddressesForContext permits historical conversion only for a replay context
// selected by the owner. New signature inputs are validated before recovery.
func GetAddressesForContext(ctx sdk.Context, V *big.Int, R *big.Int, S *big.Int, data common.Hash) (common.Address, sdk.AccAddress, cryptotypes.PubKey, error) {
	pubkey, err := RecoverPubkey(data, R, S, V, true)
	if err != nil {
		return common.Address{}, sdk.AccAddress{}, nil, err
	}

	return GetAddressesFromPubkeyBytesForContext(ctx, pubkey)
}

// GetAddressesFromPubkeyBytes validates new input. Historical callers must pass
// the owning version selector's context to the explicit replay entry point.
func GetAddressesFromPubkeyBytes(pubkey []byte) (common.Address, sdk.AccAddress, cryptotypes.PubKey, error) {
	return GetAddressesFromPubkeyBytesForContext(sdk.Context{}, pubkey)
}

// GetAddressesFromPubkeyBytesForContext preserves unchecked conversion only for
// historical tracing in [v6.0.0, v6.0.1). All other input uses current validators.
func GetAddressesFromPubkeyBytesForContext(ctx sdk.Context, pubkey []byte) (common.Address, sdk.AccAddress, cryptotypes.PubKey, error) {
	if !IsHistoricalReplay(ctx) {
		return helpers.GetAddressesFromPubkeyBytes(pubkey)
	}
	evmAddr, err := historicalPubkeyToEVMAddress(pubkey)
	if err != nil {
		return common.Address{}, sdk.AccAddress{}, nil, err
	}
	paxPubkey := historicalPubkeyBytesToPaxPubKey(pubkey)
	paxAddr := sdk.AccAddress(paxPubkey.Address())
	return evmAddr, paxAddr, &paxPubkey, nil
}

// first half of go-ethereum/core/types/transaction_signing.go:recoverPlain
func RecoverPubkey(sighash common.Hash, R, S, Vb *big.Int, homestead bool) ([]byte, error) {
	if Vb == nil || R == nil || S == nil || Vb.Sign() < 0 || Vb.BitLen() > 8 {
		return []byte{}, ethtypes.ErrInvalidSig
	}
	V := byte(Vb.Uint64() - 27)
	if !crypto.ValidateSignatureValues(V, R, S, homestead) {
		return []byte{}, ethtypes.ErrInvalidSig
	}
	// encode the signature in uncompressed format
	r, s := R.Bytes(), S.Bytes()
	sig := make([]byte, crypto.SignatureLength)
	copy(sig[32-len(r):32], r)
	copy(sig[64-len(s):64], s)
	sig[64] = V

	// recover the public key from the signature
	return crypto.Ecrecover(sighash[:], sig)
}

// PubkeyToEVMAddress validates new input; replay uses the explicit context API.
func PubkeyToEVMAddress(pub []byte) (common.Address, error) {
	return helpers.PubkeyToEVMAddress(pub)
}

// PubkeyBytesToPaxPubKey returns parser errors for new public input.
func PubkeyBytesToPaxPubKey(pub []byte) (secp256k1.PubKey, error) {
	return helpers.PubkeyBytesToPaxPubKey(pub)
}

// Historical replay retains the original prefix-only check and unchecked parse.
func historicalPubkeyToEVMAddress(pub []byte) (common.Address, error) {
	if len(pub) == 0 || pub[0] != 4 {
		return common.Address{}, errors.New("invalid public key")
	}
	var addr common.Address
	copy(addr[:], crypto.Keccak256(pub[1:])[12:])
	return addr, nil
}

func historicalPubkeyBytesToPaxPubKey(pub []byte) secp256k1.PubKey {
	pubkeyObj, _ := btcec.ParsePubKey(pub)
	return secp256k1.PubKey{Key: pubkeyObj.SerializeCompressed()}
}
