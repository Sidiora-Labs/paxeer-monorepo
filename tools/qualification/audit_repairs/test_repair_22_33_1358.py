"""Offline source proof of the root-chain image and archived-key boundary.

The Docker recipe is the production component under repair. These checks parse
its actual instructions and trace committed build/configuration consumers; they
do not execute Docker, Go, Rust, shell tools, or a network service.
"""

import hashlib
import json
from pathlib import Path
import re
import shlex
import tarfile
import tomllib


ROOT = Path(__file__).resolve().parents[3]
NODE = ROOT / "sdk/contrib/rosetta/node"
RECIPE = NODE / "Dockerfile.paxd"
RECORD = ROOT / "tools/qualification/audit_repairs/repair_22_33_1358.kvx"


def docker_stages():
    stages = {}
    current = None
    for line in RECIPE.read_text(encoding="utf-8").splitlines():
        if not line.strip() or line.lstrip().startswith("#"):
            continue
        opcode, argument = line.split(maxsplit=1)
        assert opcode in {
            "FROM", "COPY", "ENV", "RUN", "WORKDIR", "LABEL", "USER",
            "EXPOSE", "STOPSIGNAL", "ENTRYPOINT", "CMD",
        }, line
        if opcode == "FROM":
            base, keyword, name = shlex.split(argument)
            assert keyword == "AS" and name not in stages
            current = {"base": base, "instructions": []}
            stages[name] = current
        else:
            assert current is not None
            current["instructions"].append((opcode, argument))
    return stages


def instructions(stage, opcode):
    return [arg for op, arg in stage["instructions"] if op == opcode]


def environment(stage):
    values = {}
    for argument in instructions(stage, "ENV"):
        for item in shlex.split(argument):
            key, value = item.split("=", 1)
            assert key not in values
            values[key] = value
    return values


def test_rosetta_node_image_source_builds_root_chain_and_native_dependencies():
    stages = docker_stages()
    assert list(stages) == ["go-dist", "build", "runtime", "archived-fixture", "paxd"]
    go_version = re.search(r"^go (\S+)$", (ROOT / "go.mod").read_text(), re.M)[1]
    rust_version = tomllib.loads((ROOT / "rust-toolchain.toml").read_text())["toolchain"]["channel"]
    assert stages["go-dist"]["base"] == f"golang:{go_version}-bookworm"
    build = stages["build"]
    assert build["base"] == f"rust:{rust_version}-bookworm"
    assert instructions(build, "COPY") == [
        "--from=go-dist /usr/local/go /usr/local/go", ". .",
    ]
    assert instructions(build, "WORKDIR") == ["/src"]
    assert environment(build)["PATH"].split(":")[:2] == [
        "/usr/local/go/bin", "/usr/local/cargo/bin",
    ]
    runs = instructions(build, "RUN")
    assert len(runs) == 2
    assert shlex.split(runs[1]) == ["make", "-f", "chain.mk", "build", "LEDGER_ENABLED=false"]
    packages = shlex.split(runs[0].split("install -y --no-install-recommends ", 1)[1].split(" && ", 1)[0])
    assert {"build-essential", "pkg-config", "libssl-dev", "git", "ca-certificates"} <= set(packages)
    chain = (ROOT / "chain.mk").read_text()
    assert re.search(r"^build: paxd-native-libs\n\tgo build \$\(BUILD_FLAGS\) -o ./build/paxd ./daemon/paxd$", chain, re.M)
    assert "$(MAKE) -C $(PAXEER_PROJECT_HOME) -f Makefile paxd-native-libs" in chain
    assert "NativeArchiveDigest=$(native_archive_digest)" in chain
    makefile = (ROOT / "Makefile").read_text()
    assert "paxd-native-libs: $(LIBRARY) programs-release-build" in makefile
    assert "LIBRARY := $(BUILD_DIR)/liblayerx.a" in makefile
    assert 'build --locked --release --config \'profile.release.panic="unwind"\'' in makefile
    assert "-p layerx-programs-sandbox --features layerx-programs-sandbox/host-ffi" in makefile
    assert (ROOT / "daemon/paxd/main.go").is_file()
    assert not (ROOT / "simapp/simd").exists()
    assert not (ROOT / "sdk/go.mod").exists()


def test_rosetta_node_image_source_transports_binary_libraries_and_configuration():
    stages = docker_stages()
    runtime = stages["runtime"]
    assert runtime["base"] == "debian:bookworm-slim"
    copies = instructions(runtime, "COPY")
    assert copies[:3] == [
        "--from=build /src/build/paxd /usr/local/bin/paxd",
        "--from=build /src/wasm/x/wasm/artifacts/v152/api/libwasmvm152.*.so /usr/local/lib/",
        "--from=build /src/wasm/x/wasm/artifacts/v155/api/libwasmvm155.*.so /usr/local/lib/",
    ]
    for version in ("152", "155"):
        directory = ROOT / f"wasm/x/wasm/artifacts/v{version}/api"
        for arch in ("x86_64", "aarch64"):
            assert (directory / f"libwasmvm{version}.{arch}.so").is_file()
            linker = (directory / f"link_glibclinux_{arch}.go").read_text()
            assert f"-lwasmvm{version}.{arch}" in linker
    assert environment(runtime)["LD_LIBRARY_PATH"] == "/usr/local/lib"
    assert "ca-certificates libssl3 libgcc-s1" in instructions(runtime, "RUN")[0]
    sources = shlex.split(copies[3])
    assert len(copies) == 4
    assert sources[-1] == "/opt/rosetta/configuration/"
    assert sources[:-1] == [
        f"sdk/contrib/rosetta/configuration/{name}"
        for name in ("rosetta.json", "bootstrap.json", "fixture.json", "transfer.ros")
    ]
    for source in sources[:-1]:
        assert (ROOT / source).is_file()
    assert instructions(runtime, "ENTRYPOINT") == ['["/usr/local/bin/paxd"]']
    assert instructions(runtime, "USER") == ["10001:10001"]
    assert "-m 0700 /home/rosetta /home/rosetta/.pax" in instructions(runtime, "RUN")[1]
    assert instructions(runtime, "WORKDIR") == ["/home/rosetta"]
    assert instructions(runtime, "STOPSIGNAL") == ["SIGTERM"]


def test_rosetta_node_image_source_starts_registered_paxd_rosetta_path():
    runtime = docker_stages()["runtime"]
    assert json.loads(instructions(runtime, "CMD")[0]) == [
        "start", "--home", "/home/rosetta/.pax", "--pruning", "nothing",
    ]
    env = environment(runtime)
    assert {key: value for key, value in env.items() if key.startswith("PAXD_")} == {
        "PAXD_ROSETTA_ENABLE": "true", "PAXD_ROSETTA_ADDRESS": ":8080",
        "PAXD_ROSETTA_BLOCKCHAIN": "app", "PAXD_ROSETTA_NETWORK": "network",
        "PAXD_ROSETTA_OFFLINE": "false", "PAXD_ROSETTA_RETRIES": "5",
        "PAXD_GRPC_ENABLE": "true", "PAXD_GRPC_ADDRESS": "127.0.0.1:9090",
    }
    root = (ROOT / "daemon/paxd/cmd/root.go").read_text()
    assert "server.AddCommands(" in root
    registration = (ROOT / "sdk/server/util.go").read_text()
    assert "startCmd := StartCmd(appCreator, defaultNodeHome, tracerProviderOptions)" in registration
    assert 'serverCtx.Viper.SetEnvPrefix(basename)' in registration
    assert 'strings.NewReplacer(".", "_", "-", "_")' in registration
    assert "serverCtx.Viper.AutomaticEnv()" in registration
    config = (ROOT / "sdk/server/config/config.go").read_text()
    for key, getter in (
        ("enable", "Bool"), ("address", "String"), ("blockchain", "String"),
        ("network", "String"), ("offline", "Bool"), ("retries", "Int"),
    ):
        assert f'v.Get{getter}("rosetta.{key}")' in config
    start = (ROOT / "sdk/server/start.go").read_text()
    for connection in (
        "if config.Rosetta.Enable {", "if !config.GRPC.Enable {",
        "TendermintRPC:     ctx.Config.RPC.ListenAddress,",
        "GRPCEndpoint:      config.GRPC.Address,",
        "Addr:              config.Rosetta.Address,",
        "rosetta.ServerFromConfig(conf)", "rosettaSrv.Start()",
    ):
        assert connection in start
    consumer = json.loads((ROOT / "sdk/contrib/rosetta/configuration/rosetta.json").read_text())
    assert consumer["network"] == {
        "blockchain": env["PAXD_ROSETTA_BLOCKCHAIN"], "network": env["PAXD_ROSETTA_NETWORK"],
    }
    assert consumer["online_url"] == "http://rosetta" + env["PAXD_ROSETTA_ADDRESS"]
    assert instructions(runtime, "EXPOSE") == [env["PAXD_ROSETTA_ADDRESS"].lstrip(":")]


def test_rosetta_node_image_source_isolates_archived_test_keys():
    stages = docker_stages()
    assert stages["paxd"] == {"base": "runtime", "instructions": []}
    archive = stages["archived-fixture"]
    assert archive["base"] == "runtime"
    assert instructions(archive, "COPY") == [
        "--chmod=0400 sdk/contrib/rosetta/node/data.tar.gz /opt/rosetta/archived/data.tar.gz",
    ]
    assert instructions(archive, "RUN") == []
    label = instructions(archive, "LABEL")[0]
    assert "TEST ONLY" in label and "public archived simd keys" in label
    # Only this optional stage receives the archive; the default runtime has no
    # extraction command or restored home, and never chmods fixture keys 0777.
    assert {op for op, _ in archive["instructions"]} == {"LABEL", "COPY"}
    assert not any("data.tar.gz" in arg or ".simapp" in arg or "0777" in arg
                   for op, arg in stages["runtime"]["instructions"])
    fixture = json.loads((ROOT / "sdk/contrib/rosetta/configuration/fixture.json").read_text())
    assert hashlib.sha256((NODE / "data.tar.gz").read_bytes()).hexdigest() == fixture["archive_sha256"]
    with tarfile.open(NODE / "data.tar.gz", "r:gz") as archived:
        names = archived.getnames()
        assert "root/.simapp/config/priv_validator_key.json" in names
        assert any(name.startswith("root/.simapp/keyring-test/") for name in names)
        genesis_file = archived.extractfile("root/.simapp/config/genesis.json")
        assert genesis_file is not None
        genesis = json.load(genesis_file)
    assert genesis["chain_id"] == fixture["chain_id"] == "testing"
    readme = (NODE / "README.md").read_text()
    for statement in ("default image excludes", "test-only signing keys", "never use those keys",
                      "does not initialize genesis", "no registered"):
        assert statement in readme


def test_rosetta_node_image_source_preserves_withdrawal_and_records_limits():
    for name in ("Dockerfile", "dockerfile", "Containerfile"):
        assert not (NODE / name).exists()
    historical = (NODE / "Dockerfile.historical").read_bytes().splitlines(keepends=True)
    assert hashlib.sha256(b"".join(historical[2:])).hexdigest() == (
        "c9a95b7ffe543572a427bb313e52646f80518b2679a4bffad8731731ff2c0a43"
    )
    record = tomllib.loads(RECORD.read_text())
    assert record["repair"]["id"] == "22.33.1358"
    assert record["repair"]["base_revision"] == "0ba7cfa54c7f07b37201b1f12fcc2f1dd7ed0ed7"
    assert record["repair"]["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "not executed" in record["repair"]["review_only"]
    assert "No baseline failure is claimed" in record["baseline"]["verify"]
    assert set(record["finding"]) == {"rosetta-node-image-source"}
    finding = record["finding"]["rosetta-node-image-source"]
    assert finding["source"] == "sdk/contrib/rosetta/node/Dockerfile.paxd"
    expected_tests = {name for name in globals() if name.startswith("test_rosetta_node_image_source_")}
    assert set(finding["tests"]) == expected_tests
    for name in finding["tests"]:
        assert callable(globals()[name])
    for request in record["wiring"].values():
        assert request["owner_domain"] == 15
        assert (ROOT / request["path"]).is_file()
        assert request["request"]
