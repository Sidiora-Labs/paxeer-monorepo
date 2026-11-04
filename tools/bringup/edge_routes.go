package bringup

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"regexp"
	"strconv"
	"strings"
)

type EdgeRoute struct{ Name, Mode, Upstream, Port, Kind, SNI string }

var edgeDNS = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)+$`)
var edgeHost = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9.-]*[A-Za-z0-9])?$`)

func edgePort(s string) bool {
	n, e := strconv.Atoi(s)
	return e == nil && n > 0 && n <= 65535 && strconv.Itoa(n) == s
}
func (r EdgeRoute) String() string {
	return strings.Join([]string{r.Name, r.Mode, r.Upstream, r.Port, r.Kind, r.SNI}, " ")
}
func (r EdgeRoute) Validate() error {
	if !edgeDNS.MatchString(r.Name) || !edgePort(r.Port) {
		return errors.New("invalid edge name or port")
	}
	switch r.Mode + ":" + r.Kind {
	case "rpc:rpc":
		if r.Upstream != "-" || r.Port != "443" || r.SNI != "-" {
			return errors.New("invalid RPC shape")
		}
		return nil
	case "http:railway", "http:box":
		if r.Port != "443" {
			return errors.New("invalid HTTP port")
		}
	case "stream:stream":
		if r.SNI != "-" {
			return errors.New("stream TLS identity refused")
		}
	default:
		return errors.New("invalid edge mode")
	}
	host, port, e := net.SplitHostPort(r.Upstream)
	if e != nil || !edgeHost.MatchString(host) || !edgePort(port) || (r.SNI != "-" && !edgeDNS.MatchString(r.SNI)) {
		return errors.New("invalid edge upstream or TLS identity")
	}
	return nil
}
func EdgeMapLine(name, kind, upstream, sni string) (EdgeRoute, error) {
	host, port, e := net.SplitHostPort(upstream)
	if e != nil || host == "" || !edgePort(port) {
		return EdgeRoute{}, errors.New("invalid upstream")
	}
	r := EdgeRoute{Name: name, Mode: "http", Upstream: upstream, Port: "443", Kind: kind, SNI: sni}
	if strings.Contains(name, ":") {
		r.Name, r.Port, _ = strings.Cut(name, ":")
		if kind != "stream" {
			return r, errors.New("public port is only valid for stream")
		}
	} else if kind == "stream" {
		r.Port = port
	}
	if kind == "stream" {
		r.Mode = "stream"
	}
	if regexp.MustCompile(`^api[0-9]+\.mainnet-beta\.paxeer\.network$`).MatchString(r.Name) || r.Name == "paxscan.io" {
		return r, errors.New("name is served on its own host")
	}
	return r, r.Validate()
}
func ReadEdgeManifest(input io.Reader, allowEmpty bool) ([]EdgeRoute, error) {
	var rows []EdgeRoute
	seen := map[string]bool{}
	s := bufio.NewScanner(io.LimitReader(input, 4<<20))
	for s.Scan() {
		f := strings.Fields(s.Text())
		if len(f) == 0 {
			continue
		}
		if len(f) != 6 {
			return nil, errors.New("edge manifest requires six fields")
		}
		r := EdgeRoute{f[0], f[1], f[2], f[3], f[4], f[5]}
		if e := r.Validate(); e != nil {
			return nil, e
		}
		key := r.Name + ":" + r.Port
		if seen[key] {
			return nil, errors.New("duplicate edge route")
		}
		seen[key] = true
		rows = append(rows, r)
	}
	if e := s.Err(); e != nil {
		return nil, e
	}
	if len(rows) == 0 && !allowEmpty {
		return nil, errors.New("empty edge manifest")
	}
	return rows, nil
}
func RenderEdgeHTTP(w io.Writer, r EdgeRoute, webroot, certDir, resolver string, tls, shared bool) error {
	// The plain ACME listener is also used as the first half of the RPC site.
	if r.Mode != "rpc" {
		if e := r.Validate(); e != nil {
			return e
		}
		if r.Mode != "http" {
			return errors.New("HTTP renderer requires HTTP route")
		}
	}
	fmt.Fprintln(w, EdgeMarker)
	fmt.Fprintf(w, `server {
	listen 80;
	listen [::]:80;
	server_name %s;

	location ^~ /.well-known/acme-challenge/ {
		root %s;
		default_type text/plain;
		try_files $uri =404;
	}

	location / {
		return 301 https://$host$request_uri;
	}
}
`, r.Name, webroot)
	if !tls {
		return nil
	}
	listen := "\tlisten 443 ssl http2;\n\tlisten [::]:443 ssl http2;"
	if shared {
		listen = "\tlisten 127.0.0.1:10443 ssl http2 proxy_protocol;\n\tset_real_ip_from 127.0.0.1;\n\treal_ip_header proxy_protocol;"
	}
	scheme, host, identity, roots := "http", r.Name, r.SNI, "/etc/layerx/ca.crt"
	if r.Kind == "railway" {
		scheme = "https"
		roots = "/etc/ssl/certs/ca-certificates.crt"
		if identity == "-" {
			identity = strings.Split(r.Upstream, ":")[0]
		}
		host = identity
	} else if r.SNI != "-" {
		scheme = "https"
	}
	fmt.Fprintf(w, `
server {
%s
	server_name %s;

	ssl_certificate %s/%s/fullchain.pem;
	ssl_certificate_key %s/%s/privkey.pem;
	ssl_protocols TLSv1.2 TLSv1.3;

	client_max_body_size 0;
	resolver %s valid=30s ipv6=off;

	location / {
		set $edge_upstream %s;
		proxy_pass %s://$edge_upstream;
		proxy_http_version 1.1;
		proxy_set_header Host %s;
		proxy_set_header X-Forwarded-Host %s;
		proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
		proxy_set_header X-Real-IP $remote_addr;
		proxy_set_header X-Forwarded-Proto $scheme;
		proxy_set_header Origin $http_origin;
		proxy_set_header Access-Control-Request-Method $http_access_control_request_method;
		proxy_set_header Access-Control-Request-Headers $http_access_control_request_headers;
		proxy_set_header Upgrade $http_upgrade;
		proxy_set_header Connection $http_connection;
`, listen, r.Name, certDir, r.Name, certDir, r.Name, resolver, r.Upstream, scheme, host, r.Name)
	if scheme == "https" {
		fmt.Fprintf(w, "\t\tproxy_ssl_server_name on;\n\t\tproxy_ssl_name %s;\n\t\tproxy_ssl_verify on;\n\t\tproxy_ssl_verify_depth 4;\n\t\tproxy_ssl_trusted_certificate %s;\n", identity, roots)
	}
	_, e := fmt.Fprint(w, "\t\tproxy_read_timeout 3600s;\n\t\tproxy_send_timeout 3600s;\n\t}\n}\n")
	return e
}
func RenderEdgeStream(w io.Writer, rows []EdgeRoute, port, resolver string) error {
	if !edgePort(port) {
		return errors.New("invalid stream port")
	}
	var names, outer strings.Builder
	for _, r := range rows {
		if e := r.Validate(); e != nil {
			return e
		}
		if r.Mode == "stream" && r.Port == port {
			fmt.Fprintf(&names, "\t%s %s;\n", r.Name, r.Upstream)
			fmt.Fprintf(&outer, "\t%s 127.0.0.1:10444;\n", r.Name)
		}
	}
	if names.Len() == 0 {
		return errors.New("unregistered stream port")
	}
	fmt.Fprintln(w, EdgeMarker)
	if port == "443" {
		fmt.Fprintf(w, `map $ssl_preread_server_name $edge_stream_outer_443 {
%s	default 127.0.0.1:10443;
}

`, outer.String())
	}
	fmt.Fprintf(w, `map $ssl_preread_server_name $edge_stream_%s {
%s}

server {
	listen %s;
	listen [::]:%s;
	ssl_preread on;
`, port, names.String(), port, port)
	if port == "443" {
		fmt.Fprint(w, `	proxy_protocol on;
	proxy_pass $edge_stream_outer_443;
}

server {
	listen 127.0.0.1:10444 proxy_protocol;
	ssl_preread on;
`)
	}
	_, e := fmt.Fprintf(w, "\tresolver %s valid=30s ipv6=off;\n\tproxy_pass $edge_stream_%s;\n}\n", resolver, port)
	return e
}
func edgeBlocks(text, directive string) ([]string, error) {
	re := regexp.MustCompile(`(?m)^\s*` + regexp.QuoteMeta(directive) + `\s*\{`)
	var blocks []string
	for _, m := range re.FindAllStringIndex(text, -1) {
		depth := 1
		end := m[1]
		for ; end < len(text); end++ {
			if text[end] == '{' {
				depth++
			}
			if text[end] == '}' {
				depth--
			}
			if depth == 0 {
				break
			}
		}
		if depth != 0 {
			return nil, errors.New("unclosed edge block")
		}
		blocks = append(blocks, text[m[1]:end])
	}
	return blocks, nil
}
func edgeDirective(text, key, want string) error {
	re := regexp.MustCompile(`(?m)^\s*` + regexp.QuoteMeta(key) + `\s+([^;]+);`)
	m := re.FindAllStringSubmatch(text, -1)
	if len(m) != 1 || m[0][1] != want {
		return errors.New("rendered " + key + " disagrees with manifest")
	}
	return nil
}
func EdgeRouteContract(row EdgeRoute, sites string) ([]string, error) {
	if e := row.Validate(); e != nil {
		return nil, e
	}
	servers, e := edgeBlocks(sites, "server")
	if e != nil {
		return nil, e
	}
	if row.Mode == "stream" {
		mapping := regexp.MustCompile(`(?m)^\s*` + regexp.QuoteMeta(row.Name) + `\s+` + regexp.QuoteMeta(row.Upstream) + `;\s*$`)
		if len(mapping.FindAllString(sites, -1)) != 1 {
			return nil, errors.New("rendered stream upstream mismatch")
		}
		listener := regexp.MustCompile(`(?m)^\s*listen\s+(?:\[::\]:)?` + row.Port + `(?:\s|;)`)
		for _, s := range servers {
			if listener.MatchString(s) && strings.Contains(s, "proxy_pass $edge_stream_") {
				return nil, nil
			}
		}
		return nil, errors.New("rendered stream listener absent")
	}
	var matching []string
	name := regexp.MustCompile(`(?m)^\s*server_name\s+` + regexp.QuoteMeta(row.Name) + `;`)
	for _, s := range servers {
		if name.MatchString(s) && strings.Contains(s, "ssl_certificate ") {
			matching = append(matching, s)
		}
	}
	if len(matching) != 1 {
		return nil, errors.New("rendered TLS site absent or ambiguous")
	}
	site := matching[0]
	if row.Mode == "rpc" {
		if e = edgeDirective(site, "proxy_pass", "http://edge_rpc_pool"); e != nil {
			return nil, e
		}
		pools, e := edgeBlocks(sites, "upstream edge_rpc_pool")
		if e != nil || len(pools) != 1 {
			return nil, errors.New("rendered RPC pool absent or ambiguous")
		}
		member := regexp.MustCompile(`(?m)^\s*server (127\.0\.0\.1:[0-9]+) max_fails=2 fail_timeout=10s( down)?; # ([a-z0-9.-]+)\s*$`)
		var active []string
		for _, m := range member.FindAllStringSubmatch(pools[0], -1) {
			var hops []string
			for _, s := range servers {
				if regexp.MustCompile(`(?m)^\s*listen ` + regexp.QuoteMeta(m[1]) + `;`).MatchString(s) {
					hops = append(hops, s)
				}
			}
			if len(hops) != 1 {
				return nil, errors.New("RPC member hop absent")
			}
			for key, value := range map[string]string{"set": "$edge_rpc_member " + m[3], "proxy_pass": "https://$edge_rpc_member", "proxy_ssl_name": m[3], "proxy_ssl_server_name": "on", "proxy_ssl_verify": "on", "proxy_set_header Host": m[3], "proxy_ssl_trusted_certificate": "/etc/ssl/certs/ca-certificates.crt"} {
				if e = edgeDirective(hops[0], key, value); e != nil {
					return nil, e
				}
			}
			if m[2] == "" {
				active = append(active, m[3])
			}
		}
		if len(active) == 0 {
			return nil, errors.New("RPC pool has no active member")
		}
		return active, nil
	}
	scheme, identity, host, roots := "http", row.SNI, row.Name, "/etc/layerx/ca.crt"
	if identity == "-" {
		identity = strings.Split(row.Upstream, ":")[0]
	}
	if row.Kind == "railway" {
		scheme = "https"
		host = identity
		roots = "/etc/ssl/certs/ca-certificates.crt"
	} else if row.SNI != "-" {
		scheme = "https"
	}
	for key, value := range map[string]string{"set": "$edge_upstream " + row.Upstream, "proxy_pass": scheme + "://$edge_upstream", "proxy_set_header Host": host} {
		if e = edgeDirective(site, key, value); e != nil {
			return nil, e
		}
	}
	if scheme == "https" {
		for key, value := range map[string]string{"proxy_ssl_name": identity, "proxy_ssl_server_name": "on", "proxy_ssl_verify": "on", "proxy_ssl_trusted_certificate": roots} {
			if e = edgeDirective(site, key, value); e != nil {
				return nil, e
			}
		}
	} else if regexp.MustCompile(`(?m)^\s*proxy_ssl_`).MatchString(site) {
		return nil, errors.New("unexpected upstream TLS")
	}
	return nil, nil
}
func ReadEdgeContract(input io.Reader) ([]EdgeRoute, string, error) {
	b, e := io.ReadAll(io.LimitReader(input, 8<<20))
	if e != nil {
		return nil, "", e
	}
	manifest, sites, ok := strings.Cut(string(b), "@@sites")
	if !ok {
		return nil, "", errors.New("edge sites delimiter missing")
	}
	rows, e := ReadEdgeManifest(strings.NewReader(manifest), false)
	if e != nil {
		return nil, "", e
	}
	for _, r := range rows {
		if _, e = EdgeRouteContract(r, sites); e != nil {
			return nil, "", e
		}
	}
	return rows, sites, nil
}
func SelectRampRoute(rows []EdgeRoute, name string) (EdgeRoute, error) {
	var found []EdgeRoute
	for _, r := range rows {
		if r.Name == name && r.Mode == "http" && r.Port == "443" {
			found = append(found, r)
		}
	}
	if len(found) != 1 {
		return EdgeRoute{}, errors.New("ramp route absent or ambiguous")
	}
	return found[0], nil
}

func RenderEdgeRPC(w io.Writer, name string, pool, down []string, webroot, certDir, resolver string, tls, shared bool) error {
	r := EdgeRoute{name, "rpc", "-", "443", "rpc", "-"}
	if e := r.Validate(); e != nil {
		return e
	}
	seen := map[string]bool{}
	disabled := map[string]bool{}
	for _, host := range down {
		disabled[host] = true
	}
	for _, host := range pool {
		if !edgeDNS.MatchString(host) || seen[host] {
			return errors.New("invalid or duplicate RPC pool member")
		}
		seen[host] = true
	}
	if len(pool) == 0 {
		return errors.New("RPC pool empty")
	}
	if e := RenderEdgeHTTP(w, r, webroot, certDir, resolver, false, shared); e != nil {
		return e
	}
	if !tls {
		return nil
	}
	fmt.Fprint(w, "\nupstream edge_rpc_pool {\n\tleast_conn;\n")
	for i, host := range pool {
		flag := ""
		if disabled[host] {
			flag = " down"
		}
		fmt.Fprintf(w, "\tserver 127.0.0.1:%d max_fails=2 fail_timeout=10s%s; # %s\n", 18701+i, flag, host)
	}
	fmt.Fprint(w, "\tkeepalive 32;\n}\n\nmap $http_upgrade $edge_rpc_connection {\n\tdefault upgrade;\n\t'' '';\n}\n")
	for i, host := range pool {
		fmt.Fprintf(w, `
server {
	listen 127.0.0.1:%d;
	resolver %s valid=30s ipv6=off;

	location / {
		set $edge_rpc_member %s;
		proxy_pass https://$edge_rpc_member;
		proxy_http_version 1.1;
		proxy_set_header Host %s;
		proxy_set_header X-Forwarded-For $http_x_forwarded_for;
		proxy_set_header X-Real-IP $http_x_real_ip;
		proxy_set_header Upgrade $http_upgrade;
		proxy_set_header Connection $http_connection;
		proxy_ssl_server_name on;
		proxy_ssl_name %s;
		proxy_ssl_verify on;
		proxy_ssl_verify_depth 4;
		proxy_ssl_trusted_certificate /etc/ssl/certs/ca-certificates.crt;
		proxy_read_timeout 3600s;
		proxy_send_timeout 3600s;
	}
}
`, 18701+i, resolver, host, host, host)
	}
	listen := "\tlisten 443 ssl http2;\n\tlisten [::]:443 ssl http2;"
	if shared {
		listen = "\tlisten 127.0.0.1:10443 ssl http2 proxy_protocol;\n\tset_real_ip_from 127.0.0.1;\n\treal_ip_header proxy_protocol;"
	}
	_, e := fmt.Fprintf(w, `
server {
%s
	server_name %s;

	ssl_certificate %s/%s/fullchain.pem;
	ssl_certificate_key %s/%s/privkey.pem;
	ssl_protocols TLSv1.2 TLSv1.3;

	client_max_body_size 16m;

	location / {
		proxy_pass http://edge_rpc_pool;
		proxy_http_version 1.1;
		proxy_next_upstream error timeout http_502 http_503 http_504;
		proxy_next_upstream_tries 3;
		proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
		proxy_set_header X-Real-IP $remote_addr;
		proxy_set_header Upgrade $http_upgrade;
		proxy_set_header Connection $edge_rpc_connection;
		proxy_read_timeout 3600s;
		proxy_send_timeout 3600s;
	}
}
`, listen, name, certDir, name, certDir, name)
	return e
}
