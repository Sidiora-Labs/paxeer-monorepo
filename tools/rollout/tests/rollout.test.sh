#!/usr/bin/env bash
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
root="$(cd "$here/../../.." && pwd)"
rollout="$root/tools/rollout/rollout.sh"
ledger="$root/tools/rollout/ledger.sh"
manifest="$here/retained.kvx"
export ROLLOUT_MANIFEST="$manifest"
sha=0123456789abcdef0123456789abcdef01234567
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
export ROLLOUT_DRY_RUN=1
unset RAILWAY_API_TOKEN RAILWAY_PROJECT_ID

fail() {
	echo "FAIL: $*" >&2
	exit 1
}

plan_precedes() {
	awk -v a="$1" -v b="$2" 'NR > 1 && $2 == a {x = $1} NR > 1 && $2 == b {y = $1}
		END {exit !(x != "" && y != "" && x + 0 < y + 0)}' "$3"
}

"$rollout" plan --sha "$sha" >"$tmp/plan"
mapfile -t want < <(sed -n 's/^\[service\.\(.*\)\]$/\1/p' "$manifest")
[ "${#want[@]}" -eq 24 ] || fail "manifest has ${#want[@]} services, want 24"
mapfile -t got < <(awk 'NR > 1 {print $2}' "$tmp/plan")
[ "${#got[@]}" -eq "${#want[@]}" ] || fail "plan lists ${#got[@]} of ${#want[@]} services"
for s in "${want[@]}"; do
	grep -qE "^ *[0-9]+ $s " "$tmp/plan" || fail "plan misses $s"
done
awk 'NR > 1 {print $1}' "$tmp/plan" | sort -n -c || fail "plan is not in wave order"

for s in identity gateway redis-router redis-internal internal-kms internal-journeys internal-payments \
	internal-approvals internal-programs webhooks-public webhooks-ingress dashboard dashboard-web interop \
	search-front ramp wallet-gateway wallet-postgres gas-station indexer ai-evaluator; do
	grep -qE "^ *[0-9]+ $s +railway +$s " "$tmp/plan" || fail "$s is not a railway target"
done
for s in paxd program-registry archive; do
	grep -qE "^ *[0-9]+ $s +box +$s ghcr\.io/sidiora-labs/[a-z0-9-]+:${sha:0:12} unit=[a-z0-9-]+\.service host=\\\$[A-Z_]+_HOST " "$tmp/plan" ||
		fail "$s is not a box target"
done
grep -qE " identity +railway +identity commit=$sha " "$tmp/plan" || fail "railway rows do not pin the commit"
grep -qE " redis-router +railway +redis-router skip stock-image=redis$" "$tmp/plan" || fail "stock redis is not skipped"
awk 'NR > 1 && ($2 == "router" || $2 == "sequencer-primary" || $2 == "sequencer-standby" || $2 == "kernel" || $2 == "intent-ingester" || $2 == "oracle-feeder" || $2 == "perps-keeper" ||
	$2 ~ /relay-archive/ || $2 ~ /guarantor/) {exit 1}' "$tmp/plan" ||
	fail "plan includes a retired service"
plan_precedes paxd program-registry "$tmp/plan" || fail "program-registry does not follow paxd"
plan_precedes paxd archive "$tmp/plan" || fail "archive does not follow paxd"
plan_precedes wallet-postgres wallet-gateway "$tmp/plan" || fail "wallet-gateway does not follow its database"
plan_precedes webhooks-ingress webhooks-public "$tmp/plan" || fail "public webhooks do not follow ingress"
plan_precedes gas-station ai-evaluator "$tmp/plan" || fail "ai-evaluator does not follow gas-station"
grep -qE "^ *5 archive +box +archive ghcr\.io/sidiora-labs/paxd:${sha:0:12} unit=paxd\.service host=\\\$ARCHIVE_HOST health=tcp:8545$" "$tmp/plan" ||
	fail "archive does not use its paxd image, host, unit and health check"
grep -qE "^ *12 ai-evaluator +railway +ai-evaluator commit=$sha health=/healthz$" "$tmp/plan" ||
	fail "ai-evaluator does not pin its hosted deployment and health check"
python3 - "$root" "$manifest" <<'PY'
import sys
from pathlib import Path

sys.path.insert(0, str(Path(sys.argv[1]) / "tools" / "rollout"))
from rollout import services

units = {unit["name"]: unit for unit in services(sys.argv[2])}
assert not any(
    name in {"router", "sequencer-primary", "sequencer-standby", "kernel", "intent-ingester", "oracle-feeder", "perps-keeper"} or "relay-archive" in name or "guarantor" in name
    for name in units
), units.keys()
assert units["archive"] == {
    "name": "archive",
    "target": "box",
    "image": "paxd",
    "box_role": "archive",
    "box_unit": "paxd.service",
    "health": "tcp:8545",
    "order": 5,
}, units["archive"]
assert units["ai-evaluator"] == {
    "name": "ai-evaluator",
    "target": "railway",
    "image": "ai-evaluator",
    "railway_service": "ai-evaluator",
    "health": "/healthz",
    "order": 12,
}, units["ai-evaluator"]
PY
grep -qE "^ *3 paxd +box +paxd ghcr\.io/sidiora-labs/paxd:${sha:0:12} unit=paxd\.service host=\\\$PAXD_HOST health=tcp:8545$" "$tmp/plan" ||
	fail "paxd does not own its image, host, unit and health check"
sed -e '/^\[service\.paxd\]$/,/^order/s/^order = 3$/order = 5/' "$manifest" >"$tmp/paxd-last.kvx"
ROLLOUT_MANIFEST="$tmp/paxd-last.kvx" "$rollout" plan --sha "$sha" >"$tmp/paxd-last.plan"
if plan_precedes paxd program-registry "$tmp/paxd-last.plan"; then
	fail "ordering check accepted a plan listing program-registry before paxd"
fi
for order in 2 3; do
	sed -e "/^\[service\.archive\]$/,/^order/s/^order = 5$/order = $order/" "$manifest" >"$tmp/archive-$order.kvx"
	ROLLOUT_MANIFEST="$tmp/archive-$order.kvx" "$rollout" plan --sha "$sha" >"$tmp/archive-$order.plan"
	if plan_precedes paxd archive "$tmp/archive-$order.plan"; then
		fail "ordering check accepted archive at or before paxd"
	fi
done

"$rollout" apply --sha "$sha" --only paxd,identity >"$tmp/apply"
[ "$(awk 'NR > 1' "$tmp/apply" | wc -l)" -eq 2 ] || fail "--only did not narrow the plan"
grep -qE "^ *3 paxd +box +paxd " "$tmp/apply" || fail "--only misses paxd"
grep -qE "^ *2 identity +railway +identity " "$tmp/apply" || fail "--only misses identity"
for mode in plan apply; do
	for s in router sequencer-primary sequencer-standby kernel intent-ingester relay-archive-a relay-archive-b oracle-feeder perps-keeper; do
		for selection in "$s" "$s,identity"; do
			status=0
			"$rollout" "$mode" --sha "$sha" --only "$selection" >"$tmp/retired.out" 2>"$tmp/retired.err" || status=$?
			[ "$status" -eq 2 ] || fail "$mode --only $selection returned $status, want 2"
			[ ! -s "$tmp/retired.out" ] || fail "$mode --only $selection printed a partial plan"
			grep -qxF "unknown services: $s" "$tmp/retired.err" || fail "$mode --only $selection returned the wrong refusal"
		done
	done
done
"$rollout" apply --sha "$sha" --only archive,ai-evaluator >"$tmp/new-units"
[ "$(awk 'NR > 1' "$tmp/new-units" | wc -l)" -eq 2 ] || fail "--only did not select the two new units"
grep -qE "^ *5 archive +box +archive " "$tmp/new-units" || fail "--only misses archive"
grep -qE "^ *12 ai-evaluator +railway +ai-evaluator " "$tmp/new-units" || fail "--only misses ai-evaluator"

expect_reject() {
	local name="$1"
	if ROLLOUT_MANIFEST="$tmp/$name.kvx" "$rollout" plan --sha "$sha" >/dev/null 2>"$tmp/$name.err"; then
		fail "malformed manifest $name was accepted"
	fi
	grep -q "malformed manifest" "$tmp/$name.err" || fail "$name rejected without a manifest error"
}
good='[service.a]
target = "railway"
image = "layerx-gateway"
railway_service = "a"
health = "/healthz"
order = 1'
printf '%s\n' "$good" | sed 's/^image = "layerx-gateway"$/image = layerx-gateway/' >"$tmp/unquoted.kvx"
printf '%s\n' "$good" | sed '/^health/d' >"$tmp/missing.kvx"
printf '%s\n' "$good" | sed 's/"railway"/"fly"/' >"$tmp/target.kvx"
printf '%s\nbox_unit = "a.service"\n' "$good" >"$tmp/extra.kvx"
printf '%s\n%s\n' "$good" "$good" >"$tmp/duplicate.kvx"
printf '%s\n' "$good" | sed 's/^order = 1$/order = "1"/' >"$tmp/order.kvx"
printf '%s\n' "$good" | sed 's#"/healthz"#"http://10.0.0.1/healthz"#' >"$tmp/health.kvx"
printf 'target = "railway"\n%s\n' "$good" >"$tmp/orphan.kvx"
printf '[deploy.a]\ntarget = "railway"\n' >"$tmp/section.kvx"
: >"$tmp/empty.kvx"
for name in unquoted missing target extra duplicate order health orphan section empty; do
	expect_reject "$name"
done
printf '%s\n' "$good" >"$tmp/good.kvx"
ROLLOUT_MANIFEST="$tmp/good.kvx" "$rollout" plan --sha "$sha" >/dev/null || fail "well-formed manifest rejected"

if "$rollout" plan --sha "${sha:0:12}" >/dev/null 2>&1; then fail "short sha accepted"; fi
if "$rollout" plan --sha "$sha" --only nope >/dev/null 2>&1; then fail "unknown --only accepted"; fi

digest=sha256:$(printf '%064d' 0 | tr 0 a)
printf '[service.gateway]\ntarget = "railway-image"\nimage = "layerx-gateway"\nrailway_service = "gateway"\nhealth = "/healthz"\norder = 1\n' \
	>"$tmp/image.kvx"
printf '{"layerx-gateway": "%s"}\n' "$digest" >"$tmp/digests.json"
ROLLOUT_MANIFEST="$tmp/image.kvx" "$rollout" plan --sha "$sha" --digests "$tmp/digests.json" >"$tmp/image.plan"
grep -qE "^ *1 gateway +railway-image gateway image=ghcr\.io/sidiora-labs/layerx-gateway@$digest health=/healthz$" "$tmp/image.plan" ||
	fail "railway-image row does not print its image digest"
printf '{"other-image": "%s"}\n' "$digest" >"$tmp/no-digest.json"
for case in no-digest no-map; do
	args=(--digests "$tmp/no-digest.json")
	[ "$case" = no-map ] && args=()
	status=0
	ROLLOUT_MANIFEST="$tmp/image.kvx" "$rollout" plan --sha "$sha" "${args[@]}" >"$tmp/$case.out" 2>"$tmp/$case.err" || status=$?
	[ "$status" -eq 2 ] || fail "railway-image plan ($case) returned $status, want 2"
	[ ! -s "$tmp/$case.out" ] || fail "railway-image plan ($case) printed a partial plan"
done
grep -qxF "digest map has no digest for: layerx-gateway" "$tmp/no-digest.err" || fail "missing digest returned the wrong refusal"
grep -qxF -- "--digests is required for railway-image services" "$tmp/no-map.err" || fail "missing --digests returned the wrong refusal"

for shipped in rollout rollout-launchpad rollout-perps; do
	sed -n 's/^image = "\(.*\)"$/"\1": "'"$digest"'"/p' "$root/deploy/$shipped.kvx" | sort -u | paste -sd, - |
		sed 's/.*/{&}/' >"$tmp/$shipped-digests.json"
	status=0
	ROLLOUT_DRY_RUN=0 ROLLOUT_MANIFEST="$root/deploy/$shipped.kvx" "$rollout" apply --sha "$sha" \
		--digests "$tmp/$shipped-digests.json" >/dev/null 2>"$tmp/$shipped-apply.err" || status=$?
	[ "$status" -eq 1 ] && grep -qxF "RAILWAY_PROJECT_ID is not set" "$tmp/$shipped-apply.err" ||
		fail "deploy/$shipped.kvx apply did not reach the Railway project lookup ($status): $(cat "$tmp/$shipped-apply.err")"
done

export ROLLOUT_LEDGER_DIR="$tmp/ledger"
"$ledger" "$sha" identity railway 2026-10-07T10:00:00Z pass /root/lx-ops/rollout/2026-10-07/x-identity.log
"$ledger" "$sha" identity railway 2026-10-07T10:05:00Z fail /root/lx-ops/rollout/2026-10-07/x-identity.log
grep -qx "\[rollout.${sha:0:12}.identity\]" "$tmp/ledger/2026-10-07.kvx" || fail "ledger record missing"
grep -qx "\[rollout.${sha:0:12}.identity.2\]" "$tmp/ledger/2026-10-07.kvx" || fail "second ledger record missing"
if "$ledger" "$sha" identity box 2026-10-07T10:00:00Z pass 10.1.2.3:/var/log/x >/dev/null 2>&1; then
	fail "ledger accepted evidence outside /root/lx-ops"
fi

replay_logs="$(mktemp -d)"
trap 'rm -rf "$tmp" "$replay_logs"' EXIT
(
	unset ROLLOUT_DRY_RUN
	export ROLLOUT_LOG_DIR="$replay_logs" ROLLOUT_EVIDENCE_ROOT="$replay_logs" ROLLOUT_LEDGER_DIR="$tmp/replay-ledger" ROLLOUT_POLL=0
	export RAILWAY_API_TOKEN=sentinel-railway-token RAILWAY_PROJECT_ID=25709f39-612a-5373-86a3-2bc9e4478de1
	python3 - "$root" "$here/replay.json" "$tmp" <<'PY'
import contextlib
import io
import json
import re
import sys
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

sys.path.insert(0, str(Path(sys.argv[1]) / "tools" / "rollout"))
import rollout

record = json.loads(Path(sys.argv[2]).read_text())
tmp = Path(sys.argv[3])
digests = tmp / "replay-digests.json"
digests.write_text(json.dumps(record["digests"]))
pending, seen = [], []


class Railway(BaseHTTPRequestHandler):
    def log_message(self, *args):
        pass

    def do_POST(self):
        body = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
        op = re.match(r"\s*(?:query|mutation)\s+(\w+)", body["query"])
        got = {"op": op and op.group(1), "variables": body["variables"]}
        seen.append(got)
        want = pending.pop(0) if pending else None
        if self.headers.get("Authorization") != "Bearer sentinel-railway-token":
            reply = {"errors": [{"message": "bad token"}]}
        elif want is None or {"op": want["op"], "variables": want["variables"]} != got:
            reply = {"errors": [{"message": f"unexpected request {got}, transcript wants {want}"}]}
        elif "errors" in want:
            reply = {"errors": want["errors"]}
        else:
            reply = {"data": want["data"]}
        data = json.dumps(reply).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)


def validate_wake_health(case):
    sleeping = any(
        call.get("data", {}).get("serviceInstance", {}).get("latestDeployment", {}).get("status") == "SLEEPING"
        for call in case["calls"]
        if call.get("data", {}).get("serviceInstance", {}).get("latestDeployment")
    )
    if not sleeping:
        return
    proof = case["wake_health"]
    assert proof["required_for_pass"] is True
    assert f'health = "{proof["path"]}"' in case["manifest"]
    if case["exit"] == 0:
        assert proof["state"] == "ready"
        assert proof["observations"] == [
            {"event": "wake", "result": "success"},
            {"event": "health", "method": "GET", "path": proof["path"], "status": 200},
        ]
        assert case["calls"][-1]["data"]["deployment"]["status"] == "SUCCESS"
        assert case["out"] == ["pass gateway"]
    else:
        assert proof["state"] in {"unavailable", "timed-out"}
        assert case["exit"] == 1
        assert case["out"] == ["blocked gateway"]
        assert case["calls"][-1]["data"]["deployment"]["status"] == "SLEEPING"


server = ThreadingHTTPServer(("127.0.0.1", 0), Railway)
threading.Thread(target=server.serve_forever, daemon=True).start()
rollout.API = f"http://127.0.0.1:{server.server_port}/graphql/v2"
failures = []
for name, case in record["scenarios"].items():
    validate_wake_health(case)
    manifest = tmp / f"replay-{name}.kvx"
    manifest.write_text("\n".join(case["manifest"]) + "\n")
    rollout.os.environ["ROLLOUT_MANIFEST"] = str(manifest)
    pending[:] = [dict(c) for c in case["calls"]]
    seen.clear()
    out, err = io.StringIO(), io.StringIO()
    with contextlib.redirect_stdout(out), contextlib.redirect_stderr(err):
        try:
            code = rollout.main([str(digests) if a == "@digests" else a for a in case["argv"]])
        except SystemExit as e:
            code = e.code if isinstance(e.code, int) else 1
            print(e.code, file=sys.stderr)
    lines = out.getvalue().splitlines()
    problems = []
    if code != case["exit"]:
        problems.append(f"exit {code}, want {case['exit']}")
    if pending:
        problems.append(f"{len(pending)} recorded requests not made, next {pending[0]['op']}")
    if len(seen) != len(case["calls"]):
        problems.append(f"made {len(seen)} requests, transcript has {len(case['calls'])}")
    for want in case.get("out", []):
        if not any(line.startswith(want + " evidence=") for line in lines):
            problems.append(f"no outcome line {want!r}")
    if "err" in case and case["err"] not in err.getvalue().splitlines():
        problems.append(f"no refusal {case['err']!r}")
    if code == 2 and any(re.match(r"^(pass|fail|blocked) ", line) for line in lines):
        problems.append("a refused run applied a service")
    for line in lines:
        m = re.match(r"^(pass|fail|blocked) (\S+) evidence=(\S+)$", line)
        if m and not Path(m.group(3)).is_file():
            problems.append(f"evidence {m.group(3)} missing")
    evidence = "".join(Path(m.group(1)).read_text() for line in lines
                       if (m := re.match(r"^(?:pass|fail|blocked) \S+ evidence=(\S+)$", line)) and Path(m.group(1)).is_file())
    for seed in case.get("secrets", []):
        if seed in out.getvalue() + err.getvalue() + evidence:
            problems.append(f"printed or logged the secret {seed!r}")
    if case.get("secrets") and "[redacted]" not in evidence:
        problems.append("the evidence log does not carry the redacted error")
    marked = case.get("marked", {})
    if case.get("secrets") and set(marked) != set(case["secrets"]):
        problems.append("every seeded secret needs a marked context or cut")
    for seed, mark in marked.items():
        if mark == "cut":
            if not re.search(r"\.\.\. \(\d+ more characters\)", evidence):
                problems.append(f"the error carrying {seed!r} past the limit is not cut")
        elif mark not in evidence:
            problems.append(f"no redaction marker {mark!r} where {seed!r} was")
    if problems:
        failures.append(f"{name}: {'; '.join(problems)}\n  stdout: {out.getvalue()!r}\n  stderr: {err.getvalue()!r}")
server.shutdown()
ledger = "".join(p.read_text() for p in sorted((tmp / "replay-ledger").glob("*.kvx")))
outcomes = re.findall(r'outcome = "(\w+)"', ledger)
want_outcomes = [o.split()[0] for c in record["scenarios"].values() for o in c.get("out", [])]
if outcomes != want_outcomes:
    failures.append(f"ledger outcomes {outcomes}, want {want_outcomes}")
if failures:
    sys.exit("FAIL: apply replay\n" + "\n".join(failures))
print(f"apply replay: {len(record['scenarios'])} scenarios")
PY
) || fail "apply replay against the recorded Railway transcript"

echo "PASS rollout plan (${#want[@]} services), railway-image digests, shipped manifests reach the project lookup, retired service rejection, manifest rejection, ledger, apply replay"
