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

func TestBoundaryManifest_BoundaryManifest(t *testing.T) {
	documents := nativeDeploymentBoundaryDocuments(t)
	workloads := 0
	for _, document := range documents {
		metadata := nativeDeploymentObject(t, document["metadata"])
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
	stateful := nativeDeploymentResource(t, documents, "StatefulSet", "paxeer")
	spec := nativeDeploymentObject(t, stateful["spec"])
	if spec["serviceName"] != "paxeer" {
		t.Fatal("Paxeer workload lost its governing service")
	}
	template := nativeDeploymentObject(t, spec["template"])
	labels := nativeDeploymentObject(t, nativeDeploymentObject(t, template["metadata"])["labels"])
	pod := nativeDeploymentObject(t, template["spec"])
	initializers := nativeDeploymentItems(t, pod["initContainers"])
	containers := nativeDeploymentItems(t, pod["containers"])
	if len(initializers) != 1 || len(containers) != 2 || nativeDeploymentObject(t, initializers[0])["name"] != "genesis" {
		t.Fatal("Paxeer must contain genesis initialization, paxd and boundary")
	}
	native := nativeDeploymentContainer(t, nativeDeploymentPod(t, nativeDeploymentDocuments(t)), "paxd")
	paxd := nativeDeploymentContainer(t, pod, "paxd")
	genesis := nativeDeploymentObject(t, initializers[0])
	if paxd["image"] != native["image"] || genesis["image"] != native["image"] ||
		!reflect.DeepEqual(paxd["command"], native["command"]) {
		t.Fatal("boundary node and provisioner diverged from the maintained native runtime")
	}
	_, paths := nativeDeploymentProvisioning(t, pod)
	if paths["genesis.json"] != "config/genesis.json" || paths["receipt.key"] != "config/receipt.key" {
		t.Fatal("boundary native provisioning is missing genesis or receipt key")
	}
	boundary := nativeDeploymentContainer(t, pod, "boundary")
	if boundary["image"] != "ghcr.io/sidiora-labs/layerx-paxeer-boundary:0.1.0" {
		t.Fatal("wrong boundary image")
	}
	environment := make(map[string]string)
	for _, item := range nativeDeploymentItems(t, boundary["env"]) {
		entry := nativeDeploymentObject(t, item)
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
		strings.TrimSuffix(nativeDeploymentProfile(t).Node.RPC, "/") != endpoint.String() {
		t.Fatal("profile and boundary do not use the same literal loopback node")
	}
	ports := make(map[string]int)
	for _, item := range nativeDeploymentItems(t, boundary["ports"]) {
		entry := nativeDeploymentObject(t, item)
		ports[entry["name"].(string)] = entry["containerPort"].(int)
	}
	if !reflect.DeepEqual(ports, map[string]int{"https": 9443, "observer-https": 9444}) {
		t.Fatalf("wrong boundary listeners: %v", ports)
	}
	for name, target := range map[string]string{"paxeer": "https", "paxeer-boundary": "https", "paxeer-observer-boundary": "observer-https"} {
		service := nativeDeploymentResource(t, documents, "Service", name)
		serviceSpec := nativeDeploymentObject(t, service["spec"])
		selector := nativeDeploymentObject(t, serviceSpec["selector"])
		if len(selector) == 0 || selector["app"] != "paxeer" {
			t.Fatalf("boundary service %s has the wrong selector", name)
		}
		for key, value := range selector {
			if labels[key] != value {
				t.Fatalf("service %s does not select the boundary workload", name)
			}
		}
		servicePorts := nativeDeploymentItems(t, serviceSpec["ports"])
		if len(servicePorts) != 1 {
			t.Fatalf("service %s must expose exactly one HTTPS port", name)
		}
		port := nativeDeploymentObject(t, servicePorts[0])
		if port["port"] != 9443 || port["targetPort"] != target {
			t.Fatalf("service %s targets the wrong boundary listener: %v", name, port)
		}
	}
}

func TestBoundaryManifest_ManifestCommandBytes(t *testing.T) {
	original := nativeDeploymentRead(t, "deployment.yaml")
	var output bytes.Buffer
	if err := RunDeployment([]string{"manifest", "deployment-profile.json"}, &output); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(output.Bytes(), original) {
		t.Fatal("deployment command did not emit the tracked boundary manifest byte for byte")
	}
	if bytes.Equal(output.Bytes(), nativeDeploymentRead(t, "../node/deployment.yaml")) {
		t.Fatal("deployment command still emits the node manifest")
	}
}

func TestBoundaryManifest_ManifestValidation(t *testing.T) {
	original := nativeDeploymentRead(t, "deployment.yaml")
	for name, raw := range map[string][]byte{
		"empty":              nil,
		"malformed":          []byte("apiVersion: ["),
		"node manifest":      nativeDeploymentRead(t, "../node/deployment.yaml"),
		"wrong namespace":    bytes.ReplaceAll(original, []byte("namespace: layerx-testnet"), []byte("namespace: default")),
		"wrong workload":     bytes.Replace(original, []byte("name: paxeer,"), []byte("name: wrong,"), 1),
		"missing observer":   bytes.Replace(original, []byte("name: paxeer-observer-boundary,"), []byte("name: wrong-observer,"), 1),
		"trailing malformed": append(append([]byte{}, original...), []byte("---\napiVersion: [")...),
	} {
		t.Run(name, func(t *testing.T) {
			directory := t.TempDir()
			profile := filepath.Join(directory, "deployment-profile.json")
			if err := os.WriteFile(profile, nativeDeploymentRead(t, "deployment-profile.json"), 0600); err != nil {
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
