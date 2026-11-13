package policy

import (
	"math/big"

	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/policy/evm"
	"github.com/ethereum/go-ethereum/accounts/abi"
)

// ConstructTransfer retains the native account effects, including credits and
// transfers among owned accounts. Only owned sources consume the signing cap.
type ConstructTransfer struct {
	From, Asset, To    [32]byte
	Amount             *big.Int
	FromOwned, ToOwned bool
}

type constructLeg struct {
	From   [32]byte
	Asset  [32]byte
	To     [32]byte
	Amount *big.Int
}

// SetConstructStateReader installs the trusted header/address-binding adapter.
// An absent reader refuses calls whose effects depend on native state.
func (p *Policy) SetConstructStateReader(reader func(Context) (*ConstructState, error)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.constructReader = reader
}

func inspectConstruct(ctx Context, call *evm.Call, value *big.Int, out *Inspection) error {
	// These methods have no unreserved owner debit. Other mutations remain
	// refused until their asset, source and amount can be established.
	contract, err := evm.PrecompileABI(call.Destination)
	if err != nil {
		return err
	}
	method := contract.Methods[call.Method]
	if method.IsConstant() {
		return nil
	}
	switch call.Destination + "." + call.Method {
	case "layerxpay.streamMeter":
		if value.Sign() != 0 {
			return refuse(CodeDecodeError, "%s.%s is nonpayable", call.Destination, call.Method)
		}
		return nil
	case "layerxprograms.call":
		if value.Sign() != 0 {
			return refuse(CodeDecodeError, "%s.%s is nonpayable", call.Destination, call.Method)
		}
		program, programOK := call.Args["programId"].([32]byte)
		entrypoint, entryOK := call.Args["entrypoint"].(string)
		fee, feeOK := call.Args["feeLimit"].(*big.Int)
		if !programOK || !entryOK || !feeOK || fee == nil {
			return refuse(CodeDecodeError, "layerxprograms.call arguments are malformed")
		}
		rule, listed := ctx.programCalls[program]
		if !listed || !rule.entrypoints[entrypoint] || fee.Sign() < 0 || fee.Cmp(rule.feeLimit) > 0 {
			return refuse(CodeSelectorNotAllowed, "layerxprograms.call program, entrypoint or fee limit is not allowed")
		}
		out.ProgramCalls = append(out.ProgramCalls, ProgramCallInspection{ProgramID: program, Entrypoint: entrypoint, FeeLimit: new(big.Int).Set(fee)})
		return nil
	case "layerxpay.budgetAmend", "layerxpay.budgetDelegateAdd", "layerxpay.budgetDelegateRemove", "layerxpay.budgetClose", "layerxpay.budgetRevoke", "layerxpay.escrowRelease", "layerxpay.escrowTimeout":
		return nil
	}
	switch call.Destination + "." + call.Method {
	case "layerxprograms.transfer", "layerxprograms.sandboxFund", "layerxpay.budgetCreate", "layerxpay.budgetCreateNative", "layerxpay.budgetFund", "layerxpay.budgetFundNative", "layerxpay.escrowOpen":
	default:
		return refuse(CodeSelectorNotAllowed, "%s.%s has unresolved monetary effects", call.Destination, call.Method)
	}
	if value.Sign() != 0 {
		return refuse(CodeDecodeError, "%s.%s is nonpayable", call.Destination, call.Method)
	}
	state, err := ctx.constructState()
	if err != nil {
		return err
	}
	out.ReviewedBlock = state.pin.BlockHash
	effect := func(from, asset, to [32]byte, amount *big.Int, requireOwned bool) error {
		if amount == nil || amount.Sign() < 0 || amount.BitLen() > 128 {
			return refuse(CodeDecodeError, "invalid construct amount")
		}
		_, fromOwned, err := state.account(from, asset)
		if err != nil {
			return err
		}
		_, toOwned, err := state.account(to, asset)
		if err != nil {
			return err
		}
		if requireOwned && !fromOwned {
			return refuse(CodeDestinationDenied, "construct source %x is not owned by the signer", from)
		}
		out.Transfers = append(out.Transfers, ConstructTransfer{From: from, Asset: asset, To: to, Amount: new(big.Int).Set(amount), FromOwned: fromOwned, ToOwned: toOwned})
		if fromOwned {
			out.Spends = append(out.Spends, Spend{Asset: ConstructAsset(asset), Amount: new(big.Int).Set(amount)})
		}
		return nil
	}
	id := func(name string) [32]byte { return call.Args[name].([32]byte) }
	amount := func() *big.Int { return call.Args["amount"].(*big.Int) }
	switch call.Destination + "." + call.Method {
	case "layerxprograms.transfer", "layerxprograms.sandboxFund":
		arg := "legs"
		if call.Method == "sandboxFund" {
			arg = "fundingLegs"
		}
		legs := *abi.ConvertType(call.Args[arg], new([]constructLeg)).(*[]constructLeg)
		if len(legs) == 0 {
			return refuse(CodeDecodeError, "%s has no funding legs", call.Method)
		}
		total := new(big.Int)
		for _, leg := range legs {
			if call.Method == "sandboxFund" {
				if leg.Asset != id("asset") || leg.To != id("escrowAccount") {
					return refuse(CodeDecodeError, "sandbox funding leg does not match the escrow asset and destination")
				}
				total.Add(total, leg.Amount)
			}
			if err := effect(leg.From, leg.Asset, leg.To, leg.Amount, true); err != nil {
				return err
			}
		}
		if call.Method == "sandboxFund" && total.Cmp(amount()) != 0 {
			return refuse(CodeDecodeError, "sandbox funding legs do not match the declared amount")
		}
	case "layerxpay.budgetCreate", "layerxpay.budgetCreateNative":
		source := state.main
		if call.Method == "budgetCreateNative" {
			source = id("sourceAccount")
		}
		return effect(source, id("assetId"), id("budgetAccount"), amount(), true)
	case "layerxpay.budgetFund", "layerxpay.budgetFundNative":
		b, ok := state.budgets[id("budgetId")]
		if !ok || b.owner != state.main || b.closed || b.revoked || b.native != (call.Method == "budgetFundNative") {
			return refuse(CodeMissingField, "budget asset or source is unresolved or unavailable")
		}
		return effect(b.source, b.asset, b.account, amount(), true)
	case "layerxpay.escrowOpen":
		if id("owner") != state.main {
			return refuse(CodeDestinationDenied, "escrow owner is not the bound signer")
		}
		return effect(id("owner"), id("assetId"), id("escrowAccount"), amount(), true)
	}
	return nil
}
