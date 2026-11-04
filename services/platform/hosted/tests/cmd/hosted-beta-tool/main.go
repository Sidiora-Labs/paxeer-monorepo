package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"

	hosted "github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/hosted/tests"
)

func policy(args []string) (hosted.TreasuryPolicy, error) {
	if len(args) != 3 {
		return hosted.TreasuryPolicy{}, errors.New("native material requires network id, asset id and treasury recipient")
	}
	network, err := strconv.ParseUint(args[0], 10, 32)
	if err != nil {
		return hosted.TreasuryPolicy{}, err
	}
	result := hosted.TreasuryPolicy{Version: 1, NetworkID: uint32(network), AssetID: args[1], Recipient: args[2]}
	return result, result.Validate()
}

func run(args []string) (any, error) {
	if len(args) == 0 {
		return nil, errors.New("hosted-beta-tool requires a command")
	}
	switch args[0] {
	case "snapshot":
		if len(args) != 3 {
			return nil, errors.New("snapshot ROOT ARCHIVE")
		}
		var output io.Writer = io.Discard
		if args[2] != "-" {
			file, err := os.Create(args[2])
			if err != nil {
				return nil, err
			}
			defer file.Close()
			output = file
		}
		return hosted.SnapshotSource(args[1], output)
	case "image-plan":
		if len(args) != 7 {
			return nil, errors.New("image-plan NAME DOCKERFILE CONTEXT PREPARED ROOT_ARCHIVE OUTPUT_ARCHIVE")
		}
		prepared := args[4]
		if prepared == "-" {
			prepared = ""
		}
		plan, err := hosted.PlanImage(args[1], args[2], args[3], prepared, args[6])
		if err != nil {
			return nil, err
		}
		input, err := os.Open(args[5])
		if err != nil {
			return nil, err
		}
		defer input.Close()
		output, err := os.Create(args[6])
		if err != nil {
			return nil, err
		}
		defer output.Close()
		return hosted.ProjectImageContext(plan, input, output)
	case "mirror-public":
		if len(args) != 2 {
			return nil, errors.New("mirror-public KEYPAIR_JSON")
		}
		raw, err := os.ReadFile(args[1])
		if err != nil {
			return nil, err
		}
		return hosted.SolanaPublicKey(raw)
	case "native-configure":
		if len(args) != 6 {
			return nil, errors.New("native-configure HOME RECIPIENT_ACCOUNT NETWORK ASSET RECIPIENT")
		}
		expected, err := policy(args[3:])
		if err != nil {
			return nil, err
		}
		if err := hosted.ConfigureNativeHome(args[1], args[2], expected); err != nil {
			return nil, err
		}
		return map[string]bool{"configured": true}, nil
	case "native-save", "native-open", "native-secret":
		offset := 2
		if args[0] == "native-save" {
			offset = 3
		}
		if len(args) != offset+3 {
			return nil, errors.New("native material requires HOME (save only), DIRECTORY, NETWORK ASSET RECIPIENT")
		}
		expected, err := policy(args[offset:])
		if err != nil {
			return nil, err
		}
		if args[0] == "native-save" {
			return hosted.SaveNativeMaterial(args[1], args[2], expected)
		}
		material, err := hosted.OpenNativeMaterial(args[1], expected)
		if err != nil {
			return nil, err
		}
		defer material.Close()
		if args[0] == "native-secret" {
			return material.Secret("layerx-testnet")
		}
		return material, nil
	case "native-ready":
		if len(args) != 9 {
			return nil, errors.New("native-ready DIRECTORY CHECKPOINT RPC CONSENSUS TRANSACTION NETWORK ASSET RECIPIENT")
		}
		expected, err := policy(args[6:])
		if err != nil {
			return nil, err
		}
		material, err := hosted.OpenNativeMaterial(args[1], expected)
		if err != nil {
			return nil, err
		}
		defer material.Close()
		ready, err := hosted.NewNativeReadiness(material, args[2])
		if err != nil {
			return nil, err
		}
		defer ready.Close()
		return ready.Probe(context.Background(), args[3], args[4], args[5])
	case "topology-check":
		if len(args) != 1 {
			return nil, errors.New("topology-check reads its normalized topology on stdin")
		}
		var input hosted.TopologyInput
		if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil {
			return nil, err
		}
		return hosted.CheckTopology(input), nil
	default:
		return nil, fmt.Errorf("unknown hosted-beta-tool command %s", args[0])
	}
}

func main() {
	value, err := run(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := json.NewEncoder(os.Stdout).Encode(value); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
