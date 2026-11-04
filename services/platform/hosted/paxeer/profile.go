// Package paxeer renders the tracked boundary deployment and its native node
// handoff. It owns no signing operation, genesis mutation or deployment phase.
package paxeer

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// DeploymentProfile contains only native node and tenant routing inputs.
type DeploymentProfile struct {
	Schema string        `json:"schema"`
	Node   NodeProfile   `json:"node"`
	Tenant TenantProfile `json:"tenant"`
}

type NodeProfile struct {
	Manifest     string `json:"manifest"`
	Bootstrap    string `json:"bootstrap"`
	ChainID      string `json:"chain_id"`
	EVMChainID   uint64 `json:"evm_chain_id"`
	RPC          string `json:"rpc"`
	ConsensusRPC string `json:"consensus_rpc"`
}

type TenantProfile struct {
	GatewayRPC string `json:"gateway_rpc"`
}

func rootOrigin(value string, tls bool) bool {
	u, err := url.Parse(value)
	if err != nil || strings.ContainsAny(value, "\r\n\t ") {
		return false
	}
	scheme := "http"
	if tls {
		scheme = "https"
	}
	port, err := strconv.ParseUint(u.Port(), 10, 16)
	return err == nil && port > 0 && u.Scheme == scheme && u.Hostname() != "" &&
		u.User == nil && u.Path == "/" && u.RawPath == "" &&
		u.RawQuery == "" && !u.ForceQuery && u.Fragment == "" && u.Opaque == ""
}

func (p DeploymentProfile) Validate() error {
	if p.Schema != "paxeer/native-deployment/1" ||
		p.Node.Manifest != "../node/deployment.yaml" || p.Node.Bootstrap != "../node/bootstrap.sh" ||
		p.Node.ChainID != "hyperpax_125-1" || p.Node.EVMChainID != 125 ||
		!rootOrigin(p.Node.RPC, false) || !rootOrigin(p.Node.ConsensusRPC, false) ||
		!rootOrigin(p.Tenant.GatewayRPC, true) {
		return errors.New("native deployment requires the maintained node, mapped chain identity and root RPC origins")
	}
	for _, endpoint := range []string{p.Node.RPC, p.Node.ConsensusRPC} {
		u, err := url.Parse(endpoint)
		if err != nil || !net.ParseIP(u.Hostname()).IsLoopback() {
			return errors.New("boundary node RPC origins require literal loopback addresses")
		}
	}
	return nil
}

// WriteProfile is also used by provisioning tools producing a native handoff.
func WriteProfile(w io.Writer, p DeploymentProfile) error {
	if err := p.Validate(); err != nil {
		return err
	}
	return json.NewEncoder(w).Encode(p)
}

// Deployment is a read-only configuration handle. Historical deployment records
// are deliberately never opened or interpreted as resumable work.
type Deployment struct {
	profile   DeploymentProfile
	directory string
	closed    bool
}

func OpenDeployment(path string) (*Deployment, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(raw) > 16*1024 {
		return nil, errors.New("native deployment profile exceeds 16 KiB")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var p DeploymentProfile
	if err := decoder.Decode(&p); err != nil {
		return nil, fmt.Errorf("native deployment profile: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, errors.New("native deployment profile must contain one JSON document")
	}
	if err := p.Validate(); err != nil {
		return nil, err
	}
	return &Deployment{profile: p, directory: filepath.Dir(path)}, nil
}

func (d *Deployment) Close() error {
	d.closed = true
	return nil
}

func (d *Deployment) Profile() (DeploymentProfile, error) {
	if d.closed {
		return DeploymentProfile{}, errors.New("native deployment is closed")
	}
	return d.profile, nil
}

// WriteManifest validates and emits the tracked Paxeer boundary deployment.
// Node.Manifest identifies the separately maintained native node facts.
func (d *Deployment) WriteManifest(w io.Writer) error {
	if d.closed {
		return errors.New("native deployment is closed")
	}
	raw, err := os.ReadFile(filepath.Join(d.directory, "deployment.yaml"))
	if err != nil {
		return err
	}
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	count := 0
	workloads := 0
	services := make(map[string]bool)
	for {
		var document map[string]any
		if err := decoder.Decode(&document); err == io.EOF {
			break
		} else if err != nil {
			return err
		}
		metadata, ok := document["metadata"].(map[string]any)
		if document["apiVersion"] == nil || document["kind"] == nil || !ok ||
			metadata["namespace"] != "layerx-testnet" {
			return errors.New("invalid Paxeer boundary manifest")
		}
		if document["kind"] == "StatefulSet" {
			if metadata["name"] != "paxeer" {
				return errors.New("boundary workload must be paxeer")
			}
			workloads++
		}
		if document["kind"] == "Service" {
			name, ok := metadata["name"].(string)
			if !ok || services[name] {
				return errors.New("invalid or duplicate boundary service")
			}
			services[name] = true
		}
		count++
	}
	if count == 0 || workloads != 1 || !services["paxeer-boundary"] || !services["paxeer-observer-boundary"] {
		return errors.New("Paxeer manifest requires one boundary workload and both boundary services")
	}
	_, err = w.Write(raw)
	return err
}

// RunDeployment exposes only rendering and inspection. In particular a legacy
// phase is refused before any profile, key, journal or RPC endpoint is opened.
func RunDeployment(arguments []string, output io.Writer) error {
	if len(arguments) != 2 || (arguments[0] != "manifest" && arguments[0] != "profile") {
		return errors.New("usage: deployment manifest|profile PROFILE; historical deployment phases are retired")
	}
	d, err := OpenDeployment(arguments[1])
	if err != nil {
		return err
	}
	defer d.Close()
	if arguments[0] == "manifest" {
		return d.WriteManifest(output)
	}
	return WriteProfile(output, d.profile)
}
