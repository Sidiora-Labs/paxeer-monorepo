// Code generated from sdk/platform/generators/receipt.kvx. DO NOT EDIT.

package layerx

const ReceiptVersionV2 uint16 = 2
const ReceiptVersionV3 uint16 = 3
const ReceiptDomain = "LXP/v1/receipt"
const SequenceDomain = "PAXEER/v1/sequence"

type ReceiptCheck string

const (
	ReceiptCheckDecode            ReceiptCheck = "decode"
	ReceiptCheckCanonicalEncoding ReceiptCheck = "canonical-encoding"
	ReceiptCheckReceiptVersion    ReceiptCheck = "receipt-version"
	ReceiptCheckActivityID        ReceiptCheck = "activity-id"
	ReceiptCheckKeyID             ReceiptCheck = "key-id"
	ReceiptCheckKeyHeight         ReceiptCheck = "key-height"
	ReceiptCheckRawTransaction    ReceiptCheck = "raw-transaction"
	ReceiptCheckFields            ReceiptCheck = "fields"
	ReceiptCheckReturnData        ReceiptCheck = "return-data"
	ReceiptCheckLogs              ReceiptCheck = "logs"
	ReceiptCheckSequenceHash      ReceiptCheck = "sequence-hash"
	ReceiptCheckSignature         ReceiptCheck = "signature"
)

const ProgramAbiV1 uint16 = 1
const ProgramAbiV2 uint16 = 2
const ProgramAbiV3 uint16 = 3
const ProgramAbiV4 uint16 = 4
const ProgramAbiV5 uint16 = 5

func SupportsProgramGuestAbi(version uint16) bool {
	switch version {
	case ProgramAbiV1, ProgramAbiV2, ProgramAbiV3, ProgramAbiV4, ProgramAbiV5:
		return true
	default:
		return false
	}
}
