package layerxprograms

import (
	"fmt"

	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func callWithReceiptsHandler(ctx sdk.Context, p *PrecompileExecutor, call callContext, args []interface{}) ([]byte, error) {
	authorities, err := decodeCallReceiptAuthorities(call, args)
	if err != nil {
		return nil, err
	}
	return callWithAuthorities(ctx, p, call, args, "callWithReceipts", authorities)
}

func decodeCallReceiptAuthorities(call callContext, args []interface{}) ([]ReceiptAuthority, error) {
	method := programsABI.Methods["callWithReceipts"]
	if call.method == nil || call.method.Name != method.Name {
		return nil, errGateUnknownMethod
	}
	if err := pcommon.ValidateArgsLength(args, len(method.Inputs)); err != nil {
		return nil, err
	}
	tuples, ok := args[9].([]struct {
		CanonicalReceipt []byte `json:"canonicalReceipt"`
		BlockHeight      uint64 `json:"blockHeight"`
	})
	if !ok {
		return nil, fmt.Errorf("LXP_NON_CANONICAL: expected ABI receipt authority tuples, got %T", args[9])
	}
	authorities := make([]ReceiptAuthority, len(tuples))
	for i, tuple := range tuples {
		authorities[i] = ReceiptAuthority{
			CanonicalReceipt: tuple.CanonicalReceipt,
			BlockHeight:      tuple.BlockHeight,
		}
	}
	return authorities, nil
}

func init() {
	register("callWithReceipts", callWithReceiptsHandler)
}
