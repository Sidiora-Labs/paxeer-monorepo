package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	e2e "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/test/e2e/pkg"
)

// Cleanup removes the Docker Compose containers and testnet directory.
func Cleanup(testnet *e2e.Testnet) error {
	docker, endpoint, err := cleanupDockerClient()
	if err != nil {
		return err
	}
	defer docker.CloseIdleConnections()
	err = cleanupDocker(testnet, docker, endpoint)
	if err != nil {
		return err
	}
	return cleanupDir(testnet.Dir)
}

// composeProject uses the same directory-name normalization as Docker Compose.
func composeProject(dir string) (string, error) {
	if dir == "" {
		return "", errors.New("no directory set")
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	project := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_' || r == '-' {
			return r
		}
		return -1
	}, strings.ToLower(filepath.Base(absDir)))
	project = strings.TrimLeft(project, "_-")
	if project == "" {
		return "", fmt.Errorf("directory %q has no valid compose project name", dir)
	}
	return project, nil
}

// cleanupDockerClient connects to the Engine without requiring a CLI subprocess.
func cleanupDockerClient() (*http.Client, string, error) {
	host := os.Getenv("DOCKER_HOST")
	if host == "" {
		host = "unix:///var/run/docker.sock"
	}
	endpoint, err := url.Parse(host)
	if err != nil {
		return nil, "", err
	}
	transport := &http.Transport{}
	switch endpoint.Scheme {
	case "unix":
		if endpoint.Path == "" {
			return nil, "", errors.New("Docker socket path is empty")
		}
		transport.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", endpoint.Path)
		}
		host = "http://docker"
	case "tcp", "http", "https":
		if endpoint.Host == "" {
			return nil, "", errors.New("Docker host is empty")
		}
		if endpoint.Scheme == "tcp" {
			endpoint.Scheme = "http"
		}
		if os.Getenv("DOCKER_TLS_VERIFY") != "" {
			certDir := os.Getenv("DOCKER_CERT_PATH")
			if certDir == "" {
				home, err := os.UserHomeDir()
				if err != nil {
					return nil, "", err
				}
				certDir = filepath.Join(home, ".docker")
			}
			certificate, err := tls.LoadX509KeyPair(filepath.Join(certDir, "cert.pem"), filepath.Join(certDir, "key.pem"))
			if err != nil {
				return nil, "", err
			}
			ca, err := os.ReadFile(filepath.Join(certDir, "ca.pem"))
			if err != nil {
				return nil, "", err
			}
			roots := x509.NewCertPool()
			if !roots.AppendCertsFromPEM(ca) {
				return nil, "", errors.New("invalid Docker CA certificate")
			}
			transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, Certificates: []tls.Certificate{certificate}}
			endpoint.Scheme = "https"
		}
		host = strings.TrimRight(endpoint.String(), "/")
	default:
		return nil, "", fmt.Errorf("unsupported Docker host scheme %q", endpoint.Scheme)
	}
	return &http.Client{Transport: transport, Timeout: time.Minute}, host, nil
}

// cleanupDockerRequest uses the Engine's stable JSON list/removal API.
func cleanupDockerRequest(ctx context.Context, docker *http.Client, endpoint, method, path string, body, result any) error {
	var data bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&data).Encode(body); err != nil {
			return err
		}
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint+path, &data)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := docker.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message, err := io.ReadAll(io.LimitReader(response.Body, 4096))
		if err != nil {
			return err
		}
		return fmt.Errorf("Docker %s %s: status %d: %s", method, path, response.StatusCode, message)
	}
	if result != nil {
		return json.NewDecoder(response.Body).Decode(result)
	}
	return nil
}

// cleanupDocker removes only E2E resources belonging to the selected compose project.
func cleanupDocker(testnet *e2e.Testnet, docker *http.Client, endpoint string) error {
	project, err := composeProject(testnet.Dir)
	if err != nil {
		return err
	}
	logger.Info("Removing Docker containers and networks", "project", project)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	filters, err := json.Marshal(map[string][]string{"label": {"e2e", "com.docker.compose.project=" + project}})
	if err != nil {
		return err
	}
	query := url.QueryEscape(string(filters))
	for _, resource := range []struct{ list, remove string }{
		{list: "/containers/json?all=1&filters=" + query, remove: "/containers/"},
		{list: "/networks?filters=" + query, remove: "/networks/"},
	} {
		var items []struct {
			ID string `json:"Id"`
		}
		if err := cleanupDockerRequest(ctx, docker, endpoint, http.MethodGet, resource.list, nil, &items); err != nil {
			return err
		}
		for _, item := range items {
			if item.ID == "" {
				return errors.New("Docker resource has empty ID")
			}
			path := resource.remove + url.PathEscape(item.ID)
			if resource.remove == "/containers/" {
				path += "?force=1"
			}
			if err := cleanupDockerRequest(ctx, docker, endpoint, http.MethodDelete, path, nil, nil); err != nil {
				return err
			}
		}
	}
	return nil
}

// cleanupDir cleans up a testnet directory
func cleanupDir(dir string) error {
	if dir == "" {
		return errors.New("no directory set")
	}

	_, err := os.Stat(dir)
	if os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}

	logger.Info("removing testnet directory", "dir", dir)

	// On Linux, some local files in the volume will be owned by root since Tendermint
	// runs as root inside the container, so we need to clean them up from within a
	// container running as root too.
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	err = execDocker("run", "--rm", "--entrypoint", "", "-v", fmt.Sprintf("%v:/network", absDir),
		"tendermint/e2e-node", "sh", "-c", "rm -rf /network/*/")
	if err != nil {
		return err
	}

	return os.RemoveAll(dir)
}
