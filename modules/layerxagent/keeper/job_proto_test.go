package keeper_test

import (
	"bytes"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxagent/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxagent/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
)

func TestAgentJobProtoRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name    string
		deposit string
		active  bool
		data    []byte
	}{
		{"populated", "123456789012345678901234567890", true, []byte{0, 0xde, 0xad, 0xff}},
		{"zero deposit inactive", "0", false, nil},
		{"maximum deposit", "115792089237316195423570985008687907853269984665640564039457584007913129639935", true, []byte{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			amount, ok := sdk.NewIntFromString(tc.deposit)
			if !ok {
				t.Fatalf("invalid test deposit %q", tc.deposit)
			}
			job := keeper.Job{
				ID:             math.MaxUint64,
				Creator:        common.HexToAddress("0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed"),
				Target:         common.HexToAddress("0xfB6916095ca1df60bB79Ce92cE3Ea74c37c5d359"),
				CallData:       tc.data,
				ExecuteAtBlock: math.MaxUint64 - 1,
				GasLimit:       math.MaxUint64 - 2,
				Deposit:        amount,
				Active:         tc.active,
			}
			encoded := keeper.JobToProto(job)
			want := types.Job{
				Id:             job.ID,
				Creator:        "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed",
				Target:         "0xfB6916095ca1df60bB79Ce92cE3Ea74c37c5d359",
				CallData:       append([]byte(nil), tc.data...),
				ExecuteAtBlock: job.ExecuteAtBlock,
				GasLimit:       job.GasLimit,
				Deposit:        tc.deposit,
				Active:         tc.active,
			}
			if !reflect.DeepEqual(encoded, want) {
				t.Fatalf("JobToProto = %+v, want %+v", encoded, want)
			}
			decoded, err := keeper.JobFromProto(encoded)
			if err != nil {
				t.Fatalf("JobFromProto: %v", err)
			}
			if decoded.ID != job.ID || decoded.Creator != job.Creator || decoded.Target != job.Target ||
				!bytes.Equal(decoded.CallData, job.CallData) || decoded.ExecuteAtBlock != job.ExecuteAtBlock ||
				decoded.GasLimit != job.GasLimit || !decoded.Deposit.Equal(job.Deposit) || decoded.Active != job.Active {
				t.Fatalf("job changed in round trip: got %+v, want %+v", decoded, job)
			}
			if len(job.CallData) > 0 {
				job.CallData[0] = 1
				if encoded.CallData[0] != 0 || decoded.CallData[0] != 0 {
					t.Fatal("converted call data aliases keeper input")
				}
				encoded.CallData[0] = 2
				if job.CallData[0] != 1 || decoded.CallData[0] != 0 {
					t.Fatal("converted call data aliases proto input")
				}
				decoded.CallData[0] = 3
				if job.CallData[0] != 1 || encoded.CallData[0] != 2 {
					t.Fatal("decoded call data mutation changed input")
				}
			}
		})
	}
}

func TestAgentJobFromProtoRefusesMalformed(t *testing.T) {
	valid := types.Job{
		Id: 1, Creator: "0x5aaeb6053f3e94c9b9a09f33669435e7ef1beaed",
		Target: "0xfb6916095ca1df60bb79ce92ce3ea74c37c5d359", CallData: []byte{1, 2, 3},
		ExecuteAtBlock: 42, GasLimit: 21000, Deposit: "1000000000000000000", Active: true,
	}
	if _, err := keeper.JobFromProto(valid); err != nil {
		t.Fatalf("valid lowercase addresses refused: %v", err)
	}
	for _, decimal := range []string{"0", "000", "010", "009"} {
		input := valid
		input.Deposit = decimal
		got, err := keeper.JobFromProto(input)
		want := strings.TrimLeft(decimal, "0")
		if want == "" {
			want = "0"
		}
		if err != nil {
			t.Fatalf("valid decimal deposit %q refused: %v", decimal, err)
		}
		if got.Deposit.String() != want {
			t.Fatalf("decimal deposit %q decoded as %s, want %s", decimal, got.Deposit, want)
		}
	}
	for _, address := range []string{
		"", "pax1invalid", valid.Creator[2:], "0X" + valid.Creator[2:],
		"0x1234", "0x" + strings.Repeat("1", 38), "0x" + strings.Repeat("1", 39),
		"0x" + strings.Repeat("1", 41), "0x" + strings.Repeat("1", 42),
		"0x" + strings.Repeat("z", 40), " " + valid.Creator, valid.Creator + " ",
	} {
		for _, field := range []string{"creator", "target"} {
			t.Run(field+"/"+address, func(t *testing.T) {
				input := valid
				if field == "creator" {
					input.Creator = address
				} else {
					input.Target = address
				}
				got, err := keeper.JobFromProto(input)
				if err == nil || !strings.Contains(err.Error(), field+" must be a 0x hex address") {
					t.Fatalf("invalid %s %q: got error %v", field, address, err)
				}
				if !reflect.DeepEqual(got, keeper.Job{}) {
					t.Fatalf("malformed input returned a partial job: %+v", got)
				}
			})
		}
	}
	for _, deposit := range []string{
		"", "-1", "-1000000000000000000", "1.5", "1e3", "not-an-integer", " 1", "1 ", "0x10",
		"115792089237316195423570985008687907853269984665640564039457584007913129639936",
	} {
		t.Run("deposit/"+deposit, func(t *testing.T) {
			input := valid
			input.Deposit = deposit
			got, err := keeper.JobFromProto(input)
			if err == nil || !strings.Contains(err.Error(), "deposit must be a nonnegative integer") {
				t.Fatalf("invalid deposit %q: got error %v", deposit, err)
			}
			if !reflect.DeepEqual(got, keeper.Job{}) {
				t.Fatalf("malformed input returned a partial job: %+v", got)
			}
		})
	}
}
