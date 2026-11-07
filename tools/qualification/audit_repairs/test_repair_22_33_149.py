"""Offline source and committed-asset proofs; no Node, shell or service subprocesses.

The build gate supplies bash -n. Runtime concurrency, crash recovery and browser
registration are review-only, as distinguished in repair_22_33_149.kvx.
"""

import ast
import json
from pathlib import Path
import re
import shlex
import struct
import tomllib


ROOT = Path(__file__).resolve().parents[3]
APP = ROOT / "apps/wallet"
RECORD = Path(__file__).with_name("repair_22_33_149.kvx")


def source(relative):
    return (APP / relative).read_text(encoding="utf-8")


def contains(text, *facts):
    compact = re.sub(r"\s+", "", text)
    for fact in facts:
        assert re.sub(r"\s+", "", fact) in compact, fact


def between(text, start, end):
    assert text.count(start) == 1, start
    return text.split(start, 1)[1].split(end, 1)[0]


def contract():
    caching = source("src/pwa/caching.ts")
    mount = re.search(r"export const PWA_BASE_PATH = '([^']+)';", caching).group(1)
    urls = {}
    for name in ("SERVICE_WORKER_URL", "SHELL_URL"):
        value = re.search(r"export const " + name + r" = `([^`]+)`;", caching).group(1)
        urls[name] = value.replace("${PWA_BASE_PATH}", mount)
    return mount, urls


def test_release_basepath_guarded_commands_and_failure_propagation():
    shell = source("scripts/release-check.sh")
    assert shell.startswith("#!/bin/sh\nset -eu\n")
    for variable, mode, label in (
        ("manifest_output", "manifest", "manifest"),
        ("registration_output", "registration", "service worker registered"),
    ):
        block = between(shell, f"if {variable}=$(", "\nfi")
        command, branches = block.split("); then\n", 1)
        assert shlex.split(command) == ["node", "scripts/check-pwa-release.mjs", mode, "2>&1"]
        assert branches == (
            f'  step "{label}" pass "${variable}"\n'
            f'else\n  step "{label}" fail "${variable}"'
        )
    assert 'if [ "$result" != "pass" ]; then\n    failed=1\n  fi' in shell
    assert 'if [ "$failed" -ne 0 ]; then\n  echo "release-check: fail"\n  exit 1\nfi' in shell
    assert 'SERVICE_WORKER_URL = \'/sw.js\'' not in shell
    assert "public${icon.src}" not in shell


def test_release_basepath_uses_production_exports_and_build_loader():
    mount, urls = contract()
    assert mount == "/wallet"
    config = source("next.config.mjs")
    assert re.search(r"basePath:\s*'([^']+)'", config).group(1) == mount
    assert urls == {"SERVICE_WORKER_URL": "/wallet/sw.js", "SHELL_URL": "/wallet/"}
    helper = source("scripts/check-pwa-release.mjs")
    contains(
        helper,
        "import { createServer } from 'vite'",
        "const contract = await loader.ssrLoadModule('/src/pwa/caching.ts')",
        "checkRegistration(contract)",
        "finally { await loader.close(); }",
        "main().catch((error) => { console.error(error.message); process.exitCode = 1; })",
    )
    package = json.loads(source("package.json"))
    assert "vite" in package["devDependencies"]
    assert package["scripts"]["release:check"] == "sh scripts/release-check.sh"
    contains(source("scripts/build-pwa.mjs"), "await loader.ssrLoadModule('/src/pwa/build.ts')")


def test_release_basepath_manifest_producer_and_committed_png_assets():
    mount, urls = contract()
    manifest = json.loads(source("public/manifest.json"))
    producer = source("src/pwa/manifest.ts")
    for key in ("id", "start_url", "scope"):
        assert manifest[key] == urls["SHELL_URL"]
        contains(producer, f"{key}: '{urls['SHELL_URL']}'")
    assert manifest["display"] == "standalone"
    for key in ("name", "short_name", "background_color", "theme_color"):
        assert isinstance(manifest[key], str) and manifest[key]
    assert {(icon["sizes"], icon["purpose"]) for icon in manifest["icons"]} == {
        ("192x192", "any"), ("512x512", "any"),
        ("192x192", "maskable"), ("512x512", "maskable"),
    }
    declared = re.findall(r"icon\('([^']+)', (\d+), '(any|maskable)'\)", producer)
    assert len(declared) == len(manifest["icons"]) == 4
    for icon, (filename, size, purpose) in zip(manifest["icons"], declared):
        assert icon == {
            "src": f"{mount}/icons/app/{filename}", "sizes": f"{size}x{size}",
            "type": "image/png", "purpose": purpose,
        }
        asset = APP / "public" / icon["src"].removeprefix(mount + "/")
        data = asset.read_bytes()
        assert data[:8] == b"\x89PNG\r\n\x1a\n"
        assert struct.unpack(">II", data[16:24]) == (int(size), int(size))
    contains(producer, "src: `/wallet/icons/app/${file}`")
    contains(source("src/pwa/build.ts"), "const manifest = serializeManifest()", "writeFileSync(manifestPath, manifest)")


def test_release_basepath_asset_resolution_and_validation_guards():
    helper = between(source("scripts/check-pwa-release.mjs"), "export function checkManifest", "export function checkRegistration")
    contains(
        helper,
        "typeof icon.src !== 'string' || !icon.src.startsWith(`${PWA_BASE_PATH}/`)",
        "const relative = icon.src.slice(PWA_BASE_PATH.length + 1)",
        "const file = path.resolve(publicDir, relative)",
        "if (!file.startsWith(`${publicDir}${path.sep}`))",
        "bytes = readFileSync(file)",
        "problems.push(`unreadable ${file}`); continue;",
        "bytes.length < 24 || !bytes.subarray(0, 8).equals(Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]))",
        "bytes.readUInt32BE(16)", "bytes.readUInt32BE(20)",
        "if (size !== icon.sizes) problems.push",
        "if (manifest[key] !== SHELL_URL)",
        "if (problems.length > 0) throw new Error(problems.join('; '))",
    )
    assert helper.index("const relative =") < helper.index("const file =") < helper.index("bytes = readFileSync")
    assert helper.index("bytes.length < 24") < helper.index("bytes.readUInt32BE(16)")
    for size in ("192x192", "512x512"):
        for purpose in ("any", "maskable"):
            contains(helper, f"['{size}', '{purpose}']")


def test_release_basepath_registration_checks_actual_producer_and_consumer():
    helper = source("scripts/check-pwa-release.mjs")
    registration = source("src/pwa/register.ts")
    _, urls = contract()
    for name in ("importContract", "registerContract"):
        pattern = re.search(r"const " + name + r"\s*=\s*/(.+)/;", helper).group(1)
        match = re.search(pattern.replace(r"\/", "/"), registration)
        assert match, name
        if name == "registerContract":
            assert match.group(1) == urls["SHELL_URL"]
    contains(
        helper,
        "!importContract.test(registration) || registration.match(registerContract)?.[1] !== SHELL_URL",
        "SHELL_URL !== `${PWA_BASE_PATH}/` || SERVICE_WORKER_URL !== `${PWA_BASE_PATH}/sw.js`",
        "throw new Error('Worker producer or provider registration is missing')",
    )
    assert "registerServiceWorker(container," in source("src/providers/PWAProvider.tsx")
    contains(source("src/pwa/build.ts"), "fileName: () => 'sw.js'", "workerPath: path.join(options.outDir, 'sw.js')")


def test_campaign_overwrite_uses_same_production_transactional_store():
    seed = source("scripts/seed-sid-campaign.js")
    contains(
        seed,
        "const { pushStore, parseCampaign } = await loader.ssrLoadModule('/src/server/push-store.ts')",
        "await seedSidCampaign(pushStore, parseCampaign)",
        "root: path.resolve(__dirname, '..')",
        "finally { await loader.close(); }",
    )
    for forbidden in ("writeFile", "readFile", "mkdir", "campaigns.json", ".push-data"):
        assert forbidden not in seed
    store = source("src/server/push-store.ts")
    contains(
        store,
        "const DATA_DIR = process.env.PUSH_DATA_DIR ?? '/data/paxport/push'",
        "export const pushStore = createPushStore(DATA_DIR)",
        "filePath: path.join(dataDirectory, 'campaigns.json')",
        "parse: envelopeParser(parseCampaign)",
    )
    contains(source("src/server/push-service.ts"), "pushStore,", "from './push-store'", "const pending = await getPendingCampaigns()")
    contains(store, "async function getPendingCampaigns()", "return (await campaigns.read()).records.filter")


def test_campaign_overwrite_validates_before_any_mutation():
    seed = between(source("scripts/seed-sid-campaign.js"), "async function seedSidCampaign", "async function main")
    assert seed.index("parseCampaign(input)") < seed.index("try {") < seed.index("await store.addCampaign(campaign)")
    parser = between(source("src/server/push-store.ts"), "export function parseCampaign", "function envelopeParser")
    contains(
        parser,
        "if (!isRecord(input)) throw new TypeError('Campaign is invalid')",
        "id: boundedString(input.id, 'id', 1, 128)",
        "title: boundedString(input.title, 'title', 1, 120)",
        "body: boundedString(input.body, 'body', 1, 500)",
        "scheduledAt: input.scheduledAt === undefined ? undefined : parseTimestamp(input.scheduledAt, 'scheduledAt')",
    )
    store = source("src/server/push-store.ts")
    add = between(store, "async function addCampaign", "async function getCampaign")
    assert add.index("parseCampaign(input)") < add.index("await campaigns.update")
    contains(store, "if (typeof input !== 'string' || input.length < minimum || input.length > maximum)", "throw new TypeError(`${name} is invalid`)")


def test_campaign_overwrite_failure_cannot_replace_corrupt_state():
    seed = source("scripts/seed-sid-campaign.js")
    contains(seed, "if (!(error instanceof Error) || error.message !== 'Campaign already exists') throw error", "process.exitCode = 1")
    atomic = source("src/server/atomic-json-store.ts")
    update = between(atomic, "async update(mutator:", "async recoverFromBackup")
    order = [update.index(fact) for fact in (
        "await this.#readUnlocked()", "await mutator(current)",
        "this.#parse(next)", "await this.#writeUnlocked(next)",
    )]
    assert order == sorted(order)
    read = between(atomic, "async #readUnlocked()", "async #writeUnlocked")
    contains(read, "await this.#assertAvailable()", "JSON.parse(raw)", "if (error instanceof DurableStoreCorruptError) throw error", "await this.#quarantine(); throw new DurableStoreCorruptError(this.#filePath)")
    assert "if (error instanceof Error && 'code' in error && error.code === 'ENOENT')" in read
    quarantine = between(atomic, "async #quarantine()", "async #acquireFileLock")
    contains(quarantine, "await this.#assertOwnership()", "await rename(this.#filePath, target)", "await this.#syncDirectory()")
    assert "unlink(this.#filePath)" not in quarantine


def test_campaign_overwrite_concurrent_updates_share_queue_and_file_lock():
    atomic = source("src/server/atomic-json-store.ts")
    contains(atomic, "const queues = new Map<string, Promise<unknown>>()", "const previous = queues.get(key) ?? Promise.resolve()", "const current = previous.then(operation, operation)")
    update = between(atomic, "async update(mutator:", "async recoverFromBackup")
    contains(update, "return enqueue(this.#filePath, () => this.#withFileLock(async () =>")
    lock = between(atomic, "async #withFileLock", "async #assertAvailable")
    assert lock.index("await this.#acquireFileLock()") < lock.index("await operation()") < lock.index("await release()")
    acquire = between(atomic, "async #acquireFileLock", "function hasCode")
    contains(acquire, "await rename(candidate, this.#lockPath)", "if (!(await ownerIsAlive(owner)))", "throw new Error('Timed out acquiring durable store lock')")
    add = between(source("src/server/push-store.ts"), "async function addCampaign", "async function getCampaign")
    contains(add, "await campaigns.update((store) => {", "records: [...store.records, campaign]")
    assert add.index("store.records.some") < add.index("records: [...store.records, campaign]")
    publication = between(atomic, "async #writeUnlocked", "async #syncedTemporary")
    contains(publication, "const temporary = await this.#syncedTemporary(serialized)", "await rename(temporary, this.#filePath); await this.#syncDirectory()")


def test_campaign_overwrite_repeated_seed_preserves_delivery_state_and_conflicts_refuse():
    seed = between(source("scripts/seed-sid-campaign.js"), "async function seedSidCampaign", "async function main")
    add = between(source("src/server/push-store.ts"), "async function addCampaign", "async function getCampaign")
    contains(add, "if (store.records.some((item) => item.id === campaign.id)) { throw new Error('Campaign already exists'); }")
    contains(seed, "if (!(error instanceof Error) || error.message !== 'Campaign already exists') throw error", "const existing = await store.getCampaign(campaign.id)", "if (!existing || fields.some((key) => JSON.stringify(existing[key]) !== JSON.stringify(campaign[key])))", "throw new Error('Campaign id belongs to a different announcement')", "return 'existing'")
    duplicate = seed.split("catch (error)", 1)[1]
    assert "addCampaign" not in duplicate
    assert "update(" not in duplicate
    fields = re.search(r"const fields = (\[[^;]+\]);", seed).group(1)
    assert set(ast.literal_eval(fields)) == {
        "id", "title", "body", "url", "icon", "image", "tag",
        "scheduledAt", "targetTags", "targetAddresses",
    }
    for state in ("sentAt", "sentCount", "failedCount", "claim", "recipients"):
        assert state not in fields
    assert "module.exports = { SID_CAMPAIGN, seedSidCampaign }" in source("scripts/seed-sid-campaign.js")


def test_repair_record_maps_findings_to_executed_source_proofs():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    audit = tomllib.loads((ROOT / "spec/layerx-merge/programme/audit/records/22.30.116.kvx").read_text())
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text()).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    mapped = set()
    for finding in ("release-basepath", "campaign-overwrite"):
        entry = record["finding"][finding]
        original = audit["finding"][finding]
        for key in ("file", "class", "severity"):
            assert entry[key] == original[key]
        assert entry["audit"] == "22.30.116"
        assert entry["tests"] and set(entry["tests"]) <= declared
        mapped.update(entry["tests"])
    assert mapped == declared - {"test_repair_record_maps_findings_to_executed_source_proofs"}
    task = record["task"]["22"]["33"]["149"]
    assert task["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_149.py"
    assert task["build_cmd"] == "bash -n apps/wallet/scripts/release-check.sh"
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert task["wiring_requests"] == []
    assert "No pre-change verification was run" in task["baseline"]
    assert "not executed" in task["review_only"]
    assert "same candidate revision" in task["evidence"]
