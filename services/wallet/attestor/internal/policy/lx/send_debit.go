package lx

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"

	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/lxwire"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/policy"
)

// SendDebitBinding must come from the held key and its original authorization,
// rather than from fields supplied by a receipt submitter.
type SendDebitBinding struct {
	KeyID               string
	Account             common.Address
	PublicKey           [32]byte
	NetworkID           uint32
	AuthorizationDigest [32]byte
}

// AuthorizedSendDebit describes a signed send, not evidence of its commitment.
// Receipt ingestion must authenticate committed execution of EnvelopeDigest and
// durably replicate the debit before allowing another cap-sensitive sign.
type AuthorizedSendDebit struct {
	Binding        SendDebitBinding
	EnvelopeDigest [32]byte
	ReplayID       string
	Spends         []policy.Spend
}

// DecodeAuthorizedSendDebit derives debits from the canonical signed payload.
// It never writes to a ledger or treats the authorization as a committed spend.
func DecodeAuthorizedSendDebit(binding SendDebitBinding, envelope []byte) (*AuthorizedSendDebit, error) {
	if binding.KeyID == "" || binding.Account == (common.Address{}) || binding.PublicKey == ([32]byte{}) ||
		binding.NetworkID == 0 || binding.AuthorizationDigest == ([32]byte{}) {
		return nil, errors.New("send debit: held key, account, network and authorization digest are required")
	}
	registry, err := decodedOperations()
	if err != nil {
		return nil, err
	}
	activity, err := lxwire.DecodeUnsignedActivity(envelope, registry)
	if err != nil {
		return nil, fmt.Errorf("send debit: %w", err)
	}
	if activity.Type != OpAssetTransfer || !activity.PayloadHashMatches() {
		return nil, errors.New("send debit: expected an asset send with a matching payload hash")
	}
	if activity.NetworkID != binding.NetworkID || activity.AuthorityKind(binding.PublicKey) != lxwire.AuthorityOwner ||
		string(activity.ActorDID) != lxwire.DIDFromKey(binding.PublicKey) {
		return nil, errors.New("send debit: envelope does not belong to the held key and network")
	}
	send, err := lxwire.DecodeSend(activity.Payload)
	if err != nil {
		return nil, fmt.Errorf("send debit: signed authorization: %w", err)
	}
	effect, err := checkAssetSend(activity, send)
	if err != nil {
		return nil, fmt.Errorf("send debit: %w", err)
	}
	digest, err := send.AuthorizationDigest()
	if err != nil || digest != binding.AuthorizationDigest {
		return nil, errors.New("send debit: authorization digest does not match the held authorization")
	}
	owned, err := owns(lxwire.DIDFromKey(binding.PublicKey), effect.Account, ID(send.Asset))
	if err != nil || !owned {
		return nil, errors.New("send debit: source account is not held by the signing key")
	}

	// Session IDs and outer activity metadata do not identify a debit: the same
	// authorized send can be resubmitted in another session or envelope.
	identity := []byte("paxeer-attestor-authorized-send-debit-v1\x00")
	identity = binary.BigEndian.AppendUint64(identity, uint64(len(binding.KeyID)))
	identity = append(identity, binding.KeyID...)
	identity = append(identity, binding.Account[:]...)
	identity = append(identity, binding.PublicKey[:]...)
	identity = binary.BigEndian.AppendUint32(identity, binding.NetworkID)
	identity = append(identity, digest[:]...)
	replay := sha256.Sum256(identity)
	return &AuthorizedSendDebit{
		Binding:        binding,
		EnvelopeDigest: sha256.Sum256(envelope),
		ReplayID:       "lx-send/" + hex.EncodeToString(replay[:]),
		Spends:         []policy.Spend{{Asset: assetKey("lx:", ID(send.Asset)), Amount: toBig(send.Amount)}},
	}, nil
}
