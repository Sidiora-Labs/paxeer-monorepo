// Command api-runtime-docs writes the API's digest-bound network authority document.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/app"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/capabilities"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
)

const documentName = "network-authority.json"

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "api-runtime-docs:", err)
		os.Exit(1)
	}
}

func registryPath(repo string) string {
	return filepath.Join(repo, "tools", "launchpad", "production", "bootstrap", "check-registry.json")
}

func serialInterfaceDigest(repo string) (string, error) {
	raw, err := os.ReadFile(registryPath(repo))
	if err != nil {
		return "", fmt.Errorf("-repo serial interface registry: %w", err)
	}
	var registry struct {
		Tasks map[string]struct {
			InterfaceBundle struct {
				ExpectedSHA256 string `json:"expected_sha256"`
			} `json:"interface_bundle"`
		} `json:"tasks"`
	}
	if err := json.Unmarshal(raw, &registry); err != nil {
		return "", fmt.Errorf("-repo serial interface registry: %w", err)
	}
	return registry.Tasks["1.99"].InterfaceBundle.ExpectedSHA256, nil
}

func readEndpoints(path string) (deployment.PublicEndpoints, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return deployment.PublicEndpoints{}, fmt.Errorf("-endpoints: %w", err)
	}
	var endpoints *deployment.PublicEndpoints
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&endpoints); err != nil {
		return deployment.PublicEndpoints{}, fmt.Errorf("-endpoints: %w", err)
	}
	if endpoints == nil || decoder.Decode(new(any)) != io.EOF {
		return deployment.PublicEndpoints{}, errors.New("-endpoints must contain exactly one JSON object")
	}
	for _, endpoint := range []struct {
		name, value string
		allowed     []string
	}{{"RPC", endpoints.RPC, endpoints.AllowedRPCs}, {"Explorer", endpoints.Explorer, endpoints.AllowedExplorers}} {
		u, err := url.Parse(endpoint.value)
		if err != nil || len(endpoint.value) == 0 || len(endpoint.value) > 2048 || endpoint.value != strings.TrimSpace(endpoint.value) ||
			u.Scheme != "https" || u.Host == "" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.Opaque != "" ||
			len(endpoint.allowed) == 0 || len(endpoint.allowed) > 16 {
			return deployment.PublicEndpoints{}, fmt.Errorf("-endpoints %s: public_endpoint_not_approved", endpoint.name)
		}
		approved := false
		for _, candidate := range endpoint.allowed {
			if candidate == endpoint.value {
				approved = true
			}
		}
		if !approved {
			return deployment.PublicEndpoints{}, fmt.Errorf("-endpoints %s: public_endpoint_not_approved", endpoint.name)
		}
	}
	return *endpoints, nil
}

func run(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("api-runtime-docs", flag.ContinueOnError)
	repo := flags.String("repo", ".", "repository root")
	schema := flags.String("source-schema", "", "required indexer source schema")
	endpointsPath := flags.String("endpoints", "", "required PublicEndpoints JSON file")
	poolRuntime := flags.String("pool-runtime", "", "required reviewed pool runtime keccak256 hash")
	out := flags.String("out", ".", "output directory")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	if *endpointsPath == "" {
		return errors.New("-endpoints is required")
	}
	endpoints, err := readEndpoints(*endpointsPath)
	if err != nil {
		return err
	}
	serial, err := serialInterfaceDigest(*repo)
	if err != nil {
		return err
	}
	manifest := deployment.Current()
	revision, err := capabilities.NewPreflightRevision(manifest, *poolRuntime, serial)
	if err != nil {
		return fmt.Errorf("-pool-runtime or -repo serialInterfaceSha256: %w", err)
	}
	document := app.NetworkAuthorityConfig{
		SchemaVersion: 2, Manifest: manifest, ReviewedPoolRuntime: *poolRuntime,
		SerialInterfaceSHA256: serial, ExpectedRevisionSHA256: revision.RevisionSHA256,
		SourceSchema: *schema, Endpoints: endpoints,
		GoAddresses: manifest.Addresses(), FrontendAddresses: manifest.Addresses(),
	}
	if err := app.ValidateNetworkAuthorityConfig(document); err != nil {
		return fmt.Errorf("-source-schema network authority document: %w", err)
	}
	raw, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	if err := os.MkdirAll(*out, 0o750); err != nil {
		return fmt.Errorf("-out: %w", err)
	}
	if err := os.WriteFile(filepath.Join(*out, documentName), raw, 0o600); err != nil {
		return fmt.Errorf("-out: %w", err)
	}
	_, err = fmt.Fprintf(stdout, "%x %s %d\n", sha256.Sum256(raw), documentName, len(raw))
	return err
}
