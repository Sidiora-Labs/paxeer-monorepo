"""Check the Redis sidecar wiring without requiring a container runtime."""

from pathlib import Path
import re
import subprocess


SCRIPT = Path(__file__).resolve().parents[1] / "mix-in-builder.sh"
SOURCE = SCRIPT.read_text(encoding="utf-8")


def shell_function(name):
    match = re.search(rf"^{name}\(\) \{{\n(.*?)^\}}$", SOURCE, re.MULTILINE | re.DOTALL)
    assert match is not None, f"missing shell function: {name}"
    return match.group(1).replace("\\\n", " ")


def test_redis_image_is_pinned_beside_database_image():
    assert (
        "readonly DATABASE_IMAGE='postgres:16'\n"
        "readonly REDIS_IMAGE='redis:7.4-alpine'"
    ) in SOURCE


def test_redis_name_preserves_database_tree_process_and_group_identity():
    command = shell_function("run_mix_command")
    assert "database_container=$(database_container_name)" in command
    assert (
        "redis_container=explorer-mix-redis-${database_container#explorer-mix-db-}"
    ) in command
    naming = shell_function("database_container_name")
    assert 'if [ "$reuse_db" -eq 1 ]; then' in naming
    assert '"$(tree_digest)" "${database_group_suffix:-}"' in naming
    assert '"$(tree_digest)" "$$" "${database_group_suffix:-}"' in naming


def test_redis_runs_in_database_network_after_database_readiness():
    start = shell_function("start_sidecar")
    database_ready = start.index("  wait_for_database\n")
    redis_start = start[database_ready:]
    assert re.search(
        r'docker run\s+--detach\s+--name "\$redis_container"\s+'
        r'--network "container:\$database_container"\s+"\$REDIS_IMAGE"',
        redis_start,
    )
    assert redis_start.index('"$REDIS_IMAGE"') < redis_start.index("  wait_for_redis\n")


def test_reuse_db_reuses_running_redis_and_starts_stopped_redis():
    start = shell_function("start_sidecar")
    assert re.search(
        r'if \[ "\$reuse_db" -eq 1 \] && docker container inspect "\$redis_container"'
        r'.*?then\s+if \[ "\$\(docker container inspect --format '
        r"'\{\{\.State\.Running\}\}' "
        r'"\$redis_container"\)" != \'true\' \]; then\s+'
        r'docker start "\$redis_container" >/dev/null\s+fi\s+'
        r'log "reusing the Redis sidecar \$redis_container"\s+else',
        start,
        re.DOTALL,
    )


def test_redis_readiness_requires_pong_and_uses_database_attempt_bound():
    wait = shell_function("wait_for_redis")
    assert "readonly DATABASE_READY_ATTEMPTS=60" in SOURCE
    assert '  attempt=0\n' in wait
    assert (
        'until [ "$(docker exec "$redis_container" redis-cli PING 2>/dev/null)"'
        " = 'PONG' ]; do"
    ) in wait
    assert 'attempt=$((attempt + 1))' in wait
    assert 'if [ "$attempt" -ge "$DATABASE_READY_ATTEMPTS" ]; then' in wait
    assert 'die "the Redis sidecar $redis_container did not become ready"' in wait
    assert "    sleep 1\n" in wait
    assert '"$DATABASE_READY_ATTEMPTS"' in shell_function("wait_for_database")


def test_cleanup_removes_redis_before_database_and_preserves_reused_pair():
    cleanup = shell_function("remove_sidecar")
    assert (
        'if [ "$sidecar_started" -eq 1 ] && [ "$reuse_db" -eq 0 ]'
        ' && [ -n "$database_container" ]; then'
    ) in cleanup
    assert (
        'docker rm --force --volumes "$redis_container" >/dev/null 2>&1 || true\n'
        '    docker rm --force --volumes "$database_container" >/dev/null 2>&1 || true'
    ) in cleanup
    command = shell_function("run_mix_command")
    assert "trap remove_sidecar EXIT" in command
    for signal, status in (("INT", 130), ("TERM", 143), ("HUP", 129)):
        assert f"trap 'remove_sidecar; exit {status}' {signal}" in command


def test_mix_uses_shared_network_and_explicit_account_redis_url():
    mix = shell_function("mix_in_container")
    assert '--network "container:$database_container"' in mix
    assert "--env ACCOUNT_REDIS_URL=redis://127.0.0.1:6379" in mix
    command = shell_function("run_mix_command")
    assert command.index("  start_sidecar\n") < command.index('  mix_in_container run "$@"')


def test_script_has_valid_bash_syntax():
    result = subprocess.run(
        ["bash", "-n", str(SCRIPT)],
        capture_output=True,
        text=True,
        check=False,
    )
    assert result.returncode == 0, result.stderr
