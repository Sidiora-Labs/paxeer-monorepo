package keeper_test

import "testing"

func BenchmarkGetValidator(b *testing.B) {
	// 900 is the max number we are allowed to use in order to avoid simapp.CreateTestPubKeys
	// panic: encoding/hex: odd length hex string
	var powersNumber = 900

	var totalPower int64 = 0
	var powers = make([]int64, powersNumber)
	for i := range powers {
		powers[i] = int64(i)
		totalPower += int64(i)
	}

	app, ctx, _, valAddrs, vals := initValidators(b, totalPower, len(powers), powers)

	for _, validator := range vals {
		app.StakingKeeper.SetValidator(ctx, validator)
	}

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		for _, addr := range valAddrs {
			validator, found := app.StakingKeeper.GetValidator(ctx, addr)
			if !found || !validator.GetOperator().Equals(addr) {
				b.Fatalf("validator store read failed for %s", addr)
			}
		}
	}
}
