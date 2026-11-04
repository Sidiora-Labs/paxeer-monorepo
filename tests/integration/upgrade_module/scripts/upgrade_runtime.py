"""Bounded upgrade-test operations against the selected paxd and its node home."""

import contextlib
from decimal import Decimal, InvalidOperation
import fcntl
import hashlib
import json
import math
import os
from pathlib import Path
import re
import shutil
import signal
import subprocess
import sys
import time
import tomllib
import urllib.error
import urllib.request
import uuid


class CheckFailed(RuntimeError):
    pass


TRANSIENT = (OSError, TimeoutError, urllib.error.URLError)
MAX_INT = 2**63 - 1


def positive(value, label):
    if isinstance(value, bool) or not re.fullmatch(r"[0-9]+", str(value)):
        raise CheckFailed(f"{label} must be a positive int64")
    number = int(value)
    if not 0 < number <= MAX_INT:
        raise CheckFailed(f"{label} must be a positive int64")
    return number


def duration(value, label):
    number = float(value)
    if not math.isfinite(number) or not 0 < number <= 3600:
        raise CheckFailed(f"{label} must be positive and at most 3600 seconds")
    return number


def version_list(value):
    parts = value.split(",")
    if not parts or len(set(parts)) != len(parts) or any(
        not re.fullmatch(r"v[0-9]+\.[0-9]+\.[0-9]+(?:[-+][A-Za-z0-9.-]+)?", part)
        for part in parts
    ):
        raise CheckFailed("invalid upgrade version list")
    return value


def run(command, *, timeout=10, input=None, env=None):
    command = list(command)
    binary = os.environ.get("PAXD_BIN", "paxd")
    home = os.environ.get("PAX_HOME")
    if command[0] == binary and home and "--home" not in command:
        command += ["--home", home]
    result = subprocess.run(command, input=input, text=True, capture_output=True,
                            timeout=timeout, env=env)
    if result.returncode:
        raise CheckFailed(f"{command!r} exited {result.returncode}: {result.stderr.strip()}")
    return result.stdout.strip()


class RPC:
    def __init__(self, address, timeout=1):
        self.address = address.replace("tcp://", "http://").rstrip("/")
        self.timeout = duration(timeout, "RPC timeout")
        self.opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))

    def get(self, path):
        with self.opener.open(self.address + path, timeout=self.timeout) as response:
            data = response.read(4 * 1024 * 1024 + 1)
        if len(data) > 4 * 1024 * 1024:
            raise CheckFailed("RPC response exceeds limit")
        try:
            value = json.loads(data)
        except ValueError as error:
            raise CheckFailed("invalid RPC JSON") from error
        if not isinstance(value, dict) or value.get("error"):
            raise CheckFailed(f"RPC error: {value}")
        return value

    def status(self, chain, node_id=None):
        value = self.get("/status")["result"]
        info, sync = value["node_info"], value["sync_info"]
        if info["network"] != chain or (node_id is not None and info["id"] != node_id):
            raise CheckFailed("RPC identity mismatch")
        if sync["catching_up"] is not False:
            raise OSError("RPC is still catching up")
        if not isinstance(info["id"], str) or not info["id"]:
            raise CheckFailed("RPC node identity missing")
        block_hash = sync["latest_block_hash"]
        if not re.fullmatch(r"[A-Fa-f0-9]{64}", block_hash):
            raise CheckFailed("RPC committed hash missing")
        return {"chain": chain, "node_id": info["id"],
                "height": positive(sync["latest_block_height"], "committed height"),
                "hash": block_hash.upper()}

    def retained(self, checkpoint):
        value = self.get(f'/block?height={positive(checkpoint["height"], "checkpoint")}')["result"]
        header = value["block"]["header"]
        if (value["block_id"]["hash"].upper() != checkpoint["hash"]
                or int(header["height"]) != checkpoint["height"]
                or header["chain_id"] != checkpoint["chain"]):
            raise CheckFailed("lost committed state")


def process_record(pid):
    directory = Path("/proc") / str(pid)
    stat = (directory / "stat").read_text().rsplit(")", 1)[1].split()
    if stat[0] in ("Z", "X"):
        raise ProcessLookupError(pid)
    environment = dict(item.split(b"=", 1) for item in (directory / "environ").read_bytes().split(b"\0")
                       if b"=" in item)
    executable = directory / "exe"
    digest = hashlib.sha256()
    with executable.open("rb") as source:
        for chunk in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(chunk)
    return {"pid": int(pid), "ticks": stat[19], "exe": os.readlink(executable),
            "sha256": digest.hexdigest(),
            "argv": (directory / "cmdline").read_bytes().decode().rstrip("\0").split("\0"),
            "versions": environment.get(b"UPGRADE_VERSION_LIST", b"").decode(),
            "home": environment.get(b"PAX_HOME", b"").decode()}


def same_process(record):
    try:
        return process_record(record["pid"]) == record
    except (OSError, ValueError):
        return False


def stop_process(record, timeout):
    if not same_process(record):
        return
    # pidfd binds signals to this process even if the PID is subsequently recycled.
    try:
        descriptor = os.pidfd_open(record["pid"])
    except ProcessLookupError:
        return
    try:
        if not same_process(record):
            return
        signal.pidfd_send_signal(descriptor, signal.SIGTERM)
        deadline = time.monotonic() + duration(timeout, "process exit timeout")
        while same_process(record):
            if time.monotonic() >= deadline:
                signal.pidfd_send_signal(descriptor, signal.SIGKILL)
                raise CheckFailed("process did not stop within deadline")
            time.sleep(0.01)
    finally:
        os.close(descriptor)


class RestartState:
    def __init__(self, path):
        self.path = Path(path)
        self.path.parent.mkdir(parents=True, exist_ok=True)
        self.lock = self.path.with_suffix(".lock").open("a")
        try:
            fcntl.flock(self.lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except OSError as error:
            self.lock.close()
            raise CheckFailed("another upgrade operation owns this node") from error
        self.data = json.loads(self.path.read_text()) if self.path.exists() else {}

    def save(self):
        temporary = self.path.with_name(f".{self.path.name}.{uuid.uuid4().hex}")
        try:
            with temporary.open("x") as output:
                json.dump(self.data, output)
                output.flush()
                os.fsync(output.fileno())
            temporary.replace(self.path)
            descriptor = os.open(self.path.parent, os.O_DIRECTORY)
            try:
                os.fsync(descriptor)
            finally:
                os.close(descriptor)
        finally:
            temporary.unlink(missing_ok=True)

    def close(self):
        if not self.lock.closed:
            try:
                self.save()
            finally:
                self.lock.close()


def await_progress(rpc, record, chain, node_id, timeout, checkpoint=None, interval=0.1):
    deadline = time.monotonic() + duration(timeout, "progress timeout")
    first = None
    last_error = "no committed progress"
    while time.monotonic() < deadline:
        if not same_process(record):
            raise CheckFailed("process identity changed or exited")
        try:
            current = rpc.status(chain, node_id)
            if checkpoint is not None:
                rpc.retained(checkpoint)
                if current["height"] < checkpoint["height"]:
                    raise CheckFailed("committed height regressed")
            if first is None:
                first = current
            elif current["height"] > first["height"] and current["hash"] != first["hash"]:
                if not same_process(record):
                    raise CheckFailed("process identity changed or exited")
                return current
        except TRANSIENT as error:
            last_error = str(error)
        time.sleep(min(interval, max(0, deadline - time.monotonic())))
    raise CheckFailed(f"committed progress deadline exceeded: {last_error}")


class ManagedNode:
    def __init__(self, state, rpc, command, timeout=30, interval=0.1, attempts=3):
        self.state, self.rpc, self.command = state, rpc, list(command)
        self.timeout = duration(timeout, "restart timeout")
        self.interval = duration(interval, "retry interval")
        self.attempts = positive(attempts, "restart attempts")
        if self.attempts > 20:
            raise CheckFailed("restart attempts exceed 20")
        self.child = None

    def restart(self, versions, logs):
        version_list(versions)
        previous = self.state.data["process"]
        checkpoint = self.state.data["checkpoint"]
        stop_process(previous, self.timeout)
        logs = Path(logs)
        logs.mkdir(parents=True, exist_ok=True)
        stable = logs / f"paxd-{os.environ.get('ID', '0')}.log"
        if stable.exists() or stable.is_symlink():
            stable.rename(stable.with_name(f"{stable.name}.before-{time.time_ns()}"))
        env = dict(os.environ, UPGRADE_VERSION_LIST=versions)
        version = run(self.command + ["version"], timeout=self.timeout, env=env)
        last_error = "no attempt started"
        for attempt in range(1, self.attempts + 1):
            log = logs / f"paxd-{os.environ.get('ID', '0')}-attempt-{time.time_ns()}-{attempt}.log"
            temporary = stable.with_name(f".{stable.name}.{uuid.uuid4().hex}.link")
            started_ns = time.time_ns()
            with log.open("xb") as output:
                self.child = subprocess.Popen(
                    self.command + ["start", "--chain-id", "pax", "--inv-check-period",
                                    str(os.environ.get("INVARIANT_CHECK_INTERVAL", "0"))],
                    env=env, stdout=output, stderr=subprocess.STDOUT, start_new_session=True,
                )
            try:
                record = process_record(self.child.pid)
                if record["versions"] != versions:
                    raise CheckFailed("child upgrade versions differ")
                self.state.data.update(process=record, versions=versions, version=version,
                                       log=str(log), log_offset=0, started_ns=started_ns)
                self.state.save()
                temporary.symlink_to(log.name)
                temporary.replace(stable)
                current = await_progress(
                    self.rpc, record, checkpoint["chain"], checkpoint["node_id"],
                    self.timeout, checkpoint, interval=self.interval,
                )
                self.state.data["checkpoint"] = current
                self.state.save()
                return current
            except (CheckFailed, *TRANSIENT) as error:
                last_error = str(error)
                if self.child.poll() is None:
                    self.child.terminate()
                try:
                    self.child.wait(timeout=self.timeout)
                except subprocess.TimeoutExpired:
                    self.child.kill()
                    self.child.wait(timeout=self.timeout)
                temporary.unlink(missing_ok=True)
                if attempt < self.attempts:
                    time.sleep(self.interval)
        raise CheckFailed(f"restart attempts exhausted: {last_error}")


def sample_target(rpc, seconds_ahead, samples=3, window=3):
    seconds_ahead = positive(seconds_ahead, "seconds ahead")
    samples = positive(samples, "sample count")
    window = duration(window, "sample window")
    for _ in range(samples):
        first = rpc.status("pax")
        started = time.monotonic_ns()
        time.sleep(window)
        current = rpc.status("pax", first["node_id"])
        elapsed = time.monotonic_ns() - started
        blocks = current["height"] - first["height"]
        if blocks > 0 and elapsed > 0 and current["hash"] != first["hash"]:
            blocks_ahead = (seconds_ahead * 1_000_000_000 * blocks + elapsed - 1) // elapsed
            target = current["height"] + blocks_ahead
            return positive(target, "target height")
    raise CheckFailed("chain stalled during bounded target sampling")


def check_tx(response):
    if (not isinstance(response, dict) or type(response.get("code")) is not int
            or response["code"] != 0 or not isinstance(response.get("txhash"), str)
            or not re.fullmatch(r"[A-Fa-f0-9]{64}", response["txhash"])):
        raise CheckFailed(f"vote CheckTx rejected or malformed: {response}")


def yes_vote(committed, proposal_id, voter):
    vote_value = committed.get("vote", committed)
    if (str(vote_value.get("proposal_id")) != str(proposal_id)
            or vote_value.get("voter") != voter):
        raise CheckFailed("committed vote identity mismatch")
    if "options" in vote_value:
        options = vote_value["options"]
        try:
            valid = (len(options) == 1 and options[0]["option"] == "VOTE_OPTION_YES"
                     and Decimal(options[0]["weight"]) == Decimal(1))
        except (KeyError, TypeError, InvalidOperation) as error:
            raise CheckFailed("invalid committed vote weight") from error
    else:
        valid = vote_value.get("option") == "VOTE_OPTION_YES"
    if not valid:
        raise CheckFailed("committed vote is not full YES")


def vote(binary, proposal_id, timeout=60):
    proposal_id = positive(proposal_id, "proposal ID")
    voter = run([binary, "keys", "show", "node_admin", "-a"], input="12345678\n")
    if not voter or len(voter.split()) != 1:
        raise CheckFailed("voter address missing")
    response = json.loads(run(
        [binary, "tx", "gov", "vote", str(proposal_id), "yes", "--from", "node_admin",
         "--chain-id", "pax", "--fees", "2000uhpx", "-b", "sync", "-y", "--output", "json"],
        input="12345678\n", timeout=timeout,
    ))
    check_tx(response)
    deadline = time.monotonic() + timeout
    last_error = "vote not included"
    while time.monotonic() < deadline:
        try:
            committed = json.loads(run(
                [binary, "query", "gov", "vote", str(proposal_id), voter, "--output", "json"],
                timeout=min(5, max(0.01, deadline - time.monotonic())),
            ))
            yes_vote(committed, proposal_id, voter)
            return
        except (CheckFailed, subprocess.TimeoutExpired) as error:
            last_error = str(error)
        time.sleep(min(0.1, max(0, deadline - time.monotonic())))
    raise CheckFailed(f"committed vote deadline exceeded: {last_error}; tx={response['txhash']}")


def query_plan(binary):
    try:
        value = json.loads(run([binary, "query", "upgrade", "plan", "--output", "json"]))
    except CheckFailed as error:
        if "no upgrade scheduled" in str(error):
            return None
        raise
    plan = value.get("plan", value)
    if not isinstance(plan.get("name"), str) or not plan["name"]:
        raise CheckFailed("scheduled upgrade name missing")
    plan["height"] = positive(plan["height"], "scheduled height")
    return plan


def upgrade_event(text, version, height):
    height = positive(height, "target height")
    # Structured logger encodings preserve the same literal message.
    return re.search(re.escape(f'UPGRADE "{version}" NEEDED at height: {height}') + r":",
                     text.replace('\\"', '"')) is not None


def panic_evidence(state, target, text, halt, alive):
    target = positive(target, "target height")
    plan = state.get("plan")
    if not plan or positive(plan["height"], "scheduled height") != target:
        raise CheckFailed("scheduled target mismatch")
    if alive:
        raise CheckFailed("live process or target-1 stall is not panic evidence")
    if upgrade_event(text, plan["name"], target):
        if (halt.get("name") != plan["name"] or int(halt.get("height", 0)) != target
                or not 0 < state["checkpoint"]["height"] < target):
            raise CheckFailed("upgrade halt boundary does not match scheduled plan")
        return
    early = f'BINARY UPDATED BEFORE TRIGGER! UPGRADE "{plan["name"]}" - in binary but not executed on chain'
    if early in text.replace('\\"', '"'):
        info = json.loads(plan.get("info") or "{}")
        if info.get("upgradeType") == "minor":
            raise CheckFailed("minor upgrade cannot prove an early-major panic")
        if (plan["name"] not in state["versions"].split(",")
                or not 0 < state["checkpoint"]["height"] < target):
            raise CheckFailed("early-major process version or boundary mismatch")
        return
    raise CheckFailed("specific scheduled upgrade panic evidence missing")


def fresh_markers(marker, started_ns, count):
    marker = Path(marker)
    if not marker.exists():
        return False
    if marker.stat().st_mtime_ns < started_ns:
        raise CheckFailed("stale launch markers")
    return marker.read_text().splitlines() == ["Done"] * count


def await_launch(child, marker, started_ns, statuses, timeout=300, interval=0.1):
    deadline = time.monotonic() + duration(timeout, "launch timeout")
    initial = {}
    last_error = "incomplete launch markers or node identities"
    while time.monotonic() < deadline:
        code = child.poll()
        if code is not None and code != 0:
            raise CheckFailed(f"cluster launcher exited {code}")
        if fresh_markers(marker, started_ns, len(statuses)) and statuses:
            try:
                current = [status() for status in statuses]
                identities = [value["node_id"] for value in current]
                if len(set(identities)) == len(statuses):
                    progressed = True
                    for value in current:
                        if value["chain"] != "pax":
                            raise CheckFailed("launch RPC chain identity mismatch")
                        previous = initial.setdefault(value["node_id"], value)
                        progressed &= (value["height"] > previous["height"]
                                       and value["hash"] != previous["hash"])
                    if progressed:
                        if child.poll() not in (None, 0):
                            raise CheckFailed(f"cluster launcher exited {child.returncode}")
                        return
            except (*TRANSIENT, CheckFailed, subprocess.TimeoutExpired) as error:
                last_error = str(error)
        time.sleep(min(interval, max(0, deadline - time.monotonic())))
    raise CheckFailed(f"launch readiness deadline exceeded: {last_error}")


def launch(versions=None, timeout=300):
    if versions:
        version_list(versions)
    env = dict(os.environ, INVARIANT_CHECK_INTERVAL="10")
    if versions:
        env["UPGRADE_VERSION_LIST"] = versions
    run(["make", "-f", "chain.mk", "clean"], timeout=timeout)
    marker = Path("build/generated/launch.complete")
    marker.unlink(missing_ok=True)
    # make's target recreates build/generated; keep its failure log outside it.
    logs = Path("build/upgrade-launch-logs")
    logs.mkdir(parents=True, exist_ok=True)
    started = time.time_ns()
    with (logs / f"launch-{started}.log").open("xb") as output:
        child = subprocess.Popen(["make", "docker-cluster-start"], env=env, stdout=output,
                                 stderr=subprocess.STDOUT, start_new_session=True)
    try:
        def node_status(index):
            container = f"pax-node-{index}"
            local_id = run(["docker", "exec", container, "paxd", "tendermint", "show-node-id"], timeout=2)
            status = json.loads(run(["docker", "exec", container, "paxd", "status"], timeout=2))
            info = status.get("NodeInfo", status.get("node_info", {}))
            sync = status.get("SyncInfo", status.get("sync_info", {}))
            if info.get("id") != local_id or info.get("network") != "pax":
                raise CheckFailed("launch node identity mismatch")
            if sync.get("catching_up") is not False or not sync.get("latest_block_hash"):
                raise CheckFailed("launch node has no ready committed block")
            return {"chain": "pax", "node_id": local_id,
                    "height": positive(sync["latest_block_height"], "launch height"),
                    "hash": sync["latest_block_hash"]}

        await_launch(child, marker, started, [lambda i=i: node_status(i) for i in range(4)], timeout=timeout)
        with (logs / f"rpc-{started}.log").open("xb") as output:
            rpc_child = subprocess.Popen(["make", "run-rpc-node-skipbuild"], stdout=output,
                                         stderr=subprocess.STDOUT, start_new_session=True)
        try:
            run([sys.executable, "tests/integration/scripts/runner.py",
                 "tests/integration/startup/startup_test.yaml"], timeout=timeout)
            if rpc_child.poll() not in (None, 0):
                raise CheckFailed(f"RPC launcher exited {rpc_child.returncode}")
        except BaseException:
            with contextlib.suppress(ProcessLookupError):
                os.killpg(rpc_child.pid, signal.SIGTERM)
            try:
                rpc_child.wait(timeout=5)
            except subprocess.TimeoutExpired:
                with contextlib.suppress(ProcessLookupError):
                    os.killpg(rpc_child.pid, signal.SIGKILL)
                rpc_child.wait(timeout=5)
            raise
    except BaseException:
        with contextlib.suppress(ProcessLookupError):
            os.killpg(child.pid, signal.SIGTERM)
        try:
            child.wait(timeout=5)
        except subprocess.TimeoutExpired:
            with contextlib.suppress(ProcessLookupError):
                os.killpg(child.pid, signal.SIGKILL)
            child.wait(timeout=5)
        raise


def discover_process(binary, home):
    expected = str(Path(shutil.which(binary) or binary).resolve())
    candidates = []
    for directory in Path("/proc").iterdir():
        if not directory.name.isdecimal():
            continue
        try:
            record = process_record(int(directory.name))
        except (OSError, ValueError):
            continue
        args = record["argv"]
        if record["exe"] != expected or "start" not in args:
            continue
        homes = [args[index + 1] for index, arg in enumerate(args[:-1]) if arg == "--home"]
        homes += [arg.split("=", 1)[1] for arg in args if arg.startswith("--home=")]
        actual_home = homes[-1] if homes else record["home"] or str(Path.home() / ".pax")
        if Path(actual_home).resolve() == home:
            candidates.append(record)
    if len(candidates) != 1:
        raise CheckFailed(f"expected one paxd process for {home}, found {len(candidates)}")
    return candidates[0]


def rpc_address(home):
    if os.environ.get("PAXD_RPC_URL"):
        return os.environ["PAXD_RPC_URL"]
    config = home / "config/client.toml"
    if config.exists():
        return tomllib.loads(config.read_text())["node"]
    return "http://127.0.0.1:26657"


def read_attempt_log(state):
    path = Path(state["log"])
    with path.open("rb") as source:
        source.seek(state.get("log_offset", 0))
        return source.read().decode(errors="replace")


def main():
    state = None
    try:
        action, *args = sys.argv[1:]
        log_dir = Path(os.environ.get("PAX_RUN_DIR", "build/generated")) / "logs"
        stable_log = log_dir / f"paxd-{os.environ.get('ID', '0')}.log"
        if action == "launch_cluster":
            if len(args) > 1:
                raise CheckFailed("launch_cluster accepts an optional version list")
            launch(args[0] if args else None,
                   duration(os.environ.get("UPGRADE_LAUNCH_TIMEOUT", "300"), "launch timeout"))
            return 0
        if action == "verify_upgrade_needed_log":
            if len(args) != 2:
                raise CheckFailed("expected target height and version")
            if not upgrade_event(stable_log.read_text(), args[1], args[0]):
                raise CheckFailed("exact UPGRADE NEEDED event missing; a target-1 stall is not evidence")
            print("PASS")
            return 0
        binary = os.environ.get("PAXD_BIN", "paxd")
        home = Path(os.environ.get("PAX_HOME", str(Path.home() / ".pax"))).resolve()
        os.environ["PAX_HOME"] = str(home)
        timeout = duration(os.environ.get("PAXD_PROGRESS_TIMEOUT_SECONDS", "60"), "progress timeout")
        rpc = RPC(rpc_address(home))
        if action == "proposal_target_height":
            if len(args) != 1:
                raise CheckFailed("expected seconds ahead")
            print(sample_target(
                rpc, args[0],
                samples=os.environ.get("PAXD_TARGET_SAMPLES", "3"),
                window=os.environ.get("PAXD_TARGET_SAMPLE_WINDOW_SECONDS", "3"),
            ))
            return 0
        if action == "proposal_vote":
            if len(args) != 1:
                raise CheckFailed("expected proposal ID")
            vote(binary, args[0].strip("'\""), timeout)
            print(0)
            return 0
        if action not in ("paxd_upgrade", "paxd_downgrade", "verify_running", "verify_panic"):
            raise CheckFailed(f"unknown action {action}")
        state = RestartState(home / "data/upgrade-test-state.json")
        if not state.data:
            record = discover_process(binary, home)
            local_id = run([binary, "tendermint", "show-node-id"])
            checkpoint = rpc.status("pax", local_id)
            state.data = {"process": record, "versions": version_list(record["versions"]),
                          "version": run([binary, "version"]), "checkpoint": checkpoint,
                          "log": str(stable_log.resolve()), "log_offset": stable_log.stat().st_size,
                          "started_ns": time.time_ns()}
            state.save()
        record = state.data["process"]
        if action == "verify_panic":
            if len(args) != 1:
                raise CheckFailed("expected scheduled target height")
            deadline = time.monotonic() + timeout
            while same_process(record) and time.monotonic() < deadline:
                try:
                    state.data["checkpoint"] = rpc.status("pax", state.data["checkpoint"]["node_id"])
                    plan = query_plan(binary)
                    if plan:
                        state.data["plan"] = plan
                    state.save()
                except TRANSIENT:
                    pass
                time.sleep(0.1)
            if same_process(record):
                raise CheckFailed("panic deadline exceeded; live target-1 stalls fail")
            text = read_attempt_log(state.data)
            halt_path = home / "data/upgrade-info.json"
            halt = {}
            if halt_path.exists():
                if halt_path.stat().st_mtime_ns < state.data["started_ns"]:
                    raise CheckFailed("stale halt file")
                halt = json.loads(halt_path.read_text())
            panic_evidence(state.data, args[0], text, halt, False)
        elif action == "verify_running":
            if args:
                raise CheckFailed("verify_running accepts no arguments")
            if not (record["versions"] == state.data["versions"]
                    and run([binary, "version"]) == state.data["version"]):
                raise CheckFailed("expected process/version mismatch")
            local_id = run([binary, "tendermint", "show-node-id"])
            if local_id != state.data["checkpoint"]["node_id"]:
                raise CheckFailed("local node identity changed")
            state.data["checkpoint"] = await_progress(
                rpc, record, "pax", local_id, timeout, state.data["checkpoint"],
            )
            state.data["plan"] = query_plan(binary)
            state.save()
        else:
            versions = state.data["versions"].split(",")
            if action == "paxd_upgrade":
                if len(args) != 1:
                    raise CheckFailed("expected upgrade version")
                versions.append(args[0])
            elif args or len(versions) < 2:
                raise CheckFailed("downgrade requires a retained predecessor version")
            else:
                versions.pop()
            plan = query_plan(binary) if same_process(record) else state.data.get("plan")
            if plan:
                state.data["plan"] = plan
                state.save()
            node = ManagedNode(
                state, rpc, [binary, "--home", str(home)], timeout=timeout,
                interval=os.environ.get("PAXD_RESTART_RETRY_SECONDS", "1"),
                attempts=os.environ.get("PAXD_RESTART_MAX_ATTEMPTS", "3"),
            )
            node.restart(",".join(versions), log_dir)
        print("PASS")
        return 0
    except (CheckFailed, OSError, ValueError, KeyError, TypeError, subprocess.TimeoutExpired) as error:
        print(f"upgrade check failed: {error}", file=sys.stderr)
        return 1
    finally:
        if state is not None:
            state.close()


if __name__ == "__main__":
    sys.exit(main())
