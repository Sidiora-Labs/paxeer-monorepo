package bringup

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const archiveMarker = "# managed by tools/bringup/install-archive.sh"

type ArchiveState struct{ Nginx, Certificates, Webroot string }

func ArchiveName(inventory PublicInventory, name string) error {
	for _, endpoint := range inventory.Nodes {
		if endpoint == "https://"+name {
			return nil
		}
	}
	return errors.New("archive name is not a published numbered RPC node")
}

func (s ArchiveState) certificate(name string, now time.Time) bool {
	pair, err := tls.LoadX509KeyPair(filepath.Join(s.Certificates, name, "fullchain.pem"), filepath.Join(s.Certificates, name, "privkey.pem"))
	if err != nil || len(pair.Certificate) == 0 {
		return false
	}
	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	return err == nil && leaf.VerifyHostname(name) == nil && !now.Before(leaf.NotBefore) && now.Before(leaf.NotAfter)
}

func (s ArchiveState) render(name string, secure bool) string {
	text := fmt.Sprintf(`%s
server {
    listen 80;
    listen [::]:80;
    server_name %s;
    location /.well-known/acme-challenge/ { root %s; }
    location / { return 301 https://$host$request_uri; }
}
`, archiveMarker, name, s.Webroot)
	if secure {
		text += fmt.Sprintf(`
map $http_upgrade $api_connection_upgrade { default upgrade; '' close; }
map $http_upgrade $api_backend { default 127.0.0.1:8645; websocket 127.0.0.1:8646; }
server {
    listen 443 ssl http2;
    listen [::]:443 ssl http2;
    server_name %s;
    ssl_certificate %s/%s/fullchain.pem;
    ssl_certificate_key %s/%s/privkey.pem;
    ssl_protocols TLSv1.2 TLSv1.3;
    client_max_body_size 10m;
    proxy_http_version 1.1;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto https;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection $api_connection_upgrade;
    location = /health { access_log off; return 200 'ok'; add_header Content-Type text/plain; }
    location /ws { proxy_pass http://127.0.0.1:8646/; proxy_read_timeout 3600s; proxy_send_timeout 3600s; }
    location / { proxy_pass http://$api_backend; proxy_read_timeout 300s; proxy_send_timeout 300s; }
}
`, name, s.Certificates, name, s.Certificates, name)
	}
	return text
}

// Reconcile derives progress from the actual certificate and configuration on
// each invocation. A failed ACME run leaves a resumable HTTP challenge site.
func (s ArchiveState) Reconcile(inventory PublicInventory, name string, now time.Time) (string, error) {
	if err := ArchiveName(inventory, name); err != nil {
		return "", err
	}
	for _, value := range []string{s.Nginx, s.Certificates, s.Webroot} {
		if !filepath.IsAbs(value) || strings.ContainsAny(value, "\n\r;{}\" ") {
			return "", errors.New("invalid archive path")
		}
	}
	secure := s.certificate(name, now)
	available := filepath.Join(s.Nginx, "sites-available", name+".conf")
	enabled := filepath.Join(s.Nginx, "sites-enabled", name+".conf")
	current, err := os.ReadFile(available)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	desired := s.render(name, secure)
	// Adopt only the exact old bootstrap HTTP site. Operator customizations are
	// retained and require an explicit operator reconciliation if incomplete.
	legacy := strings.TrimPrefix(s.render(name, false), archiveMarker+"\n")
	managed := err != nil || string(current) == s.render(name, false) || string(current) == s.render(name, true) || string(current) == legacy
	if !managed {
		text := string(current)
		if !secure || !strings.Contains(text, "listen 443 ssl") || !strings.Contains(text, "server_name "+name+";") || !strings.Contains(text, filepath.Join(s.Certificates, name, "fullchain.pem")) || !strings.Contains(text, filepath.Join(s.Certificates, name, "privkey.pem")) {
			return "", errors.New("operator archive site requires certificate/TLS reconciliation; configuration preserved")
		}
		desired = text
	}
	if target, e := os.Readlink(enabled); e == nil {
		if target != available {
			return "", errors.New("archive enabled site ownership differs; preserved")
		}
	} else if _, e = os.Lstat(enabled); e == nil {
		return "", errors.New("archive enabled site is operator owned; preserved")
	} else if !os.IsNotExist(e) {
		return "", e
	}
	changed := string(current) != desired
	if changed {
		if err = edgeAtomic(available, []byte(desired), 0644); err != nil {
			return "", err
		}
	}
	if err = os.MkdirAll(filepath.Dir(enabled), 0755); err != nil {
		return "", err
	}
	if target, e := os.Readlink(enabled); e != nil || target != available {
		if _, e = os.Lstat(enabled); e == nil {
			return "", errors.New("archive enabled site ownership differs; preserved")
		} else if !os.IsNotExist(e) {
			return "", e
		}
		if err = os.Symlink(available, enabled); err != nil {
			return "", err
		}
		changed = true
		if err = edgeSyncDir(filepath.Dir(enabled)); err != nil {
			return "", err
		}
	}
	phase := "tls"
	if !secure {
		phase = "certificate"
	}
	if changed {
		phase += " changed"
	} else {
		phase += " unchanged"
	}
	return phase, nil
}

func ProbeArchiveHTTPS(ctx context.Context, client *http.Client, endpoint string) error {
	if !strings.HasPrefix(endpoint, "https://") {
		return errors.New("archive health requires HTTPS")
	}
	req, e := http.NewRequestWithContext(ctx, "GET", endpoint+"/health", nil)
	if e != nil {
		return e
	}
	c := *client
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	res, e := c.Do(req)
	if e != nil {
		return e
	}
	defer res.Body.Close()
	b, e := io.ReadAll(io.LimitReader(res.Body, 32))
	if e != nil {
		return e
	}
	if res.StatusCode != 200 || strings.TrimSpace(string(b)) != "ok" {
		return errors.New("archive HTTPS health refused")
	}
	return nil
}
