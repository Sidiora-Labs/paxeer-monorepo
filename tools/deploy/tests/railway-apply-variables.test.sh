#!/usr/bin/env bash
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
root="$(cd "$here/../../.." && pwd)"
tool="$root/tools/deploy/railway-apply-variables.py"
replay="$here/fixtures/railway-apply-variables-replay.json"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

fail() {
	echo "FAIL: $*" >&2
	exit 1
}

# The charts service from the real manifest; every name it needs gets a sentinel
# value from --values, except RPC_URL, which comes from the environment under --from-env.
python3 - "$root/deploy/launchpad/railway/variables.json" "$tmp" "$replay" <<'PY'
import json, sys
m = json.load(open(sys.argv[1]))
charts = m["charts"]
json.dump({"charts": charts}, open(f"{sys.argv[2]}/manifest.json", "w"))
# charts first, then a service the project does not have: apply must refuse before writing charts.
json.dump({"charts": charts, "ghost": {"fixed": {"NODE_ENV": "production"}}}, open(f"{sys.argv[2]}/ghost.json", "w"))
needed = [n for k, v in charts.items() if isinstance(v, list) and k not in ("secret", "secrets")
          for n in v if n not in charts.get("fixed", {})]
values = {n: f"sentinel-value-{i:02d}" for i, n in enumerate(needed) if n != "RPC_URL"}
values["WEBHOOK_HMAC_SECRET"] = "sentinel-webhook-secret-9d4e"
values["DATABASE_URL"] = "postgresql://charts:sentinel-db-password-77@postgres.railway.internal:5432/railway"
for document, path in charts["documentSources"].items():
    values.pop(document, None)
    values[path] = f"/app/prod/{path.lower()}.json"
json.dump({"charts": values}, open(f"{sys.argv[2]}/values.json", "w"))
bad = dict(values, REDIS_URL="redis://line-one\nline-two")
json.dump({"charts": bad}, open(f"{sys.argv[2]}/newline.json", "w"))

# Apply replay: the recorded reads plus the two writes the apply must send for charts.
replay = json.load(open(sys.argv[3]))
live = next(e for e in replay if e["op"] == "Variables")["data"]["variables"]
want = {**charts["defaults"], **charts["fixed"], **values, "RPC_URL": "https://rpc.sentinel.invalid"}
upsert = {k: v for k, v in want.items() if live.get(k) != v}
ids = {"projectId": "proj-test", "environmentId": "env-prod", "serviceId": "svc-charts"}
replay += [
    {"op": "VariablesUpsert", "variables": {"i": dict(ids, variables=upsert, skipDeploys=True)},
     "data": {"variableCollectionUpsert": True}},
    {"op": "InstanceUpdate", "variables": {"s": "svc-charts", "e": "env-prod", "i": {"registryCredentials": {
        "username": "sentinel-ghcr-user-4a1f", "password": "sentinel-ghcr-token-8c3e"}}},
     "data": {"serviceInstanceUpdate": True}},
]
json.dump(replay, open(f"{sys.argv[2]}/apply-replay.json", "w"))
print(len(upsert), file=open(f"{sys.argv[2]}/upsert-count", "w"))
PY

export RAILWAY_API_TOKEN=sentinel-railway-token-2b61
export RPC_URL=https://rpc.sentinel.invalid
run() { python3 "$tool" proj-test production --manifest "$tmp/manifest.json" --replay "$replay" "$@"; }

run --plan --values "$tmp/values.json" --from-env --registry-credentials >"$tmp/plan.out" 2>&1 || fail "plan exited $?: $(cat "$tmp/plan.out")"
cat "$tmp/plan.out"
for name in EVENT_EMITTER_ADDRESS POOL_BEACON_ADDRESS REDIS_BULL_URL WEBHOOK_HMAC_SECRET; do
	grep -q "^  missing $name = " "$tmp/plan.out" || fail "plan does not list missing $name"
done
grep -q '^  missing WEBHOOK_HMAC_SECRET = \*\*\*\*$' "$tmp/plan.out" || fail "secret value not masked"
grep -q '^  differs RUNTIME_PROFILE platform -> container$' "$tmp/plan.out" || fail "differing fixed value not shown"
grep -q '^  differs DATABASE_URL \*\*\*\* -> \*\*\*\*$' "$tmp/plan.out" || fail "password URL not masked"
grep -q '^  differs RPC_URL https://rpc.example.invalid -> https://rpc.sentinel.invalid$' "$tmp/plan.out" ||
	fail "environment value not used"
grep -q '^  extra LEGACY_CHART_SETTING (kept)$' "$tmp/plan.out" || fail "extra name not reported"
grep -q 'extra CHART_FEATURES_CONFIG_FILE' "$tmp/plan.out" && fail "document source reported as extra"
grep -q '^  registry credentials user=\$GHCR_USERNAME' "$tmp/plan.out" || fail "registry credentials not planned"
grep -q '^  missing NODE_ENV' "$tmp/plan.out" && fail "unchanged name listed as missing"
if grep -Eq 'sentinel-(webhook-secret|db-password)|live-(db|redis)-password|sentinel-railway-token' "$tmp/plan.out"; then
	fail "plan printed a secret"
fi

set +e
env -u RAILWAY_API_TOKEN python3 "$tool" proj-test production --manifest "$tmp/manifest.json" --replay "$replay" \
	--apply --values "$tmp/values.json" >"$tmp/notoken.out" 2>&1
code=$?
run --apply --values "$tmp/newline.json" >"$tmp/newline.out" 2>&1
nl=$?
env -u RPC_URL python3 "$tool" proj-test production --manifest "$tmp/manifest.json" --replay "$replay" \
	--plan --values "$tmp/values.json" --from-env >"$tmp/unset.out" 2>&1
unset_code=$?
env -u RPC_URL python3 "$tool" proj-test production --manifest "$tmp/manifest.json" --replay "$replay" \
	--apply --values "$tmp/values.json" --from-env >"$tmp/unset-apply.out" 2>&1
unset_apply=$?
run --plan --values "$tmp/values.json" >"$tmp/noenv.out" 2>&1
noenv=$?
run --apply --values "$tmp/values.json" >"$tmp/noenv-apply.out" 2>&1
noenv_apply=$?
GHCR_USERNAME=sentinel-ghcr-user-4a1f GHCR_TOKEN=sentinel-ghcr-token-8c3e python3 "$tool" proj-test production \
	--manifest "$tmp/ghost.json" --replay "$tmp/apply-replay.json" --apply --values "$tmp/values.json" --from-env \
	--registry-credentials >"$tmp/ghost.out" 2>&1
ghost=$?
GHCR_USERNAME=sentinel-ghcr-user-4a1f GHCR_TOKEN=sentinel-ghcr-token-8c3e python3 "$tool" proj-test production \
	--manifest "$tmp/manifest.json" --replay "$tmp/apply-replay.json" --apply --values "$tmp/values.json" --from-env \
	--registry-credentials >"$tmp/apply.out" 2>&1
apply_code=$?
set -e
[ "$code" -eq 2 ] || fail "--apply without a token exited $code, want 2"
grep -q 'RAILWAY_API_TOKEN is not set' "$tmp/notoken.out" || fail "no token message"
[ "$nl" -eq 2 ] || fail "newline value exited $nl, want 2"
grep -q 'charts: REDIS_URL contains a newline' "$tmp/newline.out" || fail "no newline message"
[ "$unset_code" -eq 3 ] || fail "plan with an unresolved name exited $unset_code, want 3"
grep -q '^  unresolved RPC_URL has no value' "$tmp/unset.out" || fail "plan does not list the unresolved name"
grep -q '^  missing WEBHOOK_HMAC_SECRET = \*\*\*\*$' "$tmp/unset.out" || fail "plan with an unresolved name is incomplete"
grep -q 'extra RPC_URL' "$tmp/unset.out" && fail "unresolved name reported as extra"
[ "$unset_apply" -eq 2 ] || fail "apply with an unresolved name exited $unset_apply, want 2"
grep -q 'refused charts: RPC_URL has no value' "$tmp/unset-apply.out" || fail "no apply refusal message"
# RPC_URL is exported, but without --from-env the shell environment is never a value source.
[ "$noenv" -eq 3 ] || fail "plan without --from-env exited $noenv, want 3"
grep -q '^  unresolved RPC_URL has no value (manifest, --values; --from-env not given)$' "$tmp/noenv.out" ||
	fail "plan without --from-env took RPC_URL from the environment"
grep -q 'rpc.sentinel.invalid' "$tmp/noenv.out" && fail "plan without --from-env printed the environment value"
[ "$noenv_apply" -eq 2 ] || fail "apply without --from-env exited $noenv_apply, want 2"
grep -q 'refused charts: RPC_URL has no value' "$tmp/noenv-apply.out" || fail "apply without --from-env not refused"
[ "$ghost" -eq 1 ] || fail "apply with a service missing from the project exited $ghost, want 1"
grep -q '^service ghost not in project$' "$tmp/ghost.out" || fail "missing service not named"
grep -q 'refused: 1 manifest services are not in project proj-test; nothing written' "$tmp/ghost.out" ||
	fail "no refusal for the missing service"
grep -q 'applied' "$tmp/ghost.out" && fail "apply wrote to charts before refusing the missing service"
[ "$apply_code" -eq 0 ] || fail "apply exited $apply_code: $(cat "$tmp/apply.out")"
grep -q '^  applied registry credentials$' "$tmp/apply.out" || fail "registry credentials not applied to a service without an image read"
grep -q "^  applied $(cat "$tmp/upsert-count") variables$" "$tmp/apply.out" || fail "apply did not upsert the changed variables"
grep -q 'sentinel-ghcr-token' "$tmp/apply.out" && fail "apply printed the registry token"

stats_inline_pem_plan() {
	python3 - "$root/deploy/launchpad/railway/variables.json" "$tmp" "$replay" <<'PY'
import json, sys
stats = json.load(open(sys.argv[1]))["stats"]
json.dump({"stats": stats}, open(f"{sys.argv[2]}/stats.json", "w"))
needed = [n for k, v in stats.items() if isinstance(v, list) and k not in ("secret", "secrets")
          for n in v if n not in stats["fixed"]]
values = {n: f"sentinel-stats-{i:02d}" for i, n in enumerate(needed)}
values["IDENTITY_MTLS_CLIENT_KEY_PEM"] = "-----BEGIN PRIVATE KEY-----\nsentinel-stats-pem-3b7e\n-----END PRIVATE KEY-----"
json.dump({"stats": values}, open(f"{sys.argv[2]}/stats-values.json", "w"))
project = next(e for e in json.load(open(sys.argv[3])) if e["op"] == "Project")
project["data"]["project"]["services"]["edges"] = [{"node": {"id": "svc-stats", "name": "stats"}}]
want = {**stats["defaults"], **stats["fixed"], **values}
json.dump([project,
           {"op": "Variables", "variables": {"p": "proj-test", "e": "env-prod", "s": "svc-stats"},
            "data": {"variables": {}}},
           {"op": "VariablesUpsert", "variables": {"i": {"projectId": "proj-test", "environmentId": "env-prod",
            "serviceId": "svc-stats", "variables": want, "skipDeploys": True}},
            "data": {"variableCollectionUpsert": True}}], open(f"{sys.argv[2]}/stats-replay.json", "w"))
print(len(want), file=open(f"{sys.argv[2]}/stats-upsert-count", "w"))
PY
	local mode
	for mode in plan apply; do
		python3 "$tool" proj-test production --manifest "$tmp/stats.json" --replay "$tmp/stats-replay.json" \
			--values "$tmp/stats-values.json" "--$mode" >"$tmp/stats-$mode.out" 2>&1 ||
			fail "stats $mode exited $?: $(cat "$tmp/stats-$mode.out")"
		grep -q '^  missing IDENTITY_MTLS_CLIENT_KEY_PEM = \*\*\*\*$' "$tmp/stats-$mode.out" ||
			fail "inline PEM not masked in stats $mode"
		if grep -q 'contains a newline' "$tmp/stats-$mode.out"; then
			fail "stats $mode refused inline PEM"
		fi
	done
	grep -q "^  applied $(cat "$tmp/stats-upsert-count") variables$" "$tmp/stats-apply.out" ||
		fail "stats apply did not upsert the raw inline PEM"
	python3 - "$tmp/stats-values.json" "$tmp/stats-plan.out" "$tmp/stats-apply.out" <<'PY' || fail "stats plan or apply printed a PEM line"
import json, sys
pem = json.load(open(sys.argv[1]))["stats"]["IDENTITY_MTLS_CLIENT_KEY_PEM"]
for path in sys.argv[2:]:
    out = open(path).read()
    assert not any(line in out for line in pem.splitlines()), f"{path} printed a PEM line"
PY
}
stats_inline_pem_plan

metadata_inline_materials() {
	python3 - "$root/deploy/launchpad/railway/variables.json" "$tmp" "$replay" <<'PY'
import json, secrets, subprocess, sys
from pathlib import Path
metadata = json.load(open(sys.argv[1]))["metadata"]
assert metadata["fixed"]["METADATA_INGRESS"] == "mtls"
assert not {"IDENTITY_SERVICE_URL", "IDENTITY_SERVICE_TOKEN"} & set(metadata["required"])
assert {"METADATA_TLS_KEY_PEM", "METADATA_ASSERTION_PRIVATE_KEY_HEX"} <= set(metadata["secrets"])
directory = Path(sys.argv[2])
subprocess.run(["openssl", "req", "-x509", "-newkey", "ed25519", "-noenc", "-days", "1",
                "-subj", "/CN=metadata test authority", "-keyout", str(directory / "metadata.key"),
                "-out", str(directory / "metadata.pem")], check=True, capture_output=True)
values = {name: f"metadata-value-{i:02d}" for i, name in enumerate(metadata["required"])}
values.update({"METADATA_TLS_CERT_PEM": (directory / "metadata.pem").read_text(),
               "METADATA_TLS_CLIENT_CA_PEM": (directory / "metadata.pem").read_text(),
               "METADATA_TLS_KEY_PEM": (directory / "metadata.key").read_text(),
               "METADATA_ASSERTION_PRIVATE_KEY_HEX": secrets.token_hex(64)})
json.dump({"metadata": metadata}, open(directory / "metadata-manifest.json", "w"))
json.dump({"metadata": values}, open(directory / "metadata-values.json", "w"))
project = next(e for e in json.load(open(sys.argv[3])) if e["op"] == "Project")
project["data"]["project"]["services"]["edges"] = [{"node": {"id": "svc-metadata", "name": "metadata"}}]
want = {**metadata["defaults"], **metadata["fixed"], **values}
json.dump([project,
    {"op": "Variables", "variables": {"p": "proj-test", "e": "env-prod", "s": "svc-metadata"},
     "data": {"variables": {}}},
    {"op": "VariablesUpsert", "variables": {"i": {"projectId": "proj-test", "environmentId": "env-prod",
     "serviceId": "svc-metadata", "variables": want, "skipDeploys": True}},
     "data": {"variableCollectionUpsert": True}}], open(directory / "metadata-replay.json", "w"))
print(len(want), file=open(directory / "metadata-count", "w"))
PY
	local mode name
	for mode in plan apply; do
		python3 "$tool" proj-test production --manifest "$tmp/metadata-manifest.json" \
			--replay "$tmp/metadata-replay.json" --values "$tmp/metadata-values.json" "--$mode" \
			>"$tmp/metadata-$mode.out" 2>&1 || fail "metadata $mode exited $?: $(cat "$tmp/metadata-$mode.out")"
		grep -q '^  missing METADATA_INGRESS = mtls$' "$tmp/metadata-$mode.out" || fail "metadata $mode omitted mtls"
		for name in METADATA_TLS_KEY_PEM METADATA_ASSERTION_PRIVATE_KEY_HEX; do
			grep -q "^  missing $name = \*\*\*\*$" "$tmp/metadata-$mode.out" || fail "metadata $mode did not mask $name"
		done
	done
	grep -q "^  applied $(cat "$tmp/metadata-count") variables$" "$tmp/metadata-apply.out" || fail "metadata inline materials were not applied"
	python3 - "$tmp/metadata-values.json" "$tmp/metadata-manifest.json" "$tmp/metadata-plan.out" "$tmp/metadata-apply.out" <<'PY' || fail "metadata printed secret material"
import json, sys
values = json.load(open(sys.argv[1]))["metadata"]
names = json.load(open(sys.argv[2]))["metadata"]["secrets"]
for path in sys.argv[3:]:
    output = open(path).read()
    assert all(value not in output for name in names if (value := values.get(name))), "secret value printed"
    key_lines = values["METADATA_TLS_KEY_PEM"].splitlines()[1:-1]
    assert all(line not in output for line in key_lines), "private key line printed"
PY
	echo PASS metadata-inline-materials
}
metadata_inline_materials

# The user service from the real manifest carries a signing seed: plan and apply never print it.
python3 - "$root/deploy/launchpad/railway/variables.json" "$tmp" "$replay" <<'PY'
import json, sys
user = json.load(open(sys.argv[1]))["user"]
assert "ACCOUNT_SERVICE_SIGNING_SEED" in user["secrets"], "the manifest does not list the signing seed as a secret"
json.dump({"user": user}, open(f"{sys.argv[2]}/user.json", "w"))
needed = [n for k, v in user.items() if isinstance(v, list) and k not in ("secret", "secrets")
          for n in v if n not in user["fixed"]]
values = {n: f"sentinel-user-{i:02d}" for i, n in enumerate(needed)}
values["ACCOUNT_SERVICE_SIGNING_SEED"] = "sentinel-signing-seed-6f0a"
json.dump({"user": values}, open(f"{sys.argv[2]}/user-values.json", "w"))
json.dump([v for n, v in values.items() if n in user["secrets"]] + ["sentinel-live-seed-1c9b"],
          open(f"{sys.argv[2]}/user-secret-values.json", "w"))
project = next(e for e in json.load(open(sys.argv[3])) if e["op"] == "Project")
project["data"]["project"]["services"]["edges"] = [{"node": {"id": "svc-user", "name": "user"}}]
want = {**user["defaults"], **user["fixed"], **values}
json.dump([project,
           {"op": "Variables", "variables": {"p": "proj-test", "e": "env-prod", "s": "svc-user"},
            "data": {"variables": {"ACCOUNT_SERVICE_SIGNING_SEED": "sentinel-live-seed-1c9b"}}},
           {"op": "VariablesUpsert", "variables": {"i": {"projectId": "proj-test", "environmentId": "env-prod",
            "serviceId": "svc-user", "variables": want, "skipDeploys": True}},
            "data": {"variableCollectionUpsert": True}}], open(f"{sys.argv[2]}/user-replay.json", "w"))
# The same service with the seed left out of its secrets list, and with no secrets list at all.
json.dump({"user": dict(user, secrets=[n for n in user["secrets"] if n != "ACCOUNT_SERVICE_SIGNING_SEED"])},
          open(f"{sys.argv[2]}/unlisted.json", "w"))
json.dump({"user": {k: v for k, v in user.items() if k != "secrets"}}, open(f"{sys.argv[2]}/nolist.json", "w"))
PY
user() { python3 "$tool" proj-test production --replay "$tmp/user-replay.json" --values "$tmp/user-values.json" "$@"; }
set +e
user --manifest "$tmp/user.json" --plan >"$tmp/user-plan.out" 2>&1
user_plan=$?
user --manifest "$tmp/user.json" --apply >"$tmp/user-apply.out" 2>&1
user_apply=$?
user --manifest "$tmp/unlisted.json" --plan >"$tmp/unlisted.out" 2>&1
unlisted=$?
user --manifest "$tmp/nolist.json" --apply >"$tmp/nolist.out" 2>&1
nolist=$?
set -e
[ "$user_plan" -eq 0 ] || fail "user plan exited $user_plan: $(cat "$tmp/user-plan.out")"
[ "$user_apply" -eq 0 ] || fail "user apply exited $user_apply: $(cat "$tmp/user-apply.out")"
grep -q '^  differs ACCOUNT_SERVICE_SIGNING_SEED \*\*\*\* -> \*\*\*\*$' "$tmp/user-plan.out" || fail "signing seed not masked in plan"
grep -q '^  applied ' "$tmp/user-apply.out" || fail "user apply did not upsert"
python3 - "$tmp/user-secret-values.json" "$tmp/user-plan.out" "$tmp/user-apply.out" <<'PY' || fail "user plan or apply printed a secret value"
import json, sys
secrets = json.load(open(sys.argv[1]))
for path in sys.argv[2:]:
    out = open(path).read()
    leaked = [v for v in secrets if v in out]
    assert not leaked, f"{path} printed {leaked}"
PY
[ "$unlisted" -eq 2 ] || fail "seed missing from the secrets list exited $unlisted, want 2"
grep -q "bad input: user: ACCOUNT_SERVICE_SIGNING_SEED match the secret name pattern but are not in the service's secrets list" \
	"$tmp/unlisted.out" || fail "no refusal naming the unlisted seed: $(cat "$tmp/unlisted.out")"
grep -q 'sentinel-' "$tmp/unlisted.out" && fail "refused plan printed a value"
[ "$nolist" -eq 2 ] || fail "service without a secrets list exited $nolist, want 2"
grep -q 'ACCOUNT_SERVICE_SIGNING_SEED' "$tmp/nolist.out" || fail "refusal without a secrets list does not name the seed"
grep -q 'applied' "$tmp/nolist.out" && fail "apply wrote before refusing the missing secrets list"
# A GraphQL invalid-input error that echoes the submitted input, and a transcript with no
# response for the write: neither error path prints a submitted or live secret value.
python3 - "$tmp" <<'PY'
import json, sys
replay = json.load(open(f"{sys.argv[1]}/apply-replay.json"))
upsert = next(e for e in replay if e["op"] == "VariablesUpsert")
echo = json.dumps(upsert["variables"]["i"])
upsert.pop("data")
upsert["errors"] = [{"message": f'Variable "$i" got invalid value {echo}; Bearer sentinel-railway-token-2b61',
                     "extensions": {"code": "GRAPHQL_VALIDATION_FAILED"}}]
json.dump(replay, open(f"{sys.argv[1]}/echo-replay.json", "w"))
values = json.load(open(f"{sys.argv[1]}/values.json"))["charts"]
short = {k: values[k] for k in ("WEBHOOK_HMAC_SECRET", "DATABASE_URL")}
upsert["errors"] = [{"message": f'Variable "$i" got invalid value {json.dumps(short)}; Bearer sentinel-railway-token-2b61'}]
json.dump(replay, open(f"{sys.argv[1]}/short-echo-replay.json", "w"))
PY
set +e
GHCR_USERNAME=sentinel-ghcr-user-4a1f GHCR_TOKEN=sentinel-ghcr-token-8c3e python3 "$tool" proj-test production \
	--manifest "$tmp/manifest.json" --replay "$tmp/echo-replay.json" --apply --values "$tmp/values.json" --from-env \
	--registry-credentials >"$tmp/echo.out" 2>"$tmp/echo.err"
echo_code=$?
GHCR_USERNAME=sentinel-ghcr-user-4a1f GHCR_TOKEN=sentinel-ghcr-token-8c3e python3 "$tool" proj-test production \
	--manifest "$tmp/manifest.json" --replay "$replay" --apply --values "$tmp/values.json" --from-env \
	--registry-credentials >"$tmp/miss.out" 2>"$tmp/miss.err"
miss_code=$?
GHCR_USERNAME=sentinel-ghcr-user-4a1f GHCR_TOKEN=sentinel-ghcr-token-8c3e python3 "$tool" proj-test production \
	--manifest "$tmp/manifest.json" --replay "$tmp/short-echo-replay.json" --apply --values "$tmp/values.json" --from-env \
	--registry-credentials >"$tmp/short.out" 2>"$tmp/short.err"
short_code=$?
set -e
[ "$short_code" -eq 1 ] || fail "apply against a short invalid-input error exited $short_code, want 1"
grep -q 'more characters)' "$tmp/short.err" && fail "short invalid-input error was cut: $(cat "$tmp/short.err")"
for mark in '"WEBHOOK_HMAC_SECRET": "[redacted]"' '"DATABASE_URL": "[redacted]"' 'Bearer [redacted]'; do
	grep -qF "$mark" "$tmp/short.err" || fail "short invalid-input error lacks $mark: $(cat "$tmp/short.err")"
done
[ "$echo_code" -eq 1 ] || fail "apply against an invalid-input error exited $echo_code, want 1"
grep -q '^railway call failed: Railway GraphQL error: Variable "\$i" got invalid value .*\.\.\. ([0-9]* more characters)$' "$tmp/echo.err" ||
	fail "invalid-input error not reported bounded: $(cat "$tmp/echo.err")"
[ "$(wc -c <"$tmp/echo.err")" -lt 700 ] || fail "invalid-input error is not bounded"
[ "$miss_code" -eq 1 ] || fail "apply against a transcript without the write exited $miss_code, want 1"
grep -q '^railway call failed: replay has no response for VariablesUpsert with variables i.environmentId, i.projectId, i.serviceId, i.skipDeploys, i.variables.' \
	"$tmp/miss.err" || fail "replay miss does not name the variables: $(cat "$tmp/miss.err")"
for f in echo.out echo.err miss.out miss.err short.out short.err; do
	if grep -Eq 'sentinel-(webhook-secret|db-password|ghcr-token|ghcr-user|railway-token)' "$tmp/$f"; then
		fail "$f printed a secret value: $(cat "$tmp/$f")"
	fi
done
indexer_document_sources() {
	python3 - "$root/deploy/launchpad/railway/variables.json" "$tmp" "$replay" <<'PY'
import json, sys
indexer = json.load(open(sys.argv[1]))["indexer"]
sources = indexer["documentSources"]
assert sources == {
    "EXPECTED_SCHEMA_DOCUMENT": "EXPECTED_SCHEMA_FILE",
    "INDEXER_RECEIPT_CONFIG_DOCUMENT": "INDEXER_RECEIPT_CONFIG_FILE",
    "INDEXER_LIFECYCLE_CONFIG_DOCUMENT": "INDEXER_LIFECYCLE_CONFIG_FILE",
    "DB_BUDGET_DOCUMENT": "DB_BUDGET_FILE",
    "DELIVERY_CONFIG_DOCUMENT": "DELIVERY_CONFIG_FILE",
}
json.dump({"indexer": indexer}, open(f"{sys.argv[2]}/indexer.json", "w"))
values = {n: f"sentinel-indexer-{i:02d}" for i, n in enumerate(indexer["required"])}
project = next(e for e in json.load(open(sys.argv[3])) if e["op"] == "Project")
project["data"]["project"]["services"]["edges"] = [{"node": {"id": "svc-indexer", "name": "indexer"}}]
for form in ("document", "file"):
    selected = dict(values)
    for document, path in sources.items():
        selected.pop(path if form == "document" else document, None)
        selected[document if form == "document" else path] = (
            f"sentinel-{document}" if form == "document" else f"/run/launchpad/indexer/{path}.json")
    json.dump({"indexer": selected}, open(f"{sys.argv[2]}/indexer-{form}-values.json", "w"))
    want = {**indexer["defaults"], **indexer["fixed"], **selected}
    json.dump([project,
        {"op": "Variables", "variables": {"p": "proj-test", "e": "env-prod", "s": "svc-indexer"},
         "data": {"variables": {}}},
        {"op": "VariablesUpsert", "variables": {"i": {"projectId": "proj-test", "environmentId": "env-prod",
         "serviceId": "svc-indexer", "variables": want, "skipDeploys": True}},
         "data": {"variableCollectionUpsert": True}}], open(f"{sys.argv[2]}/indexer-{form}-replay.json", "w"))
    print(len(want), file=open(f"{sys.argv[2]}/indexer-{form}-count", "w"))
    for document, path in sources.items():
        without_digest = dict(selected)
        del without_digest[document.removesuffix("_DOCUMENT") + "_SHA256"]
        json.dump({"indexer": without_digest}, open(f"{sys.argv[2]}/indexer-{form}-{document}-no-digest.json", "w"))
for document, path in sources.items():
    selected = dict(values, **{d: f"sentinel-{d}" for d in sources})
    for invalid in ("both", "neither"):
        bad = dict(selected)
        if invalid == "both":
            bad[path] = "/run/launchpad/indexer/document.json"
        else:
            del bad[document]
        json.dump({"indexer": bad}, open(f"{sys.argv[2]}/indexer-{document}-{invalid}.json", "w"))
    stale = json.load(open(f"{sys.argv[2]}/indexer-file-replay.json"))
    next(e for e in stale if e["op"] == "Variables")["data"]["variables"] = {document: "sentinel-current-document"}
    json.dump(stale, open(f"{sys.argv[2]}/indexer-{document}-stale.json", "w"))
PY
	local form mode name alternate code invalid
	for form in document file; do
		for mode in plan apply; do
			python3 "$tool" proj-test production --manifest "$tmp/indexer.json" \
				--replay "$tmp/indexer-$form-replay.json" --values "$tmp/indexer-$form-values.json" \
				"--$mode" >"$tmp/indexer-$form-$mode.out" 2>&1 ||
				fail "indexer $form $mode exited $?: $(cat "$tmp/indexer-$form-$mode.out")"
			for name in EXPECTED_SCHEMA INDEXER_RECEIPT_CONFIG INDEXER_LIFECYCLE_CONFIG DB_BUDGET DELIVERY_CONFIG; do
				if [ "$form" = document ]; then alternate=FILE; else alternate=DOCUMENT; fi
				grep -q "^  missing ${name}_${form^^} = " "$tmp/indexer-$form-$mode.out" ||
					fail "indexer $form $mode omitted $name"
				grep -q "${name}_${alternate}" "$tmp/indexer-$form-$mode.out" &&
					fail "indexer $form $mode includes the alternate source"
				grep -q "^  missing ${name}_SHA256 = " "$tmp/indexer-$form-$mode.out" ||
					fail "indexer $form $mode omitted the digest"
			done
		done
		grep -q "^  applied $(cat "$tmp/indexer-$form-count") variables$" "$tmp/indexer-$form-apply.out" ||
			fail "indexer $form apply did not upsert the selected variables"
		for name in EXPECTED_SCHEMA INDEXER_RECEIPT_CONFIG INDEXER_LIFECYCLE_CONFIG DB_BUDGET DELIVERY_CONFIG; do
			set +e
			python3 "$tool" proj-test production --manifest "$tmp/indexer.json" \
				--replay "$tmp/indexer-$form-replay.json" \
				--values "$tmp/indexer-$form-${name}_DOCUMENT-no-digest.json" --apply \
				>"$tmp/indexer-no-digest.out" 2>&1
			code=$?
			set -e
			[ "$code" -eq 2 ] || fail "indexer $form without digest exited $code"
			grep -q "${name}_SHA256 has no value" "$tmp/indexer-no-digest.out" || fail "indexer missing digest not named"
		done
	done
	for name in EXPECTED_SCHEMA INDEXER_RECEIPT_CONFIG INDEXER_LIFECYCLE_CONFIG DB_BUDGET DELIVERY_CONFIG; do
		for invalid in both neither; do
			for mode in plan apply; do
				set +e
				python3 "$tool" proj-test production --manifest "$tmp/indexer.json" \
					--replay "$tmp/indexer-document-replay.json" \
					--values "$tmp/indexer-${name}_DOCUMENT-$invalid.json" "--$mode" \
					>"$tmp/indexer-refused.out" 2>&1
				code=$?
				set -e
				if [ "$mode" = plan ]; then
					[ "$code" -eq 3 ] || fail "indexer $invalid $mode exited $code: $(cat "$tmp/indexer-refused.out")"
				else
					[ "$code" -eq 2 ] || fail "indexer $invalid $mode exited $code: $(cat "$tmp/indexer-refused.out")"
				fi
				grep -q "${name}_DOCUMENT.*${name}_FILE.*exactly one\|${name}_DOCUMENT.*exactly one.*${name}_FILE" \
					"$tmp/indexer-refused.out" || fail "indexer $invalid $mode did not name both sources"
				grep -q 'applied' "$tmp/indexer-refused.out" && fail "indexer invalid sources were applied"
			done
		done
		for mode in plan apply; do
			set +e
			python3 "$tool" proj-test production --manifest "$tmp/indexer.json" \
				--replay "$tmp/indexer-${name}_DOCUMENT-stale.json" \
				--values "$tmp/indexer-file-values.json" "--$mode" >"$tmp/indexer-stale.out" 2>&1
			code=$?
			set -e
			if [ "$mode" = plan ]; then
				[ "$code" -eq 3 ] || fail "indexer existing alternate source $mode exited $code"
			else
				[ "$code" -eq 2 ] || fail "indexer existing alternate source $mode exited $code"
			fi
			grep -q "${name}_DOCUMENT and ${name}_FILE are both set" "$tmp/indexer-stale.out" ||
				fail "indexer existing alternate source not named"
			grep -q 'applied' "$tmp/indexer-stale.out" && fail "indexer applied both sources"
		done
	done
	echo PASS indexer-document-sources
}
indexer_document_sources

api_readiness_documents() {
	python3 - "$root/deploy/launchpad/railway/variables.json" <<'PY'
import json, sys

api = json.load(open(sys.argv[1]))["api"]
api_sockets = json.load(open(sys.argv[1]))["api-sockets"]
assert api["fixed"]["API_PRIVATE_LISTEN_ADDRESS"] == "[::]:8443"
assert "API_PRIVATE_LISTEN_ADDRESS" not in api_sockets["fixed"]
assert api["documentSources"] == {
    "EXPECTED_SCHEMA_DOCUMENT": "EXPECTED_SCHEMA_FILE",
    "NETWORK_AUTHORITY_CONFIG_DOCUMENT": "NETWORK_AUTHORITY_CONFIG_FILE",
    "ACCOUNTING_READ_CONFIG_DOCUMENT": "ACCOUNTING_READ_CONFIG_FILE",
    "DB_BUDGET_DOCUMENT": "DB_BUDGET_FILE",
    "DISCOVERY_RUNTIME_CONFIG_DOCUMENT": "DISCOVERY_RUNTIME_CONFIG_FILE",
    "STREAM_RELAY_CONFIG_DOCUMENT": "STREAM_RELAY_CONFIG_FILE",
}
required = set(api["required"])
declared = required | set(api["documentSources"]) | set(api["documentSources"].values())
for name in (
    "NETWORK_AUTHORITY_CONFIG_DOCUMENT", "NETWORK_AUTHORITY_CONFIG_SHA256",
    "STREAM_RELAY_CONFIG_DOCUMENT", "STREAM_RELAY_CONFIG_SHA256",
    "DISCOVERY_RUNTIME_CONFIG_DOCUMENT", "DISCOVERY_RUNTIME_CONFIG_SHA256",
    "NETWORK_RPC_URL",
):
    assert name in declared, name
assert "NETWORK_AUTHORITY_CONFIG_SHA256" in required
assert "DISCOVERY_RUNTIME_CONFIG_DOCUMENT" in api["secrets"]
assert set(api["secrets"]) & set(api["fixed"]) == {"SECRETS_PROVIDER"}
PY
	echo PASS api-readiness-documents
}
api_readiness_documents

private_listen_address_contract() {
	python3 - "$root/deploy/launchpad/railway/variables.json" "$tmp" "$replay" "$tool" <<'PY'
import ipaddress, json, os, subprocess, sys
from pathlib import Path

contract = json.load(open(sys.argv[1]))
directory = Path(sys.argv[2])
services = ("api", "ranking", "user", "social", "metadata")
manifest = {service: contract[service] for service in services}
values = {}
project = next(entry for entry in json.load(open(sys.argv[3])) if entry["op"] == "Project")
project["data"]["project"]["services"]["edges"] = [
    {"node": {"id": f"svc-{service}", "name": service}} for service in services
]
replay = [project]
expected = {}
for service, entry in manifest.items():
    key = f"{service.upper()}_PRIVATE_LISTEN_ADDRESS"
    address = entry["fixed"][key]
    assert address == "[::]:8443", service
    host, port = address.rsplit(":", 1)
    assert ipaddress.IPv6Address(host.strip("[]")).is_unspecified, service
    assert port == "8443", service
    values[service] = {name: f"{service}-value-{index:02d}"
                       for index, name in enumerate(entry["required"])
                       if name not in entry["fixed"]}
    for document in entry.get("documentSources", {}):
        values[service][document] = "e30="
    expected[service] = {**entry["defaults"], **values[service], **entry["fixed"]}
    replay.extend([
        {"op": "Variables", "variables": {"p": "proj-test", "e": "env-prod", "s": f"svc-{service}"},
         "data": {"variables": {}}},
        {"op": "VariablesUpsert", "variables": {"i": {
            "projectId": "proj-test", "environmentId": "env-prod", "serviceId": f"svc-{service}",
            "variables": expected[service], "skipDeploys": True}},
         "data": {"variableCollectionUpsert": True}},
    ])
for name, data in (("manifest", manifest), ("values", values), ("replay", replay)):
    (directory / f"private-listeners-{name}.json").write_text(json.dumps(data))
result = subprocess.run([
    sys.executable, sys.argv[4], "proj-test", "production", "--apply",
    "--manifest", str(directory / "private-listeners-manifest.json"),
    "--values", str(directory / "private-listeners-values.json"),
    "--replay", str(directory / "private-listeners-replay.json"),
], capture_output=True, text=True, env=os.environ)
assert result.returncode == 0, result.stdout + result.stderr
applied = set()
planned = {}
service = None
for line in result.stdout.splitlines():
    if line.startswith("service "):
        service = line.split()[1]
    elif line.startswith("  missing ") and "_PRIVATE_LISTEN_ADDRESS = " in line:
        key, address = line.removeprefix("  missing ").split(" = ", 1)
        assert key == f"{service.upper()}_PRIVATE_LISTEN_ADDRESS", service
        assert address == contract[service]["fixed"][key], service
        host, port = address.rsplit(":", 1)
        assert ipaddress.IPv6Address(host.strip("[]")).is_unspecified, service
        assert port == "8443", service
        planned[service] = address
    elif line.startswith("  applied "):
        assert line == f"  applied {len(expected[service])} variables", service
        applied.add(service)
assert set(planned) == set(services), planned
assert applied == set(services), applied
PY
	echo PASS private-listen-address-contract
}
private_listen_address_contract

optional_settings_and_defaults() {
	python3 - "$tmp" "$replay" "$tool" <<'PY'
import copy, json, os, subprocess, sys
from pathlib import Path

directory = Path(sys.argv[1])
manifest_path = directory / "manifest.json"
charts = json.loads(manifest_path.read_text())["charts"]
values = json.loads((directory / "values.json").read_text())["charts"]
values["RPC_URL"] = "https://rpc.sentinel.invalid"
reads = json.load(open(sys.argv[2]))
environment = dict(os.environ)
for name in set(charts["optional"]) | set(charts["defaults"]):
    environment.pop(name, None)
base_want = {**charts["defaults"], **values, **charts["fixed"]}
ids = {"projectId": "proj-test", "environmentId": "env-prod", "serviceId": "svc-charts"}


def exercise(label, supplied, additions, env_values=None, from_env=False, existing=None):
    replay = copy.deepcopy(reads)
    have = next(entry for entry in replay if entry["op"] == "Variables")["data"]["variables"]
    have.update(existing or {})
    want = {**base_want, **additions}
    upsert = {name: value for name, value in want.items() if have.get(name) != value}
    replay.append({"op": "VariablesUpsert", "variables": {"i": dict(ids, variables=upsert, skipDeploys=True)},
                   "data": {"variableCollectionUpsert": True}})
    replay_path = directory / f"optional-{label}-replay.json"
    values_path = directory / f"optional-{label}-values.json"
    replay_path.write_text(json.dumps(replay))
    values_path.write_text(json.dumps(supplied))
    outputs = {}
    for mode in ("plan", "apply"):
        command = [sys.executable, sys.argv[3], "proj-test", "production", f"--{mode}",
                   "--manifest", str(manifest_path), "--values", str(values_path), "--replay", str(replay_path)]
        if from_env:
            command.append("--from-env")
        result = subprocess.run(command, capture_output=True, text=True,
                                env={**environment, **(env_values or {})})
        output = result.stdout + result.stderr
        assert result.returncode == 0, f"{label} {mode}: {output}"
        assert "unresolved" not in output, output
        if mode == "apply":
            assert f"  applied {len(upsert)} variables" in output, output
        outputs[mode] = output
    print(f"PASS optional-{label}")
    return outputs


token = "sentinel-optional-token-583a"
scoped = {**values, "CORS_ALLOWED_ORIGINS": "https://scoped.example.invalid",
          "CHART_OPERATOR_TOKEN": token, "LOG_LEVEL": "debug", "PORT": "9999"}
outputs = exercise("values", {"charts": scoped, "CORS_ALLOWED_ORIGINS": "https://flat.example.invalid"},
                   {"CORS_ALLOWED_ORIGINS": scoped["CORS_ALLOWED_ORIGINS"],
                    "CHART_OPERATOR_TOKEN": token, "LOG_LEVEL": "debug"},
                   env_values={"CORS_ALLOWED_ORIGINS": "https://env.example.invalid", "LOG_LEVEL": "warn"},
                   from_env=True)
for output in outputs.values():
    assert "  missing CHART_OPERATOR_TOKEN = ****" in output, output
    assert "  missing CORS_ALLOWED_ORIGINS = https://scoped.example.invalid" in output, output
    assert "  missing LOG_LEVEL = debug" in output, output
    assert "  missing PORT = 8080" in output, output
    assert token not in output, output
    assert "https://flat.example.invalid" not in output, output
    assert "https://env.example.invalid" not in output, output

outputs = exercise("global", {"charts": values, "CORS_ALLOWED_ORIGINS": "https://flat.example.invalid"},
                   {"CORS_ALLOWED_ORIGINS": "https://flat.example.invalid"},
                   env_values={"CORS_ALLOWED_ORIGINS": "https://env.example.invalid"}, from_env=True)
for output in outputs.values():
    assert "  missing CORS_ALLOWED_ORIGINS = https://flat.example.invalid" in output, output

env_settings = {"CORS_ALLOWED_ORIGINS": "https://env.example.invalid", "CHART_OPERATOR_TOKEN": token,
                "LOG_LEVEL": "warn"}
outputs = exercise("environment", {"charts": values}, env_settings, env_values=env_settings, from_env=True)
for output in outputs.values():
    assert "  missing CORS_ALLOWED_ORIGINS = https://env.example.invalid" in output, output
    assert "  missing CHART_OPERATOR_TOKEN = ****" in output, output
    assert "  missing LOG_LEVEL = warn" in output, output
    assert token not in output, output

for existing in ({}, {"CORS_ALLOWED_ORIGINS": "https://existing.example.invalid", "CHART_OPERATOR_TOKEN": token}):
    label = "kept" if existing else "absent"
    outputs = exercise(label, {"charts": values}, {}, env_values=env_settings, existing=existing)
    for output in outputs.values():
        assert "CORS_ALLOWED_ORIGINS" not in output, output
        assert "CHART_OPERATOR_TOKEN" not in output, output
        assert token not in output, output
        assert "  missing LOG_LEVEL = info" in output, output
        assert "  missing DEPLOYMENT_REALM = production" in output, output
        assert "  missing WS_MAX_CONNECTIONS = 10000" in output, output

outputs = exercise("default", {"charts": values}, {}, existing={"LOG_LEVEL": "debug"})
for output in outputs.values():
    assert "  differs LOG_LEVEL debug -> info" in output, output

outputs = exercise("empty", {"charts": {**values, "CORS_ALLOWED_ORIGINS": "", "LOG_LEVEL": ""}},
                   {"CORS_ALLOWED_ORIGINS": "", "LOG_LEVEL": "", "CHART_OPERATOR_TOKEN": token},
                   env_values=env_settings, from_env=True)
for output in outputs.values():
    assert "  missing CORS_ALLOWED_ORIGINS = \n" in output, output
    assert "  missing LOG_LEVEL = \n" in output, output
    assert "  missing CHART_OPERATOR_TOKEN = ****" in output, output
    assert token not in output, output

for label, supplied, message in (
    ("unknown-scoped", {"charts": {**values, "UNDECLARED_SETTING": "sentinel-unknown-value"}},
     "charts: unknown variable UNDECLARED_SETTING"),
    ("unknown-global", {"charts": values, "UNDECLARED_SETTING": "sentinel-unknown-value"},
     "--values: unknown variable UNDECLARED_SETTING"),
    ("unknown-service", {"charts": values, "ghost": {}}, "--values: unknown service ghost"),
    ("optional-type", {"charts": {**values, "CORS_ALLOWED_ORIGINS": False}},
     "charts: CORS_ALLOWED_ORIGINS must be a string"),
    ("optional-newline", {"charts": {**values, "CORS_ALLOWED_ORIGINS": "first\nsecond"}},
     "CORS_ALLOWED_ORIGINS contains a newline"),
):
    path = directory / f"optional-{label}-values.json"
    path.write_text(json.dumps(supplied))
    for mode in ("plan", "apply"):
        result = subprocess.run([
            sys.executable, sys.argv[3], "proj-test", "production", f"--{mode}",
            "--manifest", str(manifest_path), "--values", str(path), "--replay", sys.argv[2],
        ], capture_output=True, text=True, env=environment)
        output = result.stdout + result.stderr
        expected_code = 3 if label == "optional-newline" and mode == "plan" else 2
        assert result.returncode == expected_code, f"{label} {mode}: {output}"
        assert message in output, output
        assert "sentinel-unknown-value" not in output, output
        assert "applied" not in output, output
    print(f"PASS optional-{label}")
PY
	echo PASS optional-settings-and-defaults
}
optional_settings_and_defaults
echo PASS railway-apply-variables
