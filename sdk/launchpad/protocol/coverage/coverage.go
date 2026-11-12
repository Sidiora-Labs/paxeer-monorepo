package coverage

import (
	_ "embed"
	"encoding/json"
)

//go:embed authorities.json
var authorities []byte

type Family struct {
	Family, Primary, Rule string
	Corroboration         []string
	Source                []string
	CoverageLimit         string `json:"coverage_limit"`
}

func Families() ([]Family, error) {
	var d struct{ Families []Family }
	e := json.Unmarshal(authorities, &d)
	return d.Families, e
}

type Rule struct {
	Kind, Effect             string
	RequiresCallerPermission bool
}

// Authority defaults to supplemental. A declared ABI alone is never proof of
// concrete emission. Callers still validate registered origin and permissions.
func Authority(role, event string) Rule {
	if role == roleEventEmitter {
		switch event {
		case "RoleGranted", "RoleRevoked", "RoleAdminChanged", eventUpgraded, "TypedEmitterPermission":
			return Rule{kindPrimary, effectControlTransition, false}
		case "Swap":
			return Rule{kindConditionalPrimary, effectCoreFill, true}
		case "PoolStateUpdated":
			return Rule{kindConditionalPrimary, effectReserveState, true}
		case "ConfigUpdated":
			return Rule{kindConditionalPrimary, "configuration", true}
		case "EventLog", "EventLog1", "EventLog2":
			return Rule{"supplemental", effectNone, true}
		default:
			return Rule{kindCorroboration, effectNone, true}
		}
	}
	if role == roleSidioraFactory && event == eventMarketCreated {
		return Rule{kindPrimary, effectMarketAttachment, false}
	}
	if role == "Router" {
		switch event {
		case eventBuy, "Sell", "MultihopSwap":
			return Rule{"route_primary", "route_execution", false}
		case eventMarketCreated:
			return Rule{kindCorroboration, effectNone, false}
		}
	}
	if role == roleSidioraERC20 || role == "SidioraNFT" {
		switch event {
		case "Transfer":
			return Rule{kindPrimary, "asset_movement", false}
		case "Approval", "ApprovalForAll":
			return Rule{kindPrimary, "asset_permission", false}
		case "PoolNFTMinted", "FeeStrategyChanged":
			return Rule{kindPrimary, "nft_strategy", false}
		}
	}
	if role == "FeeAccumulator" {
		switch event {
		case "FeeRecorded", eventFeesClaimed, "FeesBurned", "AirdropTriggered", "AirdropClaimed", "LpRewardsSent", "OpticalSurplusRecorded", "OpticalSurplusClaimed":
			return Rule{kindPrimary, event, false}
		case "ProtocolFeeSwept":
			return Rule{"declared_not_emitted", effectNone, false}
		}
	}
	if role == "PECOR" {
		switch event {
		case "SimpleSwap", "MarketOrderExecuted", "NativeSwap", "FeesCollected":
			return Rule{kindPrimary, event, false}
		case "TrackerCallFailed":
			return Rule{"diagnostic", "enrichment_missing", false}
		}
	}
	if role == "MetaAGRouter" && (event == "BestRouteSwap" || event == "MultiHopSwap") {
		return Rule{kindPrimary, "route_execution", false}
	}
	if role == "TransactionTracker" {
		return Rule{kindCorroboration, effectNone, false}
	}
	if role == "PECOROrders" {
		switch event {
		case "LimitOrderCreated", "StopLimitOrderCreated":
			return Rule{kindPrimary, effectOrderCreated, false}
		case "StopLimitActivated":
			return Rule{kindPrimary, effectOrderTriggered, false}
		case "LimitOrderExecuted", "StopLimitExecuted":
			return Rule{kindPrimary, effectOrderFilled, false}
		case "OrderExecutionFailed":
			return Rule{"diagnostic", "order_unchanged", false}
		case "OrderExpiredAndRefunded":
			return Rule{kindPrimary, effectExpiredRefunded, false}
		case "LimitOrderCancelled", "StopLimitCancelled":
			return Rule{kindPrimary, effectOrderCancelled, false}
		case "OrderMinimumOutputUpdated":
			return Rule{kindPrimary, effectMinimumOutput, false}
		}
	}
	if role == rolePECORVault {
		switch event {
		case "Deposit", "Withdrawal", eventNativeDeposit, "NativeWithdrawal", "EmergencyWithdraw":
			return Rule{kindPrimary, effectFundingMovement, false}
		case "ReservesUpdated", "ReservesSync":
			return Rule{kindPrimary, effectReserveState, false}
		case "TokenRegistered", "StablecoinStatusUpdated":
			return Rule{kindPrimary, "asset_configuration", false}
		}
	}
	if role == "OpticalRegistry" {
		switch event {
		case "OpticalDeregistered", "OpticalRegistered", "OpticalIdentityReviewed", "OpticalMetadataUpdated":
			return Rule{kindPrimary, "optical_configuration", false}
		}
	}
	switch event {
	case "RoleGranted", "RoleRevoked", eventUpgraded, "PauseToggled", "OwnershipTransferStarted", "OwnershipTransferred", "GuardianTransferStarted", "GuardianTransferred":
		return Rule{kindPrimary, effectControlTransition, false}
	}
	return Rule{"supplemental", effectNone, false}
}
func CoreSwapCoverage(mirrorAuthorized bool) string {
	if mirrorAuthorized {
		return "authorized_mirror_required"
	}
	return "coverage_gap_direct_swaps"
}
