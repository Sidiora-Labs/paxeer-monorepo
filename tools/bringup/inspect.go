package bringup

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// RunInspector is shared by the installed CLI and the in-process gate.
// All errors are fixed diagnostics: custody values never become messages.
func RunInspector(ctx context.Context, args []string, input io.Reader, output io.Writer) error {
	if len(args) == 0 {
		return errors.New("bringup-inspect operation required")
	}
	if handled, err := runRepairInspector(ctx, args, input, output); handled {
		return err
	}
	if handled, err := runLiveInspector(ctx, args, input, output); handled {
		return err
	}
	inventory := func(path string) (PublicInventory, error) {
		file, err := os.Open(path)
		if err != nil {
			return PublicInventory{}, errors.New("public inventory unreadable")
		}
		defer file.Close()
		return ReadPublicInventory(file)
	}
	switch args[0] {
	case "services":
		if len(args) != 2 {
			break
		}
		file, err := os.Open(args[1])
		if err != nil {
			return errors.New("identity roster unreadable")
		}
		defer file.Close()
		rows, err := ReadServiceRoster(file)
		if err != nil {
			return err
		}
		for _, row := range rows {
			if _, err := fmt.Fprintln(output, row.Name, row.Target, row.Custody, row.CN, row.EKU, row.SANs); err != nil {
				return err
			}
		}
		return nil
	case "identity-action":
		if len(args) != 4 {
			break
		}
		days, err := strconv.ParseInt(args[2], 10, 64)
		if err != nil {
			return errors.New("invalid retained validity")
		}
		renew, err := strconv.ParseInt(args[3], 10, 64)
		if err != nil {
			return errors.New("invalid renewal window")
		}
		action, err := IdentityAction(args[1], days, renew)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(output, action)
		return err
	case "rpc":
		if len(args) != 4 {
			break
		}
		var params []json.RawMessage
		if json.Unmarshal([]byte(args[3]), &params) != nil {
			return errors.New("invalid RPC params")
		}
		result, err := callRPC(ctx, &http.Client{Timeout: 30 * time.Second}, args[1], args[2], params, "", false)
		if err != nil {
			return err
		}
		var text string
		if json.Unmarshal(result, &text) == nil {
			_, err = fmt.Fprintln(output, text)
		} else {
			_, err = fmt.Fprintln(output, string(result))
		}
		return err
	case "runtime-plan":
		if len(args) != 1 {
			break
		}
		plan, err := RuntimeBundlePlan()
		if err != nil {
			return err
		}
		return json.NewEncoder(output).Encode(plan)
	case "rpc-names", "node-endpoints", "canonical-node":
		if len(args) != 2 {
			break
		}
		nodes, err := inventory(args[1])
		if err != nil {
			return err
		}
		if args[0] == "canonical-node" {
			_, err = fmt.Fprintln(output, nodes.Canonical)
			return err
		}
		names := nodes.Endpoints()
		if args[0] == "rpc-names" {
			names = nodes.Nodes
		}
		for _, name := range names {
			if args[0] == "rpc-names" {
				name = strings.TrimPrefix(name, "https://")
			}
			if _, err := fmt.Fprintln(output, name); err != nil {
				return err
			}
		}
		return nil
	case "wallet-name":
		if len(args) != 2 {
			break
		}
		file, err := os.Open(args[1])
		if err != nil {
			return errors.New("wallet spec unreadable")
		}
		defer file.Close()
		name, err := WalletName(file)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(output, name)
		return err
	case "gateway-readiness":
		if len(args) != 1 {
			break
		}
		if err := ValidateGatewayReadiness(input, time.Now()); err != nil {
			return err
		}
		_, err := fmt.Fprintln(output, "pass readyz service=layerx-gateway backends=ready")
		return err
	case "pool-check", "gas-config":
		if len(args) != 2 {
			break
		}
		nodes, err := inventory(args[1])
		if err != nil {
			return err
		}
		if args[0] == "pool-check" {
			if err := ValidateWalletPool(nodes, input); err != nil {
				return err
			}
			_, err = fmt.Fprintln(output, "pass rpc_pool=up first=node endpoints=declared")
			return err
		}
		endpoint, err := ValidateGasConfig(nodes, input)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(output, endpoint)
		return err
	case "route-check":
		if len(args) != 4 {
			break
		}
		nodes, err := inventory(args[1])
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		return ProbeRoutes(ctx, &http.Client{Timeout: 10 * time.Second}, nodes, args[2], args[3], os.Getenv("CHECK_LIVE_GATEWAY_TOKEN"), output)
	case "inventory-report":
		if len(args) != 5 {
			break
		}
		ca, err := os.ReadFile(args[1])
		if err != nil {
			return errors.New("CA unreadable")
		}
		state := InspectBundleReport(input, ca, time.Now(), args[2], args[3], args[4])
		_, err = fmt.Fprintln(output, state.String())
		return err
	case "certificate-state":
		if len(args) != 5 {
			break
		}
		ca, err := os.ReadFile(args[1])
		if err != nil {
			return errors.New("CA unreadable")
		}
		cert, err := io.ReadAll(io.LimitReader(input, 1<<20))
		if err != nil {
			return errors.New("certificate unreadable")
		}
		state := CertificateInventory(cert, ca, time.Now(), 0, args[2], args[3], args[4])
		_, err = fmt.Fprintln(output, state.String())
		return err
	case "bundle-local":
		if len(args) != 2 {
			break
		}
		bundle, err := ReadProtectedBundle(args[1])
		if err != nil {
			return errors.New("unreadable protected bundle")
		}
		return BundleReport(bundle, output)
	case "bundle-railway":
		if len(args) != 2 {
			break
		}
		bundle, err := ReadRailwayBundle(input, args[1])
		if err != nil {
			_, writeErr := fmt.Fprintln(output, "bundle=inconsistent")
			return writeErr
		}
		return BundleReport(bundle, output)
	}
	return errors.New("invalid bringup-inspect arguments")
}
