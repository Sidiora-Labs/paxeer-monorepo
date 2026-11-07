package bringup

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"
)

const RegistryLifecycleLabel = "network.paxeer.bringup.lifecycle"
const RegistryUIDLabel = "network.paxeer.bringup.owner-uid"

// DockerPeer uses Docker's HTTP API. Tests may attach a loopback API peer;
// production always connects to the local engine socket, without a CLI.
type DockerPeer struct {
	Client   *http.Client
	Endpoint string
}

func LocalDockerPeer() DockerPeer {
	socket := os.Getenv("BRINGUP_DOCKER_SOCKET")
	if socket == "" {
		socket = "/var/run/docker.sock"
	}
	return DockerPeerAtSocket(socket)
}

// DockerPeerAtSocket supports a private local engine or an authenticated local
// forwarding socket. It never resolves a remote Docker host or invokes a CLI.
func DockerPeerAtSocket(socket string) DockerPeer {
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		if !filepath.IsAbs(socket) || filepath.Clean(socket) != socket {
			return nil, errors.New("Docker socket must be an absolute normalized path")
		}
		return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "unix", socket)
	}}
	return DockerPeer{&http.Client{Transport: transport, Timeout: 30 * time.Second}, "http://docker/v1.41"}
}
func (d DockerPeer) request(ctx context.Context, method, path string, body []byte) (int, []byte, error) {
	req, e := http.NewRequestWithContext(ctx, method, d.Endpoint+path, bytes.NewReader(body))
	if e != nil {
		return 0, nil, e
	}
	req.Header.Set("Content-Type", "application/json")
	res, e := d.Client.Do(req)
	if e != nil {
		return 0, nil, e
	}
	defer res.Body.Close()
	b, e := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	return res.StatusCode, b, e
}

type RegistryLifecycle struct {
	Directory string
	Peer      DockerPeer
}

func OpenRegistryLifecycle(directory string, peer DockerPeer) (*RegistryLifecycle, error) {
	directory, e := filepath.Abs(directory)
	if e != nil {
		return nil, e
	}
	info, e := os.Stat(directory)
	if e != nil {
		return nil, e
	}
	if info.Mode().Perm()&0077 != 0 || info.Sys().(*syscall.Stat_t).Uid != uint32(os.Getuid()) {
		return nil, errors.New("lifecycle directory is not privately owned")
	}
	return &RegistryLifecycle{directory, peer}, nil
}
func (s *RegistryLifecycle) Close() { s.Directory = "" }
func (s *RegistryLifecycle) locked(action func(map[string]any) error) error {
	if s.Directory == "" {
		return errors.New("lifecycle closed")
	}
	f, e := os.OpenFile(filepath.Join(s.Directory, "containers.lock"), os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW, 0600)
	if e != nil {
		return e
	}
	defer f.Close()
	if e = syscall.Flock(int(f.Fd()), syscall.LOCK_EX); e != nil {
		return e
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	b, e := os.ReadFile(filepath.Join(s.Directory, "lifecycle.json"))
	if e != nil {
		return e
	}
	var state map[string]any
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	if dec.Decode(&state) != nil || state["schema_version"] != json.Number("1") || state["directory"] != s.Directory || state["owner_uid"] != json.Number(fmt.Sprint(os.Getuid())) {
		return errors.New("lifecycle ownership mismatch")
	}
	id, ok := state["id"].(string)
	if !ok || !regexp.MustCompile(`^[a-f0-9]{32}$`).MatchString(id) {
		return errors.New("invalid lifecycle identity")
	}
	containers, ok := state["containers"].([]any)
	if !ok {
		return errors.New("invalid container records")
	}
	for _, entry := range containers {
		record, ok := entry.(map[string]any)
		if !ok {
			return errors.New("invalid container record")
		}
		name, ok := record["name"].(string)
		if !ok || !regexp.MustCompile(`^layerx-artifacts-`+id+`-[a-f0-9]{8}$`).MatchString(name) {
			return errors.New("foreign container record")
		}
	}
	return action(state)
}
func (s *RegistryLifecycle) persist(state map[string]any) error {
	b, e := json.MarshalIndent(state, "", "  ")
	if e != nil {
		return e
	}
	return edgeAtomic(filepath.Join(s.Directory, "lifecycle.json"), append(b, '\n'), 0600)
}
func registryRecord(state map[string]any, name string) (map[string]any, error) {
	for _, entry := range state["containers"].([]any) {
		record := entry.(map[string]any)
		if record["name"] == name {
			return record, nil
		}
	}
	return nil, errors.New("container was not durably planned")
}
func (s *RegistryLifecycle) Plan() (string, error) {
	var name string
	e := s.locked(func(state map[string]any) error {
		if state["phase"] != "prepared" {
			return errors.New("lifecycle must be prepared before another container")
		}
		var random [4]byte
		if _, e := rand.Read(random[:]); e != nil {
			return e
		}
		name = "layerx-artifacts-" + state["id"].(string) + "-" + hex.EncodeToString(random[:])
		state["containers"] = append(state["containers"].([]any), map[string]any{"name": name, "id": nil, "cgroup": nil, "phase": "planned"})
		state["phase"] = "running"
		return s.persist(state)
	})
	return name, e
}

func (s *RegistryLifecycle) Create(ctx context.Context, name string, configuration []byte) (string, error) {
	var id string
	e := s.locked(func(state map[string]any) error {
		record, e := registryRecord(state, name)
		if e != nil {
			return e
		}
		if record["id"] != nil || record["phase"] != "planned" {
			return errors.New("container creation already attempted")
		}
		var config map[string]any
		if uniqueJSON(configuration) != nil || json.Unmarshal(configuration, &config) != nil || config == nil {
			return errors.New("invalid Docker create configuration")
		}
		labels := map[string]any{}
		if old, ok := config["Labels"]; ok {
			var valid bool
			labels, valid = old.(map[string]any)
			if !valid {
				return errors.New("invalid Docker labels")
			}
		}
		for _, key := range []string{RegistryLifecycleLabel, RegistryUIDLabel} {
			if _, ok := labels[key]; ok {
				return errors.New("reserved ownership label")
			}
		}
		labels[RegistryLifecycleLabel] = state["id"]
		labels[RegistryUIDLabel] = fmt.Sprint(os.Getuid())
		config["Labels"] = labels
		// The identity and intended name already exist in lifecycle.json before the
		// create request. A lost response is reconciled by these engine labels.
		payload, e := json.Marshal(config)
		if e != nil {
			return e
		}
		status, b, e := s.Peer.request(ctx, "POST", "/containers/create?name="+url.QueryEscape(name), payload)
		if e != nil {
			return e
		}
		var result struct {
			ID string `json:"Id"`
		}
		if status != 201 || json.Unmarshal(b, &result) != nil || !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(result.ID) {
			return errors.New("Docker container creation refused or response lost")
		}
		id = result.ID
		record["id"] = id
		record["phase"] = "created"
		return s.persist(state)
	})
	return id, e
}

type dockerContainer struct {
	ID     string `json:"Id"`
	Name   string
	Config struct{ Labels map[string]string }
	State  struct{ Running bool }
}

func (s *RegistryLifecycle) inspect(ctx context.Context, reference string) (*dockerContainer, error) {
	status, b, e := s.Peer.request(ctx, "GET", "/containers/"+url.PathEscape(reference)+"/json", nil)
	if e != nil {
		return nil, e
	}
	if status == 404 {
		return nil, nil
	}
	var actual dockerContainer
	if status != 200 || json.Unmarshal(b, &actual) != nil || !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(actual.ID) {
		return nil, errors.New("Docker container inspection failed")
	}
	return &actual, nil
}
func (s *RegistryLifecycle) Cleanup(ctx context.Context, name string) error {
	return s.locked(func(state map[string]any) error {
		for _, entry := range state["containers"].([]any) {
			record := entry.(map[string]any)
			planned := record["name"].(string)
			if name != "" && name != planned {
				continue
			}
			actual, e := s.inspect(ctx, planned)
			if e != nil {
				return e
			}
			if actual != nil {
				if actual.Name != "/"+planned || actual.Config.Labels[RegistryLifecycleLabel] != state["id"] || actual.Config.Labels[RegistryUIDLabel] != fmt.Sprint(os.Getuid()) {
					return errors.New("container ownership labels differ")
				}
				if id, ok := record["id"].(string); ok && id != "" && id != actual.ID {
					return errors.New("container ownership changed")
				}
				record["id"] = actual.ID
				record["phase"] = "removing"
				if e = s.persist(state); e != nil {
					return e
				}
				// Persist the discovered identifier before removal. Never remove by a name
				// that can be reused between inspect and delete.
				status, _, e := s.Peer.request(ctx, "DELETE", "/containers/"+actual.ID+"?force=1", nil)
				if e != nil {
					return e
				}
				if status != 204 && status != 404 {
					return errors.New("owned container removal failed")
				}
				if remaining, e := s.inspect(ctx, actual.ID); e != nil {
					return e
				} else if remaining != nil {
					return errors.New("owned container remains")
				}
			}
			if remaining, e := s.inspect(ctx, planned); e != nil {
				return e
			} else if remaining != nil {
				return errors.New("planned container name remains occupied")
			}
			// A retained ID also needs an absence proof if the original name vanished.
			if id, ok := record["id"].(string); ok && id != "" {
				if remaining, e := s.inspect(ctx, id); e != nil {
					return e
				} else if remaining != nil {
					return errors.New("owned container ID remains")
				}
			}
			record["container_absent"] = true
			if e = s.persist(state); e != nil {
				return e
			}
		}
		if name != "" {
			_, e := registryRecord(state, name)
			return e
		}
		return nil
	})
}

// RegistryCreateConfiguration mirrors the existing docker create flags while
// keeping the ownership labels exclusively in the lifecycle producer.
func RegistryCreateConfiguration(image, control, quota, builder string) map[string]any {
	return map[string]any{"Image": image, "Cmd": []string{"/bin/sh", "-c", "while [ ! -f /control/command ]; do :; done; exec su -s /bin/sh layerx -c \"$(cat /control/command)\""}, "HostConfig": map[string]any{"NetworkMode": "none", "CgroupnsMode": "private", "NanoCpus": 1000000000, "Memory": 268435456, "PidsLimit": 128, "CapDrop": []string{"ALL"}, "CapAdd": []string{"SETUID", "SETGID"}, "SecurityOpt": []string{"no-new-privileges"}, "Binds": []string{strings.Join([]string{control, "/control"}, ":"), quota + ":/quota", builder + ":/builder:ro", "/sys/fs/cgroup:/host-cgroup"}}}
}
