import pathlib
import re
import shutil
import subprocess
import tempfile
import textwrap
import unittest

import yaml

ROOT = pathlib.Path(__file__).resolve().parents[2]
CHECK = ROOT / "scripts/ci/runner-routing-check.sh"
ROUTED = (
    "${{ (github.event_name == 'pull_request' && "
    "github.event.pull_request.head.repo.full_name != github.repository) && "
    "'ubuntu-24.04' || vars.CI_LINUX_RUNNER || 'ubuntu-24.04' }}"
)


class RunnerRoutingContainerRuntime(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="runner-routing-")
        self.addCleanup(self.temporary.cleanup)
        self.repo = pathlib.Path(self.temporary.name)
        (self.repo / "scripts/ci").mkdir(parents=True)
        shutil.copy2(CHECK, self.repo / "scripts/ci/runner-routing-check.sh")
        shutil.copytree(ROOT / ".github/workflows", self.repo / ".github/workflows")

    def run_check(self):
        return subprocess.run(
            [str(self.repo / "scripts/ci/runner-routing-check.sh")],
            capture_output=True,
            text=True,
            check=False,
        )

    def write_workflow(self, name, jobs):
        (self.repo / ".github/workflows" / name).write_text(
            "name: probe\non: pull_request\njobs:\n" + jobs, encoding="utf-8"
        )

    def autobahn_job(self):
        workflow = yaml.safe_load(
            (ROOT / ".github/workflows/paxeer-go-test.yml").read_text(encoding="utf-8")
        )
        return workflow["jobs"]["autobahn-integration"]

    def write_autobahn_consumer(self, job):
        self.write_workflow(
            "probe.yml",
            textwrap.indent(
                yaml.safe_dump({"autobahn": job}, sort_keys=False, width=1000), "  "
            ),
        )

    def test_autobahn_make_go_cluster_dependency(self):
        job = self.autobahn_job()
        self.assertEqual(job["runs-on"], "ubuntu-24.04")
        self.assertEqual(
            job["steps"][-1]["run"],
            "make -f chain.mk autobahn-integration-test "
            "AUTOBAHN_TEST_TIMEOUT=${{ env.GO_TEST_TIMEOUT }}",
        )
        makefile = (ROOT / "chain.mk").read_text(encoding="utf-8")
        target = re.search(
            r"^autobahn-integration-test:\n(.*?)^\.PHONY: autobahn-integration-test$",
            makefile,
            re.MULTILINE | re.DOTALL,
        )
        self.assertIsNotNone(target)
        self.assertIn(
            'GOWORK=off go -C "$(PAXEER_PROJECT_HOME)" test -tags autobahn_integration '
            '-v -count=1 -timeout "$(AUTOBAHN_TEST_TIMEOUT)" ./tests/integration/autobahn/...',
            target.group(1),
        )
        source = (ROOT / "tests/integration/autobahn/autobahn_test.go").read_text(
            encoding="utf-8"
        )
        self.assertTrue(source.startswith("//go:build autobahn_integration\n"))

        def go_function(name):
            function = re.search(
                rf"^func {re.escape(name)}\(.*?^\}}$",
                source,
                re.MULTILINE | re.DOTALL,
            )
            self.assertIsNotNone(function, name)
            return function.group(0)

        main = go_function("TestMain")
        self.assertIn("if err := setupCluster(); err != nil {", main)
        self.assertIn("os.Exit(1)", main)
        self.assertLess(main.index("setupCluster()"), main.index("code := m.Run()"))
        self.assertIn("teardownCluster()\n\tos.Exit(code)", main)
        setup = go_function("setupCluster")
        self.assertIn(
            'runMake([]string{"AUTOBAHN=true", "DOCKER_DETACH=true"}, "docker-cluster-start")',
            setup,
        )
        self.assertIn("expected, err := countPaxContainers()", setup)
        self.assertIn(
            'countLaunchComplete("build/generated/launch.complete"); n >= expected', setup
        )
        self.assertIn('exec.Command("make", target)', go_function("runMake"))
        self.assertIn(
            'exec.Command("docker", "ps", "-a",', go_function("countPaxContainers")
        )
        cluster = re.search(
            r"^docker-cluster-start: docker-cluster-stop build-docker-node\n(.*?)^\.PHONY:",
            makefile,
            re.MULTILINE | re.DOTALL,
        )
        self.assertIsNotNone(cluster)
        self.assertIn(
            "$(CLUSTER_ENV_VARS) docker compose up $$DETACH_FLAG", cluster.group(1)
        )
        self.assertIn("AUTOBAHN=$(AUTOBAHN)", makefile)
        integration = go_function("TestAutobahn")
        self.assertIn("maxFaults = (clusterSize - 1) / 3", integration)
        for name, function in (
            ("BlockProduction", "testBlockProduction"),
            ("BankTransfer", "testBankTransfer"),
            ("LivenessUnderMaxFaults", "testLivenessUnderMaxFaults"),
            ("HaltsBeyondMaxFaults", "testHaltsBeyondMaxFaults"),
            ("Recovery", "testRecovery"),
        ):
            self.assertIn(f't.Run("{name}", {function})', integration)
            self.assertIn("t.Fatalf(", go_function(function))

        result = self.run_check()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stderr, "")

        setup_command = (
            "sudo apt-get update\n"
            "sudo apt-get install --yes docker.io acl\n"
            "sudo systemctl start docker\n"
            'sudo setfacl -m "u:$(id -u):rw" /var/run/docker.sock\n'
            "docker version --format '{{.Server.Version}}'"
        )
        for variant in ("ready", "conditional", "partial", "late", "ignored-error"):
            with self.subTest(runtime_setup=variant):
                consumer = self.autobahn_job()
                consumer["runs-on"] = ROUTED
                setup_step = {"name": "Set up Docker", "run": setup_command}
                if variant == "conditional":
                    setup_step["if"] = "github.event_name != 'pull_request'"
                elif variant == "partial":
                    setup_step["run"] = "sudo apt-get install --yes docker.io acl"
                elif variant == "ignored-error":
                    setup_step["continue-on-error"] = True
                if variant == "late":
                    consumer["steps"].append(setup_step)
                else:
                    consumer["steps"].insert(0, setup_step)
                self.write_autobahn_consumer(consumer)
                result = self.run_check()
                if variant == "ready":
                    self.assertEqual(result.returncode, 0, result.stderr)
                    self.assertEqual(result.stderr, "")
                else:
                    self.assertEqual(result.returncode, 1, result.stderr)
                    self.assertIn("routed job needs a container runtime", result.stderr)

    def test_autobahn_make_go_multinode_dependency(self):
        self.test_autobahn_make_go_cluster_dependency()

    def test_autobahn_routed_make_without_runtime_is_refused(self):
        consumer = self.autobahn_job()
        consumer["runs-on"] = ROUTED
        self.write_autobahn_consumer(consumer)
        result = self.run_check()
        self.assertEqual(result.returncode, 1)
        self.assertEqual(
            result.stderr,
            ".github/workflows/probe.yml: job autobahn: routed job needs a container runtime the routed "
            "runner image lacks (step 'Autobahn integration tests', matrix {}); keep it on a hosted "
            "runner and list it as docker-command\n",
        )

    def test_repository_workflows_pass(self):
        result = self.run_check()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stderr, "")

    def test_routed_matrix_leg_reaching_the_mix_builder_is_refused(self):
        self.write_workflow(
            "probe.yml",
            "  lint:\n"
            f"    runs-on: {ROUTED}\n"
            "    strategy:\n"
            "      matrix:\n"
            "        leg: [backend, frontend]\n"
            "    steps:\n"
            "      - name: Lint\n"
            "        env:\n"
            "          LEG: ${{ matrix.leg }}\n"
            "        run: |\n"
            '          "scripts/explorer/lint-$LEG.sh"\n',
        )
        result = self.run_check()
        self.assertEqual(result.returncode, 1)
        self.assertEqual(
            result.stderr,
            ".github/workflows/probe.yml: job lint: routed job needs a container runtime the routed runner "
            "image lacks (step 'Lint', matrix {'leg': 'backend'}); keep it on a hosted runner and list it "
            "as docker-command\n",
        )

    def test_routed_job_calling_docker_is_refused(self):
        self.write_workflow(
            "probe.yml",
            "  images:\n"
            f"    runs-on: {ROUTED}\n"
            "    steps:\n"
            "      - name: Build\n"
            "        run: docker build .\n",
        )
        result = self.run_check()
        self.assertEqual(result.returncode, 1)
        self.assertEqual(
            result.stderr,
            ".github/workflows/probe.yml: job images: routed job needs a container runtime the routed runner "
            "image lacks (step 'Build', matrix {}); keep it on a hosted runner and list it as docker-command\n",
        )

    def test_routed_matrix_without_a_container_leg_passes(self):
        self.write_workflow(
            "probe.yml",
            "  lint:\n"
            f"    runs-on: {ROUTED}\n"
            "    strategy:\n"
            "      matrix:\n"
            "        leg: [frontend, services]\n"
            "    steps:\n"
            "      - name: Lint\n"
            "        env:\n"
            "          LEG: ${{ matrix.leg }}\n"
            '        run: "scripts/explorer/lint-$LEG.sh"\n',
        )
        result = self.run_check()
        self.assertEqual(result.returncode, 0, result.stderr)


if __name__ == "__main__":
    unittest.main()
