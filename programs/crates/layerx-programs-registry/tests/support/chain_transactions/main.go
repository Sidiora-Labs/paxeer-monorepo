// Generate signed deployment transactions using the node's ABI and transports.
package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"strings"

	evm "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types/ethtx"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	txtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/tx"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

func must[T any](value T, err error) T {
	if err != nil {
		panic(err)
	}
	return value
}

func serve() {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 4096), 16*1024*1024)
	fields := []string{}
	for scanner.Scan() {
		fields = append(fields, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		panic(err)
	}
	if len(fields) != 7 {
		panic("transaction request framing")
	}
	programBytes := must(hex.DecodeString(fields[0]))
	if len(programBytes) != 32 {
		panic("program ID")
	}
	var program, authority [32]byte
	copy(program[:], programBytes)
	abiVersion := must(strconv.ParseUint(fields[1], 10, 16))
	policy := must(strconv.ParseUint(fields[2], 10, 8))
	authorityBytes := must(hex.DecodeString(fields[3]))
	if len(authorityBytes) != 32 {
		panic("authority")
	}
	copy(authority[:], authorityBytes)
	code := must(hex.DecodeString(fields[4]))
	descriptor := must(hex.DecodeString(fields[5]))
	contract := must(abi.JSON(bytes.NewReader(must(os.ReadFile("precompiles/layerxprograms/abi.json")))))
	codeHash := sha256.Sum256(code)
	var calldata []byte
	if fields[6] == "-" {
		calldata = must(contract.Pack("deploy", program, uint16(abiVersion), uint8(policy), authority, codeHash, descriptor, code))
	} else {
		oldBytes := must(hex.DecodeString(fields[6]))
		if len(oldBytes) != 32 {
			panic("predecessor hash")
		}
		var oldHash [32]byte
		copy(oldHash[:], oldBytes)
		calldata = must(contract.Pack("upgrade", program, uint16(abiVersion), uint8(0), oldHash, codeHash, []byte{}, descriptor, code, big.NewInt(0)))
	}
	destination := common.BytesToAddress([]byte{0x10, 0x1d})
	key := must(crypto.GenerateKey())
	signer := types.NewEIP155Signer(big.NewInt(1))
	signed := must(types.SignTx(types.NewTx(&types.LegacyTx{GasPrice: big.NewInt(1), Gas: 1_000_000, To: &destination, Value: big.NewInt(0), Data: calldata}), signer, key))
	if must(types.Sender(signer, signed)) != crypto.PubkeyToAddress(key.PublicKey) {
		panic("transaction signer")
	}
	msg := must(evm.NewMsgEVMTransaction(must(ethtx.NewTxDataFromTx(signed))))
	if err := msg.ValidateBasic(); err != nil {
		panic(err)
	}
	body := must((&txtypes.TxBody{Messages: []*codectypes.Any{must(codectypes.NewAnyWithValue(msg))}}).Marshal())
	fmt.Printf("%x\n", must((&txtypes.TxRaw{BodyBytes: body}).Marshal()))
}

func main() {
	if len(os.Args) == 2 && os.Args[1] == "transaction" {
		serve()
		return
	}
	source := must(os.Open("programs/crates/layerx-programs-registry/src/deployment_transcripts.hex"))
	defer source.Close()
	transcript := map[string][]byte{}
	scanner := bufio.NewScanner(source)
	for scanner.Scan() {
		key, value, ok := strings.Cut(scanner.Text(), "=")
		if !ok {
			panic("transcript framing")
		}
		transcript[key] = must(hex.DecodeString(value))
	}
	if err := scanner.Err(); err != nil {
		panic(err)
	}
	contract := must(abi.JSON(bytes.NewReader(must(os.ReadFile("precompiles/layerxprograms/abi.json")))))
	code := transcript["wasm.1"]
	changed := bytes.Clone(code)
	changed[len(changed)-2] = 1
	oldHash := sha256.Sum256(code)
	signer := types.NewEIP155Signer(big.NewInt(1))
	key := must(crypto.GenerateKey())
	for _, variant := range []struct {
		name    string
		program byte
		wasm    []byte
		policy  uint8
		upgrade bool
	}{
		{"immutable", 0x31, code, 0, false},
		{"authority", 0x31, code, 1, false},
		{"changed", 0x31, changed, 1, false},
		{"other", 0x32, code, 1, false},
		{"upgrade", 0x31, changed, 1, true},
	} {
		var program, authority [32]byte
		for i := range program {
			program[i] = variant.program
			if variant.policy == 1 {
				authority[i] = 0x41
			}
		}
		codeHash := sha256.Sum256(variant.wasm)
		descriptor := bytes.Replace(bytes.Clone(transcript["interface.1"]), oldHash[:], codeHash[:], 1)
		var calldata []byte
		if variant.upgrade {
			calldata = must(contract.Pack("upgrade", program, uint16(1), uint8(0), oldHash, codeHash, []byte{}, descriptor, variant.wasm, big.NewInt(0)))
		} else {
			calldata = must(contract.Pack("deploy", program, uint16(1), variant.policy, authority, codeHash, descriptor, variant.wasm))
		}
		destination := common.BytesToAddress([]byte{0x10, 0x1d})
		signed := must(types.SignTx(types.NewTx(&types.LegacyTx{GasPrice: big.NewInt(1), Gas: 1_000_000, To: &destination, Value: big.NewInt(0), Data: calldata}), signer, key))
		sender := must(types.Sender(signer, signed))
		if sender != crypto.PubkeyToAddress(key.PublicKey) {
			panic("transaction signer")
		}
		msg := must(evm.NewMsgEVMTransaction(must(ethtx.NewTxDataFromTx(signed))))
		if err := msg.ValidateBasic(); err != nil {
			panic(err)
		}
		body := must((&txtypes.TxBody{Messages: []*codectypes.Any{must(codectypes.NewAnyWithValue(msg))}}).Marshal())
		raw := must((&txtypes.TxRaw{BodyBytes: body}).Marshal())
		fmt.Printf("%s=%x\n", variant.name, raw)
	}
}
