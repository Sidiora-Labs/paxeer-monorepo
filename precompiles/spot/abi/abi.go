// Package abi exports the ABIs of the spot precompile rails with no other
// import, so indexers decode their logs without importing the precompile.
package abi

import (
	"bytes"
	_ "embed"

	"github.com/ethereum/go-ethereum/accounts/abi"
)

var (
	//go:embed pecor.json
	pecorJSON []byte
	//go:embed orders.json
	ordersJSON []byte
	//go:embed stoporders.json
	stopOrdersJSON []byte
	//go:embed quoter.json
	quoterJSON []byte
	//go:embed vault.json
	vaultJSON []byte
	//go:embed share.json
	shareJSON []byte
	//go:embed share-solidity.json
	shareSolidityJSON []byte
)

func parse(raw []byte) (abi.ABI, error) { return abi.JSON(bytes.NewReader(raw)) }

// PECOR returns the parsed ABI of PECOR at 0x1021.
func PECOR() (abi.ABI, error) { return parse(pecorJSON) }

// Orders returns the parsed ABI of PECOROrders at 0x1022.
func Orders() (abi.ABI, error) { return parse(ordersJSON) }

// StopOrders returns the parsed ABI of PECORStopOrders at 0x1023.
func StopOrders() (abi.ABI, error) { return parse(stopOrdersJSON) }

// Quoter returns the parsed ABI of PECORQuoter at 0x1024.
func Quoter() (abi.ABI, error) { return parse(quoterJSON) }

// Vault returns the parsed ABI of PECORVault at 0x1025.
func Vault() (abi.ABI, error) { return parse(vaultJSON) }

// Share returns the NativePaxTokensERC20 deployment ABI of vault share pointers.
// Mint, burn and ownership selectors are unavailable and revert with empty data
// for every caller. See share-deployment.kvx for the deployment/refusal contract.
// Transfers exceeding the bank balance return the frozen
// ERC20InsufficientBalance(address,uint256,uint256) error.
func Share() (abi.ABI, error) { return parse(shareJSON) }

// HistoricalSolidityShare returns the historical Solidity share-token snapshot.
// It is not the interface deployed by the spot keeper.
func HistoricalSolidityShare() (abi.ABI, error) { return parse(shareSolidityJSON) }
