"""Source contracts and real local shell paths for explorer deployment repairs.

No Node, compiler, package install, network endpoint or deployment is required.
The JSON encoder and atomic filesystem API contract are checked in source;
executing that Node/browser path remains review evidence, not a claimed gate.
"""

from pathlib import Path
import re
import subprocess
import tomllib

import pytest


ROOT = Path(__file__).resolve().parents[3]
SCRIPTS = ROOT / "apps/explorer/deploy/scripts"
ENV = {"PATH": "/usr/bin:/bin", "LC_ALL": "C.UTF-8"}


def source(name):
    return (SCRIPTS / name).read_text(encoding="utf-8")


def bash(args, cwd, env=None, text=None, binary=False):
    return subprocess.run(
        ["bash", *args],
        cwd=cwd,
        env=ENV | (env or {}),
        input=text.encode("utf-8") if binary and text is not None else text,
        capture_output=True,
        text=not binary,
        timeout=10,
    )


def function(name, script):
    match = re.search(rf"^{name}\(\) \{{\n.*?^\}}", script, re.M | re.S)
    assert match, f"Missing production function {name}"
    return match.group()


@pytest.mark.parametrize(
    "name",
    ["download_assets.sh", "entrypoint.sh", "make_envs_script.sh", "collect_envs.sh"],
)
def test_shell_syntax(name, tmp_path):
    result = bash(["-n", str(SCRIPTS / name)], tmp_path)
    assert result.returncode == 0, result.stderr


def test_asset_copy_and_write_failures_are_guarded_and_collected():
    script = source("download_assets.sh")
    body = function("download_and_save_asset", script)
    for command in (
        'cp "${url#file://}" "$destination"',
        "printf '%s\\n' \"$json_content\" > \"$destination\"",
        "curl -f -s --connect-timeout 10 --max-time 60 --retry 3 --retry-delay 3 --retry-max-time 180 -o \"$destination\" \"$url\"",
    ):
        assert re.search(r"if ! " + re.escape(command) + r"; then\n.*?return 1\n\s*fi", body, re.S)
    assert 'if ! mkdir -p "$ASSETS_DIR"; then' in script
    assert '! download_and_save_asset "$env_var" "$url" "$filename"; then\n        FAILED_ASSETS+=("$env_var")' in script
    assert re.search(r'if \[ "\$\{#FAILED_ASSETS\[@\]\}" -gt 0 \]; then\n.*?\$\{FAILED_ASSETS\[\*\]\}.*?\n    exit 1', script, re.S)
    assert "if [ $? -eq 0 ]" not in body


def test_asset_failures_report_all_required_assets_and_keep_processing(tmp_path):
    assets = tmp_path / "assets"
    assets.mkdir()
    # A directory at the generated-file destination forces a real write error,
    # including when qualification runs as root.
    (assets / "footer_links.json").mkdir()
    logo = tmp_path / "logo.svg"
    logo.write_text("<svg/>\n", encoding="utf-8")
    result = bash(
        [str(SCRIPTS / "download_assets.sh"), str(assets)],
        tmp_path,
        {
            "NEXT_PUBLIC_MARKETPLACE_CONFIG_URL": f"file://{tmp_path}/missing.json",
            "NEXT_PUBLIC_FOOTER_LINKS": "{'title':'footer'}",
            "NEXT_PUBLIC_NETWORK_LOGO": f"file://{logo}",
        },
    )
    assert result.returncode == 1
    assert "2 asset preparation(s) failed" in result.stderr
    assert "NEXT_PUBLIC_MARKETPLACE_CONFIG_URL NEXT_PUBLIC_FOOTER_LINKS" in result.stderr
    assert (assets / "network_logo.svg").read_bytes() == logo.read_bytes()
    assert "✅ Done." not in result.stdout
    assert "[+] NEXT_PUBLIC_FOOTER_LINKS" not in result.stdout
    assert "[+] NEXT_PUBLIC_MARKETPLACE_CONFIG_URL" not in result.stdout


def test_asset_success_and_directory_creation_failure(tmp_path):
    assets = tmp_path / "assets with spaces"
    result = bash(
        [str(SCRIPTS / "download_assets.sh"), str(assets)],
        tmp_path,
        {"NEXT_PUBLIC_FOOTER_LINKS": "{'title':'footer'}"},
    )
    assert result.returncode == 0, result.stderr
    assert (assets / "footer_links.json").read_text() == '{"title":"footer"}\n'
    blocked = tmp_path / "file"
    blocked.write_text("occupied")
    result = bash([str(SCRIPTS / "download_assets.sh"), str(blocked / "assets")], tmp_path)
    assert result.returncode == 1
    assert "unable to create assets directory" in result.stderr


def test_entrypoint_env_failure_reports_path_before_application_exec(tmp_path):
    script = source("entrypoint.sh")
    match = re.search(r'envs_output=".*?\nfi\n', script, re.S)
    assert match
    guard = match.group()
    assert 'envs_output="./public/assets/envs.js"' in guard
    assert 'if ! ./make_envs_script.sh "$envs_output"; then' in guard
    assert "runtime environment script $envs_output" in guard
    assert '>&2\n  exit 1\nfi' in guard
    assert script.index(guard) < script.index('exec "$@"')
    assert 'exec "$@"' not in script[: script.index(guard)]
    # Use the actual generator. Missing public/assets rejects the write before
    # the Node encoder, and the real entrypoint guard must prevent app exec.
    (tmp_path / "make_envs_script.sh").symlink_to(SCRIPTS / "make_envs_script.sh")
    tail = script[script.index('echo "Starting Next.js application"') :]
    result = bash(
        ["-s", "--", "bash", "-c", "printf APPLICATION_EXECUTED"],
        tmp_path,
        text=guard + tail,
    )
    assert result.returncode == 1
    assert "./public/assets/envs.js" in result.stderr
    assert "The application cannot start." in result.stderr
    assert "APPLICATION_EXECUTED" not in result.stdout
    assert "Starting Next.js application" not in result.stdout


def test_runtime_precedence_uses_real_collected_defaults(tmp_path):
    docs = tmp_path / "ENVS.md"
    docs.write_text("NEXT_PUBLIC_RPC_URL NEXT_PUBLIC_EMPTY NEXT_PUBLIC_DEFAULT")
    result = bash(
        [str(SCRIPTS / "collect_envs.sh"), str(docs)],
        tmp_path,
        {
            "NEXT_PUBLIC_RPC_URL": "https://packaged.invalid/rpc?key=a=b",
            "NEXT_PUBLIC_EMPTY": "packaged",
            "NEXT_PUBLIC_DEFAULT": 'quotes " and backslash \\ and = preserved',
        },
    )
    assert result.returncode == 0, result.stderr
    assert "NEXT_PUBLIC_RPC_URL=https://packaged.invalid/rpc?key=a=b" in (tmp_path / ".env").read_text()
    loader = function("load_defaults", source("make_envs_script.sh"))
    result = bash(
        ["-s"],
        tmp_path,
        {"NEXT_PUBLIC_RPC_URL": "https://injected.invalid/rpc", "NEXT_PUBLIC_EMPTY": ""},
        text=loader + '\nload_defaults || exit 1\nprintf "%s\\0" "$NEXT_PUBLIC_RPC_URL" "$NEXT_PUBLIC_EMPTY" "$NEXT_PUBLIC_DEFAULT"\n',
    )
    assert result.returncode == 0, result.stderr
    assert result.stdout.split("\0") == [
        "https://injected.invalid/rpc",
        "",
        'quotes " and backslash \\ and = preserved',
        "",
    ]
    assert "explicit empty strings" in source("collect_envs.sh")
    assert '[[ ! -v "$name" ]]' in loader
    assert 'if ! export "$name=$value"; then' in loader
    assert "source .env" not in source("make_envs_script.sh")


def test_runtime_quotes_backslashes_and_multiline_survive_default_loading(tmp_path):
    value = '"quoted" \\path\\tail\r\nsecond line\nNEXT_PUBLIC_TEXT=not a key\n\u2028\u2029 ☃ $(touch unwanted)'
    (tmp_path / ".env").write_text("NEXT_PUBLIC_TEXT=conflicting default\n")
    loader = function("load_defaults", source("make_envs_script.sh"))
    result = bash(
        ["-s"],
        tmp_path,
        {"NEXT_PUBLIC_TEXT": value},
        text=loader + '\nload_defaults || exit 1\nprintf "%s\\0" "$NEXT_PUBLIC_TEXT"\n',
        binary=True,
    )
    assert result.returncode == 0, result.stderr
    assert result.stdout == value.encode("utf-8") + b"\0"
    assert not (tmp_path / "unwanted").exists()


def test_runtime_env_js_encoding_and_atomic_publish_source_contract():
    script = source("make_envs_script.sh")
    javascript = script.split("<<'JS'\n", 1)[1].split("\nJS\n", 1)[0]
    assert "Object.fromEntries(Object.entries(process.env).filter(([name]) =>" in javascript
    assert "name.startsWith('NEXT_PUBLIC_') && !name.startsWith('NEXT_PUBLIC_VERCEL')" in javascript
    assert 'const script = `window.__envs = ${JSON.stringify(envs)};\\n`;' in javascript
    assert "replace" not in javascript
    assert "const outputFile = process.argv[2];" in javascript
    assert "fs.mkdtempSync(path.join(path.dirname(outputFile), '.envs-'))" in javascript
    write = "fs.writeFileSync(temporaryFile, script, { encoding: 'utf8', mode: 0o644, flag: 'wx' });"
    rename = "fs.renameSync(temporaryFile, outputFile);"
    assert write in javascript
    assert rename in javascript
    assert javascript.index(write) < javascript.index(rename) < javascript.index("} catch (error)")
    assert "process.exitCode = 1;" in javascript
    assert "finally {" in javascript
    assert "fs.rmSync(temporaryDir, { recursive: true, force: true })" in javascript
    assert 'if ! node - "$output_file"' in script
    assert 'then\n    echo "Failed to generate runtime environment script $output_file." >&2\n    exit 1' in script
    assert "truncate" not in script
    assert "touch $output_file" not in script


def test_repair_record_maps_all_findings_to_executed_tests():
    record = tomllib.loads((ROOT / "tools/qualification/audit_repairs/repair_22_33_498.kvx").read_text())
    findings = record["finding"]
    assert set(findings) == {
        "asset-copy-failure-hidden",
        "entrypoint-env-failure",
        "runtime-env-precedence",
        "runtime-env-js-encoding",
    }
    for finding in findings.values():
        for name in finding["tests"]:
            assert callable(globals().get(name)), name
    assert "Node/browser" in record["task"]["22.33.498"]["review_only"]
