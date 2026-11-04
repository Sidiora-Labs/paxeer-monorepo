"""Bounded coordination for explicitly disposable local Pax clusters."""

import contextlib
import json
import math
import os
from pathlib import Path
import re
import shutil
import signal
import subprocess
import sys
import tempfile
import time
import urllib.error
import urllib.request


class StageFailure(RuntimeError):
    pass


def atomic_json(path, value):
    path = Path(path)
    fd, temporary = tempfile.mkstemp(prefix=f".{path.name}.", dir=path.parent)
    try:
        with os.fdopen(fd, "w") as output:
            json.dump(value, output)
            output.flush()
            os.fsync(output.fileno())
        os.replace(temporary, path)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


class Run:
    def __init__(self, root, environment):
        self.root = Path(root).resolve()
        self.env = dict(environment)
        if self.env.get("LOCALNODE_DISPOSABLE") != "1":
            raise StageFailure("LOCALNODE_DISPOSABLE=1 is required; local keys are disposable")
        self.run_id = self.env.get("LOCALNODE_RUN_ID", "")
        if not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9_-]{15,127}", self.run_id):
            raise StageFailure("use a fresh shared LOCALNODE_RUN_ID (16-128 safe characters)")
        self.node = int(self.env.get("ID", "0"))
        self.size = int(self.env.get("CLUSTER_SIZE", "1"))
        if not 0 <= self.node < self.size <= 100:
            raise StageFailure("invalid node ID or cluster size")
        timeout = float(self.env.get("LOCALNODE_STARTUP_TIMEOUT", "300"))
        if not math.isfinite(timeout) or not 0 < timeout <= 3600:
            raise StageFailure("startup timeout must be positive and at most 3600 seconds")
        self.deadline = min(
            time.time() + timeout,
            float(self.env.get("PAX_STARTUP_DEADLINE", "inf")),
        )
        self.directory = self.root / "build" / "localnode-runs" / self.run_id
        self.stage = "startup"
        self.scripts = self.root / "docker" / "localnode" / "scripts"

    def publish(self, name, value):
        atomic_json(self.directory / name, value)

    def fail(self, reason):
        if self.directory.is_dir() and not self.directory.is_symlink():
            self.publish(f"failed.{self.node}.json", {"stage": self.stage, "reason": str(reason)})

    def check(self, startup=True):
        failures = sorted(self.directory.glob("failed.*.json"))
        if failures:
            raise StageFailure(f"cluster stage failed: {failures[0].read_text()}")
        if startup and time.time() >= self.deadline:
            raise StageFailure(f"startup deadline exceeded during {self.stage}")

    def wait(self, predicate):
        while True:
            self.check()
            if predicate():
                return
            time.sleep(0.05)

    def join(self):
        if self.node == 0:
            self.directory.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
            if self.directory.parent.is_symlink():
                raise StageFailure("run parent must not be a symlink")
            self.directory.mkdir(mode=0o700)  # Exclusive: never erase or reuse an old run.
            for name in ("gentx", "exported_keys", "logs", "bin"):
                (self.directory / name).mkdir(mode=0o700)
            self.publish("run.json", {
                "run_id": self.run_id, "cluster_size": self.size,
                "chain_id": "pax", "disposable": True, "deadline": self.deadline,
            })
        if self.directory.is_symlink() or self.directory.parent.is_symlink():
            raise StageFailure("run paths must not be symlinks")
        self.wait(lambda: (self.directory / "run.json").is_file())
        metadata = json.loads((self.directory / "run.json").read_text())
        if (metadata["run_id"], metadata["cluster_size"], metadata["chain_id"], metadata["disposable"]) != (
            self.run_id, self.size, "pax", True
        ):
            raise StageFailure("coordinator context mismatch")
        self.deadline = min(self.deadline, metadata["deadline"])
        (self.directory / f"node_{self.node}").mkdir(mode=0o700)
        self.env.update(
            ID=str(self.node), CLUSTER_SIZE=str(self.size),
            PAX_RUN_DIR=str(self.directory),
            PAXD_BIN=str(self.directory / "bin" / "paxd"),
            PAX_STARTUP_DEADLINE=str(self.deadline),
        )

    def validate_stage(self):
        if self.env.get("PAX_RUN_DIR") != str(self.directory):
            raise StageFailure("stage must be launched by the disposable coordinator")
        metadata = json.loads((self.directory / "run.json").read_text())
        if metadata["run_id"] != self.run_id or metadata["chain_id"] != "pax" or not metadata["disposable"]:
            raise StageFailure("invalid local run context")
        self.check()

    def build_complete(self):
        if self.node != 0:
            raise StageFailure("only coordinator may publish the build")
        source = self.root / "build" / "paxd"
        if source.is_symlink() or not source.is_file() or not os.access(source, os.X_OK):
            raise StageFailure("build/paxd must be a regular executable")
        destination = self.directory / "bin" / "paxd"
        temporary = destination.with_suffix(".pending")
        shutil.copyfile(source, temporary)
        temporary.chmod(0o700)
        os.replace(temporary, destination)
        self.publish("build.complete", {"executable": str(destination)})

    @contextlib.contextmanager
    def build_lock(self):
        lock = self.root / "build" / "localnode-build.lock"
        lock.mkdir(mode=0o700)
        try:
            yield
        finally:
            lock.rmdir()

    def complete_init(self):
        self.publish(f"init.{self.node}.complete", {"node": self.node})

    def wait_init(self):
        self.wait(lambda: all((self.directory / f"init.{node}.complete").is_file()
                              for node in range(self.size)))

    def aggregate_init(self):
        peers, accounts = [], []
        for node in range(self.size):
            directory = self.directory / f"node_{node}"
            peers.append((directory / "persistent_peer.txt").read_text().strip())
            accounts.append((directory / "genesis_account.txt").read_text().strip())
            for gentx in (directory / "gentx").iterdir():
                destination = self.directory / "gentx" / gentx.name
                with destination.open("xb") as output:
                    output.write(gentx.read_bytes())
        for name, lines in (("persistent_peers.txt", peers), ("genesis_accounts.txt", accounts)):
            temporary = self.directory / f".{name}.pending"
            temporary.write_text("\n".join(lines) + "\n")
            os.replace(temporary, self.directory / name)

    def execute(self, stage, script):
        self.stage = stage
        process = subprocess.Popen(["bash", str(self.scripts / script)], env=self.env,
                                   cwd=self.root, start_new_session=True)
        try:
            while process.poll() is None:
                self.check()
                time.sleep(0.05)
            if process.returncode:
                raise StageFailure(f"{stage} exited {process.returncode}")
            self.check()
        finally:
            stop_process(process)


def stop_process(process):
    # The stage can have exited while leaving descendants behind.
    try:
        os.killpg(process.pid, signal.SIGTERM)
    except ProcessLookupError:
        pass
    try:
        process.wait(timeout=2)
    except subprocess.TimeoutExpired:
        pass
    try:
        os.killpg(process.pid, signal.SIGKILL)
    except ProcessLookupError:
        pass
    process.wait()


def ready_status(url, deadline):
    request = urllib.request.Request(url, headers={"Accept": "application/json"})
    # Ignore proxy environment: readiness always queries this node's loopback RPC.
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    with opener.open(request, timeout=max(0.01, min(1, deadline - time.time()))) as response:
        status = json.loads(response.read(1024 * 1024))["result"]
    if status["node_info"]["network"] != "pax":
        raise StageFailure("RPC chain identity is not pax")
    sync = status["sync_info"]
    return sync["catching_up"] is False and int(sync["latest_block_height"]) > 0


def monitor_daemon(run, is_alive, url="http://127.0.0.1:26657/status"):
    marker = run.directory / f"launch.{run.node}.complete"
    launched = False
    run.stage = "launch"
    try:
        while True:
            run.check(startup=not launched)
            if not is_alive():
                raise StageFailure("paxd exited during launch or supervision")
            try:
                healthy = ready_status(url, time.time() + 1 if launched else run.deadline)
            except (urllib.error.URLError, TimeoutError, OSError, ValueError, KeyError, TypeError):
                healthy = False
            if not is_alive():
                raise StageFailure("paxd exited during launch or supervision")
            if healthy:
                if not launched:
                    run.publish(marker.name, {"chain_id": "pax", "node": run.node})
                    launched = True
            elif launched:
                raise StageFailure("paxd readiness lost")
            time.sleep(0.1)
    except BaseException as error:
        marker.unlink(missing_ok=True)
        run.fail(error)
        raise


def supervise(run):
    run.validate_stage()
    interval = int(run.env.get("INVARIANT_CHECK_INTERVAL", "0"))
    if interval < 0:
        raise StageFailure("invariant interval must be nonnegative")
    with (run.directory / "logs" / f"paxd-{run.node}.log").open("ab") as output:
        process = subprocess.Popen([
            run.env["PAXD_BIN"], "start", "--home", run.env["PAX_HOME"],
            "--chain-id", "pax", "--inv-check-period", str(interval),
        ], stdout=output, stderr=subprocess.STDOUT, start_new_session=True)
        try:
            monitor_daemon(run, lambda: process.poll() is None)
        finally:
            stop_process(process)


@contextlib.contextmanager
def private_home(run):
    # Normal exits and handled failures remove the disposable keyring and seed inventory.
    with tempfile.TemporaryDirectory(prefix=f"pax-localnode-{run.node}-") as private:
        os.chmod(private, 0o700)
        run.env.update(PAX_PRIVATE_DIR=private, PAX_HOME=str(Path(private) / ".pax"))
        yield Path(private)


def deploy(run):
    run.join()
    with private_home(run):
        try:
            if run.node == 0:
                with run.build_lock():
                    run.execute("build", "step0_build.sh")
            run.stage = "build"
            run.wait(lambda: (run.directory / "build.complete").is_file())
            run.execute("init", "step1_configure_init.sh")
            run.complete_init()
            if run.node == 0:
                run.wait_init()
                run.aggregate_init()
                run.execute("genesis", "step2_genesis.sh")
                genesis = json.loads((run.directory / "genesis.json").read_text())
                if genesis["chain_id"] != "pax":
                    raise StageFailure("genesis chain identity is not pax")
                run.publish("genesis.complete", {"chain_id": "pax"})
            run.stage = "genesis"
            run.wait(lambda: (run.directory / "genesis.complete").is_file())
            run.execute("config", "step4_config_override.sh")
            run.stage = "launch"
            process = subprocess.Popen(["bash", str(run.scripts / "step5_start_pax.sh")],
                                       env=run.env, cwd=run.root, start_new_session=True)
            try:
                def all_launched():
                    if process.poll() is not None:
                        raise StageFailure(f"launch supervisor exited {process.returncode}")
                    return all((run.directory / f"launch.{node}.complete").is_file()
                               for node in range(run.size))

                run.wait(all_launched)
                run.check()
                print(f"All {run.size} Nodes started successfully.", flush=True)
                while True:
                    run.check(startup=False)
                    if process.poll() is not None:
                        raise StageFailure(f"launch supervisor exited {process.returncode}")
                    time.sleep(0.1)
            finally:
                stop_process(process)
        finally:
            (run.directory / f"launch.{run.node}.complete").unlink(missing_ok=True)


def main():
    os.umask(0o077)
    run = None

    def interrupted(signum, frame):
        raise StageFailure(f"interrupted by signal {signum}")

    signal.signal(signal.SIGTERM, interrupted)
    signal.signal(signal.SIGINT, interrupted)
    try:
        run = Run(Path.cwd(), os.environ)
        action = sys.argv[1]
        if action == "deploy":
            deploy(run)
        elif action == "fail":
            run.stage = sys.argv[2]
            run.fail(f"stage exited {sys.argv[3]}")
        else:
            run.validate_stage()
            if action == "build-complete":
                run.build_complete()
            elif action == "supervise":
                supervise(run)
            elif action != "check-stage":
                raise StageFailure(f"unknown action {action}")
        return 0
    except (StageFailure, OSError, ValueError, KeyError) as error:
        if run is not None:
            run.fail(error)
        print(f"localnode failed: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
