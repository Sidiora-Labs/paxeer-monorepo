import os
from pathlib import Path
import re
import shlex
import subprocess
import tempfile


ROOT = Path(__file__).resolve().parents[4]
TESTS = ROOT / "programs/sdk/c/tests"
PROGRAMS = ROOT / "programs"


def run(command, cwd=ROOT):
    print("signature regression:", shlex.join(map(str, command)), flush=True)
    subprocess.run(list(map(str, command)), cwd=cwd, check=True)


def test_repair_374_import_selection():
    groups = {
        1: {"storage_read", "storage_write", "storage_delete", "event_emit",
            "program_call", "transfer_402", "receipt_read"},
        2: {"response_write", "program_call_response", "refusal_write",
            "storage_read_scoped", "storage_write_scoped", "storage_delete_scoped",
            "storage_drop_scoped", "storage_scan_scoped", "transfer_program_402",
            "fund_program_402", "context_read", "balance_read", "hash",
            "signature_verify", "signature_recover", "bigint_mul_256",
            "bigint_div_256", "bigint_rem_256", "bigint_modexp_256"},
        3: {"oracle_read"},
        4: {"web_read"},
    }
    for selected in range(1, 5):
        source = subprocess.check_output([
            os.environ.get("CLANG", "clang"), "--target=wasm32-unknown-unknown",
            "-E", "-P", "-x", "c", f"-DLXP_PROGRAM_SELECTED_ABI={selected}",
            ROOT / "programs/sdk/c/src/host.h",
        ], text=True)
        imports = re.findall(
            r'import_module\("([^"]+)"\), import_name\("([^"]+)"\)', source)
        expected = {(f"layerx_v{version}", name)
            for version, names in groups.items() if version <= selected
            for name in names}
        if len(imports) != len(expected) or set(imports) != expected:
            raise AssertionError(f"ABI {selected} import selection: {imports!r}")
        print(f"PASS test_repair_374_import_selection abi={selected}", flush=True)


def test_repair_374_signature_status():
    with tempfile.TemporaryDirectory(prefix="layerx-signature-status-") as temporary:
        output = Path(temporary)
        c_guest = output / "c.wasm"
        rust_guest = output / "rust.wasm"
        as_guest = output / "assemblyscript.wasm"
        manifest = PROGRAMS / "Cargo.toml"
        target = PROGRAMS / "target"
        run([
            os.environ.get("CLANG", "clang"), "--target=wasm32-unknown-unknown",
            "-std=c17", "-pedantic", "-Werror", "-Wall", "-Wextra", "-Wconversion",
            "-Wshadow", "-Wvla", "-Oz", "-ffreestanding", "-fno-builtin",
            "-ffunction-sections", "-fdata-sections", "-nostdlib",
            "-I", ROOT / "programs/sdk/c/include", "-Wl,--no-entry",
            "-Wl,--export-memory", "-Wl,--gc-sections", "-Wl,--strip-all",
            "-o", c_guest, *sorted((ROOT / "programs/sdk/c/src").glob("*.c")),
            TESTS / "signature_guest.c",
        ])
        run([
            "cargo", "build", "--locked", "--manifest-path", manifest,
            "--target-dir", target, "--release", "--target", "wasm32-unknown-unknown",
            "-p", "layerx-program-sdk",
        ], cwd=PROGRAMS)
        rust_libraries = target / "wasm32-unknown-unknown/release"
        run([
            "rustc", "--edition=2021", "--crate-type=cdylib",
            "--target=wasm32-unknown-unknown", "-C", "panic=abort", "-C", "opt-level=z",
            "-C", "target-feature=-reference-types", "-C", "link-arg=--compress-relocations",
            "-C", "strip=symbols",
            "-L", "dependency=" + str(rust_libraries / "deps"),
            "--extern", "layerx_program_sdk=" + str(rust_libraries / "liblayerx_program_sdk.rlib"),
            TESTS / "signature_guest.rs", "-o", rust_guest,
        ])
        run([
            os.environ.get("ASC", "asc"), TESTS / "signature_guest.ts",
            "--config", ROOT / "programs/sdk/assemblyscript/asconfig.json",
            "--outFile", as_guest,
        ])
        run(["make", "-s", "build/liblayerx.a"])
        run([
            "cargo", "build", "--locked", "--manifest-path", manifest,
            "--target-dir", target, "--release", "--config", 'profile.release.panic="unwind"',
            "-p", "layerx-programs-sandbox", "--features", "host-ffi",
        ], cwd=PROGRAMS)
        original = (ROOT / "tests/programs/test_language_abi.c").read_text()
        entry = "int main(int argc,char **argv)"
        if original.count(entry) != 1:
            raise AssertionError("kernel language harness entrypoint changed")
        renamed = "int signature_language_reference_main(int argc,char **argv)"
        (output / "signature-language-fixture.inc").write_text(
            renamed + ";\n" + original.replace(entry, renamed, 1)
        )
        native = output / "signature-kernel"
        run([
            os.environ.get("CC", "cc"), "-std=c17", "-pedantic", "-Werror", "-Wall",
            "-Wextra", "-Wconversion", "-Wshadow", "-Wvla", "-fno-strict-aliasing",
            "-ffp-contract=off", "-O2", "-Iinclude", "-Itests/programs", "-I", output,
            TESTS / "signature_kernel.c", "-Wl,--start-group", "build/liblayerx.a",
            target / "release/liblayerx_programs_sandbox.a", "-Wl,--end-group",
            "-lssl", "-lcrypto", "-lsqlite3", "-pthread", "-ldl", "-lm", "-o", native,
        ])
        run([native, c_guest, rust_guest, as_guest])


if __name__ == "__main__":
    test_repair_374_import_selection()
    test_repair_374_signature_status()
