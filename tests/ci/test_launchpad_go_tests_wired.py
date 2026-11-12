"""Keep launchpad Go CI aligned with the production module inventory."""

import importlib.util
import json
from pathlib import Path
import re
import subprocess
import tomllib

import pytest
import yaml


ROOT = Path(__file__).resolve().parents[2]
WORKFLOW = ".github/workflows/launchpad-test.yml"
TEST = "tests/ci/test_launchpad_go_tests_wired.py"
RUNNER = "tests/ci/data/launchpad_go_tests.py"
BASELINE = "tests/ci/data/launchpad_go_test_baseline.json"


def launchpad_go_workflow():
    return yaml.load((ROOT / WORKFLOW).read_text(), Loader=yaml.BaseLoader)


def launchpad_go_modules():
    # Nested testdata modules are real producer fixtures invoked by their owning
    # suites, not independent production modules.
    return {
        manifest.parent.relative_to(ROOT).as_posix()
        for base in ("services/launchpad", "sdk/launchpad")
        for manifest in (ROOT / base).rglob("go.mod")
        if not {"testdata", "node_modules"}.intersection(
            manifest.relative_to(ROOT).parts
        )
    }


def test_launchpad_workflow_has_go_job():
    workflow = launchpad_go_workflow()
    for event in ("pull_request", "push"):
        trigger = workflow["on"][event]
        assert {"services/launchpad/**", "sdk/launchpad/**", WORKFLOW, TEST,
                "tests/ci/data/**", "services/launchpad/Makefile"} <= set(
            trigger["paths"]
        )
        assert "paths-ignore" not in trigger
    assert workflow["on"]["push"]["branches"] == ["main"]
    assert workflow["permissions"] == {"contents": "read"}

    job = workflow["jobs"]["launchpad-go-test"]
    runner = job["runs-on"]
    assert runner == (
        "${{ (github.event_name == 'pull_request' && "
        "github.event.pull_request.head.repo.full_name != github.repository) "
        "&& 'ubuntu-24.04' || vars.CI_LINUX_RUNNER || 'ubuntu-24.04' }}"
    )
    fallback = re.search(r"vars\.CI_LINUX_RUNNER\s*\|\|\s*(['\"])(.*?)\1", runner)
    assert fallback is not None
    assert fallback.group(2) == "ubuntu-24.04"
    assert job["strategy"]["fail-fast"] == "false"
    assert job["defaults"]["run"]["working-directory"] == "${{ matrix.module }}"
    assert job["env"]["GOWORK"] == "off"
    assert job["env"]["GOTOOLCHAIN"] == "go1.27.3"
    assert job["env"]["GOFLAGS"] == "-timeout=15m"
    assert job["env"]["CLAMAV_HOST"] == "127.0.0.1"
    assert job["env"]["CLAMAV_PORT"] == "23310"
    assert job["env"]["METADATA_TEST_CLAMAV_ADDR"] == "127.0.0.1:23310"
    assert job["env"]["USER_TEST_CLAMAV_ADDR"] == "127.0.0.1:23310"
    scanner = next(step for step in job["steps"]
                   if step.get("name") == "Start malware scanner fixture")
    assert "clamav/clamav@sha256:45fe5149fbee5a46509c9848ef20d0c354b7b2323c9369f285939a37cf33422a" in scanner["run"]
    assert "docker exec launchpad-clamav clamdcheck.sh" in scanner["run"]
    assert scanner["run"].rstrip().endswith("exit 1")
    fixture_dsn = (
        "postgres://kindle_fixture:kindle_fixture_password@127.0.0.1:25432/"
        "kindle_fixture?sslmode=disable"
    )
    for name in ("L34_POSTGRES_DSN", "DELIVERY_TEST_DSN", "BROKER_EPOCH_TEST_DSN"):
        assert job["env"][name] == fixture_dsn, name
    assert job["env"]["L07_POSTGRES_DSN"] == fixture_dsn.replace(
        "/kindle_fixture?", "/kindle_test?"
    )
    assert re.fullmatch(r"kd_[a-f0-9]{24}", job["env"]["L09_NAMESPACE"])
    assert re.fullmatch(r"kd_[a-f0-9]{24}", job["env"]["DELIVERY_TEST_SCHEMA"])
    assert job["env"]["L09_REDIS_URL"] != job["env"]["SHARED_TEST_REDIS_URL"]
    postgres = job["services"]["postgres"]
    assert postgres["ports"] == ["25432:5432"]
    assert postgres["env"]["POSTGRES_DB"] == "kindle_fixture"
    assert postgres["env"]["POSTGRES_USER"] == "kindle_fixture"
    assert "if" not in job
    assert "continue-on-error" not in job
    steps = job["steps"]
    for step in steps:
        assert "if" not in step
        assert "continue-on-error" not in step
        if "uses" in step:
            assert re.fullmatch(r"[\w-]+/[\w-]+@[0-9a-f]{40}", step["uses"])
    (checkout,) = [
        step for step in steps
        if step.get("uses", "").startswith("actions/checkout@")
    ]
    (setup,) = [
        step for step in steps
        if step.get("uses", "").startswith("actions/setup-go@")
    ]
    (foundry,) = [
        step for step in steps
        if step.get("uses", "").startswith("foundry-rs/foundry-toolchain@")
    ]
    assert foundry["uses"] == (
        "foundry-rs/foundry-toolchain@908c540300062bd5a7e473851cdb4282204cee09"
    )
    assert foundry["with"] == {"version": "v1.8.1", "cache": "false"}
    assert checkout["with"]["persist-credentials"] == "false"
    assert setup["with"]["go-version"] == "1.27.3"
    assert "sdk/launchpad/**/go.sum" in setup["with"]["cache-dependency-path"]
    assert "services/launchpad/**/go.sum" in setup["with"]["cache-dependency-path"]
    (test,) = [step for step in steps if step.get("run") == (
            f'python3 "$GITHUB_WORKSPACE/{RUNNER}" --module .'
        )]
    assert steps.index(checkout) < steps.index(setup) < steps.index(test)
    assert steps.index(checkout) < steps.index(foundry) < steps.index(test)
    assert "working-directory" not in test
    assert "GOWORK" not in test.get("env", {})

    wiring = workflow["jobs"]["launchpad-go-wiring"]
    assert "if" not in wiring
    assert "continue-on-error" not in wiring
    assert wiring["env"] == {"GOWORK": "off", "GOTOOLCHAIN": "go1.27.3"}
    (wiring_go,) = [step for step in wiring["steps"]
                    if step.get("uses", "").startswith("actions/setup-go@")]
    assert wiring_go["with"]["go-version"] == "1.27.3"
    (run,) = [step for step in wiring["steps"] if step.get("run", "").startswith(
        "python3 -m pytest "
    )]
    assert run["run"] == f"python3 -m pytest -q {TEST}"
    assert wiring["steps"].index(wiring_go) < wiring["steps"].index(run)
    assert "if" not in run
    assert "continue-on-error" not in run


def test_launchpad_go_job_provides_api_services_and_contracts():
    job = launchpad_go_workflow()["jobs"]["launchpad-go-test"]
    fixture_dsn = (
        "postgres://kindle_fixture:kindle_fixture_password@127.0.0.1:25432/"
        "kindle_fixture?sslmode=disable"
    )
    assert job["env"]["CORE_API_TEST_POSTGRES_URL"] == fixture_dsn
    assert job["env"]["L11_POSTGRES_DSN"] == fixture_dsn
    assert job["env"]["CORE_API_TEST_REDIS_URL"] == "redis://127.0.0.1:26379/11"
    assert job["env"]["CORE_API_TEST_NATS_URL"] == "nats://127.0.0.1:24222"
    steps = job["steps"]
    (install_nats,) = [step for step in steps if
                       "go install github.com/nats-io/nats-server/v2@v2.11.6" in step.get("run", "")]
    assert "sudo apt-get install --no-install-recommends --yes redis-server" in install_nats["run"]
    assert job["env"]["L13_PHASE3_REDIS_SERVER"] == "/usr/bin/redis-server"
    (start_nats,) = [step for step in steps if step["name"] == "Start API NATS fixture"]
    assert 'nats-server --addr 127.0.0.1 --port 24222 --jetstream ' in start_nats["run"]
    assert '--store_dir "$RUNNER_TEMP/launchpad-nats"' in start_nats["run"]
    assert '2>&1 &' in start_nats["run"]
    assert '/dev/tcp/127.0.0.1/24222' in start_nats["run"]
    assert 'exit 1' in start_nats["run"]
    (compile_contracts,) = [step for step in steps if step.get("working-directory") ==
                          "contracts/launchpad"]
    assert compile_contracts["run"].splitlines() == [
        'npm install --global "$(node -p \'require("./package.json").packageManager\')"',
        "pnpm install --ignore-workspace --frozen-lockfile",
        "pnpm compile",
    ]
    (test,) = [step for step in steps if step["name"] == "Test Go module"]
    assert steps.index(install_nats) < steps.index(start_nats) < steps.index(test)
    assert steps.index(compile_contracts) < steps.index(test)


def test_launchpad_go_matrix_lists_every_module():
    modules = launchpad_go_modules()
    assert {"sdk/launchpad/shared", "sdk/launchpad/protocol"} <= modules
    assert "services/launchpad/security/control" in modules
    assert any(module.startswith("services/launchpad/") for module in modules)
    matrix = launchpad_go_workflow()["jobs"]["launchpad-go-test"]["strategy"]["matrix"]
    assert set(matrix) == {"module"}, "Do not exclude or override module jobs"
    entries = matrix["module"]
    assert len(entries) == len(set(entries)), "Run each module exactly once"
    assert set(entries) == modules, {
        "missing": sorted(modules - set(entries)),
        "unexpected": sorted(set(entries) - modules),
    }
    for module in entries:
        assert (ROOT / module / "go.mod").is_file(), module
        assert any((ROOT / module).rglob("*_test.go")), module


def test_launchpad_makefile_ci_target_exists():
    text = (ROOT / "services/launchpad/Makefile").read_text()
    assert re.search(r"^export GOWORK := off$", text, re.M)
    assert re.search(r"^export GOTOOLCHAIN := go1.27.3$", text, re.M)
    (ci,) = re.findall(r"^ci:\s*([^\n#]+)", text, re.M)
    assert set(ci.split()) == {
        "test", "build", "vet", "fmt-check", "lint", "race", "cover-check"
    }, "CI must run the tests and retain its existing checks"
    (phony,) = re.findall(r"^\.PHONY:\s*(.+)$", text, re.M)
    assert {"ci", "test"} <= set(phony.split())
    (assignment,) = re.findall(
        r"^MODULES := (.*?)(?=\n\s*\n)", text, re.M | re.S
    )
    entries = assignment.replace("\\\n", " ").split()
    modules = [
        (ROOT / "services/launchpad" / entry).resolve().relative_to(ROOT).as_posix()
        for entry in entries
    ]
    assert len(modules) == len(set(modules))
    assert set(modules) == launchpad_go_modules()
    (recipe,) = re.findall(r"^test:[^\n]*\n(\t[^\n]+)", text, re.M)
    assert "for m in $(MODULES)" in recipe
    assert "$(call has_pkgs,$$m)" in recipe
    assert 'python3 $(TEST_RUNNER) --module "$$m" --go "$(GO)"' in recipe
    assert '|| exit 1' in recipe
    assert "$(GO) list ./...) ||" in text, "Discovery errors must fail the gate"


def test_launchpad_lint_release_and_module_selection():
    makefile = (ROOT / "services/launchpad/Makefile").read_text()
    lint_recipe = makefile.split("\nlint:", 1)[1].split("\n\ntest race cover:", 1)[0]
    assert "golangci-lint@v2.15.0" in lint_recipe
    assert lint_recipe.count("v2.15.0") == 1
    assert 'GOBIN="$(CURDIR)/build/bin" GOTOOLCHAIN=go1.27.3 $(GO) install' in lint_recipe
    assert 'command -v "$$lint"' in lint_recipe
    assert '"$$lint" version' in lint_recipe
    assert '"$$lint" run --config "$(abspath ../../.golangci.yml)"' in lint_recipe

    selected = "security/control"
    for target in ("lint", "race", "cover", "cover-check"):
        recipe = subprocess.check_output(
            ["make", "--no-print-directory", "-n", "-C",
             str(ROOT / "services/launchpad"), target, f"MODULES={selected}"],
            text=True,
        )
        assert f"for m in {selected}; do" in recipe, target
        if target == "cover-check":
            assert f'MODULES="{selected}" RUN_MANIFEST=' in recipe


def baseline_runner():
    spec = importlib.util.spec_from_file_location("launchpad_go_tests", ROOT / RUNNER)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def test_launchpad_baseline_names_only_reviewed_existing_tests():
    runner = baseline_runner()
    entries = runner.load_baseline()
    assert len(entries) == 78
    assert {entry["package"] for entry in entries} == {
        runner.SHARED + "auth"
    } | {
        runner.API + "internal/" + name for name in ("app", "broker", "cache", "rest", "store")
    } | {
        'github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/app',
        'github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/card',
        'github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/devreads',
        'github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/production/l16',
        'github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/production/l17',
        'github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/production/l18',
        'github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/reconcile',
    }
    for entry in entries:
        if entry["package"].startswith(runner.SHARED):
            directory = ROOT / "sdk/launchpad/shared" / entry["package"].removeprefix(
                runner.SHARED
            )
        elif entry["package"].startswith(runner.API):
            directory = ROOT / "services/launchpad/api" / entry["package"].removeprefix(
                runner.API
            )
        elif entry["package"].startswith(runner.INDEXER):
            directory = ROOT / "services/launchpad/indexer" / entry["package"].removeprefix(
                runner.INDEXER
            )
        elif entry["package"] in runner.PNL_PACKAGES | runner.REMAINING_PACKAGES:
            directory = next(
                manifest.parent / entry["package"].removeprefix(
                    re.search(r"^module (.+)$", manifest.read_text(), re.M)[1]
                ).lstrip("/")
                for manifest in (ROOT / "services/launchpad").rglob("go.mod")
                if entry["package"] == re.search(r"^module (.+)$", manifest.read_text(), re.M)[1]
                or entry["package"].startswith(
                    re.search(r"^module (.+)$", manifest.read_text(), re.M)[1] + "/"
                )
            )
        else:
            directory = ROOT / "sdk/launchpad/protocol" / entry["package"].removeprefix(
                runner.PROTOCOL
            ).lstrip("/")
        declarations = [
            path for path in directory.glob("*_test.go")
            if re.search(r"^func " + entry["test"] + r"\(t \*testing.T\)",
                         path.read_text(), re.M)
        ]
        assert len(declarations) == 1, entry
        if entry["package"] == runner.SHARED + "auth":
            assert "launchpad/spec/production-system/handoffs/L33/schema-proposal.sql" in entry["reason"]
            assert "not tracked" in entry["reason"]
    assert sum(entry["package"] == runner.SHARED + "auth" for entry in entries) == 6


def test_launchpad_baseline_records_nats_duplicates_window_failures():
    runner = baseline_runner()
    assert not any(entry["package"] == runner.SHARED + "delivery/natsdriver"
                   for entry in runner.load_baseline())


def test_launchpad_baseline_records_database_cancellation_failure():
    runner = baseline_runner()
    assert not any(entry["package"] == runner.SHARED + "db/production"
                   for entry in runner.load_baseline())


def test_launchpad_baseline_patterns_are_exact_and_package_scoped():
    runner = baseline_runner()
    entries = runner.load_baseline()
    for package in {entry["package"] for entry in entries}:
        pattern = runner.skip_pattern(entries, package)
        assert pattern.startswith("^(") and pattern.endswith(")$")
        expected = {entry["test"] for entry in entries if entry["package"] == package}
        for entry in entries:
            assert bool(re.search(pattern, entry["test"])) == (entry["test"] in expected)
            assert not re.search(pattern, entry["test"] + "AdditionalRegression")
            assert not re.search(pattern, "Other" + entry["test"])
    assert runner.skip_pattern(entries, runner.SHARED + "util") == ""


def test_launchpad_baseline_records_protocol_fixture_failures():
    runner = baseline_runner()
    assert not any(entry["package"] in {
        runner.PROTOCOL, runner.PROTOCOL + "/bindings", runner.PROTOCOL + "/coverage"
    } for entry in runner.load_baseline())


def test_launchpad_baseline_records_api_failures_and_retiring_owner():
    runner = baseline_runner()
    entries = [entry for entry in runner.load_baseline()
               if entry["package"].startswith(runner.API)]
    expected = {
        "app": {
            "TestDevelopmentTransactionStatusFromChainRPC",
            "TestRepair_27_log_free_receipts",
            "TestDevelopmentStreamTerminalFramesCarryCloseCode",
            "TestDevelopmentStreamSnapshotWebSocketAndSSE",
            "TestDevelopmentStreamUnknownPoolAndLegacy",
            "TestDevelopmentStreamSnapshotKnownPoolWithoutData",
            "TestDevelopmentStreamSnapshotRegistryFailureStaysRetryable",
            "TestDevelopmentStreamUnknownPoolSubscribeCloses",
        },
        "broker": {
            "TestPostgresRetainedIdentitiesSurviveInterruptedReplay",
            "TestPostgresResumePreflightsByteLimitBeforeDelivery",
        },
        "cache": {
            "TestRealTwoAuthenticatedWalletCacheIsolation",
            "TestRealHotExpiryAcrossTenReplicas",
            "TestRealExclusiveRedisFlushPreservesDurableTruth",
            "TestRealAdapterMeasurementCapture",
        },
        "rest": {
            "TestFrozenV2ResponseSchemasMatchHandlerBytes",
            "TestHolderRewardsProductionRead",
            "TestLeaderboardProductionRead",
            "TestPoolDetailProductionRead",
            "TestPoolListProductionRead",
            "TestHoldersProductionReadValidation",
            "TestPlatformMetricsProductionRead",
            "TestPoolStatsInterface22FieldsRoundTrip",
            "TestProductionCanonicalStatsAndSnapshotResume",
            "TestProductionGeneratedSDKUsesActualHTTPAndExactAmounts",
            "TestProductionPrivateSnapshotDenialIsUncacheableAndRedactsTokens",
            "TestProductionHTTPAdmissionSurgePreservesReserveAndRejectsExplicitly",
            "TestProductionRESTMeasurementHarness",
            "TestProductionRESTMeasurementAggregate",
            "TestProductionRegisteredOperationsHaveFrozenContracts",
        },
        "store": {"TestProductionCanonicalAccountingPagesKeepCommittedFrontier"},
    }
    assert {(entry["package"], entry["test"]) for entry in entries} == {
        (runner.API + "internal/" + package, name)
        for package, names in expected.items() for name in names
    }
    for entry in entries:
        assert entry["retiring_task"] == "22.39.154"
        assert re.match(r"\w+_test.go line \d+: .+", entry["reason"])
        if entry["test"] in {
            "TestDevelopmentTransactionStatusFromChainRPC", "TestRepair_27_log_free_receipts"
        }:
            assert "shared v2 receipt projection" in entry["reason"]
        if entry["package"] == runner.API + "internal/broker":
            assert "context deadline exceeded" in entry["reason"]
    assert not any(entry["test"] == "TestProductionStreamRelayPublishesCommittedFrames"
                   for entry in runner.load_baseline())


def test_launchpad_baseline_rejects_wrong_service_retiring_owner(tmp_path):
    runner = baseline_runner()
    entry = next(entry for entry in runner.load_baseline()
                 if entry["package"].startswith(runner.API))
    path = tmp_path / "wrong-owner.json"
    path.write_text(json.dumps([entry | {"retiring_task": "22.39.117"}]))
    with pytest.raises(ValueError, match="retiring task"):
        runner.load_baseline(path)


@pytest.mark.parametrize("mutation", [
    {"test": "Test.*"}, {"test": "TestOne/subcase"},
    {"package": "unreviewed/package"}, {"reason": ""},
    {"retiring_task": ""}, {"retiring_task": "22.39.154"}, {"extra": "field"},
])
def test_launchpad_baseline_rejects_invalid_entries(tmp_path, mutation):
    runner = baseline_runner()
    entry = runner.load_baseline()[0] | mutation
    path = tmp_path / "invalid.json"
    path.write_text(json.dumps([entry]))
    with pytest.raises(ValueError):
        runner.load_baseline(path)


def test_launchpad_baseline_rejects_duplicate_entries(tmp_path):
    runner = baseline_runner()
    entry = runner.load_baseline()[0]
    path = tmp_path / "duplicate.json"
    path.write_text(json.dumps([entry, entry]))
    with pytest.raises(ValueError, match="duplicate"):
        runner.load_baseline(path)


@pytest.mark.parametrize("go_flags", ["", "-timeout=15m"])
def test_launchpad_all_test_modes_use_package_baseline(go_flags):
    for target, flags in (("test", ""), ("race", "-race"), ("cover-check", "")):
        recipe = subprocess.check_output(
            ["make", "--no-print-directory", "-n", "-C",
             str(ROOT / "services/launchpad"), f"GOFLAGS={go_flags}", target], text=True
        )
        assert RUNNER in recipe
        assert '--module "$m"' in recipe
        expected_flags = " ".join(flag for flag in (go_flags, flags) if flag)
        assert f'--flags="{expected_flags}"' in recipe
        assert not re.search(r"go test.*\./\.\.\.", recipe)
        if target == "cover-check":
            assert '--profile ' in recipe
            assert "coverage_inventory.py begin" in recipe
            assert "coverage_inventory.py record" in recipe
            assert "coverage_inventory.py finish" in recipe
            assert "RUN_MANIFEST=.cover/run.json" in recipe
            assert 'MONEY="../../sdk/launchpad/shared' in recipe


def test_launchpad_runner_preserves_real_package_coverage(tmp_path):
    runner = baseline_runner()
    profile = tmp_path / "coverage.out"
    assert runner.run_tests(ROOT / "sdk/launchpad/shared/util", profile=profile) == 0
    lines = profile.read_text().splitlines()
    assert lines[0] == "mode: atomic"
    assert len(lines) > 1
    assert any(int(line.split()[-1]) > 0 for line in lines[1:])


def test_launchpad_go_job_provides_indexer_services():
    job = launchpad_go_workflow()["jobs"]["launchpad-go-test"]
    assert job["env"]["GOFLAGS"] == "-timeout=15m"
    fixture_dsn = (
        "postgres://kindle_fixture:kindle_fixture_password@127.0.0.1:25432/"
        "kindle_fixture?sslmode=disable"
    )
    for name in ("INDEXER_TEST_POSTGRES_URL", "L06_POSTGRES_DSN", "L06_COMMIT_CRASH_DSN"):
        assert job["env"][name] == fixture_dsn, name
    assert job["env"]["INDEXER_TEST_REDIS_URL"] == "redis://127.0.0.1:26379/6"
    assert len({job["env"][name] for name in (
        "INDEXER_TEST_REDIS_URL", "CORE_API_TEST_REDIS_URL",
        "L09_REDIS_URL", "SHARED_TEST_REDIS_URL"
    )}) == 4
    steps = job["steps"]
    (database,) = [step for step in steps if step["name"] == "Create the pool test database"]
    assert database["env"]["PGPASSWORD"] == "kindle_fixture_password"
    assert database["run"].splitlines() == [
        'psql --host 127.0.0.1 --port 25432 --username kindle_fixture --dbname kindle_fixture --command "ALTER ROLE kindle_fixture CREATEDB"',
        'createdb --host 127.0.0.1 --port 25432 --username kindle_fixture kindle_test',
        'createdb --host 127.0.0.1 --port 25432 --username kindle_fixture launchpad_notifications',
        'createdb --host 127.0.0.1 --port 25432 --username kindle_fixture launchpad_pnl',
    ]
    (test,) = [step for step in steps if step["name"] == "Test Go module"]
    assert steps.index(database) < steps.index(test)
    (fixtures,) = [step for step in steps
                   if step["name"] == "Record indexer fixture prerequisites"]
    assert steps.index(fixtures) < steps.index(test)
    for artifact in (
        "internal/internaltest/chainfixture/testdata/paxeer125-market.jsonl.gz",
        "internal/receipt/testdata/repair53-producer.json",
        "../../../contracts/launchpad/artifacts/contracts/core/SidioraNFT.sol/SidioraNFT.json",
        "../../../contracts/launchpad/artifacts/contracts/core/SidioraERC20.sol/SidioraERC20.json",
    ):
        assert f"test -s {artifact}" in fixtures["run"]


def assert_indexer_runs_without_baseline(entries):
    runner = baseline_runner()
    assert not any(entry["package"].startswith(runner.INDEXER) for entry in entries)
    for package in runner.INDEXER_PACKAGES:
        assert runner.skip_pattern(entries, package) == ""
    job = launchpad_go_workflow()["jobs"]["launchpad-go-test"]
    assert job["strategy"]["matrix"]["module"].count("services/launchpad/indexer") == 1
    assert "exclude" not in job["strategy"]["matrix"]
    assert "continue-on-error" not in job
    (run,) = [step for step in job["steps"] if step["name"] == "Test Go module"]
    assert run["run"] == f'python3 "$GITHUB_WORKSPACE/{RUNNER}" --module .'
    assert "if" not in run
    assert "continue-on-error" not in run


def test_launchpad_baseline_records_only_approved_indexer_failures():
    runner = baseline_runner()
    assert_indexer_runs_without_baseline(runner.load_baseline())
    record = tomllib.loads((ROOT / "services/launchpad/indexer/repair_22_39_162.kvx").read_text())
    entries = list(record["repair"].values())
    assert len(entries) == 33
    for entry in entries:
        assert entry["producer"].strip()
        directory = ROOT / "services/launchpad/indexer" / entry["package"].removeprefix(runner.INDEXER)
        assert any(re.search(r"^func " + entry["test"] + r"\(t \*testing.T\)",
                             source.read_text(), re.M)
                   for source in directory.glob("*_test.go")), entry["test"]
    expected = {
        "app": {
            "TestRenderedProductionDocumentsBoot",
            "TestRepair_55_RestartRecoversRealMarketBeyondLifetimeBounds",
            "TestRepair_55_ReceiptHintsRejectTampering",
        },
        "config": {"TestRepairM_999_ReceiptQualifiedBackfill"},
        "decode": {"TestRepairM_995_NativeReceiptAttribution"},
        "runtimedocs": {
            "TestPaxeerReportedIdentityQualifiesRealMarket",
            "TestPaxeerRehydratedAuthorityQualifiesTradesOfEarlierMarket",
            "TestPaxeerTamperedReceiptIsRejected",
            "TestPaxeerLiveQualifiedSourceCoversDiscoveredEmitters",
        },
        "processor": {
            "TestCanonicalProcessorConcurrentFencingPostgres",
            "TestCanonicalDatabaseOutageAcknowledgesNothingPostgres",
            "TestQualifiedProductionIngestionRealMarketPostgres",
            "TestRepair54Postgres_NativeReadModelProjection",
            "TestRepair_54_RecordedLiveQualification",
            "TestRepair54Postgres_AmbiguousCommitLiveRecovery",
            "TestRepair_54_NativeReadModelProjection",
            "TestRepair_54_AmbiguousCommitLiveRecovery",
            "TestReplayCheckpointAtomicAndLiveCursorIndependent",
            "TestReplayCancellationRollbackAndResumePostgres",
            "TestReplayPausedOldLeaderCannotPauseReplacementPostgres",
            "TestPrivateReplayAuthorizationPostgres",
            "TestReplayQuotaAndUnavailableRangePostgres",
        },
        "receipt": {
            "TestRepair_53_native_swap_qualification",
            "TestRepair_53_tenant_transaction_qualification",
            "TestRepair_53_approved_history_rehydration",
        },
        "source": {
            "TestRepair56Postgres_ExplorerDynamicDiscoveries",
            "TestRepair56Postgres_DiscoveryEpochBounds",
            "TestRepair_56_ExplorerDynamicDiscoveries", "TestRepair_56_DiscoveryEpochBounds",
            "TestSourceCapacityCandidate", "TestHistoricalRealRangeExplorerSuffixAndParity",
        },
        "httpapi": {"TestRepairM_996_NoCompatibilityDeadLetterEscape"},
        "chainreader": {"TestActualNFTOwnerRemainsPinnedAfterTwoTransfers"},
    }
    assert {(entry["package"], entry["test"]) for entry in entries} == {
        (runner.INDEXER + "internal/" + package, name)
        for package, names in expected.items() for name in names
    }
    receipt_entries = [entry for entry in entries
                       if entry["package"].rsplit("/", 1)[-1] in {
                           "app", "config", "decode", "runtimedocs"
                       } or entry["test"].startswith("TestRepair_56_")]
    assert len(receipt_entries) == 11
    assert all("receipt qualification rejected" in entry["reason"] for entry in receipt_entries)
    for entry in entries:
        assert not any(name in entry["reason"] for name in (
            "INDEXER_TEST_POSTGRES_URL", "INDEXER_TEST_REDIS_URL",
            "L06_POSTGRES_DSN", "L06_COMMIT_CRASH_DSN"
        ))
    by_name = {entry["test"]: entry for entry in entries}
    assert "retired compatibility webhook delivery" in by_name[
        "TestRepairM_996_NoCompatibilityDeadLetterEscape"]["reason"]
    assert "180 seconds" in by_name["TestActualNFTOwnerRemainsPinnedAfterTwoTransfers"]["reason"]
    assert "bound actual owned source capacity configuration" in by_name[
        "TestSourceCapacityCandidate"]["reason"]
    assert "real historical fixture state" in by_name[
        "TestHistoricalRealRangeExplorerSuffixAndParity"]["reason"]


@pytest.mark.parametrize("owner", ["22.39.117", "22.39.154"])
def test_launchpad_baseline_rejects_wrong_indexer_retiring_owner(tmp_path, owner):
    runner = baseline_runner()
    entries = runner.load_baseline()
    assert not any(entry["package"].startswith(runner.INDEXER)
                   and entry["retiring_task"] == owner for entry in entries)
    assert_indexer_runs_without_baseline(entries)


def test_launchpad_indexer_module_declares_test_dependencies():
    module = ROOT / "services/launchpad/indexer"
    manifest = (module / "go.mod").read_text()
    checksums = (module / "go.sum").read_text()
    for dependency in ("github.com/elliots/go-pglite", "modernc.org/sqlite"):
        (version,) = re.findall(r"^\s*" + re.escape(dependency) + r" (v\S+)\s*$",
                                manifest, re.M)
        for suffix in ("", "/go.mod"):
            assert re.search(r"^" + re.escape(dependency + " " + version + suffix)
                             + r" h1:\S+$", checksums, re.M), dependency + suffix


def test_launchpad_go_job_provides_remaining_module_fixtures():
    job = launchpad_go_workflow()["jobs"]["launchpad-go-test"]
    expected = {
        'postgres://kindle_fixture:kindle_fixture_password@127.0.0.1:25432/kindle_fixture?sslmode=disable': (
            'RANKING_TEST_POSTGRES_DSN',
            'L19_POSTGRES_DSN',
            'STATS_TEST_POSTGRES_DSN',
            'L13_POSTGRES_DSN',
            'L13_PHASE2_POSTGRES_DSN',
            'L14_POSTGRES_DSN',
            'L15_TEST_DSN',
            'L15_PHASE4_DSN',
            'L20_TEST_DSN',
            'CHARTS_TEST_POSTGRES_URL',
            'L10_TEST_POSTGRES',
            'L11_HISTORY_POSTGRES_DSN',
            'L33_POSTGRES_DSN',
            'L41_POSTGRES_URL',
            'L40_TEST_POSTGRES_DSN',
            'L40_TEST_APP_POSTGRES_DSN',
            'METADATA_TEST_DATABASE_URL',
            'KINDLE_PRODUCTION_TEST_DATABASE_URL',
            'KINDLE_PUBLICATION_TEST_DATABASE_URL',
            'SOCIAL_TEST_POSTGRES_DSN',
            'SOCIAL_FIXTURE_POSTGRES_DSN',
            'L37_TEST_DSN',
            'L37_PHASE2_DSN',
            'L38_POSTGRES_DSN',
            'USER_TEST_DATABASE_URL',
            'USER_TEST_POSTGRES_DSN',
            'L45_DATABASE_URL',
        ),
        'postgres://kindle_fixture:kindle_fixture_password@127.0.0.1:25432/launchpad_pnl?sslmode=disable': (
            'PNL_TEST_POSTGRES_DSN', 'L16_TEST_DSN', 'L17_TEST_DSN', 'L18_POSTGRES_DSN',
        ),
        'redis://127.0.0.1:26379/0': (
            'RANKING_TEST_REDIS_URL',
            'L19_REDIS_URL',
        ),
        'redis://127.0.0.1:26379/1': (
            'STATS_TEST_REDIS_URL',
            'L20_REDIS_URL',
        ),
        'redis://127.0.0.1:26379/2': (
            'CHARTS_TEST_REDIS_URL',
            'L11_HISTORY_REDIS_URL',
        ),
        'redis://127.0.0.1:26379/3': (
            'GATEWAY_TEST_REDIS_URL',
            'GATEWAY_E2E_REDIS_URL',
        ),
        'redis://127.0.0.1:26379/4': (
            'L40_TEST_REDIS_URL',
        ),
        'redis://127.0.0.1:26379/5': (
            'METADATA_TEST_REDIS_URL',
        ),
        'redis://127.0.0.1:26379/7': (
            'SOCIAL_TEST_REDIS_URL',
            'SOCIAL_FIXTURE_REDIS_URL',
            'L37_PHASE2_REDIS_URL',
        ),
        'redis://127.0.0.1:26379/12': ('PNL_TEST_REDIS_URL',),
        'redis://127.0.0.1:26379/8': (
            'USER_TEST_REDIS_URL',
        ),
        '127.0.0.1:26379': (
            'L38_REDIS_ADDR',
        ),
        'nats://127.0.0.1:24222': (
            'RANKING_TEST_NATS_URL',
            'L19_NATS_URL',
            'L13_PHASE3_NATS_URL',
            'L15_NATS_URL',
            'L15_PHASE4_NATS_URL',
            'L20_NATS_URL',
            'SOCIAL_TEST_NATS_URL',
            'L37_PHASE2_NATS_URL',
            'PNL_TEST_NATS_URL',
        ),
        'http://127.0.0.1:28545': (
            'L33_ANVIL_URL',
            'L47_ANVIL_URL',
        ),
        'http://localhost:9000': (
            'GATEWAY_TEST_MINIO_ENDPOINT',
            'METADATA_TEST_S3_ENDPOINT',
            'USER_TEST_MINIO_ENDPOINT',
            'L35_TEST_OBJECT_ENDPOINT',
            'L10_TEST_S3_ENDPOINT',
        ),
        'launchpad-test': (
            'GATEWAY_TEST_MINIO_ACCESS_KEY_ID',
            'METADATA_TEST_S3_ACCESS_KEY_ID',
            'USER_TEST_MINIO_ACCESS_KEY',
            'L35_TEST_OBJECT_ACCESS_KEY',
            'L10_TEST_S3_ACCESS_KEY_ID',
        ),
        'launchpad-test-password': (
            'GATEWAY_TEST_MINIO_SECRET_ACCESS_KEY',
            'METADATA_TEST_S3_SECRET_ACCESS_KEY',
            'USER_TEST_MINIO_SECRET_KEY',
            'L35_TEST_OBJECT_SECRET_KEY',
            'L10_TEST_S3_SECRET_ACCESS_KEY',
        ),
        'launchpad-user-test': (
            'L35_TEST_OBJECT_BUCKET',
        ),
        'kd_143975143975143975143975': (
            'L14_POSTGRES_SCHEMA',
        ),
        'kd_373975373975373975373975': (
            'L37_PHASE2_SCHEMA',
        ),
        'indexer': (
            'L37_PHASE2_CANONICAL_SCHEMA',
        ),
        'postgres://kindle_fixture:kindle_fixture_password@127.0.0.1:25432/launchpad_notifications?sslmode=disable': (
            'NOTIFICATIONS_TEST_DATABASE_URL',
        ),
    }
    for value, names in expected.items():
        for name in names:
            assert job["env"][name] == value, name
    redis_names = (
        "RANKING_TEST_REDIS_URL", "STATS_TEST_REDIS_URL", "CHARTS_TEST_REDIS_URL",
        "GATEWAY_TEST_REDIS_URL", "L40_TEST_REDIS_URL", "METADATA_TEST_REDIS_URL",
        "SOCIAL_TEST_REDIS_URL", "USER_TEST_REDIS_URL", "INDEXER_TEST_REDIS_URL",
        "CORE_API_TEST_REDIS_URL", "L09_REDIS_URL", "SHARED_TEST_REDIS_URL",
        "PNL_TEST_REDIS_URL",
    )
    assert len({job["env"][name] for name in redis_names}) == len(redis_names)
    assert job["services"]["postgres"]["image"] == "postgres:16"
    assert job["services"]["redis"]["image"] == "redis:7"
    assert job["env"]["L13_PHASE3_REDIS_SERVER"] == "/usr/bin/redis-server"
    (install,) = [step for step in job["steps"]
                  if step["name"] == "Install fixture binaries"]
    assert "sudo apt-get install --no-install-recommends --yes redis-server" in install["run"]


def test_launchpad_remaining_package_inventory_matches_module_sources():
    runner = baseline_runner()
    assert set(runner.SERVICE_PACKAGES) == {
        "ranking-algo", "stats-workers", "trading-charts", "gateway", "livestream",
        "metadata", "social", "user", "discovery", "notifications", "security",
        "security/control",
    }
    for module, packages in runner.SERVICE_PACKAGES.items():
        root = ROOT / "services/launchpad" / module
        prefix = re.search(r"^module (.+)$", (root / "go.mod").read_text(), re.M)[1]
        expected = set()
        for source in root.rglob("*.go"):
            relative = source.parent.relative_to(root)
            if "testdata" in relative.parts:
                continue
            if any((root / Path(*relative.parts[:i]) / "go.mod").exists()
                   for i in range(1, len(relative.parts) + 1)):
                continue
            expected.add(prefix + ("" if relative == Path(".") else "/" + relative.as_posix()))
        assert packages == expected, module


def test_launchpad_baseline_records_indexer_refusal_classes():
    runner = baseline_runner()
    assert_indexer_runs_without_baseline(runner.load_baseline())
    record = tomllib.loads((ROOT / "services/launchpad/indexer/repair_22_39_162.kvx").read_text())
    entries = {entry["test"]: entry for entry in record["repair"].values()}
    assert "foreign key constraint" in entries[
        "TestCanonicalProcessorConcurrentFencingPostgres"]["reason"]
    assert "cannot disallow connections for the current database" in entries[
        "TestCanonicalDatabaseOutageAcknowledgesNothingPostgres"]["reason"]
    for name in (
        "TestRepair54Postgres_NativeReadModelProjection",
        "TestRepair_54_NativeReadModelProjection",
    ):
        assert "mode in the unavailable components" in entries[name]["reason"]
    for name in (
        "TestReplayCheckpointAtomicAndLiveCursorIndependent",
        "TestReplayCancellationRollbackAndResumePostgres",
        "TestReplayPausedOldLeaderCannotPauseReplacementPostgres",
        "TestPrivateReplayAuthorizationPostgres",
        "TestReplayQuotaAndUnavailableRangePostgres",
    ):
        assert "parameter $10 (SQLSTATE 42P08)" in entries[name]["reason"]
    for entry in entries.values():
        if entry["package"].startswith(runner.INDEXER):
            assert re.match(r"\w+_test.go line \d+: .+", entry["reason"])


@pytest.mark.parametrize("owner", ["22.39.117", "22.39.154", "22.39.162"])
def test_launchpad_baseline_rejects_wrong_remaining_module_owner(tmp_path, owner):
    runner = baseline_runner()
    assert not any(entry["package"] in runner.REMAINING_PACKAGES
                   for entry in runner.load_baseline())
    record = tomllib.loads((ROOT / "services/launchpad/repair_22_39_168.kvx").read_text())
    for module, packages in runner.SERVICE_PACKAGES.items():
        retired = record["module"][module.replace("/", "_")]
        for index, case in enumerate(retired["cases"]):
            package, name = case.split("::")
            assert package in packages
            entry = {"package": package, "test": name, "reason": "retired by 22.39.168",
                     "retiring_task": owner}
            path = tmp_path / f"wrong-remaining-owner-{module.replace('/', '-')}-{index}.json"
            path.write_text(json.dumps([entry]))
            with pytest.raises(ValueError, match="retiring task"):
                runner.load_baseline(path)


def test_launchpad_go_job_creates_real_export_bucket_before_tests():
    job = launchpad_go_workflow()["jobs"]["launchpad-go-test"]
    steps = job["steps"]
    (storage,) = [step for step in steps if step["name"] == "Start object storage fixture"]
    assert 'curl --fail --aws-sigv4 "aws:amz:us-east-1:s3"' in storage["run"]
    assert '--user "$SHARED_TEST_MINIO_ACCESS_KEY_ID:$SHARED_TEST_MINIO_SECRET_ACCESS_KEY"' in storage["run"]
    assert '--request PUT http://localhost:9000/launchpad-user-test' in storage["run"]
    assert job["env"]["L35_TEST_OBJECT_BUCKET"] == "launchpad-user-test"
    (test,) = [step for step in steps if step["name"] == "Test Go module"]
    assert steps.index(storage) < steps.index(test)


@pytest.mark.parametrize("owner", ["22.39.117", "22.39.162", "22.39.168"])
def test_launchpad_baseline_rejects_wrong_pnl_retiring_owner(tmp_path, owner):
    runner = baseline_runner()
    entries = [entry for entry in runner.load_baseline()
               if entry["package"] in runner.PNL_PACKAGES]
    assert entries
    for index, entry in enumerate(entries):
        assert entry["retiring_task"] == "22.39.154"
        path = tmp_path / f"wrong-pnl-owner-{index}.json"
        path.write_text(json.dumps([entry | {"retiring_task": owner}]))
        with pytest.raises(ValueError, match="retiring task"):
            runner.load_baseline(path)


def test_launchpad_pnl_service_configuration_repairs_stay_out_of_baseline():
    runner = baseline_runner()
    actual = {(entry["package"], entry["test"]) for entry in runner.load_baseline()}
    repaired = {
        'github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/app': {
            'TestAppEndToEnd',
        },
        'github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/card': {
            'TestMintCapturesSnapshotAndPersists',
            'TestMintRejectsMissingPosition',
            'TestGetUnknownCard',
        },
        'github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/devreads': {
            'TestPublicPortfolioEmptyWalletIsValid',
            'TestPublicPortfolioProjectsDevFoldAndPages',
            'TestMeRoutesUseGatewayIdentity',
            'TestMeRoutesWithoutIdentityAreUnavailable',
            'TestChainUnavailableIsRetryable',
        },
        'github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/httpapi': {
            'TestMintAndHydrateCard',
            'TestMintRejectsNoPosition',
            'TestMintBadBody',
            'TestGetCardNotFound',
            'TestLogEventAndSharerStats',
            'TestPositionsAndPositionRoutes',
            'TestTradesRoute',
            'TestPortfolioRoute',
            'TestStatusReportsLagAndCursor',
            'TestWebhookAuth',
            'TestWebhookDispatchFoldsSwaps',
            'TestWebhookFailingEventCounted',
            'TestWebhookNonArrayBody',
        },
        'github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/position': {
            'TestProcessEventFoldsAndResolvesToken',
            'TestProcessEventPublishesPnlUpdate',
            'TestProcessEventIdempotentRedelivery',
            'TestProcessSellLegSplit',
        },
        'github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/production/l16': {
            'TestPostgresOwnedGenerationMappingAtomicity',
            'TestPostgresAtomicInboxOutbox',
            'TestPostgresLinkedTransferAtomicCarry',
            'TestPostgresRestartReversedDeliveryAndPendingPublication',
            'TestPostgresReplayActivationArchiveOutboxAtomicity',
        },
        'github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/production/l17': {
            'TestPhase2ImmutablePagesAndAtomicRollback',
            'TestPhase2DurableExport100001AndCancellation',
            'TestPhase2StrictRenderAndPersistentPrivacyFailure',
            'TestPhase2PublicHTTPAndOwnerUnavailable',
            'TestPhase3HistoryReplayOwnerAndNodeDisagreement',
            'TestPhase3CardRetryAfterSnapshotExpiryAndRevocation',
            'TestPhase3CommittedProjectionOutboxSurvivesRestartAndDuplicate',
            'TestPhase4ActualHTTPOriginMeasurement',
            'TestShareV3ImmutableSelectedIntentAndPrivacy',
        },
        'github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/production/l18': {
            'TestPhase2RealPostgresKeyRotationFirstBindingAndRollback',
            'TestPhase3RealPostgresCrossCampaignRequestAndUUIDReplay',
            'TestPhase3RealPostgresIssuedBindingAtomicRecovery',
            'TestPostgresReservationReplayIsolationAndHold',
            'TestPostgresConcurrentBudgetAndRollback',
            'TestPostgresConsentFirstBindingAndExpiry',
            'TestPostgresMissingAuthorityNeverPromisesReward',
            'TestPostgresReferralConversionHasCanonicalIdentity',
        },
        'github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/reconcile': {
            'TestReconcilerBackfillsAndAdvancesCursor',
            'TestReconcilerIdempotentWithConsumerOverlap',
            'TestReconcilerBatchKeysetDrainsLargeBlock',
            'TestReconcilerPoisonPrefixReopensWithRefresh',
        },
        'github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/referral': {
            'TestLogClickAndBindToConversion',
            'TestBindWithoutTradeIsNotConversion',
            'TestLogResolvesByCardID',
            'TestLogUnknownReferral',
            'TestLogInvalidType',
            'TestLogView',
        },
        'github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/store': {
            'TestInsertCardCreatesReferralBinding',
            'TestPortfolioEnrichesAndSumsNetWorth',
            'TestPortfolioEmptyUser',
            'TestFoldTradeAccumulatesAndIsIdempotent',
            'TestFoldTradeRealisesLoss',
            'TestListPositionsAndTrades',
            'TestFoldTradeRebuildsDelayedTrades',
            'TestCursorAndUnreconciledSwaps',
            'TestConsumerBlockTracksFoldedTrades',
            'TestLogReferralEventDedupAndStats',
            'TestLogReferralEventRejectsUnknownType',
            'TestSharerStatsEmpty',
            'TestUserHasAnyPosition',
            'TestCrossSchemaReads',
        },
    }
    assert sum(map(len, repaired.values())) == 72
    for package, names in repaired.items():
        for name in names:
            assert (package, name) not in actual, name


def test_launchpad_baseline_records_exact_remaining_module_counts():
    runner = baseline_runner()
    entries = runner.load_baseline()
    expected = {
        "ranking-algo": 0, "stats-workers": 0, "trading-charts": 0,
        "gateway": 0, "livestream": 0, "metadata": 0, "social": 0,
        "user": 0, "discovery": 0, "notifications": 0, "security": 0,
        "security/control": 0,
    }
    assert {
        module: sum(entry["package"] in packages for entry in entries)
        for module, packages in runner.SERVICE_PACKAGES.items()
    } == expected
    assert sum(entry["package"] in runner.PNL_PACKAGES for entry in entries) == 42
    for entry in entries:
        if entry["package"] in runner.REMAINING_PACKAGES | runner.PNL_PACKAGES:
            assert re.match(r"[\w.-]+_test.go line \d+: .+", entry["reason"])


def test_launchpad_go_job_builds_protocol_proofs_before_timed_tests():
    job = launchpad_go_workflow()["jobs"]["launchpad-go-test"]
    steps = job["steps"]
    (prepare,) = [step for step in steps
                  if step["name"] == "Build protocol proof dependencies"]
    (run,) = [step for step in steps if step["name"] == "Test Go module"]
    (native,) = [step for step in steps
                 if step["name"] == "Build native library for protocol producer proofs"]
    assert steps.index(native) < steps.index(prepare) < steps.index(run)
    assert prepare["working-directory"] == "."
    command = prepare["run"]
    assert command.startswith("set -euo pipefail\n")
    assert 'if [ "${{ matrix.module }}" = sdk/launchpad/protocol ]; then' in command
    assert "awk '$1 == \"go\" {print $2; exit}' go.mod" in command
    assert 'export GOTOOLCHAIN="go$(' in command
    assert "build_proof_dependencies precompiles/launchpad" in command
    download = (
        "cd sdk/launchpad/protocol/bindings/testdata/repair37/contracts\n"
        "    go mod download all"
    )
    assert download in command
    assert command.index(download) < command.index(
        "build_proof_dependencies sdk/launchpad/protocol/bindings/testdata/repair37/contracts"
    )
    assert "build_proof_dependencies sdk/launchpad/protocol/bindings/testdata/repair37/contracts" in command
    assert "go list -mod=readonly -deps -test" in command
    assert '(not .Standard) (not .ForTest) (ne .Name "main") .GoFiles' in command
    assert 'test -s "$RUNNER_TEMP/launchpad-proof-dependencies.txt"' in command
    assert "xargs go build -mod=readonly" in command
    assert "cd sdk/launchpad/protocol/coverage/testdata/repair28producer" in command
    assert 'cp go.mod "$RUNNER_TEMP/launchpad-repair28.mod"' in command
    assert 'go build -modfile="$RUNNER_TEMP/launchpad-repair28.mod" -mod=mod' in command
    assert "go test" not in command
    assert "-timeout" not in command


def test_launchpad_baseline_records_user_profile_producer_conflicts():
    runner = baseline_runner()
    package = "github.com/Sidiora-Technologies/KindleLaunch/media/user/internal/app"
    entries = {entry["test"]: entry for entry in runner.load_baseline()
               if entry["package"] == package}
    assert entries == {}
    root = ROOT / "services/launchpad/user"
    reads = (root / "internal/app/development_profiles_test.go").read_text()
    for required in ("sharedauth.OperationEnvelope{", 'Action: "profile.update"',
                     "sharedauth.OperationEnvelopeHeader", "sharedauth.OperationSignatureHeader"):
        assert required in reads
    app = (root / "internal/app/app.go").read_text()
    handlers = (root / "internal/user/handlers.go").read_text()
    assert "user.ProfileCacheKey(cfg.PublicURL, wallet)" in app
    assert "return ProfileCacheKey(h.publicURL, addr)" in handlers
    record = tomllib.loads((ROOT / "services/launchpad/repair_22_39_168.kvx").read_text())
    assert set(record["module"]["user"]["cases"]) >= {
        package + "::TestDevelopmentProfileWrite",
        package + "::TestDevelopmentProfileReads",
    }
    assert record["repair"]["user_profile"]["producer"] == "internal/user/signed_operation.go"
    assert record["repair"]["user_profile"]["cache_invalidation"] == "shared legacy read/write cache key"


def test_launchpad_baseline_records_security_canonical_field_conflict():
    runner = baseline_runner()
    package = "github.com/Sidiora-Technologies/KindleLaunch/security/policy"
    assert not any(entry["package"] == package for entry in runner.load_baseline())
    policy = (ROOT / "services/launchpad/security/policy/policy_test.go").read_text()
    case = policy.split("func TestRequestParsing(t *testing.T) {", 1)[1].split("\nfunc ", 1)[0]
    assert 'strings.NewReader(`{"Name":"ok"}`)' in case
    assert 'strings.NewReader(`{"name":"ok"}`)' not in case
    record = tomllib.loads((ROOT / "services/launchpad/repair_22_39_168.kvx").read_text())
    assert record["module"]["security"]["cases"] == [package + "::TestRequestParsing"]
    assert record["repair"]["security_canonical_field"]["producer"] == "policy/policy.go"
    assert record["repair"]["security_canonical_field"]["canonical_key"] == "Name"


def test_launchpad_module_qualification_records_follow_runner_and_baseline():
    runner = baseline_runner()
    entries = runner.load_baseline()
    job = launchpad_go_workflow()["jobs"]["launchpad-go-test"]
    retirement = tomllib.loads((ROOT / "services/launchpad/repair_22_39_168.kvx").read_text())
    for module in (
        "stats-workers", "trading-charts", "livestream", "metadata", "discovery"
    ):
        path = "services/launchpad/" + module
        record = tomllib.loads((ROOT / path / "repair_22_39_75.kvx").read_text())
        prefix = re.search(r"^module (.+)$", (ROOT / path / "go.mod").read_text(), re.M)[1]
        assert record["meta"] == {
            "task": "22.39.75", "module": path, "package_prefix": prefix,
        }
        assert path in job["strategy"]["matrix"]["module"]
        assert record["producer"] == {
            "workflow": WORKFLOW, "job": "launchpad-go-test", "runner": RUNNER,
            "toolchain": job["env"]["GOTOOLCHAIN"],
        }
        expected = sorted(entry["package"] + "::" + entry["test"] for entry in entries
                          if entry["package"] in runner.SERVICE_PACKAGES[module])
        reviewed = record["baseline"]
        assert expected == []
        current = retirement["module"][module]
        assert current["remaining_cases"] == expected
        assert sorted(current["cases"]) == reviewed["tests"]
        assert current["exclusions_removed"] == reviewed["count"]
        assert current["evidence"] == "evidence.ledger; the declared verify command invokes the baseline-aware runner in Makefile order"
        assert reviewed["retiring_task"] == "22.39.168"
        assert "not passing tests" in reviewed["policy"]
        measured = record["measurement"]
        assert re.fullmatch(r"[0-9a-f]{40}", measured["revision"])
        assert measured["command"] == "make -C services/launchpad test"
        assert measured["exit"] == 0
        assert measured["log"] == "/out/logs/verify-1.log"
        assert measured["aggregate_command"] == (
            "cg spec done 22.39.75 -f layerx-merge --root /work/paxeer-monorepo"
        )
        assert measured["aggregate_exit"] == 1
        assert "task remained unqualified" in measured["aggregate_result"]
