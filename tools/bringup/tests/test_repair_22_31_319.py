"""Real renderer/consumer qualification for audit 22.30.177.

SSH coverage uses real OpenSSH, the committed session transcript and rendered
configuration. Public TLS and JSON-RPC peers run on loopback. Recorded internal
exchanges must also match the live inspector over the private TLS route.
The actual Rust configuration loader uses the repository's vendored dependencies.
"""
import contextlib
import copy
import datetime
import hashlib
import json
import os
import re
import select
import shlex
import shutil
import socket
import socketserver
import ssl
import subprocess
import sys
import threading
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

import pytest
from cryptography import x509
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import ec
from cryptography.x509.oid import ExtendedKeyUsageOID, NameOID

ROOT = Path(__file__).resolve().parents[3]
BRINGUP = ROOT / "tools/bringup"


def run(args, *, env=None, input=None, ok=True, timeout=120):
    result = subprocess.run(
        args, cwd=ROOT, env=env, input=input, text=True, capture_output=True,
        timeout=timeout,
    )
    if ok:
        assert result.returncode == 0, result.stdout + result.stderr
    return result


def shell(command, env, *, input=None, ok=True):
    return run(
        ["bash", "-euc", 'source tools/bringup/check-live.sh; ' + command],
        env=env, input=input, ok=ok,
    )


def unused_port():
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        return sock.getsockname()[1]


@contextlib.contextmanager
def process(args, log, *, env=None, port=None):
    with log.open("w") as handle:
        child = subprocess.Popen(args, cwd=ROOT, env=env, stdout=handle, stderr=handle)
        try:
            if port:
                deadline = time.monotonic() + 10
                while True:
                    assert child.poll() is None, log.read_text()
                    try:
                        with socket.create_connection(("127.0.0.1", port), timeout=.1):
                            break
                    except OSError:
                        assert time.monotonic() < deadline, log.read_text()
                        time.sleep(.05)
            yield child
        finally:
            child.terminate()
            try:
                child.wait(timeout=5)
            except subprocess.TimeoutExpired:
                child.kill()
                child.wait()


@pytest.fixture
def material(tmp_path):
    env = {key: value for key, value in os.environ.items()
           if not key.startswith(("XWEB_", "CHECK_LIVE_", "LAYERX_", "SEARCH_FRONT_"))
           and key.lower() not in {"http_proxy", "https_proxy", "all_proxy", "no_proxy"}}
    env.update(LAYERX_CA_DIR=str(tmp_path / "ca"),
               LAYERX_TLS_DIR=str(tmp_path / "tls"),
               LAYERX_RAILWAY_BIN="/usr/bin/false", CHECK_LIVE_TIMEOUT="10",
               NO_PROXY="*", no_proxy="*")
    run(["bash", str(BRINGUP / "ca.sh"), "init"], env=env)
    ca = x509.load_pem_x509_certificate((tmp_path / "ca/ca.pem").read_bytes())
    key = serialization.load_pem_private_key((tmp_path / "ca/ca.key").read_bytes(), None)

    def certificate(name, names):
        directory = tmp_path / name
        directory.mkdir(parents=True, exist_ok=True)
        private = ec.generate_private_key(ec.SECP256R1())
        now = datetime.datetime.now(datetime.timezone.utc)
        cert = (x509.CertificateBuilder()
                .subject_name(x509.Name([x509.NameAttribute(NameOID.COMMON_NAME, names[0])]))
                .issuer_name(ca.subject).public_key(private.public_key())
                .serial_number(x509.random_serial_number())
                .not_valid_before(now - datetime.timedelta(minutes=1))
                .not_valid_after(now + datetime.timedelta(days=1))
                .add_extension(x509.BasicConstraints(ca=False, path_length=None), True)
                .add_extension(x509.ExtendedKeyUsage([ExtendedKeyUsageOID.SERVER_AUTH]), False)
                .add_extension(x509.SubjectAlternativeName([x509.DNSName(n) for n in names]), False)
                .sign(key, hashes.SHA256()))
        (directory / "cert.pem").write_bytes(cert.public_bytes(serialization.Encoding.PEM))
        (directory / "key.pem").write_bytes(private.private_bytes(
            serialization.Encoding.PEM, serialization.PrivateFormat.PKCS8,
            serialization.NoEncryption()))
        (directory / "key.pem").chmod(0o600)
        return directory

    env["CURL_CA_BUNDLE"] = env["SSL_CERT_FILE"] = str(tmp_path / "ca/ca.pem")
    return tmp_path, env, certificate


@pytest.fixture
def ssh_server(material):
    directory, env, certificate = material
    assert shutil.which("sshd"), "OpenSSH server is a declared integration prerequisite"
    port = unused_port()
    run(["ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", str(directory / "ssh-key")])
    config = directory / "sshd.conf"
    config.write_text(f"""ListenAddress 127.0.0.1
Port {port}
HostKey {directory}/ssh-key
PidFile {directory}/sshd.pid
AuthorizedKeysFile {directory}/ssh-key.pub
PermitRootLogin yes
StrictModes no
PasswordAuthentication no
KbdInteractiveAuthentication no
UsePAM no
LogLevel VERBOSE
SetEnv CURL_CA_BUNDLE={env['CURL_CA_BUNDLE']}
SetEnv NO_PROXY=* no_proxy=*
""")
    # sshd's runtime privilege-separation directory is an OS prerequisite.
    Path("/run/sshd").mkdir(exist_ok=True)
    client = directory / "ssh.conf"
    client.write_text(f"""Host *
  HostName 127.0.0.1
  User root
  Port {port}
  IdentityFile {directory}/ssh-key
  IdentitiesOnly yes
  StrictHostKeyChecking yes
  UserKnownHostsFile {directory}/known_hosts
  HostKeyAlias bringup-test
""")
    public = (directory / "ssh-key.pub").read_text().split()
    (directory / "known_hosts").write_text(f"bringup-test {public[0]} {public[1]}\n")
    bindir = directory / "bin"
    bindir.mkdir()
    # This adapter only supplies the ephemeral SSH configuration; /usr/bin/ssh
    # still performs the real authenticated protocol and executes real commands.
    wrapper = bindir / "ssh"
    wrapper.write_text(f'#!/bin/sh\nexec /usr/bin/ssh -F {shlex.quote(str(client))} "$@"\n')
    wrapper.chmod(0o755)
    env["PATH"] = str(bindir) + ":" + env["PATH"]
    env.update(EDGE_HOST="edge.127.0.0.1.nip.io", KERNEL_HOST="kernel",
               VALIDATOR_HOSTS="validator.invalid")
    with process([shutil.which("sshd"), "-D", "-e", "-f", str(config)],
                 directory / "sshd.log", port=port):
        yield directory, env, certificate


@contextlib.contextmanager
def http_server(handler, *, certificate=None, bind="127.0.0.1", port=0, ca=None):
    server = ThreadingHTTPServer((bind, port), handler)
    if certificate:
        context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
        context.load_cert_chain(certificate / "cert.pem", certificate / "key.pem")
        if ca:
            context.load_verify_locations(ca)
            context.verify_mode = ssl.CERT_REQUIRED
        server.socket = context.wrap_socket(server.socket, server_side=True)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        yield server
    finally:
        server.shutdown()
        server.server_close()
        thread.join()


class JsonHandler(BaseHTTPRequestHandler):
    def answer(self, value, status=200):
        body = json.dumps(value, separators=(",", ":")).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, *args):
        pass


def test_edge_manifest_roundtrip_both_consumers_over_ssh(ssh_server):
    directory, env, certificate = ssh_server
    nginx = shutil.which("nginx")
    assert nginx, "nginx is a declared integration prerequisite"
    names = {kind: f"{kind}.127.0.0.1.nip.io" for kind in ("box", "ramp", "railway", "stream", "rpc")}
    tls_name = "backend.127.0.0.1.nip.io"
    node = "node.127.0.0.2.nip.io"
    calls = []

    class Backend(JsonHandler):
        def do_GET(self):
            calls.append((self.headers["Host"], self.path))
            self.answer({"ready": True})

        def do_POST(self):
            request = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
            calls.append((self.headers["Host"], request))
            assert request == {"jsonrpc": "2.0", "id": 1, "method": "eth_chainId", "params": []}
            self.answer({"jsonrpc": "2.0", "id": 1, "result": "0x7d"})

    with contextlib.ExitStack() as stack:
        plain = stack.enter_context(http_server(Backend))
        secure = stack.enter_context(http_server(Backend, certificate=certificate("backend", [tls_name, names["stream"]])))
        stack.enter_context(http_server(Backend, certificate=certificate("node", [node]), bind="127.0.0.2", port=443))
        public_stream = unused_port()
        mapping = directory / "map.tsv"
        mapping.write_text(
            f"{names['box']}\tbox\t127.0.0.1:{plain.server_port}\t-\n"
            f"{names['ramp']}\tbox\t127.0.0.1:{secure.server_port}\t{tls_name}\n"
            f"{names['railway']}\trailway\t127.0.0.1:{secure.server_port}\t{tls_name}\n"
            f"{names['stream']}:{public_stream}\tstream\t127.0.0.1:{secure.server_port}\t-\n")
        nginx_root = directory / "nginx"
        cert_root = directory / "certificates"
        for name in names.values():
            bundle = certificate("public-" + name.split(".")[0], [name])
            destination = cert_root / name
            destination.mkdir(parents=True)
            shutil.copy(bundle / "cert.pem", destination / "fullchain.pem")
            shutil.copy(bundle / "key.pem", destination / "privkey.pem")
        config_env = directory / "edge.env"
        config_env.write_text(f"EDGE_APP={mapping}\nEDGE_RPC_NAME={names['rpc']}\nEDGE_RPC_POOL={node}\n")
        resolver = re.search(r"nameserver\s+(\S+)", Path("/etc/resolv.conf").read_text())[1]
        env.update(EDGE_LOCAL="1", EDGE_NGINX_DIR=str(nginx_root),
                   EDGE_CERT_DIR=str(cert_root), EDGE_WEBROOT=str(directory / "webroot"),
                   EDGE_RESOLVER=resolver, CHECK_LIVE_EDGE_NGINX_DIR=str(nginx_root),
                   CHECK_LIVE_EDGE_BOX_CA=env["CURL_CA_BUNDLE"])
        run(["bash", str(BRINGUP / "edge.sh"), "set", str(config_env), "--render-only"], env=env)
        # Keep the actual renderer files for SSH qualification. The running
        # instance changes only machine-specific listen/trust paths, preserving
        # route, upstream, Host, SNI, verification and pool directives.
        runtime = directory / "nginx-runtime"
        runtime.mkdir()
        sites = []
        streams = []
        for index, path in enumerate(sorted(nginx_root.glob("sites-available/*"))):
            text = path.read_text()
            text = re.sub(r"(?m)^\s*listen \[::\]:(?:80|443)[^;]*;\n", "", text)
            text = re.sub(r"listen 80;", "listen 127.0.0.1:18080;", text)
            text = text.replace("listen 443 ", "listen 127.0.0.1:443 ")
            text = text.replace("/etc/layerx/ca.crt", env["CURL_CA_BUNDLE"])
            text = text.replace("/etc/ssl/certs/ca-certificates.crt", env["CURL_CA_BUNDLE"])
            file = runtime / f"site-{index}.conf"
            file.write_text(text)
            sites.append(file)
        streams = list(nginx_root.glob("edge/stream/*.conf"))
        main = runtime / "nginx.conf"
        main.write_text(
            "load_module /usr/lib/nginx/modules/ngx_stream_module.so;\n"
            f"pid {runtime}/nginx.pid;\nerror_log {runtime}/error.log info;\n"
            "events {}\nhttp { access_log off;\n" + "".join(f"include {p};\n" for p in sites)
            + "}\nstream {\n" + "".join(f"include {p};\n" for p in streams) + "}\n")
        stack.enter_context(process([nginx, "-p", str(runtime), "-c", str(main), "-g", "daemon off;"],
                                    directory / "nginx.log", port=443))
        answer = shell("check_edge", env)
        assert "pass manifest names=5" in answer.stdout
        assert "mode=rpc kind=rpc" in answer.stdout
        assert "mode=stream edge=yes certificate=match" in answer.stdout
        assert "all checks passed" in answer.stdout
        ramp = shell("check_ramp_edge " + shlex.quote(names["ramp"]), env)
        assert "mode=http kind=box" in ramp.stdout and "body=match" in ramp.stdout
        assert any(host == tls_name for host, _ in calls)
        assert any(host == names["ramp"] for host, _ in calls)
        assert sum(isinstance(body, dict) for _, body in calls) >= 2
        # A changed SNI/upstream is refused against the real rendered site,
        # through both consumers; malformed rows cannot disappear from the set.
        manifest = nginx_root / "edge/manifest"
        original = manifest.read_text()
        manifest.write_text(original.replace(tls_name, "wrong.paxeer.network"))
        for command in ("check_edge", "check_ramp_edge " + shlex.quote(names["ramp"])):
            refused = shell(command, env, ok=False)
            assert refused.returncode != 0 and "invalid" in refused.stdout
        manifest.write_text(original + "discarded http old-app 443\n")
        refused = shell("check_edge", env, ok=False)
        assert refused.returncode != 0 and "contract=invalid" in refused.stdout


@contextlib.contextmanager
def https_proxy(target_port):
    """Real HTTP CONNECT transport to an in-process TLS origin, retaining SNI."""
    class Connect(socketserver.StreamRequestHandler):
        def handle(self):
            request = self.rfile.readline().decode().split()
            while self.rfile.readline() not in (b"\r\n", b"\n", b""):
                pass
            if len(request) != 3 or request[0] != "CONNECT":
                return
            with socket.create_connection(("127.0.0.1", target_port)) as upstream:
                self.wfile.write(b"HTTP/1.1 200 Connection established\r\n\r\n")
                self.wfile.flush()
                peers = [self.connection, upstream]
                while True:
                    readable, _, _ = select.select(peers, [], [], 10)
                    if not readable:
                        return
                    for source in readable:
                        data = source.recv(65536)
                        if not data:
                            return
                        (upstream if source is self.connection else self.connection).sendall(data)

    class Proxy(socketserver.ThreadingTCPServer):
        allow_reuse_address = True
        daemon_threads = True

    proxy = Proxy(("127.0.0.1", 0), Connect)
    thread = threading.Thread(target=proxy.serve_forever, daemon=True)
    thread.start()
    try:
        yield f"http://127.0.0.1:{proxy.server_address[1]}"
    finally:
        proxy.shutdown()
        proxy.server_close()
        thread.join()


def test_search_front_roundtrip_primary_backup_and_refusals(ssh_server):
    directory, env, certificate = ssh_server
    primary, backup, validator = "api1.mainnet-beta.paxeer.network", "api2.mainnet-beta.paxeer.network", "api3.mainnet-beta.paxeer.network"
    state = {primary: "ready", backup: "ready"}
    calls = []

    class Readiness(JsonHandler):
        def do_GET(self):
            host = self.headers["Host"]
            calls.append((host, self.path))
            now = time.time_ns() // 1_000_000
            mode = state.get(host, "unavailable")
            ready = mode != "unavailable"
            value = {
                "version": 1, "ready": ready, "network_id": 125, "protocol_version": 3,
                "checked_at_unix_ms": now,
                "roles": [{"role": "paid_delivery", "state": "ready" if ready else "unavailable",
                           "freshness_budget_ms": 60_000, "first_use_deadline_unix_ms": now + 60_000,
                           "dependencies": [{"dependency": key, "state": "ready" if ready else "unavailable",
                                             "critical": True, "last_success_unix_ms": now if ready else None}
                                            for key in ("index", "content_storage", "payment_journal", "settlement_authority")]}],
            }
            if mode == "stale":
                value["checked_at_unix_ms"] = now - 61_000
            if mode == "malformed":
                value["ready"] = "true"
            self.answer(value, 200 if ready else 503)

    listing = f"serve {primary}\nserve {backup}\nvalidator {validator}\n"
    env["BRINGUP_HOSTS_FILE"] = str(BRINGUP / "tests/fixtures/repair_22_39_164/hosts.env")
    sites = {}
    mapping = directory / "rpc-apps.tsv"
    mapping.write_text("")
    for index, name in enumerate((primary, backup, validator), 1):
        nginx_root = directory / f"rpc{index}"
        config = directory / f"rpc{index}.env"
        config.write_text(f"EDGE_APP={mapping}\nEDGE_RPC_NAME={name}\nEDGE_RPC_POOL={primary}\n")
        run(["bash", str(BRINGUP / "edge.sh"), "set", str(config), "--render-only"],
            env=dict(env, EDGE_LOCAL="1", EDGE_NGINX_DIR=str(nginx_root),
                     EDGE_CERT_DIR=str(directory / "certificates"),
                     EDGE_WEBROOT=str(directory / "webroot")))
        sites[f"rpc{index}.bringup.test"] = str(nginx_root / "sites-enabled")
    # Translate only the deployment's nginx directory to each rendered fixture.
    # The discovery command and authenticated SSH transport both execute unchanged.
    adapter_dir = directory / "discovery-bin"
    adapter_dir.mkdir()
    adapter = adapter_dir / "ssh"
    adapter.write_text(
        "#!" + sys.executable + "\nimport os, sys\n"
        "sites = " + repr(sites) + "\n"
        "args = sys.argv[1:]\n"
        "if args[-2] in sites:\n"
        "    args[-1] = args[-1].replace('/etc/nginx/sites-enabled', sites[args[-2]])\n"
        "os.execv('/usr/bin/ssh', ['ssh', '-F', " + repr(str(directory / "ssh.conf")) + ", *args])\n"
    )
    adapter.chmod(0o755)
    env["PATH"] = str(adapter_dir) + ":" + env["PATH"]
    assert run(["bash", str(BRINGUP / "search-front.sh"), "names"], env=env).stdout == listing
    with http_server(Readiness, certificate=certificate("readiness", [primary, backup])) as server, https_proxy(server.server_port) as proxy:
        env.update(SEARCH_FRONT_PRIMARY=primary, SEARCH_FRONT_BACKUP=backup,
                   HTTPS_PROXY=proxy, https_proxy=proxy, NO_PROXY="", no_proxy="",
                   SEARCH_FRONT_READINESS_TIMEOUT="5")

        def render():
            return run(["bash", str(BRINGUP / "search-front.sh"), "render"], env=env).stdout

        def qualify(text, ok=True):
            return run(["bash", str(BRINGUP / "search-front.sh"), "qualify"],
                       env=env, input=text, ok=ok)

        rendered = render()
        assert f"* {primary};" in rendered
        assert qualify(rendered).stdout.strip() == primary
        state[primary] = "unavailable"
        fallback = render()
        assert f"* {backup};" in fallback
        assert qualify(fallback).stdout.strip() == backup
        state[backup] = "unavailable"
        unready = render()
        assert f"{primary} 0;" in unready
        assert qualify(unready, ok=False).returncode != 0
        state[primary] = state[backup] = "ready"
        for bad in (rendered.replace(primary, validator), rendered.replace(primary, "api9.paxeer.network"),
                    rendered.replace(f"{primary} 1;", f"{backup} 1;"),
                    rendered.replace("default 0;", "default 1;"),
                    rendered.replace(f"* {primary};", f"50% {primary};\n* {backup};")):
            assert qualify(bad, ok=False).returncode != 0
        for bad_state in ("stale", "malformed", "unavailable"):
            state[primary] = bad_state
            assert qualify(rendered, ok=False).returncode != 0
        assert (primary, "/readyz") in calls and (backup, "/readyz") in calls


CONFIG_LOADER_SOURCE = (BRINGUP / "tests/xweb-loader/src/main.rs").read_text()


@pytest.fixture(scope="session")
def config_loader(tmp_path_factory):
    target = tmp_path_factory.mktemp("xweb-loader-target")
    project = tmp_path_factory.mktemp("xweb-loader-source")
    manifest = project / "Cargo.toml"
    manifest.write_text(
        '[package]\nname = "bringup-xweb-config-loader"\nversion = "0.1.0"\n'
        'edition = "2021"\n[workspace]\n[dependencies]\n'
        'ed25519-dalek = "=2.2.0"\nserde_json = "=1.0.151"\n'
        'layerx-proof = { path = ' + json.dumps(str(ROOT / "services/agent/crates/layerx-proof")) + ' }\n'
    )
    (project / "src").mkdir()
    (project / "src/main.rs").write_text(CONFIG_LOADER_SOURCE.replace(
        '"../../../../../services/interop/crates/x-websearch/src/config.rs"',
        json.dumps(str(ROOT / "services/interop/crates/x-websearch/src/config.rs"))
    ))
    run(["cargo", "build", "--offline", "--manifest-path", str(manifest),
         "--target-dir", str(target),
         "--config", 'source.crates-io.replace-with="vendored-sources"',
         "--config", 'source.vendored-sources.directory=' + json.dumps(str(ROOT / "programs/vendor"))],
        timeout=600)
    return target / "debug/bringup-xweb-config-loader"


def test_xweb_config_roundtrip_rotation_and_refusals(material, config_loader):
    directory, env, _ = material
    assets = {symbol: {"asset_id": hashlib.sha256(symbol.encode()).hexdigest(), "symbol": symbol,
                       "decimals": decimals, "paused": False, "verification": "authenticated_committed_snapshot"}
              for symbol, decimals in (("SID", 18), ("PAX", 18), ("USDC", 6), ("USDL", 6))}
    valid = json.loads((ROOT / "services/interop/crates/x-websearch/tests/fixtures/config/valid.json").read_text())
    first = valid["gateway"]["receipt_keys"][0]
    keys = [{"keyId": "0x" + first["key_id"], "publicKey": "0x" + first["public_key"],
             "firstHeight": 0, "lastHeight": 99},
            {"keyId": "0x" + hashlib.sha256(b"rotation").hexdigest(),
             "publicKey": "0x3d4017c3e843895a92b70aa74d1b7ebc9c982ccf2ec4968cc0cd55f12af4660c",
             "firstHeight": 100, "lastHeight": 18446744073709551615}]
    history = {"keys": keys}
    calls = []

    class Node(JsonHandler):
        def do_POST(self):
            request = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
            calls.append((self.path, request, self.headers["Content-Type"]))
            assert request["jsonrpc"] == "2.0" and request["id"] == 1 and request["params"] == []
            if request["method"] == "px_listAssets" and self.path == "/gateway":
                result = {"height": 125, "assets": list(assets.values())}
            elif request["method"] == "px_getReceiptKeys" and self.path == "/":
                result = history
            else:
                self.answer({"jsonrpc": "2.0", "id": 1, "error": {"code": -32601, "message": "Method not found"}})
                return
            self.answer({"jsonrpc": "2.0", "id": 1, "result": result})

    with http_server(Node) as server:
        env.update(XWEB_SERVE_NAMES="api1.paxeer.network api2.paxeer.network", XWEB_SEEDS="https://paxeer.app/",
                   XWEB_GATEWAY_ENDPOINT=f"http://127.0.0.1:{server.server_port}/gateway",
                   XWEB_RECEIPT_KEYS_ENDPOINT=f"http://127.0.0.1:{server.server_port}/")
        for symbol, asset in assets.items():
            env["XWEB_ASSET_ID_" + symbol] = asset["asset_id"]
            env["XWEB_DECIMALS_" + symbol] = str(asset["decimals"])
        files = []
        for mode, count in (("serve", 2), ("attestor", 4)):
            destination = directory / mode
            run(["bash", str(BRINGUP / "xweb-config.sh"), "render", mode, str(destination)], env=env)
            generated = sorted(destination.glob("*.json"))
            assert len(generated) == count
            for path in generated:
                value = json.loads(path.read_text())
                assert set(value["gateway"]) == {"endpoint", "receipt_keys"}
                assert "kernel" not in value
                loaded = json.loads(run([str(config_loader), str(path)]).stdout)
                assert len(loaded["receipt_keys"]) == 2
                for source, actual in zip(keys, loaded["receipt_keys"]):
                    assert actual == {"key_id": list(bytes.fromhex(source["keyId"][2:])),
                                      "public_key": list(bytes.fromhex(source["publicKey"][2:])),
                                      "first_height": source["firstHeight"], "last_height": source["lastHeight"]}
            files.extend(generated)
        original = json.loads(files[0].read_text())
        for field, value in (("receipt_keys", []), ("sequencer_id", first["key_id"])):
            malformed = copy.deepcopy(original)
            malformed["gateway"][field] = value
            bad = directory / "invalid.json"
            bad.write_text(json.dumps(malformed))
            refusal = run([str(config_loader), str(bad)], ok=False)
            assert refusal.returncode != 0 and "gateway." in refusal.stderr
        for change in ("reversed", "weak", "overlap", "kernel"):
            malformed = copy.deepcopy(original)
            entry = malformed["gateway"]["receipt_keys"][0]
            if change == "reversed":
                entry["first_height"] = entry["last_height"] + 1
            elif change == "weak":
                entry["public_key"] = "00" * 32
            elif change == "overlap":
                malformed["gateway"]["receipt_keys"].append(copy.deepcopy(entry))
            else:
                malformed["kernel"] = {"endpoint": env["XWEB_GATEWAY_ENDPOINT"]}
            bad.write_text(json.dumps(malformed))
            assert run([str(config_loader), str(bad)], ok=False).returncode != 0
        for invalid in ({"keys": []}, {"keys": [dict(keys[0], lastHeight=-1)]},
                        {"keys": [dict(keys[0], firstHeight=True)]}, {"keys": [keys[0], keys[0]]}):
            history = invalid
            for mode in ("serve", "attestor"):
                output = directory / ("refused-" + mode)
                refused = run(["bash", str(BRINGUP / "xweb-config.sh"), "render", mode, str(output)], env=env, ok=False)
                assert refused.returncode != 0
                assert not list(output.glob("*.json"))
        assert any(path == "/" and req["method"] == "px_getReceiptKeys" for path, req, _ in calls)
        assert any(path == "/gateway" and req["method"] == "px_listAssets" for path, req, _ in calls)
        assert all(content_type == "application/json" for _, _, content_type in calls)


def test_internal_probe_kernel_identity_over_private_route_over_ssh(ssh_server):
    directory, env, certificate = ssh_server
    hosts = directory / "hosts.env"
    hosts.write_text("KERNEL_HOST=kernel\n")
    env["BRINGUP_HOSTS_FILE"] = str(hosts)
    issued = run(["bash", str(BRINGUP / "ca.sh"), "issue", "human-event-client"], env=env)
    assert "target=box:KERNEL_HOST" in issued.stdout
    identity = directory / "tls/human-event-client"
    cert = x509.load_pem_x509_certificate((identity / "cert.pem").read_bytes())
    assert cert.subject.get_attributes_for_oid(NameOID.COMMON_NAME)[0].value == "layerx-human-events"
    assert cert.extensions.get_extension_for_class(x509.SubjectAlternativeName).value.get_values_for_type(
        x509.UniformResourceIdentifier) == ["urn:layerx:webhooks:role:producer"]
    seen = []

    class Internal(JsonHandler):
        def do_GET(self):
            peer = self.connection.getpeercert()
            seen.append((self.headers["Host"], self.path, peer))
            authorized = (("commonName", "layerx-human-events"),) in peer["subject"]
            self.answer({"ready": authorized}, 200 if authorized else 403)

    names = [f"internal-{role}.railway.internal" for role in ("kms", "journeys", "payments", "approvals", "programs")]
    with http_server(Internal, certificate=certificate("internal", names), ca=env["CURL_CA_BUNDLE"]) as server:
        for role in ("kms", "journeys", "payments", "approvals", "programs"):
            env["CHECK_LIVE_INTERNAL_CONNECT_" + role.upper()] = f"127.0.0.1:{server.server_port}"
            result = shell(f"internal_probe internal-{role} {role}", env)
            assert f'@@{role} 0 200 {{"ready":true}}' in result.stdout
        assert len(seen) == 5
        assert {name for name, _, _ in seen} == {name + ":9443" for name in names}
        assert all(path == "/readyz" for _, path, _ in seen)
        assert list((directory / "tls").glob("*/key.pem")) == [identity / "key.pem"]
        key = identity / "key.der"
        original = key.read_bytes()
        key.write_bytes(b"corrupted credential")
        corrupted = shell("internal_probe internal-kms kms", env)
        assert corrupted.stdout.strip() == "@@kms 1 000 refused"
        assert len(seen) == 5
        key.write_bytes(original)
        recovered = shell("internal_probe internal-kms kms", env)
        assert recovered.stdout.strip() == '@@kms 0 200 {"ready":true}'
        assert len(seen) == 6
        # No identity exists on a role; a missing kernel credential fails closed.
        missing_env = dict(env, LAYERX_TLS_DIR=str(directory / "role-without-identity"))
        missing = shell("internal_probe internal-kms kms", missing_env)
        assert missing.stdout.strip() == "@@kms 1 000 refused"
        assert len(seen) == 6
        wrong = shell("internal_probe unknown-service kms", env)
        assert wrong.stdout.strip() == '@@kms 0 200 {"ready":true}'
        assert len(seen) == 7


def test_receipt_history_rpc_refuses_malformed_envelopes():
    """Exercise the production embedded RPC client without a shell or toolchain."""
    source = (BRINGUP / "xweb-config.sh").read_text().split("xweb_rpc_py='\n", 1)[1].split("\n'\n", 1)[0]
    client = {}
    exec(compile(source, str(BRINGUP / "xweb-config.sh"), "exec"), client)
    payload = b'{"jsonrpc":"2.0","id":1,"result":{"keys":[]}}'
    calls = []

    class Node(JsonHandler):
        def do_POST(self):
            calls.append((self.path, self.headers["Content-Type"],
                          json.loads(self.rfile.read(int(self.headers["Content-Length"])))))
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(payload)))
            self.end_headers()
            self.wfile.write(payload)

    with http_server(Node) as server:
        endpoint = f"http://127.0.0.1:{server.server_port}/node"
        assert client["rpc"](endpoint, "px_getReceiptKeys", []) == ({"keys": []}, None)
        for payload in (
            b'{"jsonrpc":"1.0","id":1,"result":{"keys":[]}}',
            b'{"jsonrpc":"2.0","id":true,"result":{"keys":[]}}',
            b'{"jsonrpc":"2.0","id":1,"id":1,"result":{"keys":[]}}',
            b'{"jsonrpc":"2.0","id":1,"result":{"keys":[],"keys":[]}}',
            b'{"jsonrpc":"2.0","id":1,"result":{"keys":[]},"error":{"code":-32603}}',
            b'{"jsonrpc":"2.0","id":1}',
        ):
            value, error = client["rpc"](endpoint, "px_getReceiptKeys", [])
            assert value is None and error is not None
        for error_value in (None, False, 1, "unavailable", ["unavailable"], {},
                            {"code": True, "message": "unavailable"},
                            {"code": -32603}, {"code": -32603, "message": []}):
            payload = json.dumps({"jsonrpc": "2.0", "id": 1, "error": error_value}).encode()
            assert client["rpc"](endpoint, "px_getReceiptKeys", []) == (None, "malformed")
        payload = b'{"jsonrpc":"2.0","id":1,"error":{"code":-32603,"message":"unavailable"}}'
        assert client["rpc"](endpoint, "px_getReceiptKeys", []) == (None, "error=-32603")
    assert len(calls) == 17
    assert all(path == "/node" and content_type == "application/json" and
               request == {"jsonrpc": "2.0", "id": 1, "method": "px_getReceiptKeys", "params": []}
               for path, content_type, request in calls)


@pytest.fixture
def recorded_session(tmp_path):
    """Only the SSH/DNS peers are replayed; bringup clients execute unchanged."""
    fixture = BRINGUP / "tests/fixtures/repair_22_31_319"
    state_path = tmp_path / "session-state.json"
    state = {
        "bindings": {}, "responses": {}, "log": str(tmp_path / "session.jsonl"),
    }
    bindir = tmp_path / "recorded-peers"
    bindir.mkdir()
    for name in ("ssh", "getent"):
        adapter = bindir / name
        adapter.write_text(
            "#!/bin/sh\nexec " + shlex.join([sys.executable, str(fixture / "replay.py"), name]) + ' "$@"\n'
        )
        adapter.chmod(0o755)

    def save():
        state_path.write_text(json.dumps(state))

    def calls():
        log = Path(state["log"])
        return [json.loads(line) for line in log.read_text().splitlines()] if log.exists() else []

    yield fixture, state, save, calls, {
        "PATH": str(bindir) + ":" + os.environ["PATH"],
        "BRINGUP_TRANSCRIPT_STATE": str(state_path),
    }
    assert all(call["matched"] is not None for call in calls()), calls()


def test_edge_manifest_roundtrip_both_consumers(material, recorded_session):
    directory, env, certificate = material
    fixture, session, save, calls, peer_env = recorded_session
    names = {kind: f"{kind}.bringup.test" for kind in ("box", "ramp", "railway", "stream", "rpc")}
    public_calls = []
    chain_id = "0x7d"

    class Public(JsonHandler):
        def do_GET(self):
            public_calls.append((self.headers["Host"], self.path))
            assert self.path == "/readyz"
            assert self.headers["Host"] in {names[kind] for kind in ("box", "ramp", "railway")}
            self.answer({"ready": True})

        def do_POST(self):
            request = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
            public_calls.append((self.headers["Host"], request))
            assert self.path == "/" and self.headers["Host"] == names["rpc"]
            assert self.headers["Content-Type"] == "application/json"
            assert request == {"jsonrpc": "2.0", "id": 1, "method": "eth_chainId", "params": []}
            self.answer({"jsonrpc": "2.0", "id": 1, "result": chain_id})

    with http_server(Public, certificate=certificate("public", list(names.values()))) as public, \
            https_proxy(public.server_port) as proxy, \
            http_server(Public, certificate=fixture) as stream:
        mapping = directory / "map.tsv"
        mapping.write_text(
            f"{names['box']}\tbox\t10.0.0.5:8080\t-\n"
            f"{names['ramp']}\tbox\t10.0.0.6:9443\tbackend.bringup.test\n"
            f"{names['railway']}\trailway\tservice.up.railway.app:443\tbackend.bringup.test\n"
            f"{names['stream']}:{stream.server_port}\tstream\t10.0.0.7:9443\t-\n"
        )
        nginx_root = directory / "nginx"
        cert_root = directory / "certificates"
        for name in names.values():
            bundle = cert_root / name
            bundle.mkdir(parents=True)
            # The renderer tests certificate presence; real public/stream TLS
            # handshakes above independently use the matching test material.
            shutil.copy(fixture / "cert.pem", bundle / "fullchain.pem")
            shutil.copy(fixture / "key.pem", bundle / "privkey.pem")
        config_env = directory / "edge.env"
        config_env.write_text(
            f"EDGE_APP={mapping}\nEDGE_RPC_NAME={names['rpc']}\n"
            "EDGE_RPC_POOL='api1.mainnet-beta.paxeer.network api2.mainnet-beta.paxeer.network'\n"
        )
        env.update(peer_env, EDGE_LOCAL="1", EDGE_NGINX_DIR=str(nginx_root),
                   EDGE_CERT_DIR=str(cert_root), EDGE_WEBROOT=str(directory / "webroot"),
                   CHECK_LIVE_EDGE_NGINX_DIR=str(nginx_root),
                   CHECK_LIVE_EDGE_BOX_CA=env["CURL_CA_BUNDLE"],
                   EDGE_HOST="edge.bringup.test", VALIDATOR_HOSTS="validator.bringup.test",
                   HTTPS_PROXY=proxy, https_proxy=proxy, NO_PROXY="", no_proxy="")
        # Render-only is the production CLI, requiring neither nginx nor SSH.
        run(["bash", str(BRINGUP / "edge.sh"), "set", str(config_env), "--render-only"], env=env)
        manifest = nginx_root / "edge/manifest"
        original = manifest.read_text()
        assert len(original.splitlines()) == 5
        assert all(len(line.split()) == 6 for line in original.splitlines())
        snapshot = directory / "edge-stdout.txt"
        sites = "".join(path.read_text() for pattern in ("sites-available/*", "edge/stream/*.conf")
                        for path in sorted(nginx_root.glob(pattern)))

        def capture(rows):
            snapshot.write_text(rows + "@@sites\n" + sites)

        capture(original)
        session["bindings"] = {"NGINX_DIR": str(nginx_root), "SNAPSHOT": str(snapshot),
                               "BOX_CA": env["CURL_CA_BUNDLE"]}
        save()
        answer = shell("check_edge", env)
        assert "pass manifest names=5" in answer.stdout
        assert "mode=rpc kind=rpc" in answer.stdout
        assert "mode=stream edge=yes certificate=match" in answer.stdout
        assert "all checks passed" in answer.stdout
        ramp_command = "check_ramp_edge " + shlex.quote(names["ramp"])
        ramp = shell(ramp_command, env)
        assert "mode=http kind=box" in ramp.stdout and "body=match" in ramp.stdout
        matched = {call["matched"] for call in calls()}
        assert {"edge-read", "box-upstream", "ramp-upstream", "railway-upstream", "stream-upstream",
                "rpc-api1.mainnet-beta.paxeer.network", "rpc-api2.mainnet-beta.paxeer.network"} <= matched
        assert {host for host, _ in public_calls} == {names[kind] for kind in ("box", "ramp", "railway", "rpc")}
        for changed in (original.replace("backend.bringup.test", "wrong.bringup.test"),
                        original.replace("10.0.0.6:9443", "10.0.0.9:9443"),
                        original + "discarded http old-app 443\n"):
            capture(changed)
            before = len(calls())
            for command in ("check_edge", ramp_command):
                refused = shell(command, env, ok=False)
                assert refused.returncode != 0 and "invalid" in refused.stdout
            assert all(call["matched"] == "edge-read" for call in calls()[before:])
        capture(original)
        session["responses"]["ramp-upstream"] = '{"ready":false}'
        save()
        for command in ("check_edge", ramp_command):
            refused = shell(command, env, ok=False)
            assert refused.returncode != 0 and "upstream=differ" in refused.stdout
        session["responses"].clear()
        save()
        chain_id = "0x1"
        refused = shell("check_edge", env, ok=False)
        assert refused.returncode != 0 and "chain-id=invalid" in refused.stdout


def test_internal_probe_kernel_identity_over_private_route(ssh_server, recorded_session):
    directory, live_env, certificate = ssh_server
    fixture, session, save, calls, peer_env = recorded_session
    live_env.update(KERNEL_HOST="kernel.bringup.test", LAYERX_TLS_DIR=str(directory / "kernel-tls"),
               CHECK_LIVE_TIMEOUT="10")
    hosts = directory / "hosts.env"
    hosts.write_text("KERNEL_HOST=kernel.bringup.test\n")
    live_env["BRINGUP_HOSTS_FILE"] = str(hosts)
    run(["bash", str(BRINGUP / "ca.sh"), "issue", "human-event-client"], env=live_env)
    env = dict(live_env, **peer_env)
    # Bind the caller to the actual credential producer and deployment plan.
    roster = run(["bash", str(BRINGUP / "ca.sh"), "services"], env=env).stdout
    rows = {line.split()[0]: line.split() for line in roster.splitlines()}
    assert rows["human-event-client"] == [
        "human-event-client", "box:KERNEL_HOST", "volume", "layerx-human-events", "clientAuth",
        "URI:urn:layerx:webhooks:role:producer",
    ]
    deployment = (ROOT / "docker/platform-internal/files.tsv").read_text()
    assert "human-event-client" not in deployment
    assert "key.pem" not in deployment
    session["bindings"] = {"TLS_DIR": env["LAYERX_TLS_DIR"],
                           "MISSING_TLS_DIR": str(directory / "role-without-identity"),
                           "INSPECTOR": env.get("BRINGUP_INSPECT_BIN", "bringup-inspect")}
    seen = []

    class Internal(JsonHandler):
        def do_GET(self):
            peer = self.connection.getpeercert()
            seen.append((self.headers["Host"], self.path, peer))
            authorized = (("commonName", "layerx-human-events"),) in peer["subject"]
            self.answer({"ready": authorized}, 200 if authorized else 403)

    roles = ("kms", "journeys", "payments", "approvals", "programs")
    names = [f"internal-{role}.railway.internal" for role in roles]
    with http_server(Internal, certificate=certificate("internal", names), ca=env["CURL_CA_BUNDLE"]) as server:
        session["bindings"]["PRIVATE_TARGET"] = f"127.0.0.1:{server.server_port}"
        save()
        # Each recorded exchange must also be produced by the real inspector
        # over authenticated SSH and the private TLS route in this run.
        transcript = json.loads((fixture / "session.json").read_text())
        for exchange in transcript["exchanges"]:
            if not exchange["id"].startswith("internal-"):
                continue
            arguments = []
            for argument in exchange["argv"]:
                for key, value in session["bindings"].items():
                    argument = argument.replace("{" + key + "}", value)
                arguments.append(argument)
            produced = run(["ssh", *arguments], env=live_env, ok=False)
            assert produced.returncode == exchange["exit"], produced.stdout + produced.stderr
            assert produced.stdout == exchange["stdout"]
        assert len(seen) == 5
        assert {host for host, _, _ in seen} == {name + ":9443" for name in names}
        assert all(path == "/readyz" for _, path, _ in seen)

        for role in roles:
            assert f"/data/tls/internal-{role}/key.der" in deployment
            env["CHECK_LIVE_INTERNAL_CONNECT_" + role.upper()] = session["bindings"]["PRIVATE_TARGET"]
            answer = shell(f"internal_probe internal-{role} {role}", env)
            assert answer.stdout.strip() == f'@@{role} 0 200 {{"ready":true}}'
        assert len(calls()) == 5
        assert {call["matched"] for call in calls()} == {"internal-" + role for role in roles}
        assert all(call["argv"][-2] == "kernel.bringup.test" for call in calls())
        missing_env = dict(env, LAYERX_TLS_DIR=session["bindings"]["MISSING_TLS_DIR"])
        missing = shell("internal_probe internal-kms kms", missing_env)
        assert missing.stdout.strip() == "@@kms 1 000 refused"
        assert calls()[-1]["matched"] == "internal-missing-identity"
        before = len(calls())
        bad_env = dict(env, CHECK_LIVE_INTERNAL_CONNECT_KMS="bad route;exit 0")
        malformed = shell("internal_probe internal-kms kms", bad_env)
        assert malformed.stdout.strip() == "@@kms 1 000 refused"
        assert len(calls()) == before + 1
        assert calls()[-1]["matched"] == "internal-invalid-route"
        assert len(seen) == 5
    # check_internal must use this authorized caller instead of running the
    # command inside a role whose deployment has no client PEM material.
    source = (BRINGUP / "check-live.sh").read_text()
    readiness_loop = source.split("for group in kms journeys payments approvals programs; do", 1)[1].split("done", 1)[0]
    assert 'internal_probe "$app" "$group"' in readiness_loop
    assert "railway_ssh" not in readiness_loop
    assert list((directory / "kernel-tls").rglob("key.pem")) == [
        directory / "kernel-tls/human-event-client/key.pem"
    ]
