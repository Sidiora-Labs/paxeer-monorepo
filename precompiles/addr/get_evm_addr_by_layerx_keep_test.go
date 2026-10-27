package addr_test

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/addr"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestGetEvmAddrByLayerXServesBoundAddress(t *testing.T) {
	h := newLayerXHarness(t, "layerx-binding-test")
	_, recipient := testkeeper.MockAddressPair()
	_, caller := testkeeper.MockAddressPair()
	require.NotEqual(t, recipient, caller)
	didPublicKey, private := newDidKey(t)

	require.Equal(t, common.HexToAddress("0x0000000000000000000000000000000000001004"), h.p.Address())
	method := h.p.ABI.Methods[addr.GetEvmAddrByLayerXMethod]
	require.Equal(t, "getEvmAddrByLayerX(bytes32)", method.Sig)
	require.Equal(t, []byte{0xc4, 0x13, 0xba, 0xd3}, method.ID)
	require.Equal(t, "view", method.StateMutability)

	ret, _, err := h.run(layerXCall{caller: recipient}, addr.BindLayerXMethod, didPublicKey, h.sign(private, recipient, h.nonce(recipient)))
	require.NoError(t, err)
	require.Empty(t, ret)

	ret, _, err = h.run(layerXCall{caller: caller, readOnly: true}, addr.GetEvmAddrByLayerXMethod, didPublicKey)
	require.NoError(t, err)
	require.Len(t, ret, 32)
	require.Equal(t, []interface{}{recipient}, h.unpack(addr.GetEvmAddrByLayerXMethod, ret))

	unboundKey, _ := newDidKey(t)
	require.NotEqual(t, didPublicKey, unboundKey)
	ret, _, err = h.run(layerXCall{caller: caller, readOnly: true}, addr.GetEvmAddrByLayerXMethod, unboundKey)
	require.ErrorContains(t, err, "is not bound to an EVM address")
	require.Empty(t, ret)
}
