"""Source-contract checks and real helper tests; no wasmd/rly toolchain required."""

import ast
import hashlib
import importlib.util
import inspect
import json
from pathlib import Path
import subprocess
import selectors
import sys

import pytest


ROOT = Path(__file__).resolve().parents[3]
HARNESS_ROOT = ROOT / "wasm/contrib/relayer-tests"


def load_harness():
    spec = importlib.util.spec_from_file_location("relayer_harness", HARNESS_ROOT / "harness.py")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


harness = load_harness()


def source(name):
    return (HARNESS_ROOT / name).read_text()


@pytest.mark.parametrize("name", ["init_two_chainz_relayer.sh", "one_chain.sh", "test_ibc_transfer.sh"])
def test_repaired_shell_syntax(name):
    result = subprocess.run(["bash", "-n", str(HARNESS_ROOT / name)], capture_output=True, text=True)
    assert result.returncode == 0, result.stderr


def test_finding_14_shared_home_and_all_relayer_calls():
    init = source("init_two_chainz_relayer.sh")
    transfer = source("test_ibc_transfer.sh")
    for script in (init, transfer):
        assert 'RELAYER_HOME="$SCRIPTDIR/.relayer"' in script
        assert '"$HARNESS" homes "$SCRIPTDIR"' in script or '"$SCRIPTDIR/harness.py" homes "$SCRIPTDIR"' in script
    calls = [line.strip() for line in init.splitlines() if line.strip().startswith("rly ")]
    assert len(calls) == 7
    assert all(line.startswith('rly --home "$RELAYER_HOME" ') for line in calls)
    for command in ("config init", "config add-chains", "config add-paths", "keys restore ibc-0", "keys restore ibc-1", "light init ibc-0", "light init ibc-1"):
        assert any(command in line for line in calls)
    assert 'rm -rf -- "$WASMD_DATA" "$RELAYER_HOME"' in init
    assert 'exec python3 "$SCRIPTDIR/harness.py" transfer "$RELAYER_HOME" "$SCRIPTDIR/data"' in transfer
    call = ast.parse(inspect.getsource(harness.Relayer.call).lstrip()).body[0]
    command = next(node.value for node in ast.walk(call) if isinstance(node, ast.Assign) and any(isinstance(t, ast.Name) and t.id == "command" for t in node.targets))
    assert [item.value for item in command.elts[:2]] == ["rly", "--home"]
    assert isinstance(command.elts[2], ast.Attribute) and command.elts[2].attr == "home"
    workflow = inspect.getsource(harness.run_transfer)
    for command in ('"tx", "link"', '"paths", "show"', '"q", "bal"', '"tx", "transfer"', '"tx", "relay-packets"', '"tx", "relay-acknowledgements"', '"q", "txs"'):
        assert command in workflow
    assert '"--path", "demo"' in workflow


def test_home_cleanup_refuses_symlinks(tmp_path):
    outside = tmp_path / "outside"
    outside.mkdir()
    root = tmp_path / "harness"
    root.mkdir()
    (root / ".relayer").symlink_to(outside, target_is_directory=True)
    with pytest.raises(harness.Refusal, match="symlink"):
        harness.check_homes(root)
    assert outside.is_dir()
    with pytest.raises(harness.Refusal, match="identity"):
        harness.chain_home(root, "../outside")


def test_finding_15_scoped_lifecycle_source():
    init = source("init_two_chainz_relayer.sh")
    one = source("one_chain.sh")
    assert "killall" not in init
    assert init.index('"$HARNESS" stop "$WASMD_DATA"') < init.index("rm -rf")
    for script in (init, one):
        assert "trap cleanup EXIT" in script
        assert "trap 'exit 130' INT" in script
        assert "trap 'exit 143' TERM" in script
    assert 'supervisors+=("$!")' in init and 'wait "$supervisor"' in init
    assert 'child=$!' in one and 'wait "$child"' in one
    assert '"$HARNESS" record "$CHAINDIR" "$CHAINID" "$child" "$BINARY"' in one
    assert '"$HARNESS" stop "$CHAINDIR" "$CHAINID" "$child"' in one
    assert '"$BINARY" --home "$CHAINDIR/$CHAINID" start' in one
    stop = inspect.getsource(harness.stop_children)
    assert stop.index("if owned():") < stop.index("signal_owned(pid, identity, signal.SIGTERM)")
    signal_source = inspect.getsource(harness.signal_owned)
    assert signal_source.index("current == identity") < signal_source.index("signal.pidfd_send_signal")
    assert "os.pidfd_open(pid)" in signal_source
    assert "PID identity changed; refusing signal" in stop
    assert "while owned()" in stop and "time.monotonic()" in stop


def spawn_process(home):
    # This is an actual local process, testing the generic process-ownership
    # component. It does not stand in for wasmd or claim to exercise consensus.
    child = subprocess.Popen(
        [sys.executable, "-c", "import time; print('ready', flush=True); time.sleep(60)",
         "--home", str(home)],
        stdout=subprocess.PIPE, text=True,
    )
    try:
        with selectors.DefaultSelector() as ready:
            ready.register(child.stdout, selectors.EVENT_READ)
            assert ready.select(timeout=2), "Child did not acknowledge startup"
            assert child.stdout.readline() == "ready\n"
    except BaseException:
        child.terminate()
        child.wait(timeout=2)
        raise
    finally:
        child.stdout.close()
    return child


def test_finding_15_real_process_ownership_and_reopen(tmp_path):
    home = harness.chain_home(tmp_path, "ibc-0")
    home.mkdir()
    child = spawn_process(home)
    unrelated = spawn_process(tmp_path / "unrelated")
    try:
        saved = harness.record_child(tmp_path, "ibc-0", child.pid, sys.executable)
        record = tmp_path / "pids/ibc-0.json"
        assert json.loads(record.read_text()) == saved
        reopened = load_harness()
        assert reopened.process_identity(child.pid)["birth"] == saved["birth"]
        for field, wrong in (("birth", "other-process"), ("binary", "/wrong/binary"), ("home", str(tmp_path / "unrelated"))):
            record.write_text(json.dumps({**saved, field: wrong}))
            with pytest.raises(reopened.Refusal):
                reopened.stop_children(tmp_path)
            assert child.poll() is None
            assert unrelated.poll() is None
        record.write_text(json.dumps(saved))
        with pytest.raises(reopened.Refusal, match="another child"):
            reopened.stop_children(tmp_path, "ibc-0", unrelated.pid)
        reopened.stop_children(tmp_path)
        assert child.wait(timeout=2) == -15
        assert unrelated.poll() is None
        assert not record.exists()
        # A new component instance sees the persisted stopped state.
        load_harness().stop_children(tmp_path)
        assert unrelated.poll() is None
    finally:
        for process in (child, unrelated):
            if process.poll() is None:
                process.terminate()
            process.wait(timeout=2)


def test_finding_15_wrong_home_and_stale_record(tmp_path):
    child = spawn_process(tmp_path / "outside")
    try:
        with pytest.raises(harness.Refusal, match="home mismatch"):
            harness.record_child(tmp_path, "ibc-0", child.pid, sys.executable)
        assert child.poll() is None
        saved = harness.record_child(tmp_path, "outside", child.pid, sys.executable)
        child.terminate()
        child.wait(timeout=2)
        assert harness.process_identity(saved["pid"]) is None
        harness.stop_children(tmp_path)
        assert not (tmp_path / "pids/outside.json").exists()
    finally:
        if child.poll() is None:
            child.terminate()
        child.wait(timeout=2)


def test_finding_13_both_platforms_and_generated_ports(tmp_path):
    script = source("one_chain.sh")
    rewrites = [line for line in script.splitlines() if "localhost:6060" in line]
    assert len(rewrites) == 2
    assert all('$PROFPORT' in line and '$P2PPORT' not in line for line in rewrites)
    assert script.index('"$HARNESS" config ') < script.index('start --pruning=')
    assert '--grpc.address="0.0.0.0:$GRPCPORT"' in script
    assert sum("sed -i" in line and "$GRPCPORT" in line for line in script.splitlines()) == 2
    config = tmp_path / "config.toml"
    config.write_text('[rpc]\nladdr = "tcp://0.0.0.0:26557"\npprof_laddr = "localhost:6061"\n[p2p]\nladdr = "tcp://0.0.0.0:26556"\n')
    app = tmp_path / "app.toml"
    app.write_text('[grpc]\naddress = "0.0.0.0:9091"\n')
    requested = ("26557", "26556", "6061", "9091")
    assert harness.check_config(config, requested) == 9091
    for values in (("26557", "26556", "26556", "9091"), ("0", "1", "2", "3"), ("1", "2", "3", "65536"), ("1", "2", "3", "x")):
        with pytest.raises(harness.Refusal):
            harness.ports(values)
    for wrong in ('[grpc]\naddress = "0.0.0.0:9090"\n',):
        app.write_text(wrong)
        with pytest.raises(harness.Refusal, match="gRPC"):
            harness.check_config(config, requested)
    app.write_text('[grpc]\naddress = "0.0.0.0:9091"\n')
    config.write_text(config.read_text().replace("localhost:6061", "localhost:26556"))
    with pytest.raises(harness.Refusal, match="Profiling"):
        harness.check_config(config, requested)
    init = source("init_two_chainz_relayer.sh")
    assert "chainid0=ibc-0" in init and "chainid1=ibc-1" in init
    for chain in ("ibc-0", "ibc-1"):
        assert json.loads(source(f"configs/wasmd/chains/{chain}.json"))["chain-id"] == chain


@pytest.mark.parametrize("invalid", ["", " ", "null", "{}", "[]", "not a balance", "100test garbage", "1test,2test", "-1test", "1.5test"])
def test_finding_16_reject_empty_or_unparseable_balance(invalid):
    with pytest.raises(harness.Refusal):
        harness.balance(invalid)


def test_finding_16_ics20_denom_and_strict_amounts():
    assert harness.balance("100000000000test,9ibc/ABC\n") == {"test": 100000000000, "ibc/ABC": 9}
    dst = {"port-id": "transfer", "channel-id": "channel-7"}
    assert harness.voucher_denom(dst) == "ibc/" + hashlib.sha256(b"transfer/channel-7/test").hexdigest().upper()
    assert harness.voucher_denom(dst) != harness.voucher_denom({**dst, "channel-id": "channel-0"})
    receipt = inspect.getsource(harness.receipt_complete)
    assert 'ack == {"result": "AQ=="}' in receipt
    assert 'event["packet_sequence"] == packet["sequence"]' in receipt
    assert '"write_acknowledgement"' in receipt and '"acknowledge_packet"' in receipt
    assert 'current_dst.get(denom, 0) - initial_dst.get(denom, 0)' in receipt
    assert 'initial_src["test"] - current_src["test"]' in receipt
    assert 'successful and acknowledged and delta == 1000000 and debit == 1000000' in receipt
    # No evidence of delivery is never a successful receipt, even when balances
    # have moved by exactly the requested amount.
    assert not harness.receipt_complete({"sequence": "1"}, {}, dst, [], [], {"test": 2000000}, {}, {"test": 1000000}, {harness.voucher_denom(dst): 1000000})


def test_finding_16_producer_fields_packet_identity_poll_and_evidence():
    producer = (ROOT / "interchain/modules/core/04-channel/keeper/events.go").read_text()
    keys = (ROOT / "interchain/modules/core/04-channel/types/events.go").read_text()
    for field in ("packet_sequence", "packet_src_port", "packet_src_channel", "packet_dst_port", "packet_dst_channel", "packet_data", "packet_ack"):
        assert f'"{field}"' in keys
    assert "string(packet.GetData())" in producer and "string(acknowledgement)" in producer
    identity = inspect.getsource(harness.find_packet)
    assert 'txhash in previous' in identity
    assert '"amount": "1000000"' in identity and '"receiver": receiver' in identity and '"sender": sender' in identity
    assert 'len(packets) <= 1' in identity
    workflow = inspect.getsource(harness.run_transfer)
    assert workflow.index("initial_src =") < workflow.index('"tx", "transfer"')
    assert workflow.index("previous =") < workflow.index('"tx", "transfer"')
    assert "while time.monotonic() < relayer.deadline" in workflow
    assert 'receipt_complete(' in workflow and 'packet.json' in workflow and 'receipt.json' in workflow
    assert 'Timed out awaiting successful acknowledgement and exact balances' in workflow
    call = inspect.getsource(harness.Relayer.call)
    assert '.stdout' in call and '.stderr' in call and 'result.returncode == 0' in call
    assert 'timeout=min(10, left)' in call
    for text in ("", "null", "{}", '{"txs":[],"total_count":"1"}'):
        with pytest.raises((harness.Refusal, ValueError, TypeError)):
            harness.transactions(text)
    assert harness.transactions('[]') == []
    assert workflow.count('"--ibc-denoms"') == 4
    parser = inspect.getsource(harness.events)
    assert 'tx["tx_result"]["events"]' in parser
    assert parser.count("validate=True") == 2


def test_repair_record_maps_every_finding_and_limits():
    record = (ROOT / "tools/qualification/audit_repairs/repair_22_33_1230.kvx").read_text()
    for finding in (13, 14, 15, 16):
        assert f"[finding.{finding}]" in record
        assert f"test_finding_{finding}_" in record
    assert "not run" in record and "hf-done" in record
    assert "review_only" in record and "wiring_requests" in record
