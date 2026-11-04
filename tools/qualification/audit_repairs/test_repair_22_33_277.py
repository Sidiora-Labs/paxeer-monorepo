"""Offline source proof and real filesystem recovery for bridge dry runs.

Native chains and Go are deliberately not invoked by this Python gate. The
record distinguishes their reviewed shell wiring from the helper behavior
executed here. hf-build owns the prescribed bash -n invocation.
"""

import ast
import base64
import json
from pathlib import Path
import re
import runpy
import shlex
import shutil
import tomllib

import pytest


ROOT = Path(__file__).resolve().parents[3]
TESTS = ROOT / "services/bridge/deploy/tests"
RECORD = Path(__file__).with_name("repair_22_33_277.kvx")


def source(name):
    return (TESTS / name).read_text()


def shell(name):
    # Exclude embedded languages so an assertion cannot match a Go comment or
    # Python string in place of the shell command that actually performs it.
    text = source(name)
    text = re.sub(r"(<< '([A-Z]+)'[^\n]*\n).*?\n\2\n", r"\1", text, flags=re.S)
    return "\n".join(
        line for line in text.replace("\\\n", " ").splitlines()
        if not line.lstrip().startswith("#")
    )


def function(text, name):
    match = re.search(rf"^{name}\(\) \{{\n(.*?)^\}}", text, re.M | re.S)
    assert match, f"missing shell function {name}"
    return match[1]


def ordered(text, *facts):
    offset = 0
    for fact in facts:
        position = text.find(fact, offset)
        assert position >= 0, f"missing or misordered: {fact}"
        offset = position + len(fact)


def recovery():
    return runpy.run_path(str(TESTS / "recovery.py"))


def test_generated_envelopes_reach_real_governance_consumer():
    helper = shell("persisted-paxeer.sh")
    submit = function(helper, "submit_bridge_proposal")
    ordered(
        submit,
        'paxeer_tx gov submit-proposal layerxbridge-proposal "$file"',
        "jq -e '.code == 0'",
        '.type == "submit_proposal"',
        'paxeer_tx gov vote "$id" yes',
        'paxd query gov proposal "$id"',
        'expect "governance outcome of ${file##*/}" "$wanted" "$status"',
    )
    assert '--broadcast-mode block' in function(helper, "paxeer_tx")
    for chain in ("evm", "solana"):
        text = shell(f"{chain}-dry-run-check.sh")
        ordered(
            text,
            'go run ./services/bridge/deploy/proposals/cmd/paxeer-bridge-proposals',
            '-proposals "$WORK/proposals"',
            '. "$SCRIPT_DIR/persisted-paxeer.sh"',
            'prepare_paxeer',
            'submit_bridge_proposal "$WORK/proposals/04-proposal-open-chain.json" PROPOSAL_STATUS_PASSED',
            'run_checklist "$WORK/chains"',
        )
        assert 'app.Setup(' not in source(f"{chain}-dry-run-check.sh")
        assert 'bridgetestutil' not in source(f"{chain}-dry-run-check.sh")
        assert 'func TestPaxeerSide(t *testing.T)' in source(f"{chain}-dry-run-check.sh")
        assert 'bridgecli.DecodeBridgeProposal(' in source(f"{chain}-dry-run-check.sh")
    cli = (ROOT / "modules/layerxbridge/client/cli/tx.go").read_text()
    handler = (ROOT / "modules/layerxbridge/handler.go").read_text()
    assert 'ProposalCommandName = "layerxbridge-proposal"' in cli
    assert 'NewSubmitBridgeProposalMsg(clientCtx.Codec, contents' in cli
    ordered(handler, 'proposal.ValidateBasic()', 'proposal.GetMessages()', 'ctx.CacheContext()', 'execute(cacheCtx, msgServer, msg)', 'write()')
    assert 'msgServer.RegisterSidioraPair(goCtx, m)' in handler


def test_evm_recovery_proof():
    text = shell("evm-dry-run-check.sh")
    start = function(text, "start_anvil")
    command = next(line for line in start.splitlines() if line.lstrip().startswith("anvil "))
    tokens = shlex.split(command)
    assert tokens[tokens.index("--state") + 1] == "$WORK/anvil-state.json"
    assert '--preserve-historical-states' in tokens
    assert tokens[tokens.index("--host") + 1] == "127.0.0.1"
    release = function(text, "release")
    ordered(release, 'cast wallet sign --no-hash', '{args: [$asset, $amount, $recipient, $burn, $nonce, $signatures]}',
            'recovery.py" save "$WORK/releases" "$nonce"',
            'send "$DEPOSITOR_KEY" "$VAULT"', 'NullifierUsed')
    restart = text[text.index('> "$WORK/governance-before.json"'):]
    ordered(restart, 'stop "$PAXEER_PID"', 'stop "$ANVIL_PID"',
            '[ -s "$WORK/anvil-state.json" ]', 'start_anvil', 'start_paxeer',
            'assert_paxeer_recovery', 'checklist-recovered.log',
            'assert_readback', 'for nonce in 1 2',
            'recovery.py" read "$WORK/releases" "$nonce"',
            'cast call --from "$DEPOSITOR"', 'NullifierUsed',
            'cast send --private-key "$DEPOSITOR_KEY" --gas-limit 500000 --json',
            '"${release_args[@]}"', '0x0', 'assert_readback',
            'stop "$PROXY_PID"', 'replay_fixture "$RECORDING"')
    retry = restart[:restart.index('stop "$PROXY_PID"')]
    assert 'wallet sign' not in retry
    assert 'random32' not in retry
    readback = function(text, "assert_readback")
    for fact in ('native coin outstanding', 'token outstanding', "'balanceOf(address)'", 'eth_getBalance', "'nullified(bytes32)'", '.nullifiers[]'):
        assert fact in readback
    assert 'assertRefusals(t, registerPath, &register)' in source("evm-dry-run-check.sh")


def test_solana_governance_bypass():
    text = shell("solana-dry-run-check.sh")
    ordered(text,
            'submit_bridge_proposal "$WORK/proposals/04-proposal-open-chain.json" PROPOSAL_STATUS_PASSED',
            'Sidiora unregistered before the generated pair proposal',
            'endswith(".MsgSetCap")',
            'submit_bridge_proposal "$WORK/missing-registration.json" PROPOSAL_STATUS_FAILED',
            "grep -qF 'register the Sidiora pair before setting its cap'",
            'missing registration left Sidiora unregistered and uncapped',
            '.messages[0].authority = $unauthorized',
            'if paxeer_tx gov submit-proposal layerxbridge-proposal "$WORK/unauthorized-registration.json"',
            'fail "governance accepted an unauthorized Sidiora registration"',
            "grep -qF 'not the governance module account'",
            'unauthorized registration left Sidiora unregistered and uncapped',
            'submit_bridge_proposal "$WORK/proposals/05-proposal-sidiora-cap.json" PROPOSAL_STATUS_PASSED')
    raw = source("solana-dry-run-check.sh")
    assert 'EnsureSidioraDenom(' not in raw
    assert 'k.SetCap(' not in raw
    assert 'messages[0].(*types.MsgRegisterSidioraPair)' in raw
    assert 'messages[1].(*types.MsgSetCap)' in raw
    producer = (ROOT / "services/bridge/deploy/proposals/proposals.go").read_text()
    ordered(producer[producer.index('func (b Bundle) Proposals()'):],
            'Name: OpenChainProposalFile', 'b.SidioraPair', 'pair, sidiora)', 'Name: SidioraCapProposalFile')
    keeper = (ROOT / "modules/layerxbridge/keeper/msg_server.go").read_text()
    ordered(keeper[keeper.index('func (k Keeper) RegisterSidioraPair'):],
            'msg.ValidateBasic()', 'k.requireAuthority(ctx, msg.Authority)', 'k.EnsureSidioraDenom(')


def test_solana_recovery_proof():
    text = shell("solana-dry-run-check.sh")
    start = function(text, "start_validator")
    assert '--ledger "$WORK/ledger"' in start
    assert '--reset' not in start
    ordered(start, 'if [ "${1:-}" = bootstrap ]', 'genesis_args=(--mint', 'solana-test-validator', '"${genesis_args[@]}"', 'solana-test-validator did not become healthy', 'settle "$VALIDATOR_URL"')
    assert 'rm -rf "$WORK/ledger"' not in text
    release = function(text, "release")
    ordered(release, 'cast wallet sign --no-hash', '"${TX[@]}" secp-data',
            'recovery.py" save "$WORK/releases" "$paxeer_nonce" "$WORK/release.json"',
            'send_instructions "the release of burn')
    restart = text[text.index('> "$WORK/governance-before.json"'):]
    ordered(restart, 'stop "$PAXEER_PID"', 'stop "$VALIDATOR_PID"',
            '[ -d "$WORK/ledger" ]', 'start_validator\n', 'start_paxeer',
            'assert_paxeer_recovery', 'checklist-recovered.log', 'assert_readback',
            'for nonce in 1 2', 'recovery.py" read "$WORK/releases" "$nonce"',
            '"${TX[@]}" send "$VALIDATOR_URL" "$WORK/retry.json" "$WORK/keys/publisher.json"',
            '[ "$status" -ne 0 ]', '$REPLAYED_ERROR', 'assert_readback',
            'stop "$PROXY_PID"', 'replay_fixture "$RECORDING"')
    retry = restart[:restart.index('stop "$PROXY_PID"')]
    assert 'wallet sign' not in retry
    assert 'secp-data' not in retry
    assert 'bootstrap' not in retry
    readback = function(text, "assert_readback")
    for fact in ('.nullifiers', 'PXBRNUL0', '.balances', 'token_amount', 'outstanding amount of $mint', '${data:162:16}'):
        assert fact in readback


def test_paxd_reopens_database_without_initialization_or_governance_replay():
    text = shell("persisted-paxeer.sh")
    prepare = function(text, "prepare_paxeer")
    ordered(prepare, 'PAXEER_HOME="$WORK/paxd"', 'paxd init', 'recovery.py" configure', 'paxd keys add', 'paxd add-genesis-account', 'paxd gentx', 'paxd collect-gentxs', 'start_paxeer')
    start = function(text, "start_paxeer")
    assert 'paxd start --home "$PAXEER_HOME"' in start
    assert 'paxd init' not in start
    assert 'submit-proposal' not in start
    assert 'catching_up == false' in start
    assert 'eth_chainId' in start
    recovered = function(text, "assert_paxeer_recovery")
    assert 'paxd query gov proposals' in recovered
    assert 'jq -S' in recovered
    assert '|| fail "paxd lost or changed its governance state on restart"' in recovered
    for chain in ('evm', 'solana'):
        stop = function(shell(f"{chain}-dry-run-check.sh"), "stop")
        ordered(stop, 'kill "$target"', 'wait "$target"', 'if kill -0 "$target"', 'fail "the node process $target did not stop"')


@pytest.mark.parametrize("chain", ["evm", "solana"])
def test_recovery_journal_stops_and_reopens_component(tmp_path, chain):
    # The actual journal transports committed recorded bytes. This proves its
    # durability, not the chain semantics represented by those recorded bytes.
    raw = (TESTS / f"fixtures/{chain}_dry_run.json").read_bytes()
    journal = recovery()["ReleaseJournal"](tmp_path / "releases")
    journal.save(1, raw)
    path = journal.path(1)
    assert path.stat().st_mode & 0o777 == 0o600
    with pytest.raises(FileExistsError):
        journal.save(1, raw)
    del journal
    # A fresh module load and object have no reference to the previous store.
    reopened = recovery()["ReleaseJournal"](tmp_path / "releases")
    assert reopened.read(1) == raw
    assert json.loads(reopened.read(1))["expectations"] == json.loads(raw)["expectations"]
    with pytest.raises(FileNotFoundError):
        reopened.read(2)


@pytest.mark.parametrize("nonce", ["0", "-1", "../1", "/1", "1/2", "", "1\n", "01"])
def test_journal_refuses_invalid_nonce(tmp_path, nonce):
    journal = recovery()["ReleaseJournal"](tmp_path)
    with pytest.raises(ValueError, match="positive integer"):
        journal.path(nonce)


def test_journal_refuses_corrupted_input_and_reopened_state(tmp_path):
    journal = recovery()["ReleaseJournal"](tmp_path)
    with pytest.raises(json.JSONDecodeError):
        journal.save(1, b"{")
    assert not journal.path(1).exists()
    journal.path(1).write_bytes(b"{")
    del journal
    with pytest.raises(json.JSONDecodeError):
        recovery()["ReleaseJournal"](tmp_path).read(1)


def test_configuration_updates_real_committed_toml_without_other_changes(tmp_path):
    path = tmp_path / "config.toml"
    shutil.copyfile(ROOT / "docker/localnode/config/config.toml", path)
    before = tomllib.loads(path.read_text())
    update = recovery()["set_toml"]
    for section, key, value in (("rpc", "laddr", "tcp://127.0.0.1:31001"),
                                ("p2p", "laddr", "tcp://127.0.0.1:31002"),
                                ("p2p", "bootstrap-peers", ""),
                                ("p2p", "persistent-peers", ""),
                                ("p2p", "pex", False)):
        update(path, section, key, value)
        before[section][key] = value
    assert tomllib.loads(path.read_text()) == before
    saved = path.read_bytes()
    with pytest.raises(ValueError, match="missing"):
        update(path, "p2p", "nonexistent-key", True)
    assert path.read_bytes() == saved


def test_configuration_keys_match_current_production_templates():
    tree = ast.parse(source("recovery.py"))
    configure = next(node for node in tree.body if isinstance(node, ast.FunctionDef) and node.name == "configure")
    loops = [node for node in configure.body if isinstance(node, ast.For) and isinstance(node.target, ast.Tuple)]
    assert len(loops) == 2
    templates = [ROOT / "consensus/config/toml.go", ROOT / "sdk/server/config/toml.go"]
    for loop, template in zip(loops, templates):
        text = template.read_text()
        if template.name == 'toml.go' and 'sdk' in template.parts:
            text += (ROOT / "rpc/config/config.go").read_text()
        for entry in loop.iter.elts:
            section, key = (ast.literal_eval(node) for node in entry.elts[:2])
            if section:
                assert f"[{section}]" in text
            assert re.search(rf"^{re.escape(key)}\s*=", text, re.M), (section, key)
    assert 'bridge.get(key)' in source("recovery.py")
    assert 'dormant bridge genesis' in source("recovery.py")
    # configure may validate bridge state, but must not seed registration.
    for node in ast.walk(configure):
        if isinstance(node, ast.Assign):
            for target in node.targets:
                assert not (isinstance(target, ast.Subscript) and isinstance(target.value, ast.Name) and target.value.id == "bridge")


def test_committed_solana_asset_readback_covers_outstanding_amounts():
    fixture = json.loads((TESTS / "fixtures/solana_dry_run.json").read_text())
    assets = []
    for entry in fixture["endpoints"]["solana"]["*"]:
        if entry["method"] == "getAccountInfo":
            raw = base64.b64decode(entry["result"]["value"]["data"][0])
            if raw[:8] == b"PXBRAST0":
                assert len(raw) == 89
                assert raw[8:10] == b"\x00\x01"
                assets.append(int.from_bytes(raw[81:89], "big"))
    expected = [int(balance["amount"]) for balance in fixture["expectations"]["balances"]
                if balance["owner"] == fixture["record"]["vault_authority"]]
    assert len(assets) == len(expected) == 2
    assert assets == expected
    layout = (ROOT / "contracts/bridge/solana/src/state.rs").read_text()
    assert 'outstanding: read_u64(&bytes[81..89])?' in layout


def test_embedded_python_parses_and_record_maps_every_finding():
    for name in ("evm-dry-run-check.sh", "solana-dry-run-check.sh", "persisted-paxeer.sh"):
        for block in re.findall(r"<< 'PY'\n(.*?)\nPY", source(name), re.S):
            ast.parse(block)
    record = tomllib.loads(RECORD.read_text())
    assert set(record['finding']) == {'evm-recovery-proof', 'solana-governance-bypass', 'solana-recovery-proof'}
    declared = {node.name for node in ast.parse(Path(__file__).read_text()).body if isinstance(node, ast.FunctionDef)}
    for finding in record['finding'].values():
        assert finding['tests']
        assert set(finding['tests']) <= declared
    assert record['task']['baseline'].startswith('Not run:')
    assert 'review' in record['task']['native_recovery_evidence'].lower()
    assert record['task']['wiring_requests'] == []
