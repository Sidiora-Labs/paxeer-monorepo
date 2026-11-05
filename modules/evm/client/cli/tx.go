package cli

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/hd"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rlp"
	gethrpc "github.com/ethereum/go-ethereum/rpc"
	"github.com/spf13/cobra"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/artifacts/native"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/artifacts/wpax"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types/ethtx"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/flags"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/tx"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/legacy"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keyring"
	"github.com/Sidiora-Labs/Paxeer-X-Network/utils"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

const (
	FlagGasFeeCap = "gas-fee-cap"
	FlagGas       = "gas-limit"
	FlagValue     = "value"
	FlagRPC       = "evm-rpc"
	FlagNonce     = "nonce"
)

// GetTxCmd returns the transaction commands for this module
func GetTxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      fmt.Sprintf("%s transactions subcommands", types.ModuleName),
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(CmdAssociateAddress())
	cmd.AddCommand(CmdSend())
	cmd.AddCommand(CmdDeployContract())
	cmd.AddCommand(CmdCallContract())
	cmd.AddCommand(CmdDeployWPAX())
	cmd.AddCommand(CmdERC20Send())
	cmd.AddCommand(CmdCallPrecompile())
	cmd.AddCommand(NativeSendTxCmd())
	cmd.AddCommand(RegisterCwPointerCmd())
	cmd.AddCommand(RegisterEvmPointerCmd())
	cmd.AddCommand(NewAddERCNativePointerProposalTxCmd())
	cmd.AddCommand(NewBindERCNativePointerProposalTxCmd())
	cmd.AddCommand(AssociateContractAddressCmd())
	cmd.AddCommand(NativeAssociateCmd())
	cmd.AddCommand(PrintClaimTxPayloadCmd())
	cmd.AddCommand(PrintClaimTxBySenderPayloadCmd())
	cmd.AddCommand(PrintClaimSpecificTxPayloadCmd())

	return cmd
}

func CmdAssociateAddress() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "associate-address [optional priv key hex] --evm-rpc=<url> --from=<sender>",
		Short: "associate EVM and Pax address for the sender",
		Long:  "",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			var privHex string
			if len(args) == 1 {
				privHex = args[0]
			} else {
				txf, err := tx.NewFactoryCLI(clientCtx, cmd.Flags())
				if err != nil {
					return err
				}
				kb := txf.Keybase()
				info, err := kb.Key(clientCtx.GetFromName())
				if err != nil {
					return err
				}
				localInfo, ok := info.(keyring.LocalInfo)
				if !ok {
					return errors.New("can only associate address for local keys")
				}
				if localInfo.GetAlgo() != hd.Secp256k1Type {
					return errors.New("can only use addresses using secp256k1")
				}
				priv, err := legacy.PrivKeyFromBytes([]byte(localInfo.PrivKeyArmor))
				if err != nil {
					return err
				}
				privHex = hex.EncodeToString(priv.Bytes())
			}

			emptyHash := crypto.Keccak256Hash([]byte{})
			key, err := crypto.HexToECDSA(privHex)
			if err != nil {
				return err
			}
			sig, err := crypto.Sign(emptyHash[:], key)
			if err != nil {
				return err
			}
			R, S, _, err := ethtx.DecodeSignature(sig)
			if err != nil {
				return err
			}
			V := big.NewInt(int64(sig[64]))
			txData := evmrpc.AssociateRequest{V: hex.EncodeToString(V.Bytes()), R: hex.EncodeToString(R.Bytes()), S: hex.EncodeToString(S.Bytes())}
			bz, err := json.Marshal(txData)
			if err != nil {
				return err
			}
			body := fmt.Sprintf("{\"jsonrpc\": \"2.0\",\"method\": \"pax_associate\",\"params\":[%s],\"id\":\"associate_addr\"}", string(bz))
			rpc, err := cmd.Flags().GetString(FlagRPC)
			if err != nil {
				return err
			}
			resBody, err := postCLIRequest(cmd.Context(), rpc, body)
			if err != nil {
				return err
			}
			fmt.Printf("Response: %s\n", string(resBody))

			return nil
		},
	}

	cmd.Flags().String(FlagRPC, fmt.Sprintf("http://%s:8545", evmrpc.LocalAddress), "RPC endpoint to send request to")
	flags.AddTxFlagsToCmd(cmd)

	return cmd
}

func CmdSend() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "send [to EVM address] [amount in wei] --from=<sender> --gas-fee-cap=<cap> --gas-limit=<limit> --evm-rpc=<url>",
		Short: "send uhpx to EVM address",
		Long:  "",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			key, err := getPrivateKey(cmd)
			if err != nil {
				return err
			}

			rpc, err := cmd.Flags().GetString(FlagRPC)
			if err != nil {
				return err
			}
			var nonce uint64
			if n, err := cmd.Flags().GetInt64(FlagNonce); err == nil && n >= 0 {
				nonce = uint64(n)
			} else {
				nonce, err = getNonce(cmd.Context(), rpc, key.PublicKey)
				if err != nil {
					return err
				}
			}

			to := common.HexToAddress(args[0])
			val, success := new(big.Int).SetString(args[1], 10)
			if !success {
				return fmt.Errorf("%s is an invalid amount to send", args[1])
			}
			txData, err := getTxData(cmd)
			if err != nil {
				return err
			}
			txData.Nonce = nonce
			txData.Value = val
			txData.Data = []byte("")
			txData.To = &to
			resp, err := sendTx(cmd.Context(), txData, rpc, key)
			if err != nil {
				return err
			}

			fmt.Println("Transaction hash:", resp.Hex())

			return nil
		},
	}

	cmd.Flags().Uint64(FlagGasFeeCap, 1000000000000, "Gas fee cap for the transaction")
	cmd.Flags().Uint64(FlagGas, 21000, "Gas limit for the transaction")
	cmd.Flags().String(FlagRPC, fmt.Sprintf("http://%s:8545", evmrpc.LocalAddress), "RPC endpoint to send request to")
	cmd.Flags().Int64(FlagNonce, -1, "Nonce override for the transaction. Negative value means no override")
	flags.AddTxFlagsToCmd(cmd)

	return cmd
}

type Response struct {
	Jsonrpc string `json:"jsonrpc"`
	ID      string `json:"id"`
	Result  string `json:"result"`
}

func CmdDeployContract() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy [path to binary] --from=<sender> --gas-fee-cap=<cap> --gas-limit=<limit> --evm-rpc=<url>",
		Short: "Deploy an EVM contract for binary at specified path",
		Long:  "Deploy bytecode from an even-length hexadecimal file. Leading and trailing whitespace and an optional lowercase 0x prefix are accepted; embedded whitespace is rejected.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			bz, err := readContractBinary(args[0])
			if err != nil {
				return err
			}

			key, err := getPrivateKey(cmd)
			if err != nil {
				return err
			}

			rpc, err := cmd.Flags().GetString(FlagRPC)
			if err != nil {
				return err
			}
			var nonce uint64
			if n, err := cmd.Flags().GetInt64(FlagNonce); err == nil && n >= 0 {
				nonce = uint64(n)
			} else {
				nonce, err = getNonce(cmd.Context(), rpc, key.PublicKey)
				if err != nil {
					return err
				}
			}

			txData, err := getTxData(cmd)
			if err != nil {
				return err
			}
			txData.Nonce = nonce
			txData.Value = utils.Big0
			txData.Data = bz

			resp, err := sendTx(cmd.Context(), txData, rpc, key)
			if err != nil {
				return err
			}

			senderAddr := crypto.PubkeyToAddress(key.PublicKey)
			data, err := rlp.EncodeToBytes([]interface{}{senderAddr, nonce})
			if err != nil {
				return err
			}
			hash := crypto.Keccak256Hash(data)
			contractAddress := hash.Bytes()[12:]
			contractAddressHex := hex.EncodeToString(contractAddress)

			fmt.Println("Deployer:", senderAddr)
			fmt.Println("Deployed to:", fmt.Sprintf("0x%s", contractAddressHex))
			fmt.Println("Transaction hash:", resp.Hex())
			return nil
		},
	}

	cmd.Flags().Uint64(FlagGasFeeCap, 1000000000000, "Gas fee cap for the transaction")
	cmd.Flags().Uint64(FlagGas, 5000000, "Gas limit for the transaction")
	cmd.Flags().String(FlagRPC, fmt.Sprintf("http://%s:8545", evmrpc.LocalAddress), "RPC endpoint to send request to")
	cmd.Flags().Int64(FlagNonce, -1, "Nonce override for the transaction. Negative value means no override")
	flags.AddTxFlagsToCmd(cmd)

	return cmd
}

func CmdCallContract() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "call-contract [addr] [payload hex] --value=<payment> --from=<sender> --gas-fee-cap=<cap> --gas-limit=<limit> --evm-rpc=<url>",
		Short: "Call EVM contract with a bytes payload in hex",
		Long:  "",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			contract := common.HexToAddress(args[0])
			payload, err := hex.DecodeString(args[1])
			if err != nil {
				return err
			}

			key, err := getPrivateKey(cmd)
			if err != nil {
				return err
			}

			rpc, err := cmd.Flags().GetString(FlagRPC)
			if err != nil {
				return err
			}
			var nonce uint64
			if n, err := cmd.Flags().GetInt64(FlagNonce); err == nil && n >= 0 {
				nonce = uint64(n)
			} else {
				nonce, err = getNonce(cmd.Context(), rpc, key.PublicKey)
				if err != nil {
					return err
				}
			}

			value, err := cmd.Flags().GetString(FlagValue)
			if err != nil {
				return err
			}
			valueBig, success := new(big.Int).SetString(value, 10)
			if !success || valueBig.Cmp(utils.Big0) < 0 {
				return fmt.Errorf("%s is not a valid value. Must be a decimal nonnegative integer", value)
			}

			txData, err := getTxData(cmd)
			if err != nil {
				return err
			}
			txData.Nonce = nonce
			txData.Value = valueBig
			txData.Data = payload
			txData.To = &contract

			resp, err := sendTx(cmd.Context(), txData, rpc, key)
			if err != nil {
				return err
			}

			fmt.Println("Transaction hash:", resp.Hex())
			return nil
		},
	}

	cmd.Flags().Uint64(FlagGasFeeCap, 1000000000000, "Gas fee cap for the transaction")
	cmd.Flags().Uint64(FlagGas, 7000000, "Gas limit for the transaction")
	cmd.Flags().String(FlagValue, "0", "Value for the transaction")
	cmd.Flags().String(FlagRPC, fmt.Sprintf("http://%s:8545", evmrpc.LocalAddress), "RPC endpoint to send request to")
	cmd.Flags().Int64(FlagNonce, -1, "Nonce override for the transaction. Negative value means no override")
	flags.AddTxFlagsToCmd(cmd)

	return cmd
}

func CmdERC20Send() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "erc20-send [addr] [recipient] [amount] --from=<sender> --gas-fee-cap=<cap> --gas-limit=<limit> --evm-rpc=<url>",
		Short: "send recipient <amount> (in smallest unit) ERC20 tokens",
		Long:  "",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			contract := common.HexToAddress(args[0])
			recipient := common.HexToAddress(args[1])
			amt, ok := new(big.Int).SetString(args[2], 10)
			if !ok {
				return fmt.Errorf("unable to parse amount: %s", args[2])
			}
			abi, err := native.NativeMetaData.GetAbi()
			if err != nil {
				return err
			}
			payload, err := abi.Pack("transfer", recipient, amt)
			if err != nil {
				return err
			}

			key, err := getPrivateKey(cmd)
			if err != nil {
				return err
			}

			rpc, err := cmd.Flags().GetString(FlagRPC)
			if err != nil {
				return err
			}
			var nonce uint64
			if n, err := cmd.Flags().GetInt64(FlagNonce); err == nil && n >= 0 {
				nonce = uint64(n)
			} else {
				nonce, err = getNonce(cmd.Context(), rpc, key.PublicKey)
				if err != nil {
					return err
				}
			}

			txData, err := getTxData(cmd)
			if err != nil {
				return err
			}
			txData.Nonce = nonce
			txData.Value = utils.Big0
			txData.Data = payload
			txData.To = &contract

			resp, err := sendTx(cmd.Context(), txData, rpc, key)
			if err != nil {
				return err
			}

			fmt.Println("Transaction hash:", resp.Hex())
			return nil
		},
	}

	cmd.Flags().Uint64(FlagGasFeeCap, 1000000000000, "Gas fee cap for the transaction")
	cmd.Flags().Uint64(FlagGas, 7000000, "Gas limit for the transaction")
	cmd.Flags().String(FlagRPC, fmt.Sprintf("http://%s:8545", evmrpc.LocalAddress), "RPC endpoint to send request to")
	cmd.Flags().Int64(FlagNonce, -1, "Nonce override for the transaction. Negative value means no override")
	flags.AddTxFlagsToCmd(cmd)

	return cmd
}

// BuildPrecompileCall resolves the destination and encodes the CLI arguments
// together, before a command reads a key or contacts an RPC endpoint.
func BuildPrecompileCall(name string, args []string) (precompiles.PrecompileInfo, []byte, error) {
	info, err := precompiles.GetPrecompileInfoE(name)
	if err != nil {
		return precompiles.PrecompileInfo{}, nil, err
	}
	if len(args) == 0 {
		return precompiles.PrecompileInfo{}, nil, errors.New("precompile method is required")
	}
	method, ok := info.ABI.Methods[args[0]]
	if !ok {
		return precompiles.PrecompileInfo{}, nil, fmt.Errorf("method %s not found in %s ABI", args[0], name)
	}
	if len(args)-1 != len(method.Inputs) {
		return precompiles.PrecompileInfo{}, nil, fmt.Errorf("method %s expects %d arguments, got %d", args[0], len(method.Inputs), len(args)-1)
	}
	payload, err := getMethodPayload(info.ABI, args)
	if err != nil {
		return precompiles.PrecompileInfo{}, nil, err
	}
	return info, payload, nil
}

func CmdCallPrecompile() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "call-precompile [precompile name] [method] [args...] --value=<payment> --from=<sender> --gas-fee-cap=<cap> --gas-limit=<limit> --evm-rpc=<url>",
		Short: "call method on precompile",
		Long:  "",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			pInfo, payload, err := BuildPrecompileCall(args[0], args[1:])
			if err != nil {
				return err
			}

			key, err := getPrivateKey(cmd)
			if err != nil {
				return err
			}

			rpc, err := cmd.Flags().GetString(FlagRPC)
			if err != nil {
				return err
			}
			var nonce uint64
			if n, err := cmd.Flags().GetInt64(FlagNonce); err == nil && n >= 0 {
				nonce = uint64(n)
			} else {
				nonce, err = getNonce(cmd.Context(), rpc, key.PublicKey)
				if err != nil {
					return err
				}
			}

			value, err := cmd.Flags().GetString(FlagValue)
			if err != nil {
				return err
			}

			valueBig := big.NewInt(0)
			if value != "" {
				var success bool
				valueBig, success = new(big.Int).SetString(value, 10)
				if !success || valueBig.Cmp(utils.Big0) < 0 {
					return fmt.Errorf("%s is not a valid value. Must be a decimal nonnegative integer", value)
				}
			}

			txData, err := getTxData(cmd)
			if err != nil {
				return err
			}
			txData.Nonce = nonce
			txData.Value = valueBig
			txData.Data = payload
			to := pInfo.Address
			txData.To = &to

			resp, err := sendTx(cmd.Context(), txData, rpc, key)
			if err != nil {
				return err
			}

			fmt.Println("Transaction hash:", resp.Hex())
			return nil
		},
	}

	cmd.Flags().Uint64(FlagGasFeeCap, 1000000000000, "Gas fee cap for the transaction")
	cmd.Flags().Uint64(FlagGas, 7000000, "Gas limit for the transaction")
	cmd.Flags().String(FlagRPC, fmt.Sprintf("http://%s:8545", evmrpc.LocalAddress), "RPC endpoint to send request to")
	cmd.Flags().Int64(FlagNonce, -1, "Nonce override for the transaction. Negative value means no override")
	cmd.Flags().String(FlagValue, "", "Value for the transaction")
	flags.AddTxFlagsToCmd(cmd)

	return cmd
}

func CmdDeployWPAX() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy-wpax --from=<sender> --gas-fee-cap=<cap> --gas-limit=<limit> --evm-rpc=<url>",
		Short: "Deploy ERC20 contract for a native Pax token",
		Long:  "",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			contractData := wpax.GetBin()

			key, err := getPrivateKey(cmd)
			if err != nil {
				return err
			}

			rpc, err := cmd.Flags().GetString(FlagRPC)
			if err != nil {
				return err
			}
			var nonce uint64
			if n, err := cmd.Flags().GetInt64(FlagNonce); err == nil && n >= 0 {
				nonce = uint64(n)
			} else {
				nonce, err = getNonce(cmd.Context(), rpc, key.PublicKey)
				if err != nil {
					return err
				}
			}

			txData, err := getTxData(cmd)
			if err != nil {
				return err
			}
			txData.Nonce = nonce
			txData.Value = utils.Big0
			txData.Data = contractData

			resp, err := sendTx(cmd.Context(), txData, rpc, key)
			if err != nil {
				return err
			}

			senderAddr := crypto.PubkeyToAddress(key.PublicKey)
			data, err := rlp.EncodeToBytes([]interface{}{senderAddr, nonce})
			if err != nil {
				return err
			}
			hash := crypto.Keccak256Hash(data)
			contractAddress := hash.Bytes()[12:]
			contractAddressHex := hex.EncodeToString(contractAddress)

			fmt.Println("Deployer:", senderAddr)
			fmt.Println("Deployed to:", fmt.Sprintf("0x%s", contractAddressHex))
			fmt.Println("Transaction hash:", resp.Hex())
			return nil
		},
	}

	cmd.Flags().Uint64(FlagGasFeeCap, 1000000000000, "Gas fee cap for the transaction")
	cmd.Flags().Uint64(FlagGas, 5000000, "Gas limit for the transaction")
	cmd.Flags().String(FlagRPC, fmt.Sprintf("http://%s:8545", evmrpc.LocalAddress), "RPC endpoint to send request to")
	cmd.Flags().Int64(FlagNonce, -1, "Nonce override for the transaction. Negative value means no override")
	flags.AddTxFlagsToCmd(cmd)

	return cmd
}

func getPrivateKey(cmd *cobra.Command) (*ecdsa.PrivateKey, error) {
	clientCtx, err := client.GetClientTxContext(cmd)
	if err != nil {
		return nil, err
	}
	txf, err := tx.NewFactoryCLI(clientCtx, cmd.Flags())
	if err != nil {
		return nil, err
	}
	kb := txf.Keybase()
	info, err := kb.Key(clientCtx.GetFromName())
	if err != nil {
		return nil, err
	}
	localInfo, ok := info.(keyring.LocalInfo)
	if !ok {
		return nil, errors.New("can only associate address for local keys")
	}
	if localInfo.GetAlgo() != hd.Secp256k1Type {
		return nil, errors.New("can only use addresses using secp256k1")
	}
	priv, err := legacy.PrivKeyFromBytes([]byte(localInfo.PrivKeyArmor))
	if err != nil {
		return nil, err
	}
	privHex := hex.EncodeToString(priv.Bytes())
	key, _ := crypto.HexToECDSA(privHex)
	return key, nil
}

const (
	cliRPCTimeout       = 15 * time.Second
	maxCLIResponseBytes = 1 << 20
)

func readContractBinary(path string) ([]byte, error) {
	code, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read contract binary %q: %w", path, err)
	}
	text := strings.TrimPrefix(strings.TrimSpace(string(code)), "0x")
	binary, err := hex.DecodeString(text)
	if err != nil {
		return nil, fmt.Errorf("decode contract binary %q: %w", path, err)
	}
	return binary, nil
}

// cliRPCTransport caps responses for metadata lookups, association and submission.
type cliRPCTransport struct{}

func (cliRPCTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	res, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	responseBody := res.Body
	defer func() { _ = responseBody.Close() }()
	body, err := io.ReadAll(io.LimitReader(responseBody, maxCLIResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read RPC response: %w", err)
	}
	if len(body) > maxCLIResponseBytes {
		return nil, fmt.Errorf("RPC response exceeds %d bytes", maxCLIResponseBytes)
	}
	res.Body = io.NopCloser(bytes.NewReader(body))
	res.ContentLength = int64(len(body))
	return res, nil
}

func cliHTTPClient() *http.Client {
	return &http.Client{Timeout: cliRPCTimeout, Transport: cliRPCTransport{}}
}

func postCLIRequest(ctx context.Context, endpoint, body string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, cliRPCTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := cliHTTPClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("RPC HTTP status %s", res.Status)
	}
	return io.ReadAll(res.Body)
}

type cliRPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *cliRPCError) Error() string {
	return fmt.Sprintf("JSON-RPC error %d: %s", e.Code, e.Message)
}

type cliQuantityResponse struct {
	Result *string      `json:"result"`
	Error  *cliRPCError `json:"error"`
}

func queryCLIQuantity(ctx context.Context, endpoint, query string) (string, error) {
	body, err := postCLIRequest(ctx, endpoint, query)
	if err != nil {
		return "", err
	}
	var response cliQuantityResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("decode RPC response: %w", err)
	}
	if response.Error != nil {
		return "", response.Error
	}
	if response.Result == nil {
		return "", errors.New("RPC response has no string result")
	}
	return *response.Result, nil
}

func getNonce(ctx context.Context, rpc string, key ecdsa.PublicKey) (uint64, error) {
	query := fmt.Sprintf("{\"jsonrpc\":\"2.0\",\"method\":\"eth_getTransactionCount\",\"params\":[\"%s\",\"pending\"],\"id\":\"send-cli\"}", crypto.PubkeyToAddress(key).Hex())
	result, err := queryCLIQuantity(ctx, rpc, query)
	if err != nil {
		return 0, fmt.Errorf("get nonce: %w", err)
	}
	nonce, err := hexutil.DecodeUint64(result)
	if err != nil {
		return 0, fmt.Errorf("invalid nonce result %q: %w", result, err)
	}
	return nonce, nil
}

func getChainId(ctx context.Context, rpc string) (*big.Int, error) {
	query := `{"jsonrpc":"2.0","method":"eth_chainId","params":[],"id":"send-cli"}`
	result, err := queryCLIQuantity(ctx, rpc, query)
	if err != nil {
		return nil, fmt.Errorf("get chain ID: %w", err)
	}
	chainID, err := hexutil.DecodeBig(result)
	if err != nil {
		return nil, fmt.Errorf("invalid chain ID result %q: %w", result, err)
	}
	return chainID, nil
}

func getTxData(cmd *cobra.Command) (*ethtypes.DynamicFeeTx, error) {
	gasFeeCap, err := cmd.Flags().GetUint64(FlagGasFeeCap)
	if err != nil {
		return nil, err
	}
	gasLimit, err := cmd.Flags().GetUint64(FlagGas)
	if err != nil {
		return nil, err
	}
	rpc, err := cmd.Flags().GetString(FlagRPC)
	if err != nil {
		return nil, err
	}
	chainID, err := getChainId(cmd.Context(), rpc)
	if err != nil {
		return nil, err
	}
	return &ethtypes.DynamicFeeTx{
		GasFeeCap: new(big.Int).SetUint64(gasFeeCap),
		GasTipCap: new(big.Int).SetUint64(gasFeeCap),
		Gas:       gasLimit,
		ChainID:   chainID,
	}, nil
}

func sendTx(ctx context.Context, txData *ethtypes.DynamicFeeTx, rpcUrl string, key *ecdsa.PrivateKey) (common.Hash, error) {
	ctx, cancel := context.WithTimeout(ctx, cliRPCTimeout)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return common.Hash{}, err
	}
	ethCfg := types.DefaultChainConfig().EthereumConfig(txData.ChainID)
	signer := ethtypes.MakeSigner(ethCfg, utils.Big1, 1)
	signedTx, err := ethtypes.SignTx(ethtypes.NewTx(txData), signer, key)
	if err != nil {
		return common.Hash{}, err
	}

	rpcClient, err := gethrpc.DialOptions(ctx, rpcUrl, gethrpc.WithHTTPClient(cliHTTPClient()))
	if err != nil {
		return common.Hash{}, err
	}

	ethClient := ethclient.NewClient(rpcClient)
	defer ethClient.Close()
	if err := ethClient.SendTransaction(ctx, signedTx); err != nil {
		return common.Hash{}, err
	}

	return signedTx.Hash(), nil
}
