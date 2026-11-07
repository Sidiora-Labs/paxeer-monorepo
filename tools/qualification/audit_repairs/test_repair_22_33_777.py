"""Source-contract proof plus real target producer -> fixture consumer transport.

No Prometheus, Go, Docker, Bash subprocess or live node is required by this gate.
PromQL firing is specified here and executed by test.sh's promtool gate at deploy
qualification; it is not simulated by a replacement evaluator in these tests.
"""

import json
from pathlib import Path
import re
import runpy
import shlex
import tomllib

import pytest
import yaml


ROOT = Path(__file__).resolve().parents[3]
MONITORING = ROOT / "tools/ops/monitoring"
RECORD = Path(__file__).with_name("repair_22_33_777.kvx")


@pytest.fixture
def component():
    return runpy.run_path(str(MONITORING / "readiness.py"))


@pytest.fixture
def rendered(tmp_path, component):
    # Use the committed deploy input, including its real named nodes and port.
    assignments = {}
    for line in (MONITORING / "hosts.env.example").read_text().splitlines():
        if line and not line.startswith("#"):
            key, value = line.split("=", 1)
            assignments[key] = shlex.split(value)[0] if shlex.split(value) else ""
    component["render_targets"](
        assignments["HOSTS"], assignments["COMET_RPC_PORT"], tmp_path / "targets"
    )
    (tmp_path / "prometheus.yml").write_bytes((MONITORING / "prometheus.yml").read_bytes())
    return tmp_path


def load(name):
    return yaml.safe_load((MONITORING / name).read_text())


def all_rules():
    return {rule["alert"]: rule for group in load("rules.yml")["groups"] for rule in group["rules"]}


def test_monitoring_retired_jobs(component, rendered):
    config = load("prometheus.yml")
    jobs = {job["job_name"]: job for job in config["scrape_configs"]}
    assert {"cometbft", "node-readyz", "node-health"} <= jobs.keys()
    assert not any("sequencer" in job or "kernel" in job for job in jobs)
    labels = component["deployed_labels"](rendered)
    for job, endpoint, module in [
        ("node-readyz", "status", "node_consensus"),
        ("node-health", "health", "http_2xx"),
    ]:
        assert jobs[job]["metrics_path"] == "/probe"
        assert jobs[job]["params"] == {"module": [module]}
        assert labels[job] == [
            {"job": job, "instance": f"http://{host}.example.internal:26657/{endpoint}", "host": host}
            for host in ("validator-a", "full-1")
        ]
    # Container mounts transport the rendered discovery files and actual rules.
    services = load("docker-compose.yml")["services"]
    assert "./rendered/targets:/etc/prometheus/targets:ro" in services["prometheus"]["volumes"]
    assert "./rules.yml:/etc/prometheus/rules.yml:ro" in services["prometheus"]["volumes"]
    assert "./blackbox.yml:/etc/blackbox/blackbox.yml:ro" in services["blackbox"]["volumes"]
    assert config["rule_files"] == ["/etc/prometheus/rules.yml"]


def test_monitoring_retired_render(component, rendered, tmp_path):
    script = (MONITORING / "render.sh").read_text()
    tokens = shlex.split(script, comments=True)
    assert "set -euo pipefail" in script
    assert ['python3', '$here/readiness.py', 'targets', '$HOSTS', '$COMET_RPC_PORT', '$out/targets'] == tokens[
        tokens.index("python3"):tokens.index("python3") + 6
    ]
    assert 'cp "$here/prometheus.yml" "$out/prometheus.yml"' in script
    assert '[ -n "$HOSTS" ] || {' in script
    assert 'COMET_RPC_PORT=26657' in script
    for file in ("render.sh", "hosts.env.example", "prometheus.yml", "rules.yml", "rules_test.yml"):
        text = (MONITORING / file).read_text()
        assert "sequencer" not in text.lower()
        assert "kernel-readyz" not in text
    assert sorted(path.name for path in (rendered / "targets").iterdir()) == ["node-health.json", "node-readyz.json"]
    component["render_targets"]("replacement=node-new.internal", "27657", tmp_path / "replacement")
    group = json.loads((tmp_path / "replacement/node-readyz.json").read_text())
    assert group == [{"targets": ["http://node-new.internal:27657/status"], "labels": {"host": "replacement"}}]
    installer = (MONITORING / "install.sh").read_text()
    assert 'apt-get install -y python3' in installer
    assert '"$here/readiness.py" "$dest/"' in installer
    assert installer.index('"$here/readiness.py" "$dest/"') < installer.index('"$dest/render.sh" "$env_file"')


@pytest.mark.parametrize("hosts,port", [
    ("", "26657"), ("missing-equals", "26657"), ("v=", "26657"),
    ("v=node/path", "26657"), ("v=node v=other", "26657"),
    ("v=node", "0"), ("v=node", "65536"), ("v=node", "bad"),
])
def test_target_producer_refuses_invalid_deployment(component, tmp_path, hosts, port):
    with pytest.raises(ValueError):
        component["render_targets"](hosts, port, tmp_path / "targets")
    assert not (tmp_path / "targets").exists()


def test_monitoring_unwired_kernel(component, rendered):
    rule = all_rules()["NodeNotReady"]
    assert rule["expr"] == 'probe_success{job="node-readyz"} == 0'
    assert rule["for"] == "1m"
    assert rule["labels"] == {"severity": "critical"}
    assert component["deployed_labels"](rendered)["node-readyz"]
    # Verify actual node status producer fields/routes against the probe contract.
    routes = (ROOT / "consensus/internal/rpc/core/routes.go").read_text()
    status = (ROOT / "consensus/internal/rpc/core/status.go").read_text()
    types = (ROOT / "consensus/rpc/coretypes/responses.go").read_text()
    assert re.search(r'"status":\s+NewRPCFunc\(svc.Status\)', routes)
    assert re.search(r'"health":\s+NewRPCFunc\(svc.Health\)', routes)
    assert 'catchingUp = latestHeight == 0 || latestHeight != lastCommittedBlockHeight' in status
    assert 'json:"catching_up"' in types
    assert 'json:"latest_block_height,string"' in types
    probe = load("blackbox.yml")["modules"]["node_consensus"]
    assert probe["prober"] == "http"
    assert probe["http"]["valid_status_codes"] == [200]
    patterns = probe["http"]["fail_if_body_not_matches_regexp"]
    assert patterns == [r'"catching_up"\s*:\s*false', r'"latest_block_height"\s*:\s*"[1-9][0-9]*"']
    # Check the configured body predicates, without substituting a fake node.
    for body, expected in [
        ('{"result":{"sync_info":{"catching_up":false,"latest_block_height":"125"}}}', True),
        ('{"result":{"sync_info":{"catching_up":true,"latest_block_height":"125"}}}', False),
        ('{"result":{"sync_info":{"catching_up":false,"latest_block_height":"0"}}}', False),
        ('{"error":{"code":-32603}}', False),
    ]:
        assert all(re.search(pattern, body) for pattern in patterns) == expected


def test_monitoring_retired_alerts():
    rules = all_rules()
    assert not {"KernelReadyzDown", "SequencerNotReady", "StandbyNotReady"} & rules.keys()
    assert rules["NodeRPCDown"]["expr"] == 'probe_success{job="node-health"} == 0'
    assert rules["NodeReadinessProbeDown"]["expr"] == 'up{job=~"node-readyz|node-health"} == 0'
    for name in ("NodeRPCDown", "NodeReadinessProbeDown"):
        assert rules[name]["for"] == "2m"
        assert rules[name]["labels"]["severity"] == "critical"
    assert rules["ChainHeightStalled"]["expr"] == 'changes({__name__=~"(cometbft|tendermint)_consensus_height", job="cometbft"}[60s]) == 0'
    assert rules["CometMetricsDown"]["labels"]["severity"] == "critical"


def test_monitoring_fixture_shape(component, rendered, tmp_path):
    output = tmp_path / "bound_rules_test.yml"
    component["bind_rule_tests"](rendered, MONITORING / "rules.yml", MONITORING / "rules_test.yml", output)
    fixtures = yaml.safe_load(output.read_text())
    labels = component["deployed_labels"](rendered)
    assert fixtures["rule_files"] == ["rules.yml"]
    assert "__target__" not in output.read_text()
    assert "@instance@" not in output.read_text()
    checked = set()
    for case in fixtures["tests"]:
        for check in case["alert_rule_test"]:
            name = check["alertname"]
            if name not in {"NodeNotReady", "NodeRPCDown", "NodeReadinessProbeDown"}:
                continue
            for alert in check["exp_alerts"]:
                expected = dict(alert["exp_labels"])
                assert expected.pop("severity") == "critical"
                assert expected in labels[expected["job"]]
                metric = "up" if name == "NodeReadinessProbeDown" else "probe_success"
                encoded = metric + "{" + ",".join(f"{k}={json.dumps(v)}" for k, v in expected.items()) + "}"
                # Dict order differs after adding severity; compare parsed labels.
                assert any(parse_series(s["series"]) == parse_series(encoded) for s in case["input_series"])
                assert expected["instance"] in alert["exp_annotations"]["summary"]
                checked.add((name, expected["job"]))
    assert checked == {
        ("NodeNotReady", "node-readyz"), ("NodeRPCDown", "node-health"),
        ("NodeReadinessProbeDown", "node-readyz"), ("NodeReadinessProbeDown", "node-health"),
    }
    # Preserve pending, firing and recovery specifications for each readiness rule.
    for name, before, firing, recovered, values in [
        ("NodeNotReady", "45s", "1m", "1m15s", "0x4 1x7"),
        ("NodeRPCDown", "1m45s", "2m", "2m15s", "0x8 1x3"),
        ("NodeReadinessProbeDown", "1m45s", "2m", "2m15s", "0x8 1x3"),
    ]:
        cases = [case for case in fixtures["tests"] if any(
            check["alertname"] == name and check["eval_time"] == firing and check["exp_alerts"]
            for check in case["alert_rule_test"]
        ) and any(s["values"] == values for s in case["input_series"])]
        assert cases
        for case in cases:
            checks = {check["eval_time"]: check["exp_alerts"] for check in case["alert_rule_test"] if check["alertname"] == name}
            assert checks[before] == []
            assert len(checks[firing]) == 1
            assert checks[recovered] == []
    script = (MONITORING / "test.sh").read_text()
    shlex.split(script, comments=True)
    assert 'set -euo pipefail' in script
    assert script.index('"$here/render.sh"') < script.index('python3 "$here/readiness.py" fixtures')
    assert script.index('python3 "$here/readiness.py" fixtures') < script.index('prom test rules rules_test.yml')
    assert '"$here/rules_test.yml" "$work/rules_test.yml"' in script
    assert 'cp "$here/rules.yml" "$here/rules_test.yml"' not in script


def parse_series(series):
    metric, body = series.rstrip("}").split("{", 1)
    return metric, dict(re.findall(r'(\w+)="([^"]+)"', body))


@pytest.mark.parametrize("damage", ["missing-job", "empty-targets", "wrong-file", "wrong-label", "wrong-module"])
def test_fixture_consumer_rejects_unwired_producer(component, rendered, tmp_path, damage):
    path = rendered / "prometheus.yml"
    config = yaml.safe_load(path.read_text())
    job = next(job for job in config["scrape_configs"] if job["job_name"] == "node-readyz")
    if damage == "missing-job":
        config["scrape_configs"].remove(job)
    elif damage == "empty-targets":
        (rendered / "targets/node-readyz.json").write_text("[]")
    elif damage == "wrong-file":
        job["file_sd_configs"][0]["files"] = ["/etc/prometheus/targets/retired.json"]
    elif damage == "wrong-label":
        job["relabel_configs"][1]["target_label"] = "retired_instance"
    else:
        job["params"]["module"] = ["http_2xx"]
    path.write_text(yaml.safe_dump(config))
    output = tmp_path / "must-not-exist.yml"
    with pytest.raises(ValueError):
        component["bind_rule_tests"](rendered, MONITORING / "rules.yml", MONITORING / "rules_test.yml", output)
    assert not output.exists()


def test_fixture_consumer_rejects_invented_rule_job(component, rendered, tmp_path):
    rules = load("rules.yml")
    rule = next(r for g in rules["groups"] for r in g["rules"] if r["alert"] == "NodeNotReady")
    rule["expr"] = 'probe_success{job="kernel-readyz"} == 0'
    path = tmp_path / "unwired-rules.yml"
    path.write_text(yaml.safe_dump(rules))
    with pytest.raises(ValueError, match="no deployed producer"):
        component["bind_rule_tests"](rendered, path, MONITORING / "rules_test.yml", tmp_path / "out.yml")


def test_repair_record_names_each_finding_and_proof():
    record = tomllib.loads(RECORD.read_text())
    findings = record["finding"]
    for finding, test in {
        "monitoring-retired-jobs": "test_monitoring_retired_jobs",
        "monitoring-retired-render": "test_monitoring_retired_render",
        "monitoring-unwired-kernel": "test_monitoring_unwired_kernel",
        "monitoring-retired-alerts": "test_monitoring_retired_alerts",
        "monitoring-fixture-shape": "test_monitoring_fixture_shape",
    }.items():
        assert findings[finding]["test"] == test
    task = record["task"]["22"]["33"]["777"]
    assert task["base_revision"] == "04c1e9407e75f7ac92c260452f2232ab07bf3ef1"
    assert task["baseline"]
    assert task["review_only"]
    assert task["wiring_requests"]
    assert "hf-done" in task["evidence"]
