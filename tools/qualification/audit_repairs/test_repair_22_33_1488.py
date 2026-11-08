"""Offline source qualification for audit 22.30.88's eight medium repairs.

Read the production producers and consumers, parse their configuration and
journey Python, and compare the boundary contracts. bash -n is the declared
hf-build gate, not an additional subprocess here. Native execution, Kubernetes
availability and actual signer/receipt ceremonies require owner qualification;
this module never substitutes a fake node or CLI for those components.
"""

import ast
import json
from pathlib import Path
import re
import shlex
import tomllib
from urllib.parse import urlsplit

import yaml


ROOT = Path(__file__).resolve().parents[3]
RAMPS = ROOT / "services/platform/ramps"
RECORD = Path(__file__).with_name("repair_22_33_1488.kvx")


def source(name):
    return (RAMPS / name).read_text(encoding="utf-8")


def hosted(name):
    return (ROOT / "services/platform/hosted" / name).read_text(encoding="utf-8")


def documents(text):
    result = list(yaml.safe_load_all(text))
    assert result and all(isinstance(item, dict) for item in result)
    return result


def resource(items, kind, name):
    matches = [item for item in items if item["kind"] == kind
               and item["metadata"]["name"] == name]
    assert len(matches) == 1, (kind, name)
    return matches[0]


def json_examples(text):
    return [json.loads(block) for block in re.findall(r"```json\n(.*?)\n```", text, re.S)]


def rust_struct(text, name):
    match = re.search(r"(?:pub )?struct " + name + r"\s*\{(.*?)\n\}", text, re.S)
    assert match, name
    return match[1]


def journey_python():
    shell = source("sandbox-journey.sh")
    blocks = re.findall(r"<<'PY'\n(.*?)\nPY\n", shell, re.S)
    assert len(blocks) == 1
    return blocks[0], ast.parse(blocks[0])


def function(name):
    text, tree = journey_python()
    matches = [node for node in tree.body if isinstance(node, ast.FunctionDef)
               and node.name == name]
    assert len(matches) == 1, name
    return matches[0], ast.get_source_segment(text, matches[0])


def assert_facts(text, *facts):
    for fact in facts:
        assert fact in text, fact


def test_runbook_signer_matches_production_kms_consumer_and_rotation():
    operations = source("OPERATIONS.md")
    clients = source("toolkit/src/clients.rs")
    signer = clients[clients.index('path: "/v1/signatures"'):]
    decoder = signer[:signer.index("pub struct PaxeerSubmission")]
    assert_facts(operations, "non-exporting KMS/HSM", "EIP-1559 signing hash",
                 "standard padded base64", "64-byte `(r,s)`", "`recovery_id` as 0 or 1",
                 "independently authenticated KMS public-key ceremony",
                 "receipt-key rotation", "retain old pins for historical recovery",
                 "authorised height interval")
    assert_facts(decoder, 'algorithm: "secp256k1"', 'message: base64_encode(&call.signing_hash())',
                 'signature: String', 'recovery_id: u8', '#[serde(deny_unknown_fields)]',
                 'let compact: [u8; 64]', 'if recovery_id > 1',
                 'bytes[64] = 27 + recovery_id', 'recover_sender(call, &signature)',
                 '!= operator')
    assert_facts(clients, 'if self.first_height > self.last_height',
                 'key_id == [0; 32]', 'key.is_weak()')


def test_runbook_work_sequence_examples_match_strict_handler_and_customer_authority():
    operations = source("OPERATIONS.md")
    examples = json_examples(operations)
    assert len(examples) == 2
    for example in examples:
        assert set(example) <= {"order_digest", "action", "account_sequence"}
        assert len(example["order_digest"]) == 32
        assert all(type(byte) is int and 0 <= byte <= 255 for byte in example["order_digest"])
        assert example.get("account_sequence") is None
    assert examples[0]["action"] == "submit_layerx"
    assert examples[1]["action"] == "resolve_layerx"
    handler = source("reference/src/main.rs")
    work = handler[handler.index('fn perform_work('):handler.index('fn rebalance_presentation(')]
    assert_facts(work, '(RampDirection::OnRamp, None) => 0',
                 '(RampDirection::OffRamp, Some(sequence)) => sequence',
                 '_ => return Err(error(400, "work_invalid"))',
                 'engine.submit_layerx(work.order_digest, sequence, now())',
                 'work.account_sequence.is_some() && !matches!(work.action, WorkAction::SubmitLayerx)')
    assert_facts(source("toolkit/src/clients.rs"), '.pending_nonce(&self.operator_address)',
                 'if order.direction() == RampDirection::OffRamp',
                 'return Err(RampError::PayerGrantRequired)')
    assert_facts(operations, 'omit `account_sequence` or send JSON `null`',
                 "pending EVM nonce", 'RampEngine::submit_customer_payment',
                 'currently no reference HTTP request', 'keep off-ramp\nenablement closed')
    assert_facts(source("toolkit/src/engine.rs"), 'pub fn submit_customer_payment(')


def test_runbook_finality_matches_transaction_recovery_and_bft_refusals():
    operations = source("OPERATIONS.md")
    assert_facts(operations, '`pax_sequencerReceipt` using the stored transaction hash',
                 'Retain the exact signed\nbytes', 'only a `committed` leg',
                 'Block commit is network finality', 'operation/transaction\nmismatch',
                 'changed committed block identity', 'Keep inventory unavailable',
                 'do not credit from stale inclusion', 'external-custody disclosure')
    engine = source("toolkit/src/engine.rs")
    assert_facts(engine, 'self.layerx.resolve_transaction(', 'if leg.rung != Rung::Committed')
    clients = source("toolkit/src/clients.rs")
    assert_facts(clients, '.sequencer_receipt(&tx_hash)',
                 'self.authenticate_payment_transaction(order, tx_hash, raw)?',
                 'let committed = matches!(answer, SequencerReceipt::Committed { .. })')
    journal = source("toolkit/src/journal.rs")
    assert_facts(journal, '.is_some_and(|value| value != observation.transaction_hash)',
                 '.is_some_and(|height| Some(height) != observation.block_height)',
                 'committed != observation.block_height.is_some()')
    for obsolete in ('required_confirmations', 'human reorg', 'confirmation countdown'):
        assert obsolete not in operations


def test_readme_missing_purpose_examples_and_conflict_bind_real_order_schema():
    readme = source("README.md")
    on, off = json_examples(readme)
    assert set(on) == {"order_id", "quote_id"}
    assert set(off) == {"order_id", "quote_id", "customer_budget_id", "customer_budget_purpose"}
    for field in ("customer_budget_id", "customer_budget_purpose"):
        assert len(off[field]) == 32 and any(off[field])
        assert all(type(byte) is int and 0 <= byte <= 255 for byte in off[field])
    lib = source("toolkit/src/lib.rs")
    create = rust_struct(lib, "CreateOrder")
    assert_facts(create, '#[serde(rename = "customer_budget_id", alias = "payer_grant")]',
                 '#[serde(rename = "customer_budget_purpose", alias = "payer_grant_purpose")]')
    assert_facts(lib, 'request.payer_grant_purpose == Some([0; 32])',
                 'request.payer_grant.is_none() || request.payer_grant_purpose.is_none()',
                 'self.payer_grant_purpose')
    handler = source("reference/src/main.rs")
    creation = handler[handler.index('fn create_order('):handler.index('fn source_settlement(')]
    assert_facts(creation, 'existing.order.customer != principal',
                 'existing.order.quote.quote_id != create.quote_id',
                 'existing.order.payer_grant != create.payer_grant',
                 'existing.order.payer_grant_purpose != create.payer_grant_purpose',
                 'error(409, "order_id_conflict")', 'created(existing.presentation())')
    assert_facts(readme, 'HTTP 409 `order_id_conflict`', 'duplicate-field error',
                 'order digest', 'budget ID/purpose and application context')


def test_readme_payment_path_matches_native_compiler_transport_and_receipt_verifier():
    readme = source("README.md")
    assert_facts(readme, 'chain ID is **125**', '`eth_sendRawTransaction`', '`pax_sequencerReceipt`',
                 '`budgetSpend(bytes32,bytes32,uint128)`', '`submit_customer_payment`',
                 'independently pinned Ed25519 receipt', 'height ranges', '`PayerGrantRequired`',
                 'base64 compact 64-byte `signature` and `recovery_id`',
                 'committed verified\npayment evidence')
    lib = source("toolkit/src/lib.rs")
    assert_facts(lib, 'pub fn compile_customer_payment(', 'customer_payment_authorization()',
                 'purpose_hash:', 'pub fn verify_order_receipt(')
    clients = source("toolkit/src/clients.rs")
    assert_facts(clients, '.send_raw_transaction(&raw_transaction)',
                 'self.verify_customer_budget(order, height)?',
                 'verify_order_receipt(order, &evidence)?', 'if leg.height != receipt.height')
    assert 'layerx-ramp-toolkit' == tomllib.loads(source("toolkit/Cargo.toml"))["package"]["name"]
    assert 'layerx-reference-ramp' == tomllib.loads(source("reference/Cargo.toml"))["package"]["name"]


def test_example_retired_endpoint_aligns_pinned_merged_tls_relay_and_secret_mount():
    config = json.loads(source("config.example.json"))
    layerx = config["layerx"]
    assert layerx["rpc_endpoint"] == 'https://layerx-gateway.layerx-testnet.svc.cluster.local/'
    assert layerx["chain_id"] == 125
    assert layerx["rpc_trust_anchor_der"] == '/run/secrets/paxeer-rpc-ca.der'
    fields = re.findall(r'^    pub (\w+):', rust_struct(source("toolkit/src/clients.rs"), 'LayerxConfig'), re.M)
    assert set(layerx) == set(fields)
    docs = documents(source("deployment.yaml"))
    pod = resource(docs, 'StatefulSet', 'layerx-reference-ramp')["spec"]["template"]["spec"]
    volumes = {volume['name']: volume for volume in pod['volumes']}
    secret_sources = volumes['secrets-source']['projected']['sources']
    assert {'secret': {'name': 'layerx-reference-ramp-layerx'}} in secret_sources
    mount = next(item for item in pod['containers'][0]['volumeMounts']
                 if item['name'] == 'runtime-secrets')
    assert mount['mountPath'] == '/run/secrets' and mount['readOnly'] is True
    materialize = pod['initContainers'][0]['command'][-1]
    assert_facts(materialize, 'umask 077', 'cp /source-secrets/* /runtime-secrets/',
                 'chmod 600 /runtime-config/config.json /runtime-secrets/*')
    env = source('railway.env.example')
    assert_facts(env, 'Compatibility variable name', 'merged node root TLS endpoint',
                 'Service 443 -> pod 9443', 'RAMP_RPC_TRUST_ANCHOR_DER=',
                 '/run/secrets/paxeer-rpc-ca.der')
    init = source('env-init.sh')
    assert_facts(init, 'rpc_endpoint: env.RAMP_SEQUENCER_RPC_ENDPOINT',
                 'rpc_trust_anchor_der: ($s + "/paxeer-rpc-ca.der")')


def peers_allow(rule, namespace, app, port, direction):
    if not any(item.get('protocol', 'TCP') == 'TCP' and item['port'] == port
               for item in rule['ports']):
        return False
    for peer in rule[direction]:
        if peer.get('namespaceSelector', {}).get('matchLabels') != {
                'kubernetes.io/metadata.name': namespace}:
            continue
        if peer.get('podSelector', {}).get('matchLabels') == {'app': app}:
            return True
    return False


def test_deployment_egress_disagreement_private_services_select_actual_ports_in_both_directions():
    config = json.loads(source('config.example.json'))
    docs = documents(source('deployment.yaml'))
    policy = resource(docs, 'NetworkPolicy', 'layerx-reference-ramp')['spec']
    assert policy['policyTypes'] == ['Ingress', 'Egress']
    services = [*docs, *documents(hosted('identity/deployment.yaml')),
                *documents(hosted('gateway/deployment.yaml'))]
    ingress = resource(docs, 'NetworkPolicy', 'layerx-reference-ramp-upstream')['spec']
    selected = ingress['podSelector']['matchExpressions'][0]
    assert selected['key'] == 'app' and selected['operator'] == 'In'
    apps = set()
    for endpoint in (config['identity']['endpoint'], config['compliance']['endpoint'],
                     config['provider']['endpoint'], config['layerx']['signer_endpoint'],
                     config['paxeer']['custody_endpoint'], config['layerx']['rpc_endpoint']):
        url = urlsplit(endpoint)
        assert url.scheme == 'https' and url.path in ('', '/')
        name, namespace, suffix = url.hostname.split('.', 2)
        assert namespace == 'layerx-testnet' and suffix == 'svc.cluster.local'
        service = resource(services, 'Service', name)
        live_namespace = service['metadata']['namespace']
        assert live_namespace == ('layerx-mainnet-beta' if name == 'layerx-gateway' else namespace)
        # The retired testnet ramp still pins its original testnet policies.
        # Those policies must not grant access to the current mainnet gateway.
        if name == 'layerx-gateway':
            assert not any(peers_allow(rule, live_namespace, name, 9443, 'to')
                           for rule in policy['egress'])
        app = service['spec']['selector']['app']
        apps.add(app)
        port = next(item for item in service['spec']['ports'] if item['port'] == (url.port or 443))
        target = port['targetPort']
        if isinstance(target, str):
            kind = 'StatefulSet' if app == 'layerx-node' else 'Deployment'
            containers = resource(services, kind, app)['spec']['template']['spec']['containers']
            target = next(item['containerPort'] for container in containers for item in container['ports']
                          if item['name'] == target)
        assert target == 9443
        assert any(peers_allow(rule, namespace, app, target, 'to') for rule in policy['egress'])
        assert app in selected['values']
        assert any(peers_allow(rule, namespace, 'layerx-reference-ramp', target, 'from')
                   for rule in ingress['ingress'])
    assert apps == {'layerx-identity', 'compliance-boundary', 'provider-egress',
                    'kms-boundary', 'paxeer-custody', 'layerx-gateway'}
    assert set(selected['values']) == apps
    for rule in policy['egress']:
        for peer in rule['to']:
            assert set(peer) == {'namespaceSelector', 'podSelector'}
            assert peer['namespaceSelector']['matchLabels']
            assert peer['podSelector']['matchLabels']
    assert not any(peers_allow(rule, 'layerx-platform', 'layerx-gateway', 9443, 'to')
                   for rule in policy['egress'])
    assert not any(peers_allow(rule, 'layerx-testnet', 'unrelated', 9443, 'to')
                   for rule in policy['egress'])
    assert not any(peers_allow(rule, 'layerx-testnet', 'provider-egress', 443, 'to')
                   for rule in policy['egress'])
    dns = policy['egress'][0]
    assert dns['to'] == [{'namespaceSelector': {'matchLabels': {'kubernetes.io/metadata.name': 'kube-system'}},
                          'podSelector': {'matchLabels': {'k8s-app': 'kube-dns'}}}]
    assert {(port['protocol'], port['port']) for port in dns['ports']} == {('UDP', 53), ('TCP', 53)}


def test_merged_relay_producer_route_and_native_service_are_documented_without_public_provider_egress():
    assert_facts(hosted('gateway/src/main.rs'),
                 'public_reads::route(config, request)'
    )
    assert_facts(hosted('gateway/src/public_reads.rs'),
                 'matches!(request.path.as_str(), "/" | "/openrpc.json")',
                 'return Some(super::rpc::route(config, request))')
    assert_facts(hosted('gateway/src/evm.rs'), 'pax_sequencerReceipt')
    docs = documents(hosted('node/deployment.yaml'))
    native = resource(docs, 'Service', 'layerx-native-chain')
    assert native['spec']['selector'] == {'app': 'layerx-node'}
    rpc = next(port for port in native['spec']['ports'] if port['name'] == 'rpc')
    assert rpc == {'name': 'rpc', 'port': 8545, 'targetPort': 'rpc'}
    operations = source('OPERATIONS.md')
    assert_facts(operations, 'paxd` Service `layerx-native-chain` on 8545',
                 'There is no public\nprovider egress', 'four owner boundary Services',
                 'before claiming a reachable runtime deployment')


def test_journey_retired_verifier_current_config_and_cli_producer_consumer_contract():
    shell = source('sandbox-journey.sh')
    python, tree = journey_python()
    assert tree.body
    for retired in ('--activity-id', 'authorized-batches/by-activity', 'AuthorizedReceiptBatch',
                    'verify_receipt_outcome', 'LAYERX_RAMP_AUTHORITY_URL',
                    'LAYERX_RAMP_AUTHORITY_TOKEN_FILE', 'sequencer_public_key', 'protocol_version'):
        assert retired not in shell
    cli = source('toolkit/src/bin/ramp-verify-receipt.rs')
    assert_facts(cli, 'args.len() != 8', 'args[0] != "--config-file"',
                 'args[2] != "--order-file"', 'args[4] != "--transaction-hash"',
                 'args[6] != "--output-file"', 'value.starts_with("0x") && value.len() == 66',
                 'resolve_with_evidence(&order, transaction_hash)',
                 'verify_order_receipt(&order, &evidence)', 'if checked != leg',
                 'if !maintained_batch',
                 'checked.rung != layerx_types::rung::Rung::Receipted',
                 'leg.rung != layerx_types::rung::Rung::Committed',
                 'checked.rung = layerx_types::rung::Rung::Committed',
                 '.create_new(true)', '.mode(0o600)')
    _, config = function('verifier_layerx_config')
    assert_facts(config, 'read_protected("LAYERX_RAMP_RECEIPT_VERIFIER_CONFIG")',
                 'set(config) == {"client_tls", "layerx"}', 'layerx["chain_id"] == 125')
    layerx_sets = [node for node in ast.walk(ast.parse(config)) if isinstance(node, ast.Set)
                   and len(node.elts) > 4]
    assert len(layerx_sets) == 1
    rust_fields = set(re.findall(r'^    pub (\w+):',
                      rust_struct(source('toolkit/src/clients.rs'), 'LayerxConfig'), re.M))
    assert {ast.literal_eval(item) for item in layerx_sets[0].elts} == rust_fields
    node, verify = function('verify_done')
    calls = [item for item in ast.walk(node) if isinstance(item, ast.Call)
             and ast.unparse(item.func) == 'subprocess.run']
    assert len(calls) == 1
    argv = calls[0].args[0]
    assert isinstance(argv, ast.List) and len(argv.elts) == 9
    assert [ast.literal_eval(argv.elts[i]) for i in (1, 3, 5, 7)] == [
        '--config-file', '--order-file', '--transaction-hash', '--output-file']
    assert ast.unparse(argv.elts[6]) == 'transaction'
    assert ast.unparse(calls[0].keywords[-1].value) == 'False'
    assert_facts(verify, 'completed.returncode == 0', 'os.O_NOFOLLOW | os.O_NONBLOCK',
                 'stat.S_IMODE(info.st_mode) == 0o600', 'info.st_nlink == 1',
                 'stream.read(16385)', 'len(encoded) <= 16384',
                 'return validate_cli_receipt(snapshot, json.loads(encoded), layerx)')
    assert_facts(python, 'report["on"] = verify_done(on_done)', 'report["off"] = verify_done(off_done)')


def test_journey_cli_result_checks_exact_identity_height_and_committed_rung():
    cli = source('toolkit/src/bin/ramp-verify-receipt.rs')
    result = cli.split('let result = json!({', 1)[1].split('\n    });', 1)[0]
    producer_keys = set(re.findall(r'"(\w+)":', result))
    node, consumer = function('validate_cli_receipt')
    consumer_keys = {item.slice.value for item in ast.walk(node)
                     if isinstance(item, ast.Subscript) and isinstance(item.value, ast.Name)
                     and item.value.id == 'verified' and isinstance(item.slice, ast.Constant)}
    assert consumer_keys <= producer_keys
    assert {'transaction_hash', 'receipt_digest', 'height', 'rung', 'order_digest', 'chain_id'} <= consumer_keys
    assert_facts(result, '"transaction_hash": format!("0x{}", hex(&leg.tx_hash))',
                 '"receipt_digest": hex(&leg.receipt_digest)', '"height": leg.height',
                 '"rung": leg.rung.name()')
    assert_facts(consumer, 'verified["transaction_hash"] == transaction',
                 'view["transaction_hash"] == transaction',
                 'verified["receipt_digest"] == octets(view["receipt_digest"]).hex()',
                 'verified["rung"] == snapshot["evidence"]["rung"] == "committed"',
                 'integer(verified["height"], 1)',
                 'pin["firstHeight"] <= height <= pin["lastHeight"]',
                 'verified["build_revision"] == expected_revision',
                 'verified["build_source_digest"] == expected_digest')
    _, presentation = function('presentation')
    assert 'evidence["rung"] == "committed"' in presentation
    shell = source('sandbox-journey.sh').split('\nPY\nfi\n', 1)[1]
    command = re.search(r'^  "\$LAYERX_RAMP_RECEIPT_VERIFIER_BINARY" (.*?)(?=\n  printf)', shell, re.M | re.S)
    assert command
    argv = shlex.split(command.group(0).replace('\\\n', ' '))
    assert argv == ['$LAYERX_RAMP_RECEIPT_VERIFIER_BINARY', '--config-file',
                    '$LAYERX_RAMP_RECEIPT_VERIFIER_CONFIG', '--order-file', '$order_file',
                    '--transaction-hash', '$tx_hash', '--output-file', '$result_file']
    assert_facts(shell, 'order-${tx_hash}.json', 'receipt-${tx_hash}.json',
                 '$proof[0].transaction_hash == $tx_hash',
                 '$proof[0].receipt_digest == $receipt_digest',
                 '$proof[0].height | type == "number" and . == floor and . > 0',
                 '$proof[0].rung == "committed"', '$proof[0].chain_id == 125')
    assert 'receipt_digest="0x' not in shell


def test_repair_record_maps_all_findings_and_states_proof_limits_and_wiring():
    record = tomllib.loads(RECORD.read_text(encoding='utf-8'))
    assert record['repair']['task'] == '22.33.1488'
    assert 'No pre-change verification was run' in record['repair']['baseline']
    assert 'No baseline failures are claimed' in record['repair']['baseline']
    assert record['repair']['base'] == '04c1e9407e75f7ac92c260452f2232ab07bf3ef1'
    tests = {node.name for node in ast.parse(Path(__file__).read_text()).body
             if isinstance(node, ast.FunctionDef) and node.name.startswith('test_')}
    assert set(record['finding']) == {'runbook-signer', 'runbook-work-sequence', 'runbook-finality',
                                     'readme-missing-purpose', 'readme-payment-path',
                                     'example-retired-endpoint', 'deployment-egress-disagreement',
                                     'journey-retired-verifier'}
    for finding in record['finding'].values():
        assert finding['test'] in tests and finding['change']
    assert record['proof']['source'] and record['proof']['review_only']
    assert 'native node-to-CLI' in record['proof']['review_only']
    for path in ('services/platform/hosted/gateway/deployment.yaml',
                 'docker/ramps/files.tsv', 'tools/bringup'):
        assert path in str(record['wiring'])
