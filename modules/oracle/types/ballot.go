package types

import (
	"math"
	"math/big"
	"sort"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

// NOTE: we don't need to implement proto interface on this file
//       these are not used in store or rpc response

// VoteForTally is a convenience wrapper to reduce redundant lookup cost
type VoteForTally struct {
	Denom        string
	ExchangeRate sdk.Dec
	Voter        sdk.ValAddress
	Power        int64
}

// NewVoteForTally returns a new VoteForTally instance
func NewVoteForTally(rate sdk.Dec, denom string, voter sdk.ValAddress, power int64) VoteForTally {
	return VoteForTally{
		ExchangeRate: rate,
		Denom:        denom,
		Voter:        voter,
		Power:        power,
	}
}

// ExchangeRateBallot is a convenience wrapper around a ExchangeRateVote slice
type ExchangeRateBallot []VoteForTally

// ToMap return organized exchange rate map by validator
func (pb ExchangeRateBallot) ToMap() map[string]sdk.Dec {
	exchangeRateMap := make(map[string]sdk.Dec)
	for _, vote := range pb {
		if vote.ExchangeRate.IsPositive() {
			exchangeRateMap[string(vote.Voter)] = vote.ExchangeRate
		}
	}

	return exchangeRateMap
}

// ToCrossRate return cross_rate(base/exchange_rate) ballot
func (pb ExchangeRateBallot) ToCrossRate(bases map[string]sdk.Dec) (cb ExchangeRateBallot) {
	for i := range pb {
		vote := pb[i]

		if exchangeRateRT, ok := bases[string(vote.Voter)]; ok && vote.ExchangeRate.IsPositive() {
			// Quo will panic on overflow, so we wrap it in a defer/recover
			func() {
				defer func() {
					if r := recover(); r != nil {
						// if overflow, set exchange rate to 0 and power to 0
						vote.ExchangeRate = sdk.ZeroDec()
						vote.Power = 0
					}
				}()
				vote.ExchangeRate = exchangeRateRT.Quo(vote.ExchangeRate)
			}()
		} else {
			// If we can't get reference Pax exchange rate, we just convert the vote as abstain vote
			vote.ExchangeRate = sdk.ZeroDec()
			vote.Power = 0
		}

		cb = append(cb, vote)
	}

	return
}

// ToCrossRateWithSort return cross_rate(base/exchange_rate) ballot
func (pb ExchangeRateBallot) ToCrossRateWithSort(bases map[string]sdk.Dec) (cb ExchangeRateBallot) {
	cb = pb.ToCrossRate(bases)
	sort.Sort(cb)
	return
}

// Power returns the total amount of voting power in the ballot
func (pb ExchangeRateBallot) Power() int64 {
	totalPower := int64(0)
	for _, vote := range pb {
		totalPower += vote.Power
	}

	return totalPower
}

// WeightedMedian returns the median weighted by the power of the ExchangeRateVote.
// CONTRACT: ballot must be sorted
func (pb ExchangeRateBallot) WeightedMedian() sdk.Dec {
	totalPower := pb.Power()
	if totalPower == 0 {
		return sdk.ZeroDec()
	}
	halfPower := totalPower/2 + totalPower%2
	if pb.Len() > 0 {
		pivot := int64(0)
		for _, v := range pb {
			votePower := v.Power
			if votePower == 0 {
				continue
			}

			pivot += votePower
			if pivot >= halfPower {
				return v.ExchangeRate
			}
		}
	}
	return sdk.ZeroDec()
}

// WeightedMedianWithAssertion returns the median weighted by the power of the ExchangeRateVote.
// CONTRACT: ballot must be sorted
func (pb ExchangeRateBallot) WeightedMedianWithAssertion() sdk.Dec {
	if !sort.IsSorted(pb) {
		panic("ballot must be sorted")
	}

	totalPower := pb.Power()
	if totalPower == 0 {
		return sdk.ZeroDec()
	}
	halfPower := totalPower/2 + totalPower%2
	if pb.Len() > 0 {
		pivot := int64(0)
		for _, v := range pb {
			votePower := v.Power
			if votePower == 0 {
				continue
			}

			pivot += votePower
			if pivot >= halfPower {
				return v.ExchangeRate
			}
		}
	}
	return sdk.ZeroDec()
}

// StandardDeviation is the voting-power weighted root mean square distance from
// the weighted median: sqrt(sum(power * (rate - median)^2) / sum(power)).
// Zero-power observations are excluded, including their rates. The integer square
// root rounds down to sdk.Precision decimal places, without intermediate decimal
// rounding. Invalid inputs or an unrepresentable result panic rather than become
// a valid zero spread. Positive power must fit the ballot's int64 power contract.
func (pb ExchangeRateBallot) StandardDeviation(median sdk.Dec) sdk.Dec {
	if median.IsNil() || !median.IsInValidRange() {
		panic("oracle deviation: invalid median")
	}
	totalPower := int64(0)
	sum := new(big.Int)
	for _, vote := range pb {
		if vote.Power == 0 {
			continue
		}
		if vote.Power < 0 || vote.Power > math.MaxInt64-totalPower {
			panic("oracle deviation: invalid or overflowing voting power")
		}
		if vote.ExchangeRate.IsNil() || !vote.ExchangeRate.IsInValidRange() {
			panic("oracle deviation: invalid exchange rate")
		}
		totalPower += vote.Power
		deviation := new(big.Int).Sub(vote.ExchangeRate.BigInt(), median.BigInt())
		squared := new(big.Int).Mul(deviation, deviation)
		sum.Add(sum, squared.Mul(squared, big.NewInt(vote.Power)))
	}
	if totalPower == 0 {
		return sdk.ZeroDec()
	}
	// Valid rates have at most 315 bits and total power at most 63 bits,
	// bounding the accumulator to 695 bits even for opposite signed rates.
	variance := sum.Quo(sum, big.NewInt(totalPower))
	root := new(big.Int).Sqrt(variance)
	return sdk.NewDecFromBigIntWithPrec(root, sdk.Precision)
}

// Len implements sort.Interface
func (pb ExchangeRateBallot) Len() int {
	return len(pb)
}

// Less reports whether the element with
// index i should sort before the element with index j.
func (pb ExchangeRateBallot) Less(i, j int) bool {
	return pb[i].ExchangeRate.LT(pb[j].ExchangeRate)
}

// Swap implements sort.Interface.
func (pb ExchangeRateBallot) Swap(i, j int) {
	pb[i], pb[j] = pb[j], pb[i]
}

// Claim is an interface that directs its rewards to an attached bank account.
type Claim struct {
	Power     int64
	Weight    int64
	WinCount  int64
	DidVote   bool
	Recipient sdk.ValAddress
}

// NewClaim generates a Claim instance.
func NewClaim(power, weight, winCount int64, recipient sdk.ValAddress, didVote bool) Claim {
	return Claim{
		Power:     power,
		Weight:    weight,
		WinCount:  winCount,
		DidVote:   didVote,
		Recipient: recipient,
	}
}
