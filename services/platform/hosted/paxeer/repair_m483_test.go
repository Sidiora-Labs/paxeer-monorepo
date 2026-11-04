package paxeer

import (
	"bytes"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestRepairM_483_BoundaryManifest(t *testing.T) {
	documents := repair1319BoundaryDocuments(t)
	workloads := 0
	for _, document := range documents {
		metadata := repair1319Object(t, document["metadata"])
		if metadata["namespace"] != "layerx-testnet" {
			t.Fatalf("boundary resource outside layerx-testnet: %v", metadata)
		}
		if document["kind"] == "StatefulSet" {
			workloads++
		}
	}
	if workloads != 1 {
		t.Fatalf("expected one Paxeer workload, got %d", workloads)
	}
	stateful := repair1319Resource(t, documents, "StatefulSet", "paxeer")
	spec := repair1319Object(t, stateful["spec"])
	if spec["serviceName"] != "paxeer" {
		t.Fatal("Paxeer workload lost its governing service")
	}
	template := repair1319Object(t, spec["template"])
	labels := repair1319Object(t, repair1319Object(t, template["metadata"])["labels"])
	pod := repair1319Object(t, template["spec"])
	initializers := repair1319Items(t, pod["initContainers"])
	containers := repair1319Items(t, pod["containers"])
	if len(initializers) != 1 || len(containers) != 2 || repair1319Object(t, initializers[0])["name"] != "genesis" {
		t.Fatal("Paxeer must contain genesis initialization, paxd and boundary")
	}
	native := repair1319Container(t, repair1319Pod(t, repair1319Documents(t)), "paxd")
	paxd := repair1319Container(t, pod, "paxd")
	genesis := repair1319Object(t, initializers[0])
	if paxd["image"] != native["image"] || genesis["image"] != native["image"] ||
		!reflect.DeepEqual(paxd["command"], native["command"]) {
		t.Fatal("boundary node and provisioner diverged from the maintained native runtime")
	}
	_, paths := repair1319Provisioning(t, pod)
	if paths["genesis.json"] != "config/genesis.json" || paths["receipt.key"] != "config/receipt.key" {
		t.Fatal("boundary native provisioning is missing genesis or receipt key")
	}
	boundary := repair1319Container(t, pod, "boundary")
	if boundary["image"] != "ghcr.io/sidiora-labs/layerx-paxeer-boundary:0.1.0" {
		t.Fatal("wrong boundary image")
	}
	environment := make(map[string]string)
	for _, item := range repair1319Items(t, boundary["env"]) {
		entry := repair1319Object(t, item)
		name, named := entry["name"].(string)
		value, literal := entry["value"].(string)
		if !named || !literal || environment[name] != "" {
			t.Fatalf("nonliteral or duplicate boundary environment: %v", entry)
		}
		environment[name] = value
	}
	if environment["LAYERX_PAXEER_BOUNDARY_LISTEN"] != "0.0.0.0:9443" ||
		environment["LAYERX_PAXEER_NODE_URL"] != "http://127.0.0.1:8545" {
		t.Fatal("boundary must listen on 9443 and use the literal local native node")
	}
	endpoint, err := url.Parse(environment["LAYERX_PAXEER_NODE_URL"])
	if err != nil || !net.ParseIP(endpoint.Hostname()).IsLoopback() ||
		strings.TrimSuffix(repair1319Profile(t).Node.RPC, "/") != endpoint.String() {
		t.Fatal("profile and boundary do not use the same literal loopback node")
	}
	ports := make(map[string]int)
	for _, item := range repair1319Items(t, boundary["ports"]) {
		entry := repair1319Object(t, item)
		ports[entry["name"].(string)] = entry["containerPort"].(int)
	}
	if !reflect.DeepEqual(ports, map[string]int{"https": 9443, "observer-https": 9444}) {
		t.Fatalf("wrong boundary listeners: %v", ports)
	}
	for name, target := range map[string]string{"paxeer": "https", "paxeer-boundary": "https", "paxeer-observer-boundary": "observer-https"} {
		service := repair1319Resource(t, documents, "Service", name)
		serviceSpec := repair1319Object(t, service["spec"])
		selector := repair1319Object(t, serviceSpec["selector"])
		if len(selector) == 0 || selector["app"] != "paxeer" {
			t.Fatalf("boundary service %s has the wrong selector", name)
		}
		for key, value := range selector {
			if labels[key] != value {
				t.Fatalf("service %s does not select the boundary workload", name)
			}
		}
		servicePorts := repair1319Items(t, serviceSpec["ports"])
		if len(servicePorts) != 1 {
			t.Fatalf("service %s must expose exactly one HTTPS port", name)
		}
		port := repair1319Object(t, servicePorts[0])
		if port["port"] != 9443 || port["targetPort"] != target {
			t.Fatalf("service %s targets the wrong boundary listener: %v", name, port)
		}
	}
}

func TestRepairM_483_ManifestCommandBytes(t *testing.T) {
	original := repair1319Read(t, "deployment.yaml")
	var output bytes.Buffer
	if err := RunDeployment([]string{"manifest", "deployment-profile.json"}, &output); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(output.Bytes(), original) {
		t.Fatal("deployment command did not emit the tracked boundary manifest byte for byte")
	}
	if bytes.Equal(output.Bytes(), repair1319Read(t, "../node/deployment.yaml")) {
		t.Fatal("deployment command still emits the node manifest")
	}
}

func TestRepairM_483_ManifestValidation(t *testing.T) {
	original := repair1319Read(t, "deployment.yaml")
	for name, raw := range map[string][]byte{
		"empty":              nil,
		"malformed":          []byte("apiVersion: ["),
		"node manifest":      repair1319Read(t, "../node/deployment.yaml"),
		"wrong namespace":    bytes.ReplaceAll(original, []byte("namespace: layerx-testnet"), []byte("namespace: default")),
		"wrong workload":     bytes.Replace(original, []byte("name: paxeer,"), []byte("name: wrong,"), 1),
		"missing observer":   bytes.Replace(original, []byte("name: paxeer-observer-boundary,"), []byte("name: wrong-observer,"), 1),
		"trailing malformed": append(append([]byte{}, original...), []byte("---\napiVersion: [")...),
	} {
		t.Run(name, func(t *testing.T) {
			directory := t.TempDir()
			profile := filepath.Join(directory, "deployment-profile.json")
			if err := os.WriteFile(profile, repair1319Read(t, "deployment-profile.json"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(directory, "deployment.yaml"), raw, 0600); err != nil {
				t.Fatal(err)
			}
			var output bytes.Buffer
			if err := RunDeployment([]string{"manifest", profile}, &output); err == nil || output.Len() != 0 {
				t.Fatalf("invalid boundary manifest emitted bytes: %v, %q", err, output.String())
			}
		})
	}
}
