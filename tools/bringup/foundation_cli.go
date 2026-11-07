package bringup

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func fleetEnvironmentLimits() (FleetLimits, error) {
	limits := FleetLimits{10 * time.Second, 1800 * time.Second, 120 * time.Second}
	for name, target := range map[string]*time.Duration{"SYNC_FLEET_CONNECT_TIMEOUT": &limits.Connect, "SYNC_FLEET_TRANSFER_TIMEOUT": &limits.Transfer, "SYNC_FLEET_RECOVERY_TIMEOUT": &limits.Recovery} {
		if raw := os.Getenv(name); raw != "" {
			seconds, e := strconv.ParseUint(raw, 10, 32)
			if e != nil {
				return limits, e
			}
			*target = time.Duration(seconds) * time.Second
		}
	}
	return limits, limits.validate()
}
func mintDeploymentInputs(certs, modules, deploy string) (MintInputs, error) {
	input := MintInputs{Now: time.Now()}
	var e error
	input.Modules, e = os.ReadFile(modules)
	if e != nil {
		return input, e
	}
	for i := 1; i <= 5; i++ {
		raw, e := os.ReadFile(filepath.Join(certs, fmt.Sprintf("attestor-%d.crt", i)))
		if e != nil {
			return input, e
		}
		input.Certificates = append(input.Certificates, raw)
		raw, e = os.ReadFile(filepath.Join(deploy, fmt.Sprintf("attestor-%d.env.example", i)))
		if e != nil {
			return input, e
		}
		values := map[string]string{}
		for _, line := range strings.Split(string(raw), "\n") {
			if key, value, ok := strings.Cut(line, "="); ok && (key == "ATTESTOR_AUTHORITY_ISSUER" || key == "ATTESTOR_AUTHORITY_TENANT") {
				if _, seen := values[key]; seen {
					return input, errors.New("attestor authority repeated")
				}
				values[key] = value
			}
		}
		issuer, tenant := values["ATTESTOR_AUTHORITY_ISSUER"], values["ATTESTOR_AUTHORITY_TENANT"]
		if i == 1 {
			input.Issuer, input.Tenant = issuer, tenant
		} else if input.Issuer != issuer || input.Tenant != tenant {
			return input, errors.New("attestor deployment authority differs")
		}
	}
	rollout, e := os.ReadFile(filepath.Join(deploy, "../../../deploy/rollout.kvx"))
	if e != nil {
		return input, e
	}
	human, e := os.ReadFile(filepath.Join(deploy, "human.toml"))
	if e != nil {
		return input, e
	}
	// Keep the current rollout producer authoritative for deployed service names.
	plan, e := NewSecretPlan(bytes.NewReader(rollout), bytes.NewReader(human), input.Modules, "")
	if e != nil {
		return input, e
	}
	input.RailwayServices = map[string]string{"wallet-gateway": "wallet-gateway"}
	for _, target := range plan.Targets {
		if target.Target == "railway" {
			input.RailwayServices[target.Role] = target.Consumer
		}
	}
	return input, nil
}
func runFoundationInspector(ctx context.Context, args []string, input io.Reader, output io.Writer) (bool, error) {
	invalid := errors.New("invalid foundation arguments")
	switch args[0] {
	case "fleet-release":
		if len(args) != 4 {
			return true, invalid
		}
		limits, e := fleetEnvironmentLimits()
		if e != nil {
			return true, e
		}
		path, sha, e := FetchFleetRelease(ctx, limits, args[1], args[2], args[3])
		if e != nil {
			return true, e
		}
		_, e = fmt.Fprintln(output, path+"\n"+sha)
		return true, e
	case "fleet-resync", "fleet-recover":
		if len(args) != 2 {
			return true, invalid
		}
		limits, e := fleetEnvironmentLimits()
		if e != nil {
			return true, e
		}
		transport, closeFn, e := OperatorFleetSSH(limits)
		if e != nil {
			return true, e
		}
		defer closeFn()
		owner, e := OpenFleetOwner(args[1], transport)
		if e != nil {
			return true, e
		}
		defer owner.Close()
		if args[0] == "fleet-recover" {
			return true, owner.Recover(ctx)
		}
		var plan FleetResyncPlan
		decoder := json.NewDecoder(io.LimitReader(input, 65536))
		decoder.DisallowUnknownFields()
		if e = decoder.Decode(&plan); e != nil {
			return true, e
		}
		return true, owner.Resync(ctx, plan)
	case "native-foundation-provenance":
		if len(args) != 5 {
			return true, invalid
		}
		raw, e := io.ReadAll(io.LimitReader(input, (16<<20)+1))
		if e != nil || len(raw) > 16<<20 {
			return true, invalid
		}
		if e = QualifyNativeFoundation(args[1], args[2], args[3], args[4], raw); e != nil {
			return true, e
		}
		_, e = output.Write(raw)
		return true, e
	case "runtime-sources":
		if len(args) != 2 {
			return true, invalid
		}
		plan, e := RuntimeBundlePlan()
		if e == nil {
			e = ValidateRuntimeSources(plan, args[1])
		}
		if e != nil {
			return true, e
		}
		_, e = fmt.Fprintln(output, "pass merged-runtime source-closure=complete")
		return true, e
	case "native-genesis-check":
		if len(args) != 3 {
			return true, invalid
		}
		g, e := ReadNativeGenesisFiles(args[1], args[2])
		if e != nil {
			return true, e
		}
		return true, WriteNativeGenesisReport(output, g)
	case "foundation-observe":
		if len(args) != 6 {
			return true, invalid
		}
		g, e := ReadNativeGenesisFiles(args[2], args[3])
		if e != nil {
			return true, e
		}
		state, e := OpenFoundationState(args[1], g)
		if e != nil {
			return true, e
		}
		defer state.Close()
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		checkpoint, e := state.Observe(ctx, &http.Client{Timeout: 10 * time.Second}, args[4], args[5])
		if e != nil {
			return true, e
		}
		return true, json.NewEncoder(output).Encode(checkpoint)
	case "mint":
		if len(args) != 5 {
			return true, invalid
		}
		values, e := mintDeploymentInputs(args[2], args[3], args[4])
		if e != nil {
			return true, e
		}
		paths, e := MintSecrets(args[1], values)
		if e != nil {
			return true, e
		}
		for _, path := range paths {
			if _, e = fmt.Fprintln(output, path); e != nil {
				return true, e
			}
		}
		return true, nil
	case "mint-consume":
		if len(args) != 2 {
			return true, invalid
		}
		values, e := ReadMintEnvironment(args[1], input)
		if e != nil {
			return true, e
		}
		var names []string
		for name := range values {
			names = append(names, name)
		}
		return true, json.NewEncoder(output).Encode(names)
	default:
		return false, nil
	}
}
