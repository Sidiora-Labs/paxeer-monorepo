package layerx

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"io"
	"strconv"
	"strings"
)

const PortableReceiptFormat = "layerx-receipt-proof-v2"

type PortableReceiptKey struct {
	KeyID       string `json:"keyId"`
	PublicKey   string `json:"publicKey"`
	FirstHeight string `json:"firstHeight"`
	LastHeight  string `json:"lastHeight"`
}

type PortableCommittedEvidence struct {
	BlockHeader      CommittedHeader
	ReceiptInclusion ReceiptInclusion
	Commit           CometCommit
	ValidatorSet     []CometValidator
}

type PortableReceipt struct {
	Format            string             `json:"format"`
	VerificationLevel string             `json:"verificationLevel"`
	RawTransaction    string             `json:"rawTransaction"`
	SignedReceipt     string             `json:"signedReceipt"`
	ReceiptDigest     string             `json:"receiptDigest"`
	ReceiptKey        PortableReceiptKey `json:"receiptKey"`
	ReceiptJSON       json.RawMessage    `json:"receiptJson"`
	BlockHeader       *CommittedHeader   `json:"blockHeader,omitempty"`
	ReceiptInclusion  *ReceiptInclusion  `json:"receiptInclusion,omitempty"`
	Commit            *CometCommit       `json:"commit,omitempty"`
	ValidatorSet      []CometValidator   `json:"validatorSet,omitempty"`
}

func portableHex(value []byte) string { return "0x" + hex.EncodeToString(value) }

func portableKey(key ReceiptKey) PortableReceiptKey {
	return PortableReceiptKey{KeyID: portableHex(key.KeyID[:]), PublicKey: portableHex(key.PublicKey[:]),
		FirstHeight: "0x" + strconv.FormatUint(key.FirstHeight, 16), LastHeight: "0x" + strconv.FormatUint(key.LastHeight, 16)}
}

func ExportPortableReceipt(receiptJSON, rawTransaction []byte, keys ReceiptKeys, committed *PortableCommittedEvidence) (PortableReceipt, error) {
	r, err := VerifySequencerReceipt(receiptJSON, rawTransaction, keys)
	if err != nil {
		return PortableReceipt{}, err
	}
	height := binary.BigEndian.Uint64(r.Receipt.BatchID[24:])
	var key ReceiptKey
	for _, pin := range keys.Pins {
		if pin.KeyID == r.Receipt.ContextHash && pin.FirstHeight <= height && height <= pin.LastHeight {
			key = pin
			break
		}
	}
	p := PortableReceipt{Format: PortableReceiptFormat, VerificationLevel: "receipted", RawTransaction: portableHex(rawTransaction),
		SignedReceipt: portableHex(r.CanonicalBytes), ReceiptDigest: portableHex(r.ReceiptDigest[:]), ReceiptKey: portableKey(key),
		ReceiptJSON: append(json.RawMessage(nil), receiptJSON...)}
	if committed != nil {
		p.VerificationLevel = "committed"
		p.BlockHeader = &committed.BlockHeader
		p.ReceiptInclusion = &committed.ReceiptInclusion
		p.Commit = &committed.Commit
		p.ValidatorSet = committed.ValidatorSet
	}
	if _, err := p.verify(keys); err != nil {
		return PortableReceipt{}, err
	}
	return p, nil
}

func (p PortableReceipt) Export() ([]byte, error) { return p.export() }

func (p PortableReceipt) export() ([]byte, error) {
	if err := p.portableShape(); err != nil {
		return nil, err
	}
	data, err := json.Marshal(p)
	if err != nil {
		return nil, receiptFailure("portable-decode")
	}
	if err := portableDocument(data); err != nil {
		return nil, err
	}
	return data, nil
}

func VerifyPortableReceipt(data []byte, keys ReceiptKeys) (SequencerReceiptVerification, error) {
	var p PortableReceipt
	if err := json.Unmarshal(data, &p); err != nil {
		return SequencerReceiptVerification{}, receiptFailure("portable-decode")
	}
	return p.verify(keys)
}

func (p PortableReceipt) Verify(keys ReceiptKeys) (SequencerReceiptVerification, error) {
	return p.verify(keys)
}

func (p PortableReceipt) verify(keys ReceiptKeys) (SequencerReceiptVerification, error) {
	fail := func(check ReceiptCheck) (SequencerReceiptVerification, error) {
		return SequencerReceiptVerification{}, receiptFailure(check)
	}
	if _, err := p.export(); err != nil {
		return SequencerReceiptVerification{}, err
	}
	var pinned []ReceiptKey
	for _, key := range keys.Pins {
		if portableKey(key) == p.ReceiptKey {
			pinned = append(pinned, key)
		}
	}
	if len(pinned) == 0 {
		return fail(ReceiptCheckKeyID)
	}
	raw, _ := receiptHex(p.RawTransaction)
	r, err := VerifySequencerReceipt(p.ReceiptJSON, raw, ReceiptKeys{Pins: pinned, PreviousSequenceHash: keys.PreviousSequenceHash})
	if err != nil {
		return SequencerReceiptVerification{}, err
	}
	if p.SignedReceipt != portableHex(r.CanonicalBytes) {
		return fail(ReceiptCheckCanonicalEncoding)
	}
	if p.ReceiptDigest != portableHex(r.ReceiptDigest[:]) {
		return fail("portable-digest")
	}
	if p.VerificationLevel == "receipted" {
		return r, nil
	}
	if !bytes.Equal(raw, p.ReceiptInclusion.Transaction) {
		return fail(ReceiptCheckRawTransaction)
	}
	committed, err := VerifyCommittedReceipt(r, *p.BlockHeader, *p.ReceiptInclusion, *p.Commit, p.ValidatorSet)
	if err != nil {
		return SequencerReceiptVerification{}, err
	}
	return committed.SequencerReceiptVerification, nil
}

func (p PortableReceipt) portableShape() error {
	if p.Format != PortableReceiptFormat {
		return receiptFailure("portable-format")
	}
	switch p.VerificationLevel {
	case "receipted":
		if p.BlockHeader != nil || p.ReceiptInclusion != nil || p.Commit != nil || p.ValidatorSet != nil {
			return receiptFailure("portable-level")
		}
	case "committed":
		if p.BlockHeader == nil || p.ReceiptInclusion == nil || p.Commit == nil || len(p.ValidatorSet) == 0 {
			return receiptFailure("portable-level")
		}
	default:
		return receiptFailure("portable-level")
	}
	for _, field := range []struct {
		value string
		size  int
	}{
		{p.RawTransaction, 0}, {p.SignedReceipt, 0}, {p.ReceiptDigest, 32}, {p.ReceiptKey.KeyID, 32}, {p.ReceiptKey.PublicKey, 32},
	} {
		if len(field.value) > 2+2*maximumMessageBytes {
			return receiptFailure("portable-fields")
		}
		value, ok := receiptHex(field.value)
		if !ok || len(value) == 0 || field.size != 0 && len(value) != field.size || field.value != portableHex(value) {
			return receiptFailure("portable-fields")
		}
	}
	first, ok := receiptQuantity(p.ReceiptKey.FirstHeight, 64)
	if !ok {
		return receiptFailure("portable-fields")
	}
	last, ok := receiptQuantity(p.ReceiptKey.LastHeight, 64)
	if !ok || first.Cmp(last) > 0 {
		return receiptFailure("portable-fields")
	}
	var view sequencerReceiptJSON
	if err := portableDecode(p.ReceiptJSON, &view); err != nil {
		return err
	}
	return nil
}

func (p *PortableReceipt) UnmarshalJSON(data []byte) error {
	type wire PortableReceipt
	var value wire
	if err := portableDecode(data, &value); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return receiptFailure("portable-decode")
	}
	for _, name := range []string{"format", "verificationLevel", "rawTransaction", "signedReceipt", "receiptDigest", "receiptKey", "receiptJson"} {
		if v, ok := fields[name]; !ok || bytes.Equal(v, []byte("null")) {
			return receiptFailure("portable-fields")
		}
	}
	for _, name := range []string{"blockHeader", "receiptInclusion", "commit", "validatorSet"} {
		v, present := fields[name]
		if value.VerificationLevel == "receipted" && present || value.VerificationLevel == "committed" && (!present || bytes.Equal(v, []byte("null"))) {
			return receiptFailure("portable-level")
		}
	}
	result := PortableReceipt(value)
	if err := result.portableShape(); err != nil {
		return err
	}
	*p = result
	return nil
}

func portableDecode(data []byte, value any) error {
	if err := portableDocument(data); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return receiptFailure("portable-decode")
	}
	return nil
}

func portableDocument(data []byte) error {
	if len(data) == 0 || len(data) > 16*maximumMessageBytes {
		return receiptFailure("portable-decode")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var visit func(int) error
	visit = func(depth int) error {
		if depth > 64 {
			return receiptFailure("portable-decode")
		}
		token, err := decoder.Token()
		if err != nil {
			return receiptFailure("portable-decode")
		}
		delim, composite := token.(json.Delim)
		if !composite {
			return nil
		}
		switch delim {
		case '{':
			seen := make(map[string]bool)
			for decoder.More() {
				token, err := decoder.Token()
				if err != nil {
					return receiptFailure("portable-decode")
				}
				key, ok := token.(string)
				if !ok {
					return receiptFailure("portable-decode")
				}
				key = strings.ToLower(key)
				if seen[key] {
					return receiptFailure("portable-fields")
				}
				seen[key] = true
				switch key {
				case "batchid", "asset", "previousstateroot", "resultingstateroot":
					return receiptFailure("portable-authorized-batch")
				}
				if err := visit(depth + 1); err != nil {
					return err
				}
			}
		case '[':
			for decoder.More() {
				if err := visit(depth + 1); err != nil {
					return err
				}
			}
		default:
			return receiptFailure("portable-decode")
		}
		if _, err := decoder.Token(); err != nil {
			return receiptFailure("portable-decode")
		}
		return nil
	}
	if err := visit(0); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return receiptFailure("portable-decode")
	}
	return nil
}
