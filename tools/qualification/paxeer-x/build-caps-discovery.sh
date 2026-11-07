#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/../../.."
: "${CAPS_DISCOVERY_BUILD_DIR:?private absolute output directory required}"
: "${CAPS_RUST_TOOLCHAIN:?explicit Rust toolchain directory required}"
: "${CARGO_TARGET_DIR:?private Rust target directory required}"
export RUSTC="$CAPS_RUST_TOOLCHAIN/bin/rustc"
export RUSTDOC="$CAPS_RUST_TOOLCHAIN/bin/rustdoc"
export CARGO_BUILD_JOBS=5
umask 077
python3 - <<'PREPARE'
import os,pathlib,subprocess
out=pathlib.Path(os.environ['CAPS_DISCOVERY_BUILD_DIR'])
if not out.is_absolute() or out.is_symlink():raise SystemExit('private absolute build output required')
if subprocess.check_output(['git','status','--porcelain']):raise SystemExit('clean committed source required')
out.mkdir(parents=True,exist_ok=True);out.chmod(0o700)
if (out/'manifest.json').exists():raise SystemExit('refuse reusing task artifact manifest')
PREPARE
make -j5 BUILD_DIR="$CAPS_DISCOVERY_BUILD_DIR/native" PROGRAMS_CARGO="$CAPS_RUST_TOOLCHAIN/bin/cargo" LXP_REVISION="$(git rev-parse HEAD)" build programs-build > "$CAPS_DISCOVERY_BUILD_DIR/native-build.log" 2>&1
cc -std=c17 -pedantic -Werror -Wall -Wextra -Wconversion -Wshadow -Wvla -fno-strict-aliasing -ffp-contract=off -O2 -ffunction-sections -fdata-sections -Iinclude -I"$CAPS_DISCOVERY_BUILD_DIR/native/generated" tests/qualification/lxp_wallet_caps_discovery.c "$CAPS_DISCOVERY_BUILD_DIR/native/liblayerx.a" programs/target/debug/liblayerx_programs_sandbox.a "$CAPS_DISCOVERY_BUILD_DIR/native/liblayerx.a" -Wl,--gc-sections -lcrypto -lsqlite3 -pthread -ldl -lm -o "$CAPS_DISCOVERY_BUILD_DIR/lxp_wallet_caps_discovery" > "$CAPS_DISCOVERY_BUILD_DIR/fixture-build.log" 2>&1
"$CAPS_RUST_TOOLCHAIN/bin/cargo" test --locked --manifest-path services/agent/Cargo.toml -p layerx-client --test caps_discovery --no-run --message-format=json > "$CAPS_DISCOVERY_BUILD_DIR/client-build.jsonl" 2> "$CAPS_DISCOVERY_BUILD_DIR/client-build.log"
python3 - <<'MANIFEST'
import importlib.util,json,os,pathlib,subprocess,sys
sys.path.insert(0, 'tools/qualification/paxeer-x')
import caps_discovery_contract as contract
def publish_and_admit(out, source, native, rust):
    path = contract.publish(out, source, native, rust)
    spec = importlib.util.spec_from_file_location('caps_discovery_gate',
                                                 pathlib.Path(contract.__file__).with_name('caps-discovery.py'))
    gate = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(gate)
    value = gate.admit_manifest(path, source)
    if value['artifacts'] != {'native': contract.artifact(native), 'rust': contract.artifact(rust)}:
        raise RuntimeError('producer-to-consumer artifact identity mismatch')
    return path

out=pathlib.Path(os.environ['CAPS_DISCOVERY_BUILD_DIR'])
rows=[json.loads(line) for line in (out/'client-build.jsonl').read_text().splitlines()]
paths=[r['executable'] for r in rows if r.get('reason')=='compiler-artifact' and r.get('target',{}).get('name')=='caps_discovery' and r.get('executable')]
if len(paths)!=1:raise SystemExit('missing or ambiguous Rust caps test executable')
def git(*args):return subprocess.check_output(['git',*args],text=True).strip()
if git('status','--porcelain'):raise SystemExit('source changed during build')
publish_and_admit(out, {'revision':git('rev-parse','HEAD'),'source_tree':git('rev-parse','HEAD^{tree}')},
                 out/'lxp_wallet_caps_discovery', paths[0])
MANIFEST
