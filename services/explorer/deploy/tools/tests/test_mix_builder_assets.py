"""Check page asset preparation and its application selection."""

import re
import subprocess

from test_mix_builder_redis import SCRIPT, SOURCE, shell_function


ROOT = SCRIPT.parents[4]
PREPARE = ROOT / "docker/explorer-elixir-builder/build-web-assets.sh"


def selected(*args):
    function = "web_assets_needed() {\n" + shell_function("web_assets_needed") + "\n}\n"
    result = subprocess.run(
        ["sh", "-c", function + 'web_assets_needed "$@"', "assets", *args],
        capture_output=True,
        text=True,
        check=False,
    )
    assert result.returncode in (0, 1), result.stderr
    return result.returncode == 0


def test_web_tests_prepare_assets_for_directory_file_and_umbrella_calls():
    assert selected("test")
    assert selected("test", "--seed", "100")
    assert selected("test", "apps/block_scout_web/test")
    assert selected("test", "apps/block_scout_web/test/example_test.exs:12")
    assert selected("test", "apps/explorer/test", "apps/block_scout_web/test")


def test_other_application_tests_and_mix_commands_do_not_prepare_assets():
    assert not selected("compile")
    assert not selected("format", "apps/block_scout_web/lib/example.ex")
    assert not selected("test", "apps/explorer/test")
    assert not selected("test", "apps/ethereum_jsonrpc/test/example_test.exs", "--seed", "100")


def test_assets_are_selected_independently_of_the_browser_driver_override():
    assert "MIX_IN_BUILDER_BROWSER_DRIVER" not in shell_function("web_assets_needed")
    invocation = shell_function("mix_in_container")
    assert '--env MIX_IN_BUILDER_WEB_ASSETS="$web_assets"' in invocation
    assert '--env BLOCKSCOUT_STATIC_DIR="$build_path/web-assets/$mix_env/static"' in invocation
    command = shell_function("run_mix_command")
    assert 'if web_assets_needed "$@"; then' in command
    assert 'web_assets=1' in command
    assert 'web_assets=0' in command


def test_assets_are_built_from_the_lock_before_migrations_and_test_start():
    source = PREPARE.read_text(encoding="utf-8")
    assert source.index("npm ci --no-audit --no-fund") < source.index("npm run deploy")
    assert "build-web-assets" in SOURCE
    assert SOURCE.index("  build-web-assets\n") < SOURCE.index("  mix ecto.create --quiet\n")
    assert SOURCE.index("  mix ecto.migrate\n") < SOURCE.index('\nrun_mix_with_retry "$@"')


def test_assets_and_their_input_digest_are_retained_in_the_build_volume():
    source = PREPARE.read_text(encoding="utf-8")
    assert 'cache_dir=$MIX_BUILD_PATH/web-assets/$MIX_ENV' in source
    assert 'export BLOCKSCOUT_STATIC_DIR=$cache_dir/static' in source
    assert 'digest=$(assets_digest)' in source
    assert '[ "$(cat "$cache_dir/inputs")" = "$digest" ]' in source
    assert source.index("reusing web page assets") < source.index("npm ci")
    assert re.search(r"npm run deploy.*printf '%s' \"\$digest\" > \"\$cache_dir/inputs\"", source, re.DOTALL)
    assert "./node_modules/*" in source
    assert '/app/mix.lock' in source
    assert 'node --version' in source
    assert 'npm --version' in source


def test_compiler_tools_are_required_without_excluding_cases():
    assert "--exclude compiler" not in SOURCE
    assert "required compiler tools are unavailable" in SOURCE
    assert "command -v node" in shell_function("compiler_binaries_available")
    assert "command -v vyper" in shell_function("compiler_binaries_available")


def test_builder_recipe_and_asset_preparation_are_image_inputs():
    inputs = shell_function("image_input_digest")
    assert '"$repo_root"/docker/explorer-elixir-builder/*' in inputs
    dockerfile = (PREPARE.parent / "Dockerfile").read_text(encoding="utf-8")
    assert "COPY docker/explorer-elixir-builder/build-web-assets.sh /usr/local/bin/build-web-assets" in dockerfile
    assert "node:22.16.0-alpine3.22" in dockerfile
    assert "rust:1.85.1-alpine3.21" in dockerfile
    assert "solc@0.8.17" in dockerfile
    assert "vyper==0.4.3" in dockerfile
    assert "cargo build --locked --release" in dockerfile
