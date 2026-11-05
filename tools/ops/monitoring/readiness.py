"""Render paxd probe targets and bind rule fixtures to deployed scrape labels."""

import argparse
import json
from pathlib import Path
import re


READINESS_JOBS = {"node-readyz": "status", "node-health": "health"}


def render_targets(hosts, port, destination):
    if not str(port).isdigit() or not 1 <= int(port) <= 65535:
        raise ValueError("COMET_RPC_PORT must be a TCP port")
    nodes = []
    for pair in hosts.split():
        name, separator, address = pair.partition("=")
        if not separator or not re.fullmatch(r"[A-Za-z0-9_.-]+", name):
            raise ValueError("HOSTS must contain name=address pairs")
        if not re.fullmatch(r"[A-Za-z0-9_.-]+|\[[0-9A-Fa-f:]+\]", address):
            raise ValueError("HOSTS address must be a hostname, IPv4 or bracketed IPv6")
        if any(node[0] == name for node in nodes):
            raise ValueError("HOSTS names must be unique")
        nodes.append((name, address))
    if not nodes:
        raise ValueError("HOSTS must include a merged node")
    destination = Path(destination)
    destination.mkdir(parents=True, exist_ok=True)
    for job, endpoint in READINESS_JOBS.items():
        groups = [
            {"targets": [f"http://{address}:{port}/{endpoint}"], "labels": {"host": name}}
            for name, address in nodes
        ]
        (destination / f"{job}.json").write_text(json.dumps(groups) + "\n")


def deployed_labels(rendered):
    """Validate the blackbox transport before deriving representative fixture labels."""
    import yaml

    rendered = Path(rendered)
    config = yaml.safe_load((rendered / "prometheus.yml").read_text())
    jobs = config["scrape_configs"]
    labels = {}
    for job_name in READINESS_JOBS:
        matches = [job for job in jobs if job["job_name"] == job_name]
        if len(matches) != 1:
            raise ValueError(f"missing or duplicate readiness producer: {job_name}")
        job = matches[0]
        module = "node_consensus" if job_name == "node-readyz" else "http_2xx"
        if job.get("metrics_path") != "/probe" or job.get("params") != {"module": [module]}:
            raise ValueError(f"invalid readiness probe module: {job_name}")
        if job.get("relabel_configs") != [
            {"source_labels": ["__address__"], "target_label": "__param_target"},
            {"source_labels": ["__param_target"], "target_label": "instance"},
            {"target_label": "__address__", "replacement": "blackbox:9115"},
        ]:
            raise ValueError(f"invalid readiness label transport: {job_name}")
        files = [file for discovery in job["file_sd_configs"] for file in discovery["files"]]
        expected = f"/etc/prometheus/targets/{job_name}.json"
        if files != [expected]:
            raise ValueError(f"invalid readiness target file: {job_name}")
        groups = json.loads((rendered / "targets" / Path(expected).name).read_text())
        targets = []
        for group in groups:
            if set(group["labels"]) != {"host"} or not group["labels"]["host"]:
                raise ValueError(f"invalid readiness host labels: {job_name}")
            for target in group["targets"]:
                if not target.endswith("/" + READINESS_JOBS[job_name]):
                    raise ValueError(f"invalid readiness endpoint: {job_name}")
                targets.append({"job": job_name, "instance": target, **group["labels"]})
        if not targets:
            raise ValueError(f"empty readiness producer: {job_name}")
        labels[job_name] = targets
    return labels


def bind_rule_tests(rendered, rules_file, template_file, output_file):
    import yaml

    labels = deployed_labels(rendered)
    rules = yaml.safe_load(Path(rules_file).read_text())
    fixtures = yaml.safe_load(Path(template_file).read_text())
    readiness_rules = {}
    rule_definitions = {}
    for group in rules["groups"]:
        for rule in group["rules"]:
            selectors = re.findall(r'(?:probe_success|up)\{job(=|=~)"([^"]+)"\}', rule["expr"])
            for operator, selector in selectors:
                jobs = selector.split("|") if operator == "=~" else [selector]
                if any(job in READINESS_JOBS or "readyz" in job for job in jobs):
                    if any(job not in labels for job in jobs):
                        raise ValueError(f"readiness alert has no deployed producer: {rule['alert']}")
                    readiness_rules[rule["alert"]] = jobs
                    rule_definitions[rule["alert"]] = rule
    expected_rules = {"NodeNotReady", "NodeRPCDown", "NodeReadinessProbeDown"}
    if set(readiness_rules) != expected_rules:
        raise ValueError("every merged-node readiness alert must have a deployed producer")

    def target(marker):
        job, index = marker.split(":")
        return labels[job][int(index)]

    firing_producers = set()
    for case in fixtures["tests"]:
        series_labels = []
        for series in case["input_series"]:
            match = re.fullmatch(r'([a-z_]+)\{__target__="([^"]+)"\}', series["series"])
            if match:
                metric, marker = match.groups()
                if metric not in {"probe_success", "up"}:
                    raise ValueError("readiness fixtures require an emitted probe metric")
                actual = target(marker)
                series["series"] = metric + "{" + ",".join(
                    f"{key}={json.dumps(value)}" for key, value in actual.items()
                ) + "}"
                series_labels.append((metric, actual))
        for check in case["alert_rule_test"]:
            name = check["alertname"]
            if name in readiness_rules:
                metric = "up" if name == "NodeReadinessProbeDown" else "probe_success"
                for job in readiness_rules[name]:
                    if not any(m == metric and label["job"] == job for m, label in series_labels):
                        raise ValueError(f"readiness fixture lacks a rendered producer: {name}/{job}")
            for alert in check["exp_alerts"]:
                expected = alert["exp_labels"]
                marker = expected.pop("__target__", None)
                if marker:
                    actual = target(marker)
                    expected.update(actual)
                    for key, value in alert["exp_annotations"].items():
                        alert["exp_annotations"][key] = value.replace("@instance@", actual["instance"])
                if name in readiness_rules:
                    metric = "up" if name == "NodeReadinessProbeDown" else "probe_success"
                    actual = {key: expected[key] for key in ("job", "instance", "host")}
                    rule = rule_definitions[name]
                    if actual["job"] not in readiness_rules[name] or (metric, actual) not in series_labels:
                        raise ValueError(f"readiness expectation has no matching producer: {name}")
                    if expected != {**actual, **rule["labels"]}:
                        raise ValueError(f"readiness expectation has incorrect labels: {name}")
                    summary = rule["annotations"]["summary"].replace("{{ $labels.instance }}", actual["instance"])
                    if alert["exp_annotations"] != {"summary": summary}:
                        raise ValueError(f"readiness expectation has incorrect annotations: {name}")
                    firing_producers.add((name, actual["job"]))
    seen = {check["alertname"] for case in fixtures["tests"] for check in case["alert_rule_test"]}
    required_producers = {(name, job) for name, jobs in readiness_rules.items() for job in jobs}
    if firing_producers != required_producers:
        raise ValueError("readiness producer has no firing fixture")
    if not expected_rules <= seen:
        raise ValueError("readiness rule has no firing fixture")
    Path(output_file).write_text(yaml.safe_dump(fixtures, sort_keys=False))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    targets = commands.add_parser("targets")
    targets.add_argument("hosts")
    targets.add_argument("port")
    targets.add_argument("destination")
    fixtures = commands.add_parser("fixtures")
    fixtures.add_argument("rendered")
    fixtures.add_argument("rules")
    fixtures.add_argument("template")
    fixtures.add_argument("output")
    args = parser.parse_args()
    if args.command == "targets":
        render_targets(args.hosts, args.port, args.destination)
    else:
        bind_rule_tests(args.rendered, args.rules, args.template, args.output)


if __name__ == "__main__":
    main()
