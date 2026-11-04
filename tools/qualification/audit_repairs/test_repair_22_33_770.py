"""Offline source proof for the production RPC simulation/exposure profile."""

import ast
import ipaddress
from pathlib import Path
import re
import tomllib
from urllib.parse import urlsplit


ROOT = Path(__file__).resolve().parents[3]
APP = ROOT / "docker/rpcnode/config/app.toml"
RECORD = Path(__file__).with_name("repair_22_33_770.kvx")


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def app_config():
    return tomllib.loads(APP.read_text(encoding="utf-8"))


def go_function(text, declaration):
    """Isolate a top-level Go function for source assertions, without compiling."""
    start = text.index(declaration)
    end = text.find("\nfunc ", start + len(declaration))
    return text[start:] if end == -1 else text[start:end]


def test_rpc_unbounded_simulation_finite_production_limit():
    evm = app_config()["evm"]
    limit = evm["max_concurrent_simulation_calls"]
    assert type(limit) is int
    assert limit == 4
    assert limit > 0
    assert evm["enable_test_api"] is False
    assert "max-concurrent-simulation-calls" not in evm


def test_rpc_unbounded_simulation_private_auxiliary_api_policy():
    config = app_config()
    expected = {
        "api": "tcp://127.0.0.1:1317",
        "grpc": "127.0.0.1:9090",
        "grpc-web": "127.0.0.1:9091",
    }
    for section, address in expected.items():
        listener = config[section]
        assert listener["enable"] is True
        assert listener["address"] == address
        parsed = urlsplit(address if "://" in address else "tcp://" + address)
        assert ipaddress.ip_address(parsed.hostname).is_loopback
        assert parsed.port in {1317, 9090, 9091}
    assert config["api"]["swagger"] is False
    assert config["api"]["enabled-unsafe-cors"] is False
    assert config["grpc-web"]["enable-unsafe-cors"] is False
    assert config["rosetta"]["enable"] is False


def test_rpc_unbounded_simulation_explicit_private_diagnostics_override():
    text = APP.read_text(encoding="utf-8")
    policy = text.split("# Diagnostics override:", 1)[1].split(
        "# Legacy pax_*", 1
    )[0]
    assignments = [
        line[2:] for line in policy.splitlines()
        if re.match(r"^# [a-z_]+ = ", line)
    ]
    override = tomllib.loads("\n".join(assignments))
    assert override == {
        "http_address": "127.0.0.1",
        "ws_enabled": False,
        "enable_test_api": True,
        "max_concurrent_simulation_calls": 4,
    }
    assert "separate private, non-live-chain node only" in policy
    assert "private tunnel or an authenticated reverse proxy" in policy
    assert "Keep auxiliary APIs on loopback and unsafe CORS off" in policy
    assert "Never apply this" in policy and "override to a public node" in policy
    assert "Restore the production settings and restart" in policy
    assert "http_address" not in app_config()["evm"]
    assert "ws_enabled" not in app_config()["evm"]
    reader = source("rpc/config/config.go")
    for name, field, cast in (
        ("HTTPAddress", "HTTPAddress", "String"),
        ("WSEnabled", "WSEnabled", "Bool"),
        ("EnableTestAPI", "EnableTestAPI", "Bool"),
        ("MaxConcurrentSimulationCalls", "MaxConcurrentSimulationCalls", "Int"),
    ):
        key = {
            "HTTPAddress": "http_address",
            "WSEnabled": "ws_enabled",
            "EnableTestAPI": "enable_test_api",
            "MaxConcurrentSimulationCalls": "max_concurrent_simulation_calls",
        }[name]
        assert re.search(rf'flag{name}\s*= "evm\.{key}"', reader)
        if name == "HTTPAddress":
            assert re.search(
                r"if v := opts.Get\(flagHTTPAddress\); v != nil \{\s*"
                r"cfg.HTTPAddress, err = cast.ToStringE\(v\)\s*"
                r"if err != nil \{\s*return cfg, err\s*}\s*"
                r"if net.ParseIP\(cfg.HTTPAddress\) == nil \{\s*"
                r'return cfg, fmt.Errorf\("evm.http_address must be a literal IP address"\)',
                reader,
            )
        else:
            assert re.search(
                rf"if v := opts.Get\(flag{name}\); v != nil \{{\s*"
                rf"if cfg\.{field}, err = cast.To{cast}E\(v\); err != nil",
                reader,
            )


def test_rpc_unbounded_simulation_config_reaches_real_limiter_and_test_gate():
    # Trace the committed producer/copy/decoder/server/consumer source path.
    init = source("docker/rpcnode/scripts/step1_configure_init.sh")
    assert "cp docker/rpcnode/config/app.toml ~/.pax/config/app.toml" in init
    assert not re.search(r"sed[^\n]*(?:simulation_calls|enable_test_api)", init)
    config = source("rpc/config/config.go")
    assert re.search(
        r'MaxConcurrentSimulationCalls int `mapstructure:"max_concurrent_simulation_calls"`',
        config,
    )
    assert 'EnableTestAPI bool `mapstructure:"enable_test_api"`' in config
    node = source("node/app.go")
    assert "app.evmRPCConfig, err = ReadEVMRPCConfig(appOpts)" in node
    assert "config, err := evmrpcconfig.ReadConfig(appOpts)" in node
    servers = source("rpc/server.go")
    for constructor in ("NewEVMHTTPServer", "NewEVMWebSocketServer"):
        assert f"evmrpc.{constructor}(app.evmRPCConfig," in node
        body = go_function(servers, f"func {constructor}(")
        assert re.search(
            r"MaxConcurrentSimulationCalls:\s*config.MaxConcurrentSimulationCalls,", body
        )
        assert "NewSimulationAPI(" in body
        assert "simulateConfig," in body
    simulation = source("rpc/simulate.go")
    factory = go_function(simulation, "func NewSimulationAPI(")
    assert re.search(
        r"if config.MaxConcurrentSimulationCalls > 0 \{\s*"
        r"api.requestLimiter = semaphore.NewWeighted\(int64\(config.MaxConcurrentSimulationCalls\)\)",
        factory,
    )
    for method in ("Call", "EstimateGas", "EstimateGasAfterCalls"):
        body = go_function(simulation, f"func (s *SimulationAPI) {method}(")
        assert "if s.requestLimiter != nil {" in body
        assert re.search(
            r"if !s.requestLimiter.TryAcquire\(1\) \{\s*"
            r'returnErr = errors.New\("[^"\n]*rejected due to rate limit: server busy"\)\s*return\s*}',
            body,
        )
        assert "defer s.requestLimiter.Release(1)" in body
        assert body.index("TryAcquire(1)") < body.index("s.backend.")
    http = go_function(servers, "func NewEVMHTTPServer(")
    assert re.search(
        r"if config.EnableTestAPI && !evmCfg.IsLiveChainID\(ctx\) \{\s*"
        r'logger.Info\("Enabling Test EVM APIs"\)\s*'
        r"apis = append\(apis, rpc.API\{\s*"
        r'Namespace: "test",\s*Service:\s*NewTestAPI\(\),', http
    )


def test_rpc_unbounded_simulation_auxiliary_policy_reaches_sdk_consumers():
    reader = source("sdk/server/config/config.go")
    for section in ("api", "grpc", "grpc-web"):
        for key in ("enable", "address"):
            getter = "Bool" if key == "enable" else "String"
            assert f'v.Get{getter}("{section}.{key}")' in reader
    assert 'v.GetBool("api.enabled-unsafe-cors")' in reader
    assert 'v.GetBool("grpc-web.enable-unsafe-cors")' in reader
    api = source("sdk/server/api/server.go")
    assert "tmrpcserver.Listen(cfg.API.Address, tmCfg.MaxOpenConnections)" in api
    assert "if cfg.API.EnableUnsafeCORS {" in api
    start = source("sdk/server/start.go")
    assert "servergrpc.StartGRPCServer(clientCtx, app, config.GRPC.Address)" in start
    assert "servergrpc.StartGRPCWeb(grpcSrv, config)" in start
    grpc = source("sdk/server/grpc/server.go")
    assert 'net.Listen("tcp", address)' in grpc
    web = source("sdk/server/grpc/grpc_web.go")
    assert "if config.GRPCWeb.EnableUnsafeCORS {" in web
    assert re.search(r"Addr:\s*config.GRPCWeb.Address,", web)
    assert "grpcWebSrv.ListenAndServe()" in web


def test_rpc_unbounded_simulation_record_maps_proof_and_limits():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    repair = record["repair"]
    assert repair["id"] == "22.33.770"
    assert repair["requirement"] == "22998.1"
    assert repair["base_revision"] == "76c050242a1edcad6212aa52fac15670ecb5612d"
    assert repair["verify_cmd"] == (
        "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_770.py"
    )
    assert repair["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in repair["evidence"]
    assert "not executed" in repair["review_only"]
    assert "No pre-change verification was run" in repair["baseline"]
    finding = record["finding"]["rpc-unbounded-simulation"]
    assert finding["audit"] == "22.30.174"
    assert finding["file"] == "docker/rpcnode/config/app.toml"
    assert finding["class"] == "config-drift"
    assert finding["severity"] == "medium"
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == declared
    assert repair["simulation_limit"] == app_config()["evm"]["max_concurrent_simulation_calls"]
    assert repair["wiring_requests"]
    for request in repair["wiring_requests"]:
        assert "docker/rpcnode" in request
        assert "owning domain 16" in request
