"""Python qualification and source contracts for repair 22.33.1019.

The CLI input and Linux listener checks execute production Python in-process.
Go source contracts bind the native production-path tests to the repair. They
do not claim to execute Go: the prescribed Python gate forbids a Go toolchain
or a native test artifact. The repair record retains that qualification gap.
"""
import ast
import importlib.util
import multiprocessing
import os
from pathlib import Path
import re
import socket
import threading
import unittest

import pytest


HERE = Path(__file__).resolve().parent
APP = HERE.parents[1] / 'app' / 'app.go'
TOKENS = re.compile(r'//[^\n]*|/\*[\s\S]*?\*/|`[^`]*`|"(?:\\.|[^"\\])*"|\'(?:\\.|[^\'\\])*\'|[{}]')


def load_python(name, path):
    spec = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def go_source(path):
    return TOKENS.sub(lambda match: '' if match[0].startswith(('//', '/*')) else match[0], path.read_text())


def go_body(path, declaration):
    source = go_source(path)
    start = source.index('{', source.index(declaration))
    depth = 0
    for token in TOKENS.finditer(source, start):
        if token[0] == '{':
            depth += 1
        elif token[0] == '}':
            depth -= 1
            if depth == 0:
                return source[start + 1:token.start()]
    raise AssertionError('unterminated Go declaration: ' + declaration)


def native_test(name):
    return go_body(HERE / 'repair_1019_test.go', 'func ' + name + '(t *testing.T)')


def test_nonclaim_market_preserves_history_and_eligible_reward_source_contract():
    body = go_body(HERE / 'journeys.go', 'func (s *JourneyService) Rewards(')
    assert 'claimableRaw := "0"' in body
    strategy = body[body.index('switch p.Capability.Strategy'):body.index('earned, _ :=')]
    assert 'case "CLAIM":' in strategy
    assert 'if !p.Capability.Available' in strategy
    assert 'claimableRaw = p.ClaimableRaw' in strategy
    alternatives = strategy.split('case "BURN", "AIRDROP", "LP_REWARDS":', 1)[1].split('default:', 1)
    assert 'return' not in alternatives[0]
    assert 'claimableRaw =' not in alternatives[0]
    assert 'return out, ErrUnavailable' in alternatives[1]
    assert 'claimed := relevant.claimed' in body
    assert 'SetString(claimed, 10)' in body
    assert 'earned.Add(earned, claimable)' in body
    assert 'ClaimedRaw: iface.AggregateUInt(claimed)' in body
    entitlement = go_body(HERE / 'journeys.go', 'func (s *JourneyService) HolderEntitlement(')
    assert 's.ledger.EpochEntitlement(ctx, s.creator, m.Pool, epoch, who)' in entitlement
    preparation = go_body(HERE / 'creator.go', 'func (c *Creator) PrepareClaim(')
    assert 'p.Capability.Strategy != "CLAIM"' in preparation
    proof = native_test('TestRepair1019MixedStrategyRewards')
    for criterion in ('[]uint8{1, 2, 3}', 'repair62Rewards(', 'f.handler(f.chain.Alice)',
                      '"11000000"', '"10000000"', '*page.NextCursor',
                      'PrepareClaim(', 'map[string]uint8{"1": 255}', 'ErrUnavailable'):
        assert criterion in proof


def test_community_policy_restart_source_contract():
    body = go_body(HERE / 'journeys.go', 'func (w *CommunityWatcher) RunOnce(')
    assert 'SELECT owner,policy_digest FROM l18_community_history' in body
    assert 'ORDER BY revision DESC LIMIT 1' in body
    assert '.Scan(&owner, &policyDigest)' in body
    assert 'changed := e == pgx.ErrNoRows || owner != preview.Owner || policyDigest != w.policyDigest' in body
    assert 'if changed {' in body and 'w.Observe(ctx, uuid.NewString(), item.ID)' in body
    observe = go_body(HERE / 'journeys.go', 'func (w *CommunityWatcher) Observe(')
    assert 'policy_digest' in observe
    assert 'w.policyDigest' in observe
    assert '"reward.community_authority", out' in observe
    assert observe.index('emit(ctx, tx,') < observe.rindex('tx.Commit(ctx)')
    proof = native_test('TestRepair1019CommunityPolicyRestart')
    assert '[][]string{{f.chain.Alice}, {}, {f.chain.Alice}}' in proof
    assert '[]string{"active", "revoked", "active"}' in proof
    assert 'communityHistory["1"]' in proof and 'len(f.db.outbox)' in proof
    reopen = go_body(HERE / 'repair_1019_test.go', 'func repair1019Reopen(')
    assert reopen.index('f.ledger.pool.Close()') < reopen.index('NewRuntimeLedger(')
    assert reopen.index('f.rpc.Close()') < reopen.index('rpc.DialHTTP(')
    assert 'NewCommittedCreator(' in reopen and 'NewCommunityWatcher(journey, approved)' in reopen


def test_reward_outbox_policy_covers_every_real_producer_and_app_worker():
    emitted = set()
    for path in HERE.glob('*.go'):
        if not path.name.endswith('_test.go') and path.name != 'outbox.go':
            source = go_source(path)
            emitted.update(re.findall(r'emit\(ctx,\s*tx,\s*[^,]+,\s*"([a-z_.]+)"\s*,', source))
            emitted.update('referral.' + operation for operation in re.findall(
                r'a\.command\(ctx,\s*who,\s*request,\s*"([a-z]+)"', source))
    policy = go_body(HERE / 'outbox.go', 'func RewardDeliveryPolicy(')
    subjects = set(re.findall(r'"([a-z_]+(?:\.[a-z_]+)+)"', policy))
    assert subjects == emitted
    assert len(subjects) == 18
    assert 'p.Subjects[subject], p.EventTypes[subject] = true, true' in policy
    assert 'range base.Subjects' in policy and 'range base.EventTypes' in policy
    composition = go_body(APP, 'func newPnLJourneys(')
    assert composition.index('l18.RewardDeliveryPolicy(policy)') < composition.index('delivery.NewStoreWithEpochGuard(')
    assert 'l18.NewOutboxBridge(ctx, ledger, outbox, 32)' in composition
    assert 'rewardOutbox: rewardOutbox' in composition
    assert 'Publisher: publisher' in composition
    jobs = go_body(APP, 'func (a *App) startJourneyJobs(')
    assert 'a.journeys.rewardOutbox.RunOnce' in jobs
    assert 'a.journeys.relay.RunOnce' in jobs


def test_reward_outbox_atomic_handoff_and_native_transport_proof_contract():
    source = HERE / 'outbox.go'
    body = go_body(source, 'func (b *OutboxBridge) RunOnce(')
    for required in ('b.ledger.begin(ctx)', 'published_at IS NULL ORDER BY sequence LIMIT $1 FOR UPDATE SKIP LOCKED',
                     'rows.Err()', 'rows.Close()', 'b.store.Enqueue(ctx, tx, event.topic, envelope)',
                     'UPDATE l18_outbox SET published_at=clock_timestamp()', 'tag.RowsAffected() != 1',
                     'b.ledger.epochGuard(ctx, tx)', 'context.WithoutCancel(ctx)', 'tx.Rollback(cleanup)'):
        assert required in body
    assert body.index('b.store.Enqueue(') < body.index('UPDATE l18_outbox') < body.index('tx.Commit(ctx)')
    assert body.count('tx.Commit(ctx)') == 1
    assert '.Publish(' not in body
    assert 'PayloadHash: delivery.Hash(event.payload), Payload: event.payload' in body
    assert 'fingerprint(struct{ Schema, Identity, Topic string }' in body
    constructor = go_body(source, 'func NewOutboxBridge(')
    assert 'batch < 1 || batch > 100' in constructor
    assert 'has_column_privilege(current_user,$1,\'published_at\',\'UPDATE\')' in constructor
    assert 'return nil, delivery.ErrAuthorization' in constructor
    grant = go_body(source, 'func OutboxRuntimeGrantSQL(')
    assert r'GRANT UPDATE (\"published_at\") ON TABLE ' in grant
    assert 'GRANT ALL' not in grant
    proof = native_test('TestRepair1019OutboxCrashRecovery')
    for stage in ('emit(ctx, tx,', 'tx.Commit(ctx)', 'bridge.RunOnce(ctx)', 'failHandoff = true',
                  'store.Claim(ctx, "before-crash"', 'pub.Publish(', 'dropPublishAck.Store(true)',
                  'dropConsumerAck.Store(true)', 'store.Consume(', 'l.pool.Close()',
                  'delivery.NewStoreWithEpochGuard(', 'relay.RunOnce(ctx)', 'sub.NextMsg(',
                  'Nats-Msg-Id', 'state=\'published\' AND attempts=2', 'recovered != payload'):
        assert stage in proof
    peer = go_source(HERE / 'repair_1019_transport_test.go')
    assert 'nats.Connect(' in peer and 'Nats-Expected-Stream: L18' in peer
    assert 'case "PUB", "HPUB":' in peer and 'savedWire' in peer
    store = go_source(HERE / 'repair_1019_store_test.go')
    assert 's.state = *s.saved' in store and 'duplicate consumer effect' in store
    assert 'l18_outbox' in native_test('TestRepair1019OutboxRuntimeRights')


def test_native_fixtures_have_no_external_runtime_dependencies():
    for name in ('repair_1019_test.go', 'repair_1019_store_test.go',
                 'repair_1019_transport_test.go', 'repair_62_support_test.go'):
        source = go_source(HERE / name)
        for forbidden in ('os.Getenv(', 'exec.Command(', 'phase2Storage(', 't.Skip(', 't.Skipf(', 't.SkipNow('):
            assert forbidden not in source, (name, forbidden)
    transcript = (HERE / 'repair_62_support_test.go').read_text()
    assert 'pgproto3.NewBackend(conn, conn)' in transcript
    assert 'pgxpool.NewWithConfig(' in transcript and 'rpc.DialHTTP(' in transcript


def test_qualification_input_requirements_execute_production_code():
    tests = load_python('repair1019_qualify_tests', HERE / 'qualify_test.py')
    suite = unittest.defaultTestLoader.loadTestsFromTestCase(tests.InputRequirements)
    result = unittest.TestResult()
    suite.run(result)
    assert result.testsRun == 3
    assert not result.skipped
    assert not result.failures and not result.errors, (result.failures, result.errors)


def test_qualification_listener_ownership_and_peer_preservation():
    qualify = load_python('repair1019_qualify_listener', HERE / 'qualify.py')
    observed = []
    with socket.socket() as server:
        server.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        server.bind(('127.0.0.1', 0))
        server.listen()
        server.settimeout(2)
        qualify.PORT = server.getsockname()[1]
        assert qualify.owns_listener(multiprocessing.current_process())
        with pytest.raises(RuntimeError, match='occupied'):
            qualify.refuse_occupied_port()

        def respond():
            connection, _ = server.accept()
            with connection:
                connection.settimeout(2)
                body = connection.recv(64)
                observed.append(body)
                connection.sendall(body)

        worker = threading.Thread(target=respond, daemon=True)
        worker.start()
        try:
            with socket.create_connection(('127.0.0.1', qualify.PORT), timeout=2) as client:
                client.sendall(b'l18-owned-peer')
                assert client.recv(64) == b'l18-owned-peer'
        finally:
            worker.join(timeout=3)
        assert not worker.is_alive()
        assert observed == [b'l18-owned-peer']
        assert int(qualify.process_start(os.getpid())) > 0
    assert not qualify.owns_listener(multiprocessing.current_process())
    qualify.refuse_occupied_port()


def test_qualification_uses_owned_runner_and_operator_database(tmp_path):
    qualify = load_python('repair1019_qualify_runner', HERE / 'qualify.py')
    tree = ast.parse((HERE / 'qualify.py').read_text())
    main = next(node for node in tree.body if isinstance(node, ast.FunctionDef) and node.name == 'main')
    update = next(node for node in ast.walk(main) if isinstance(node, ast.Expr)
                  and isinstance(node.value, ast.Call) and isinstance(node.value.func, ast.Attribute)
                  and isinstance(node.value.func.value, ast.Name) and node.value.func.value.id == 'env'
                  and node.value.func.attr == 'update')
    dsn = 'postgresql://operator@localhost:6543/disposable?sslmode=require'
    _, admitted = qualify.qualification_inputs(tmp_path, {'L18_POSTGRES_DSN': dsn})
    scope = {'env': {}, 'dsn': admitted, 'fixture': tmp_path / 'fixture.json', 'identity': 'owned-pid-start'}
    # Execute the actual production environment binding without invoking a
    # compiler, external fixture process, or substitute implementation.
    exec(compile(ast.Module(body=[update], type_ignores=[]), str(HERE / 'qualify.py'), 'exec'), scope)
    assert scope['env']['L18_POSTGRES_DSN'] == dsn
    assert scope['env']['PNL_TEST_POSTGRES_DSN'] == dsn
    assert scope['env']['L18_ANVIL_IDENTITY'] == 'owned-pid-start'
    source = (HERE / 'qualify.py').read_text()
    assert '/tmp/' not in source and 'run-bounded-check.py' not in source
    assert "cmd = ['node'," in source and "cmd = ['go', 'test'," in source
    assert 'code = run_bounded(cmd,' in source
    assert 'return child.wait(timeout=timeout)' in source
    assert 'stop(child)' in source
    assert 'wait_ready(child)' in source and source.count('assert_owned(child)') >= 3
    existing = (HERE / 'qualify_test.py').read_text()
    for test in ('occupied_port_preserves_peer', 'dead_child_never_accepts_peer_readiness',
                 'exhaustion_fails_and_cleanup_is_bounded', 'live_child_cannot_borrow_peer_listener',
                 'launcher_signal_unwinds_child_group_cleanup', 'existing_evidence_is_not_reused_or_truncated'):
        assert 'def test_' + test + '(' in existing


def test_repair_record_keeps_proofs_and_qualification_limits_explicit():
    record = (HERE / 'repair_22_33_1019.kvx').read_text()
    for finding in ('nonclaim-market-breaks-listing', 'community-policy-restart',
                    'reward-outbox-has-no-relay', 'qualification-ephemeral-runner'):
        assert finding + ' = ' in record
    assert 'Go execution is not qualified by this Python-only gate' in record
    assert 'wiring_request_grants = ' in record
    assert 'wiring_request_broker = ' in record
    assert 'baseline = ' in record
