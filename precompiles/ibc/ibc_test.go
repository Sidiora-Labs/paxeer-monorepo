package ibc_test

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmcfg "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	transfer "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/transfer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/transfer/types"
	clienttypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/02-client/types"
	connectiontypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/03-connection/types"
	channeltypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/04-channel/types"
	commitmenttypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/23-commitment/types"
	host "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/24-host"
	"github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/exported"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/ibc"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/utils"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

type MockTransferKeeper struct{}

func (tk *MockTransferKeeper) Transfer(goCtx context.Context, msg *types.MsgTransfer) (*types.MsgTransferResponse, error) {
	return nil, nil
}

func (tk *MockTransferKeeper) SendTransfer(
	ctx sdk.Context,
	sourcePort,
	sourceChannel string,
	token sdk.Coin,
	sender sdk.AccAddress,
	receiver string,
	timeoutHeight clienttypes.Height,
	timeoutTimestamp uint64,
) error {
	return nil
}

type MockMemoTransferKeeper struct {
	t        require.TestingT
	wantMemo string
}

func (tk *MockMemoTransferKeeper) Transfer(goCtx context.Context, msg *types.MsgTransfer) (*types.MsgTransferResponse, error) {
	require.Equal(tk.t, tk.wantMemo, msg.Memo)
	return nil, nil
}

func (tk *MockMemoTransferKeeper) SendTransfer(
	ctx sdk.Context,
	sourcePort,
	sourceChannel string,
	token sdk.Coin,
	sender sdk.AccAddress,
	receiver string,
	timeoutHeight clienttypes.Height,
	timeoutTimestamp uint64,
) error {
	return nil
}

type MockFailedTransferTransferKeeper struct{}

func (tk *MockFailedTransferTransferKeeper) Transfer(goCtx context.Context, msg *types.MsgTransfer) (*types.MsgTransferResponse, error) {
	return nil, errors.New("failed to send transfer")
}

func (tk *MockFailedTransferTransferKeeper) SendTransfer(
	ctx sdk.Context,
	sourcePort,
	sourceChannel string,
	token sdk.Coin,
	sender sdk.AccAddress,
	receiver string,
	timeoutHeight clienttypes.Height,
	timeoutTimestamp uint64,
) error {
	return nil
}

func TestPrecompile_Run(t *testing.T) {
	senderPaxAddress, senderEvmAddress := testkeeper.MockAddressPair()
	receiverAddress := "cosmos1yykwxjzr2tv4mhx5tsf8090sdg96f2ax8fydk2"

	pre, _ := ibc.NewPrecompile(&utils.EmptyKeepers{})
	testTransfer, _ := pre.ABI.MethodById(pre.GetExecutor().(*ibc.PrecompileExecutor).TransferID)
	packedTrue, _ := testTransfer.Outputs.Pack(true)

	type fields struct {
		transferKeeper utils.TransferKeeper
	}

	type input struct {
		receiverAddr     string
		sourcePort       string
		sourceChannel    string
		denom            string
		amount           *big.Int
		revisionNumber   uint64
		revisionHeight   uint64
		timeoutTimestamp uint64
		memo             string
	}
	type args struct {
		caller             common.Address
		callingContract    common.Address
		input              *input
		suppliedGas        uint64
		value              *big.Int
		isFromDelegateCall bool
	}

	commonArgs := args{
		caller:          senderEvmAddress,
		callingContract: senderEvmAddress,
		input: &input{
			receiverAddr:     receiverAddress,
			sourcePort:       "transfer",
			sourceChannel:    "channel-0",
			denom:            "denom",
			amount:           big.NewInt(100),
			revisionNumber:   1,
			revisionHeight:   1,
			timeoutTimestamp: 1,
		},
		suppliedGas: uint64(1000000),
		value:       nil,
	}

	tests := []struct {
		name             string
		fields           fields
		args             args
		wantBz           []byte
		wantRemainingGas uint64
		wantErr          bool
		wantErrMsg       string
	}{
		{
			name:             "successful transfer: with amount > 0 between EVM addresses",
			fields:           fields{transferKeeper: &MockTransferKeeper{}},
			args:             commonArgs,
			wantBz:           packedTrue,
			wantRemainingGas: 998877,
			wantErr:          false,
		},
		{
			name:       "failed transfer: internal error",
			fields:     fields{transferKeeper: &MockFailedTransferTransferKeeper{}},
			args:       commonArgs,
			wantBz:     nil,
			wantErr:    true,
			wantErrMsg: "failed to send transfer",
		},
		{
			name:       "failed transfer: caller not whitelisted",
			fields:     fields{transferKeeper: &MockTransferKeeper{}},
			args:       args{caller: senderEvmAddress, callingContract: common.Address{}, input: commonArgs.input, suppliedGas: 1000000, value: nil, isFromDelegateCall: true},
			wantBz:     nil,
			wantErr:    true,
			wantErrMsg: "cannot delegatecall IBC",
		},
		{
			name:       "failed transfer: value is not nil",
			fields:     fields{transferKeeper: &MockTransferKeeper{}},
			args:       args{caller: senderEvmAddress, callingContract: common.Address{}, input: commonArgs.input, suppliedGas: 1000000, value: big.NewInt(100), isFromDelegateCall: true},
			wantBz:     nil,
			wantErr:    true,
			wantErrMsg: "sending funds to a non-payable function",
		},
		{
			name:   "failed transfer: empty sourcePort",
			fields: fields{transferKeeper: &MockTransferKeeper{}},
			args: args{
				caller:          senderEvmAddress,
				callingContract: senderEvmAddress,
				input: &input{
					receiverAddr:     receiverAddress,
					sourcePort:       "", // empty sourcePort
					sourceChannel:    "channel-0",
					denom:            "denom",
					amount:           big.NewInt(100),
					revisionNumber:   1,
					revisionHeight:   1,
					timeoutTimestamp: 1,
				},
				suppliedGas: uint64(1000000),
				value:       nil,
			},
			wantBz:     nil,
			wantErr:    true,
			wantErrMsg: "port cannot be empty",
		},
		{
			name:   "failed transfer: empty sourceChannel",
			fields: fields{transferKeeper: &MockTransferKeeper{}},
			args: args{
				caller:          senderEvmAddress,
				callingContract: senderEvmAddress,
				input: &input{
					receiverAddr:     receiverAddress,
					sourcePort:       "transfer",
					sourceChannel:    "",
					denom:            "denom",
					amount:           big.NewInt(100),
					revisionNumber:   1,
					revisionHeight:   1,
					timeoutTimestamp: 1,
				},
				suppliedGas: uint64(1000000),
				value:       nil,
			},
			wantBz:     nil,
			wantErr:    true,
			wantErrMsg: "channelID cannot be empty",
		},
		{
			name:   "failed transfer: invalid denom",
			fields: fields{transferKeeper: &MockTransferKeeper{}},
			args: args{
				caller:          senderEvmAddress,
				callingContract: senderEvmAddress,
				input: &input{
					receiverAddr:     receiverAddress,
					sourcePort:       "transfer",
					sourceChannel:    "channel-0",
					denom:            "",
					amount:           big.NewInt(100),
					revisionNumber:   1,
					revisionHeight:   1,
					timeoutTimestamp: 1,
				},
				suppliedGas: uint64(1000000),
				value:       nil,
			},
			wantBz:     nil,
			wantErr:    true,
			wantErrMsg: "invalid denom",
		},
		{
			name:   "failed transfer: invalid receiver address",
			fields: fields{transferKeeper: &MockTransferKeeper{}},
			args: args{
				caller:          senderEvmAddress,
				callingContract: senderEvmAddress,
				input: &input{
					receiverAddr:     "",
					sourcePort:       "transfer",
					sourceChannel:    "channel-0",
					denom:            "",
					amount:           big.NewInt(100),
					revisionNumber:   1,
					revisionHeight:   1,
					timeoutTimestamp: 1,
				},
				suppliedGas: uint64(1000000),
				value:       nil,
			},
			wantBz:     nil,
			wantErr:    true,
			wantErrMsg: "receiverAddress is not a string or empty",
		},
		{
			name:   "memo is added to the transfer if passed",
			fields: fields{transferKeeper: &MockMemoTransferKeeper{t: t, wantMemo: "test memo"}},
			args: args{
				caller:          senderEvmAddress,
				callingContract: senderEvmAddress,
				input: &input{
					receiverAddr:  receiverAddress,
					sourcePort:    "transfer",
					sourceChannel: "channel-0",
					denom:         "uhpx",
					amount:        big.NewInt(100),
					memo:          "test memo",
				},
				suppliedGas: uint64(1000000),
				value:       nil,
			},
			wantBz:           packedTrue,
			wantRemainingGas: 998877,
			wantErr:          false,
		},
		{
			name:   "memo is not added to the transfer if not passed",
			fields: fields{transferKeeper: &MockMemoTransferKeeper{t: t, wantMemo: ""}},
			args: args{
				caller:          senderEvmAddress,
				callingContract: senderEvmAddress,
				input: &input{
					receiverAddr:  receiverAddress,
					sourcePort:    "transfer",
					sourceChannel: "channel-0",
					denom:         "uhpx",
					amount:        big.NewInt(100),
				},
				suppliedGas: uint64(1000000),
				value:       nil,
			},
			wantBz:           packedTrue,
			wantRemainingGas: 998877,
			wantErr:          false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testApp := testkeeper.EVMTestApp
			ctx := testApp.NewContext(false, tmtypes.Header{}).WithBlockHeight(2)
			k := &testApp.EvmKeeper
			k.SetAddressMapping(ctx, senderPaxAddress, senderEvmAddress)
			stateDb := state.NewDBImpl(ctx, k, true)
			evm := vm.EVM{
				StateDB:   stateDb,
				TxContext: vm.TxContext{Origin: senderEvmAddress},
			}
			p, _ := ibc.NewPrecompile(&app.PrecompileKeepers{
				TransferKeeper: tt.fields.transferKeeper,
				EVMKeeper:      k,
			})
			transfer, err := p.ABI.MethodById(p.GetExecutor().(*ibc.PrecompileExecutor).TransferID)
			require.Nil(t, err)
			inputs, err := transfer.Inputs.Pack(tt.args.input.receiverAddr,
				tt.args.input.sourcePort, tt.args.input.sourceChannel, tt.args.input.denom, tt.args.input.amount,
				tt.args.input.revisionNumber, tt.args.input.revisionHeight, tt.args.input.timeoutTimestamp,
				tt.args.input.memo)
			require.Nil(t, err)
			gotBz, gotRemainingGas, err := p.RunAndCalculateGas(&evm, tt.args.caller, tt.args.callingContract, append(p.GetExecutor().(*ibc.PrecompileExecutor).TransferID, inputs...), tt.args.suppliedGas, tt.args.value, nil, false, tt.args.isFromDelegateCall)
			if (err != nil) != tt.wantErr {
				t.Errorf("Run() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				require.Equal(t, vm.ErrExecutionReverted, err)
				require.Nil(t, gotBz)
			} else if !reflect.DeepEqual(gotBz, tt.wantBz) {
				t.Errorf("Run() gotRet = %v, want %v", gotBz, tt.wantBz)
			}

			if !reflect.DeepEqual(gotRemainingGas, tt.wantRemainingGas) {
				t.Errorf("Run() gotRemainingGas = %v, want %v", gotRemainingGas, tt.wantRemainingGas)
			}
		})
	}
}

func TestTransferWithDefaultTimeoutPrecompile_Run(t *testing.T) {
	senderPaxAddress, senderEvmAddress := testkeeper.MockAddressPair()
	receiverAddress := "cosmos1yykwxjzr2tv4mhx5tsf8090sdg96f2ax8fydk2"

	type fields struct {
		transferKeeper   utils.TransferKeeper
		clientKeeper     utils.ClientKeeper
		connectionKeeper utils.ConnectionKeeper
		channelKeeper    utils.ChannelKeeper
	}

	type input struct {
		receiverAddr  string
		sourcePort    string
		sourceChannel string
		denom         string
		amount        *big.Int
		memo          string
	}
	type args struct {
		caller             common.Address
		callingContract    common.Address
		input              *input
		suppliedGas        uint64
		value              *big.Int
		isFromDelegateCall bool
	}

	commonArgs := args{
		caller:          senderEvmAddress,
		callingContract: senderEvmAddress,
		input: &input{
			receiverAddr:  receiverAddress,
			sourcePort:    "transfer",
			sourceChannel: "channel-0",
			denom:         "denom",
			amount:        big.NewInt(100),
		},
		suppliedGas: uint64(1000000),
		value:       nil,
	}

	tests := []struct {
		name             string
		fields           fields
		args             args
		wantBz           []byte
		wantRemainingGas uint64
		wantErr          bool
		wantErrMsg       string
	}{
		{
			name:       "failed transfer: caller not whitelisted",
			fields:     fields{transferKeeper: &MockTransferKeeper{}},
			args:       args{caller: senderEvmAddress, callingContract: common.Address{}, input: commonArgs.input, suppliedGas: 1000000, value: nil, isFromDelegateCall: true},
			wantBz:     nil,
			wantErr:    true,
			wantErrMsg: "cannot delegatecall IBC",
		},
		{
			name:       "failed transfer: value is not nil",
			fields:     fields{transferKeeper: &MockTransferKeeper{}},
			args:       args{caller: senderEvmAddress, callingContract: common.Address{}, input: commonArgs.input, suppliedGas: 1000000, value: big.NewInt(100), isFromDelegateCall: true},
			wantBz:     nil,
			wantErr:    true,
			wantErrMsg: "sending funds to a non-payable function",
		},
		{
			name:   "failed transfer: empty sourcePort",
			fields: fields{transferKeeper: &MockTransferKeeper{}},
			args: args{
				caller:          senderEvmAddress,
				callingContract: senderEvmAddress,
				input: &input{
					receiverAddr:  receiverAddress,
					sourcePort:    "", // empty sourcePort
					sourceChannel: "channel-0",
					denom:         "denom",
					amount:        big.NewInt(100),
				},
				suppliedGas: uint64(1000000),
				value:       nil,
			},
			wantBz:     nil,
			wantErr:    true,
			wantErrMsg: "port cannot be empty",
		},
		{
			name:   "failed transfer: empty sourceChannel",
			fields: fields{transferKeeper: &MockTransferKeeper{}},
			args: args{
				caller:          senderEvmAddress,
				callingContract: senderEvmAddress,
				input: &input{
					receiverAddr:  receiverAddress,
					sourcePort:    "transfer",
					sourceChannel: "",
					denom:         "denom",
					amount:        big.NewInt(100),
				},
				suppliedGas: uint64(1000000),
				value:       nil,
			},
			wantBz:     nil,
			wantErr:    true,
			wantErrMsg: "channelID cannot be empty",
		},
		{
			name:   "failed transfer: invalid denom",
			fields: fields{transferKeeper: &MockTransferKeeper{}},
			args: args{
				caller:          senderEvmAddress,
				callingContract: senderEvmAddress,
				input: &input{
					receiverAddr:  receiverAddress,
					sourcePort:    "transfer",
					sourceChannel: "channel-0",
					denom:         "",
					amount:        big.NewInt(100),
				},
				suppliedGas: uint64(1000000),
				value:       nil,
			},
			wantBz:     nil,
			wantErr:    true,
			wantErrMsg: "invalid denom",
		},
		{
			name:   "failed transfer: invalid receiver address",
			fields: fields{transferKeeper: &MockTransferKeeper{}},
			args: args{
				caller:          senderEvmAddress,
				callingContract: senderEvmAddress,
				input: &input{
					receiverAddr:  "",
					sourcePort:    "transfer",
					sourceChannel: "channel-0",
					denom:         "",
					amount:        big.NewInt(100),
				},
				suppliedGas: uint64(1000000),
				value:       nil,
			},
			wantBz:     nil,
			wantErr:    true,
			wantErrMsg: "receiverAddress is not a string or empty",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testApp := testkeeper.EVMTestApp
			ctx := testApp.NewContext(false, tmtypes.Header{}).WithBlockHeight(2)
			k := &testApp.EvmKeeper
			k.SetAddressMapping(ctx, senderPaxAddress, senderEvmAddress)
			stateDb := state.NewDBImpl(ctx, k, true)
			evm := vm.EVM{
				StateDB:   stateDb,
				TxContext: vm.TxContext{Origin: senderEvmAddress},
			}

			p, _ := ibc.NewPrecompile(&app.PrecompileKeepers{
				TransferKeeper:   tt.fields.transferKeeper,
				EVMKeeper:        k,
				ClientKeeper:     tt.fields.clientKeeper,
				ChannelKeeper:    tt.fields.channelKeeper,
				ConnectionKeeper: tt.fields.connectionKeeper,
			})
			transfer, err := p.ABI.MethodById(p.GetExecutor().(*ibc.PrecompileExecutor).TransferWithDefaultTimeoutID)
			require.Nil(t, err)
			inputs, err := transfer.Inputs.Pack(tt.args.input.receiverAddr,
				tt.args.input.sourcePort, tt.args.input.sourceChannel, tt.args.input.denom, tt.args.input.amount,
				tt.args.input.memo)
			require.Nil(t, err)
			gotBz, gotRemainingGas, err := p.RunAndCalculateGas(&evm,
				tt.args.caller,
				tt.args.callingContract,
				append(p.GetExecutor().(*ibc.PrecompileExecutor).TransferWithDefaultTimeoutID, inputs...),
				tt.args.suppliedGas,
				tt.args.value,
				nil,
				false,
				tt.args.isFromDelegateCall,
			)
			if (err != nil) != tt.wantErr {
				t.Errorf("Run() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				require.Equal(t, vm.ErrExecutionReverted, err)
				require.Nil(t, gotBz)
			} else if !reflect.DeepEqual(gotBz, tt.wantBz) {
				t.Errorf("Run() gotRet = %v, want %v", gotBz, tt.wantBz)
			}
			if !reflect.DeepEqual(gotRemainingGas, tt.wantRemainingGas) {
				t.Errorf("Run() gotRemainingGas = %v, want %v", gotRemainingGas, tt.wantRemainingGas)
			}
		})
	}
}

func TestPrecompile_GetAdjustedHeight(t *testing.T) {
	type args struct {
		latestConsensusHeight clienttypes.Height
	}
	tests := []struct {
		name    string
		args    args
		want    clienttypes.Height
		wantErr bool
	}{
		{
			name: "height is adjusted with defaults",
			args: args{
				latestConsensusHeight: clienttypes.NewHeight(2, 3),
			},
			want: clienttypes.Height{
				RevisionNumber: 2,
				RevisionHeight: 1003,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ibc.GetAdjustedHeight(tt.args.latestConsensusHeight)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetAdjustedHeight() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetAdjustedHeight() got = %v, want %v", got, tt.want)
			}
		})
	}
}

type MockClientKeeper struct {
	consensusState       *MockConsensusState
	returnConsensusState bool
}

func (ck *MockClientKeeper) GetClientState(ctx sdk.Context, clientID string) (exported.ClientState, bool) {
	return nil, false
}

func (ck *MockClientKeeper) GetClientConsensusState(ctx sdk.Context, clientID string, height exported.Height) (exported.ConsensusState, bool) {
	if ck.consensusState == nil {
		return nil, ck.returnConsensusState
	}
	return ck.consensusState, ck.returnConsensusState
}

type MockConsensusState struct {
	timestamp uint64
}

func (m *MockConsensusState) Reset() {
	panic("implement me")
}

func (m *MockConsensusState) String() string {
	panic("implement me")
}

func (m *MockConsensusState) ProtoMessage() {
	panic("implement me")
}

func (m *MockConsensusState) ClientType() string {
	return "mock"
}

func (m *MockConsensusState) GetRoot() exported.Root {
	return nil
}

func (m *MockConsensusState) GetTimestamp() uint64 {
	return m.timestamp
}

func (m *MockConsensusState) ValidateBasic() error {
	return nil
}

func TestPrecompile_GetAdjustedTimestamp(t *testing.T) {
	type fields struct {
		transferKeeper   utils.TransferKeeper
		evmKeeper        utils.EVMKeeper
		clientKeeper     utils.ClientKeeper
		connectionKeeper utils.ConnectionKeeper
		channelKeeper    utils.ChannelKeeper
	}
	type args struct {
		ctx      sdk.Context
		clientId string
		height   clienttypes.Height
	}
	timestampSeconds := 1714680155
	ctx := sdk.Context{}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    uint64
		wantErr string
	}{
		{
			name: "if consensus timestamp is less than the given time, return the given time adjusted with default",
			fields: fields{
				clientKeeper: &MockClientKeeper{
					consensusState: &MockConsensusState{
						timestamp: uint64(timestampSeconds-1) * 1_000_000_000,
					},
					returnConsensusState: true,
				},
			},
			args: args{
				ctx: ctx.WithBlockTime(time.Unix(int64(timestampSeconds), 0)),
			},
			want:    uint64(timestampSeconds)*1_000_000_000 + uint64((time.Duration(10) * time.Minute).Nanoseconds()),
			wantErr: "",
		},
		{
			name: "if consensus state is not found, refuse even with a valid block time",
			fields: fields{
				clientKeeper: &MockClientKeeper{
					returnConsensusState: false,
				},
			},
			args: args{
				ctx:      ctx.WithBlockTime(time.Unix(int64(timestampSeconds), 0)),
				clientId: "07-tendermint-0",
				height:   clienttypes.NewHeight(1, 10),
			},
			want:    0,
			wantErr: "consensus state for client 07-tendermint-0 at height 1-10 not found",
		},
		{
			name: "if consensus state is nil despite being found, refuse",
			fields: fields{
				clientKeeper: &MockClientKeeper{
					returnConsensusState: true,
				},
			},
			args: args{
				ctx:      ctx.WithBlockTime(time.Unix(int64(timestampSeconds), 0)),
				clientId: "07-tendermint-0",
				height:   clienttypes.NewHeight(1, 10),
			},
			want:    0,
			wantErr: "consensus state for client 07-tendermint-0 at height 1-10 not found",
		},
		{
			name: "if time from local clock can not be retrieved, return error",
			fields: fields{
				clientKeeper: &MockClientKeeper{
					consensusState:       &MockConsensusState{timestamp: uint64(timestampSeconds) * 1_000_000_000},
					returnConsensusState: true,
				},
			},
			args: args{
				ctx: ctx.WithBlockTime(time.Unix(int64(0), 0)),
			},
			wantErr: "block time is not greater than Jan 1st, 1970 12:00 AM",
		},
		{
			name: "if consensus timestamp is > than the given time, return the consensus time adjusted with default",
			fields: fields{
				clientKeeper: &MockClientKeeper{
					consensusState: &MockConsensusState{
						timestamp: uint64(timestampSeconds+1) * 1_000_000_000,
					},
					returnConsensusState: true,
				},
			},
			args: args{
				ctx: ctx.WithBlockTime(time.Unix(int64(timestampSeconds), 0)),
			},
			want:    uint64(timestampSeconds+1)*1_000_000_000 + uint64((time.Duration(10) * time.Minute).Nanoseconds()),
			wantErr: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, _ := ibc.NewPrecompile(&app.PrecompileKeepers{
				TransferKeeper:   tt.fields.transferKeeper,
				EVMKeeper:        tt.fields.evmKeeper,
				ClientKeeper:     tt.fields.clientKeeper,
				ChannelKeeper:    tt.fields.channelKeeper,
				ConnectionKeeper: tt.fields.connectionKeeper,
			})
			got, err := p.GetExecutor().(*ibc.PrecompileExecutor).GetAdjustedTimestamp(tt.args.ctx, tt.args.clientId, tt.args.height)
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.want, got)
		})
	}
}

const realIBCAmount = int64(12345)
const realIBCFunding = int64(1000000)
const realIBCMemo = "repair-1265: positive transfer"

var realIBCTime = time.Unix(1700000000, 0).UTC()

type realIBCFixture struct {
	app    *app.App
	ctx    sdk.Context
	dir    string
	caller common.Address
	sender sdk.AccAddress
}

func newRealIBCFixture(t *testing.T) *realIBCFixture {
	t.Helper()
	f := &realIBCFixture{dir: t.TempDir(), caller: common.HexToAddress("0x1234567890123456789012345678901234567890")}
	f.sender = sdk.AccAddress(f.caller.Bytes())
	f.open(t, false)
	t.Cleanup(func() {
		if f.app != nil {
			require.NoError(t, f.app.Close())
		}
	})
	f.app.EvmKeeper.SetAddressMapping(f.ctx, f.sender, f.caller)
	coins := sdk.NewCoins(sdk.NewInt64Coin("uhpx", realIBCFunding))
	require.NoError(t, f.app.BankKeeper.MintCoins(f.ctx, "evm", coins))
	require.NoError(t, f.app.BankKeeper.SendCoinsFromModuleToAccount(f.ctx, "evm", f.sender, coins))
	return f
}

func (f *realIBCFixture) open(t *testing.T, restart bool) {
	t.Helper()
	db, err := sdk.NewLevelDB("ibc-repair-1265", f.dir)
	require.NoError(t, err)
	encoding := app.MakeEncodingConfig()
	f.app = app.New(db, nil, true, map[int64]bool{}, f.dir, 1, true,
		tmcfg.DefaultConfig().SetRoot(f.dir), encoding, nil, app.TestAppOpts{}, app.EmptyWasmOpts, nil)
	if !restart {
		genesis, err := json.Marshal(app.NewDefaultGenesisState(encoding.Marshaler))
		require.NoError(t, err)
		_, err = f.app.InitChain(context.Background(), &abci.RequestInitChain{
			Time: realIBCTime, ChainId: "pax-test", ConsensusParams: app.DefaultConsensusParams, AppStateBytes: genesis,
		})
		require.NoError(t, err)
	}
	header := tmtypes.Header{ChainID: "pax-test", Height: 1, Time: realIBCTime}
	if restart {
		header.Height = f.app.LastBlockHeight() + 1
		f.ctx, _ = f.app.NewUncachedContext(false, header).CacheContext()
	} else {
		f.ctx = f.app.NewContext(false, header)
	}
	f.app.CapabilityKeeper.InitMemStore(f.ctx)
}

func (f *realIBCFixture) reopen(t *testing.T) {
	t.Helper()
	require.NoError(t, f.app.SetSealedStateToCommit(f.ctx, f.ctx.MultiStore().(sdk.CacheMultiStore)))
	_, err := f.app.Commit(context.Background())
	require.NoError(t, err)
	require.NoError(t, f.app.Close())
	f.app = nil
	blockTime := f.ctx.BlockTime()
	f.open(t, true)
	f.ctx = f.ctx.WithBlockTime(blockTime)
}

func (f *realIBCFixture) channel(t *testing.T, client exported.ClientState, consensus exported.ConsensusState) string {
	t.Helper()
	clientID, err := f.app.IBCKeeper.ClientKeeper.CreateClient(f.ctx, client, consensus)
	require.NoError(t, err)
	conn := connectiontypes.NewConnectionEnd(connectiontypes.OPEN, clientID,
		connectiontypes.NewCounterparty(clientID, "connection-0", commitmenttypes.NewMerklePrefix([]byte(host.StoreKey))),
		[]*connectiontypes.Version{connectiontypes.DefaultIBCVersion}, 0)
	require.NoError(t, conn.ValidateBasic())
	f.app.IBCKeeper.ConnectionKeeper.SetConnection(f.ctx, "connection-0", conn)
	cap, err := f.app.ScopedIBCKeeper.NewCapability(f.ctx, host.ChannelCapabilityPath("transfer", "channel-0"))
	require.NoError(t, err)
	counterparty := channeltypes.NewCounterparty("transfer", "channel-0")
	require.NoError(t, transfer.NewIBCModule(f.app.TransferKeeper).OnChanOpenInit(f.ctx,
		channeltypes.UNORDERED, []string{"connection-0"}, "transfer", "channel-0", cap, counterparty, types.Version))
	f.app.IBCKeeper.ChannelKeeper.SetChannel(f.ctx, "transfer", "channel-0",
		channeltypes.NewChannel(channeltypes.OPEN, channeltypes.UNORDERED, counterparty, []string{"connection-0"}, types.Version))
	f.app.IBCKeeper.ChannelKeeper.SetNextSequenceSend(f.ctx, "transfer", "channel-0", 1)
	f.app.IBCKeeper.ChannelKeeper.SetNextSequenceRecv(f.ctx, "transfer", "channel-0", 1)
	f.app.IBCKeeper.ChannelKeeper.SetNextSequenceAck(f.ctx, "transfer", "channel-0", 1)
	return clientID
}

func realIBCArgs(method, receiver string, revision, height uint64) []interface{} {
	args := []interface{}{receiver, "transfer", "channel-0", "uhpx", big.NewInt(realIBCAmount)}
	if method == ibc.TransferMethod {
		args = append(args, revision, height, uint64(realIBCTime.Add(10*time.Minute).UnixNano()))
	}
	return append(args, realIBCMemo)
}

func realIBCPacket(t *testing.T, events sdk.Events) channeltypes.Packet {
	t.Helper()
	var packet channeltypes.Packet
	count := 0
	for _, event := range events {
		if event.Type != channeltypes.EventTypeSendPacket {
			continue
		}
		count++
		attrs := make(map[string]string)
		for _, attr := range event.Attributes {
			attrs[string(attr.Key)] = string(attr.Value)
		}
		var err error
		packet.Data, err = hex.DecodeString(attrs[channeltypes.AttributeKeyDataHex])
		require.NoError(t, err)
		_, err = fmt.Sscan(attrs[channeltypes.AttributeKeySequence], &packet.Sequence)
		require.NoError(t, err)
		packet.SourcePort = attrs[channeltypes.AttributeKeySrcPort]
		packet.SourceChannel = attrs[channeltypes.AttributeKeySrcChannel]
		packet.DestinationPort = attrs[channeltypes.AttributeKeyDstPort]
		packet.DestinationChannel = attrs[channeltypes.AttributeKeyDstChannel]
		packet.TimeoutHeight, err = clienttypes.ParseHeight(attrs[channeltypes.AttributeKeyTimeoutHeight])
		require.NoError(t, err)
		_, err = fmt.Sscan(attrs[channeltypes.AttributeKeyTimeoutTimestamp], &packet.TimeoutTimestamp)
		require.NoError(t, err)
	}
	require.Equal(t, 1, count, "one real transfer must produce exactly one packet")
	require.NoError(t, packet.ValidateBasic())
	return packet
}

func realIBCCommitment(packet channeltypes.Packet) []byte {
	dataHash := sha256.Sum256(packet.Data)
	preimage := make([]byte, 24, 56)
	binary.BigEndian.PutUint64(preimage[0:8], packet.TimeoutTimestamp)
	binary.BigEndian.PutUint64(preimage[8:16], packet.TimeoutHeight.RevisionNumber)
	binary.BigEndian.PutUint64(preimage[16:24], packet.TimeoutHeight.RevisionHeight)
	sum := sha256.Sum256(append(preimage, dataHash[:]...))
	return sum[:]
}

func (f *realIBCFixture) assertSent(t *testing.T, ctx sdk.Context, packet channeltypes.Packet, receiver string) {
	t.Helper()
	var data types.FungibleTokenPacketData
	require.NoError(t, json.Unmarshal(packet.Data, &data))
	require.Equal(t, types.FungibleTokenPacketData{
		Denom: "uhpx", Amount: "12345", Sender: f.sender.String(), Receiver: receiver, Memo: realIBCMemo,
	}, data)
	require.Equal(t, uint64(1), packet.Sequence)
	require.Equal(t, "transfer", packet.SourcePort)
	require.Equal(t, "channel-0", packet.SourceChannel)
	require.Equal(t, "transfer", packet.DestinationPort)
	require.Equal(t, "channel-0", packet.DestinationChannel)
	require.Equal(t, realIBCCommitment(packet), f.app.IBCKeeper.ChannelKeeper.GetPacketCommitment(ctx, "transfer", "channel-0", 1))
	next, found := f.app.IBCKeeper.ChannelKeeper.GetNextSequenceSend(ctx, "transfer", "channel-0")
	require.True(t, found)
	require.Equal(t, uint64(2), next)
	require.Equal(t, sdk.NewInt(realIBCFunding-realIBCAmount), f.app.BankKeeper.GetBalance(ctx, f.sender, "uhpx").Amount)
	require.Equal(t, sdk.NewInt(realIBCAmount), f.app.BankKeeper.GetBalance(ctx, types.GetEscrowAddress("transfer", "channel-0"), "uhpx").Amount)
}

func (f *realIBCFixture) send(t *testing.T, method, receiver string, revision, height uint64) channeltypes.Packet {
	t.Helper()
	p, err := ibc.NewPrecompile(f.app.GetPrecompileKeepers())
	require.NoError(t, err)
	input, err := p.GetABI().Pack(method, realIBCArgs(method, receiver, revision, height)...)
	require.NoError(t, err)
	f.ctx = f.ctx.WithEventManager(sdk.NewEventManager())
	db := state.NewDBImpl(f.ctx, &f.app.EvmKeeper, false)
	defer db.Cleanup()
	output, remaining, err := p.RunAndCalculateGas(&vm.EVM{StateDB: db, TxContext: vm.TxContext{Origin: f.caller}},
		f.caller, f.caller, input, 1000000, nil, nil, false, false)
	require.NoError(t, err, "precompile error: %v", db.GetPrecompileError())
	want, err := p.GetABI().Methods[method].Outputs.Pack(true)
	require.NoError(t, err)
	require.Equal(t, want, output)
	require.Less(t, remaining, uint64(1000000))
	_, err = db.Finalize()
	require.NoError(t, err)
	packet := realIBCPacket(t, f.ctx.EventManager().Events())
	f.assertSent(t, f.ctx, packet, receiver)
	return packet
}
