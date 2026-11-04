#!/usr/bin/env python3
"""Decode the edge renderer's manifest and bind each route to its rendered site."""
import re
import sys

DNS = r"[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?\.[a-z0-9.-]+"
UPSTREAM = rf"{DNS}|[0-9.]+"


def manifest_rows(text):
    rows = []
    seen = set()
    for line in text.splitlines():
        if not line.strip():
            continue
        fields = line.split()
        if len(fields) != 6:
            raise ValueError("manifest requires six fields")
        name, mode, upstream, port, kind, sni = fields
        if not re.fullmatch(DNS, name) or not port.isdecimal() or not 1 <= int(port) <= 65535:
            raise ValueError("manifest name or port")
        if (mode, kind) not in {("http", "box"), ("http", "railway"), ("stream", "stream"), ("rpc", "rpc")}:
            raise ValueError("manifest mode or kind")
        if mode == "rpc":
            if (upstream, port, sni) != ("-", "443", "-"):
                raise ValueError("RPC manifest shape")
        else:
            host, separator, upstream_port = upstream.rpartition(":")
            if not separator or not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9.-]*", host) or not upstream_port.isdecimal() or not 1 <= int(upstream_port) <= 65535:
                raise ValueError("manifest upstream")
            if sni != "-" and not re.fullmatch(DNS, sni):
                raise ValueError("manifest TLS identity")
            if mode == "stream" and sni != "-":
                raise ValueError("stream passes TLS through")
            if mode == "http" and port != "443":
                raise ValueError("HTTP public port")
        if (name, port) in seen:
            raise ValueError("duplicate route")
        seen.add((name, port))
        rows.append(fields)
    if not rows:
        raise ValueError("empty manifest")
    return rows


def blocks(text, directive):
    """Return balanced nginx blocks, including nested location blocks."""
    for match in re.finditer(r"(?m)^\s*" + directive + r"\s*\{", text):
        start = match.end()
        depth = 1
        for end in range(start, len(text)):
            depth += (text[end] == "{") - (text[end] == "}")
            if depth == 0:
                yield text[start:end]
                break
        else:
            raise ValueError("unclosed nginx block")


def require_directive(block, key, value):
    matches = re.findall(r"(?m)^\s*" + re.escape(key) + r"\s+([^;]+);", block)
    if matches != [value]:
        raise ValueError("rendered " + key + " disagrees with manifest")


def route_contract(row, sites):
    name, mode, upstream, port, kind, sni = row
    servers = list(blocks(sites, "server"))
    if mode == "stream":
        # The renderer sends a name through an SNI map to this exact upstream.
        mapping = rf"(?m)^\s*{re.escape(name)}\s+{re.escape(upstream)};\s*$"
        if len(re.findall(mapping, sites)) != 1:
            raise ValueError("rendered stream upstream")
        if not any(re.search(rf"(?m)^\s*listen\s+(?:\[::\]:)?{port}(?:\s|;)", b) and "proxy_pass $edge_stream_" in b for b in servers):
            raise ValueError("rendered stream listener")
        return []
    candidates = [b for b in servers if re.search(r"(?m)^\s*server_name\s+" + re.escape(name) + r";", b) and "ssl_certificate " in b]
    if len(candidates) != 1:
        raise ValueError("rendered TLS site")
    site = candidates[0]
    if mode == "rpc":
        require_directive(site, "proxy_pass", "http://edge_rpc_pool")
        pools = list(blocks(sites, "upstream edge_rpc_pool"))
        if len(pools) != 1:
            raise ValueError("rendered RPC pool")
        members = []
        for address, down, host in re.findall(r"(?m)^\s*server (127\.0\.0\.1:[0-9]+) max_fails=2 fail_timeout=10s( down)?; # (" + DNS + r")\s*$", pools[0]):
            hops = [b for b in servers if re.search(r"(?m)^\s*listen " + re.escape(address) + r";", b)]
            if len(hops) != 1:
                raise ValueError("RPC member hop")
            hop = hops[0]
            for key, value in (("set", "$edge_rpc_member " + host), ("proxy_pass", "https://$edge_rpc_member"), ("proxy_ssl_name", host), ("proxy_ssl_server_name", "on"), ("proxy_ssl_verify", "on"), ("proxy_set_header Host", host), ("proxy_ssl_trusted_certificate", "/etc/ssl/certs/ca-certificates.crt")):
                require_directive(hop, key, value)
            if not down:
                members.append(host)
        if not members:
            raise ValueError("RPC pool has no active member")
        return members
    tls = kind == "railway" or sni != "-"
    identity = upstream.rsplit(":", 1)[0] if sni == "-" else sni
    host_header = identity if kind == "railway" else name
    require_directive(site, "set", "$edge_upstream " + upstream)
    require_directive(site, "proxy_pass", ("https" if tls else "http") + "://$edge_upstream")
    require_directive(site, "proxy_set_header Host", host_header)
    if tls:
        for key, value in (("proxy_ssl_name", identity), ("proxy_ssl_server_name", "on"), ("proxy_ssl_verify", "on"), ("proxy_ssl_trusted_certificate", "/etc/layerx/ca.crt" if kind == "box" else "/etc/ssl/certs/ca-certificates.crt")):
            require_directive(site, key, value)
    elif re.search(r"(?m)^\s*proxy_ssl_", site):
        raise ValueError("unexpected upstream TLS")
    return []


def main():
    try:
        text = sys.stdin.read()
        manifest, sites = text.split("@@sites", 1)
        rows = manifest_rows(manifest)
        if len(sys.argv) == 1:
            for row in rows:
                route_contract(row, sites)
                print(" ".join(row))
        else:
            row = next(row for row in rows if row[0] == sys.argv[1] and row[3] == sys.argv[2])
            print("\n".join(route_contract(row, sites)))
    except (ValueError, StopIteration) as error:
        sys.exit("edge contract: " + str(error))


if __name__ == "__main__":
    main()
