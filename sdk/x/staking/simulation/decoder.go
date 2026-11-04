package simulation

import (
	"bytes"
	"fmt"

	gogotypes "github.com/gogo/protobuf/types"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/kv"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
)

// NewDecodeStore returns a decoder function closure that unmarshals the KVPair's
// Value to the corresponding staking type.
func NewDecodeStore(cdc codec.Codec) func(kvA, kvB kv.Pair) string {
	return func(kvA, kvB kv.Pair) string {
		switch {
		case bytes.Equal(kvA.Key[:1], types.LastTotalPowerKey):
			var powerA, powerB sdk.IntProto

			cdc.MustUnmarshal(kvA.Value, &powerA)
			cdc.MustUnmarshal(kvB.Value, &powerB)

			return fmt.Sprintf("%v\n%v", powerA, powerB)
		case bytes.Equal(kvA.Key[:1], types.ValidatorsKey):
			var validatorA, validatorB types.Validator

			cdc.MustUnmarshal(kvA.Value, &validatorA)
			cdc.MustUnmarshal(kvB.Value, &validatorB)

			return fmt.Sprintf("%v\n%v", validatorA, validatorB)
		case bytes.Equal(kvA.Key[:1], types.LastValidatorPowerKey):
			var powerA, powerB gogotypes.Int64Value

			cdc.MustUnmarshal(kvA.Value, &powerA)
			cdc.MustUnmarshal(kvB.Value, &powerB)

			return fmt.Sprintf("%v\n%v", powerA.Value, powerB.Value)
		case bytes.Equal(kvA.Key[:1], types.ValidatorsByConsAddrKey),
			bytes.Equal(kvA.Key[:1], types.ValidatorsByPowerIndexKey):
			return fmt.Sprintf("%v\n%v", sdk.ValAddress(kvA.Value), sdk.ValAddress(kvB.Value))

		case bytes.Equal(kvA.Key[:1], types.DelegationKey):
			var delegationA, delegationB types.Delegation

			cdc.MustUnmarshal(kvA.Value, &delegationA)
			cdc.MustUnmarshal(kvB.Value, &delegationB)

			return fmt.Sprintf("%v\n%v", delegationA, delegationB)
		case bytes.Equal(kvA.Key[:1], types.UnbondingDelegationByValIndexKey),
			bytes.Equal(kvA.Key[:1], types.RedelegationByValSrcIndexKey),
			bytes.Equal(kvA.Key[:1], types.RedelegationByValDstIndexKey):
			return fmt.Sprintf("%X\n%X", kvA.Key, kvB.Key)
		case bytes.Equal(kvA.Key[:1], types.UnbondingDelegationKey):
			var ubdA, ubdB types.UnbondingDelegation

			cdc.MustUnmarshal(kvA.Value, &ubdA)
			cdc.MustUnmarshal(kvB.Value, &ubdB)

			return fmt.Sprintf("%v\n%v", ubdA, ubdB)
		case bytes.Equal(kvA.Key[:1], types.RedelegationKey):
			var redA, redB types.Redelegation

			cdc.MustUnmarshal(kvA.Value, &redA)
			cdc.MustUnmarshal(kvB.Value, &redB)

			return fmt.Sprintf("%v\n%v", redA, redB)
		case bytes.Equal(kvA.Key[:1], types.UnbondingQueueKey):
			var pairsA, pairsB types.DVPairs

			cdc.MustUnmarshal(kvA.Value, &pairsA)
			cdc.MustUnmarshal(kvB.Value, &pairsB)

			return fmt.Sprintf("%v\n%v", pairsA, pairsB)
		case bytes.Equal(kvA.Key[:1], types.RedelegationQueueKey):
			var tripletsA, tripletsB types.DVVTriplets

			cdc.MustUnmarshal(kvA.Value, &tripletsA)
			cdc.MustUnmarshal(kvB.Value, &tripletsB)

			return fmt.Sprintf("%v\n%v", tripletsA, tripletsB)
		case bytes.Equal(kvA.Key[:1], types.ValidatorQueueKey):
			var addressesA, addressesB types.ValAddresses

			cdc.MustUnmarshal(kvA.Value, &addressesA)
			cdc.MustUnmarshal(kvB.Value, &addressesB)

			return fmt.Sprintf("%v\n%v", addressesA, addressesB)
		case bytes.Equal(kvA.Key[:1], types.HistoricalInfoKey):
			var historicalA, historicalB types.HistoricalInfo

			cdc.MustUnmarshal(kvA.Value, &historicalA)
			cdc.MustUnmarshal(kvB.Value, &historicalB)

			return fmt.Sprintf("%v\n%v", historicalA, historicalB)
		default:
			panic(fmt.Sprintf("invalid staking key prefix %X", kvA.Key[:1]))
		}
	}
}
