import json
import os
import shlex
import struct
import subprocess
from pathlib import Path

import pytest


ROOT = Path(__file__).resolve().parents[2]
BIN = ROOT / "build/bin"
OPTIONAL_MODULES = {
    "escrow": 2,
    "budget": 3,
    "stream": 4,
    "service": 5,
    "perps": 6,
    "spot": 10,
}
READER_SOURCE = r"""
#include "layerx/lxp_genesis.h"
#include "layerx/lxp_kernel.h"
#include "layerx/lxp_tools.h"

#include <stdio.h>
#include <string.h>

static lxp_genesis_manifest manifest;
static uint8_t arena_bytes[8388608U];

static lxp_result read_modules(void *context, lxp_genesis_cli_action action,
                               lxp_byte_span input, uint8_t root[32])
{
    lxp_genesis_module_plan *plan = context;
    lxp_arena arena;
    if (action != LXP_GENESIS_BUILD) return LXP_ERR_NON_CANONICAL;
    lxp_result status = lxp_genesis_parse(
        input.bytes, input.length, LXP_GENESIS_INPUT_MANIFEST, &manifest);
    if (status == LXP_OK)
        status = lxp_arena_init(&arena, arena_bytes, sizeof(arena_bytes));
    if (status == LXP_OK)
        status = lxp_genesis_verify_signature(&manifest, &arena);
    if (status == LXP_OK)
        status = lxp_genesis_module_plan_resolve(&manifest, plan);
    if (status == LXP_OK)
        memcpy(root, manifest.genesis_state_root, 32U);
    return status;
}

int main(void)
{
    static uint8_t bytes[LXP_GENESIS_MAX_ENCODED_BYTES];
    uint8_t output[LXP_GENESIS_OUTPUT_BYTES];
    lxp_genesis_module_plan plan;
    size_t length = fread(bytes, 1U, sizeof(bytes), stdin);
    if (length == 0U || ferror(stdin) || fgetc(stdin) != EOF) return 1;
    lxp_result status = lxp_genesis_cli_main(
        LXP_GENESIS_BUILD, (lxp_byte_span){bytes, length},
        read_modules, &plan, output);
    if (fwrite(output, 1U, sizeof(output), stdout) != sizeof(output)) return 1;
    if (status != LXP_OK) return 1;
    printf("{\"protocol_version\":%u,\"modules\":[",
           (unsigned)manifest.protocol_version);
    for (size_t i = 0U; i < plan.count; ++i) {
        const lxp_module_iface *module = plan.modules[i];
        printf("%s{\"module\":%u,\"ordinals\":[", i ? "," : "",
               (unsigned)module->module_id);
        for (size_t j = 0U; j < module->activity_type_count; ++j) {
            if ((module->activity_types[j] >> 16U) != module->module_id)
                return 1;
            printf("%s%u", j ? "," : "",
                   (unsigned)(module->activity_types[j] & 65535U));
        }
        printf("]}");
    }
    puts("]}");
    return fflush(stdout) == 0 && !ferror(stdout) ? 0 : 1;
}
"""


def _run(command, *, cwd, input=None):
    result = subprocess.run(
        command, cwd=cwd, input=input, capture_output=True, timeout=30, check=False
    )
    assert result.returncode == 0, (command, result.stdout, result.stderr)
    assert result.stderr == b"", (command, result.stderr)
    return result.stdout


@pytest.fixture(scope="module")
def module_table_reader(tmp_path_factory):
    directory = tmp_path_factory.mktemp("module-table-reader")
    source = directory / "reader.c"
    source.write_text(READER_SOURCE)
    reader = directory / "reader"
    library = ROOT / "build/liblayerx.a"
    _run(
        [
            *shlex.split(os.environ.get("CC", "cc")),
            "-std=c17",
            "-Wall",
            "-Wextra",
            "-Werror",
            "-Iinclude",
            "-Ibuild/generated",
            source,
            ROOT / "cmd/layerx-genesis/lxp_genesis_cli.c",
            library,
            ROOT / "programs/target/debug/liblayerx_programs_sandbox.a",
            library,
            "-lssl",
            "-lcrypto",
            "-pthread",
            "-ldl",
            "-lm",
            "-o",
            reader,
        ],
        cwd=ROOT,
    )
    return reader


@pytest.mark.parametrize(
    ("enabled", "disabled"),
    [
        pytest.param((), (), id="defaults"),
        *[
            pytest.param((name,), (), id=name)
            for name in OPTIONAL_MODULES
        ],
        pytest.param(tuple(OPTIONAL_MODULES), (), id="all-enabled"),
        pytest.param((), tuple(OPTIONAL_MODULES), id="all-disabled"),
    ],
)
def test_registry_and_genesis_module_tables_agree(
    tmp_path, module_table_reader, enabled, disabled
):
    asset_id = bytes([0xA1]) * 32
    parameters = [(b"parameter-version", 1)]
    parameters.extend((b"module-enable:" + name.encode(), 1) for name in enabled)
    parameters.extend((b"module-enable:" + name.encode(), 0) for name in disabled)
    request = b"".join(
        [
            struct.pack(">4sBHIQH", b"LXGB", 1, 3, 42, 1700000000000, len(parameters)),
            *[
                struct.pack(">H32s32s", 7, key, value.to_bytes(32, "big"))
                for key, value in sorted(parameters)
            ],
            struct.pack(
                ">H32s33s16s",
                1,
                b"\x01" + bytes(31),
                b"\x02" + bytes(31) + b"\x03",
                bytes(16),
            ),
            asset_id,
            struct.pack(">I9QQB", 1, 1, 1, 1, 1, 1, 8, 8, 64, 8, 1, 1),
            struct.pack(
                ">I7Q6Q", 1, 1, 1, 2, 4, 1, 1, 100, 100, 1, 1, 10, 1, 1000
            ),
        ]
    )
    request_path = tmp_path / "request.lxgb"
    request_path.write_bytes(request)
    signer_path = tmp_path / "signer.key"
    signer_path.write_bytes(bytes([0x22]) * 32)
    signer_path.chmod(0o600)
    output = tmp_path / "genesis"
    assert _run(
        [BIN / "layerx-genesis-build", request_path, signer_path, output],
        cwd=tmp_path,
    ) == b""

    genesis = (output / "genesis.manifest").read_bytes()
    decoded = _run([module_table_reader], cwd=tmp_path, input=genesis)
    assert len(decoded) > 42
    assert decoded[:6] == b"LXGN\x01\x01"
    assert struct.unpack_from(">i", decoded, 6)[0] == 0
    suffix = struct.unpack_from(">I32sI32sI32sI64s", genesis, len(genesis) - 176)
    assert suffix[::2] == (32, 32, 32, 64)
    assert decoded[10:42] == suffix[1]
    assert decoded[10:42] != bytes(32)
    genesis_table = json.loads(decoded[42:])
    assert genesis_table["protocol_version"] == 3

    registry = json.loads(
        _run(
            [
                BIN / "layerx-module-registry",
                "generate",
                "--network-id", "42",
                "--protocol-version", "3",
                "--asset", asset_id.hex(),
                "--symbol", "LXT",
                "--currency", "LXT",
                "--decimals", "18",
                *[arg for name in enabled for arg in ("--enable-module", name)],
            ],
            cwd=tmp_path,
        )
    )
    assert registry["schema_version"] == 2
    expected_ids = sorted([1, 7, 9, *[OPTIONAL_MODULES[name] for name in enabled]])
    for table in (registry["modules"], genesis_table["modules"]):
        module_ids = [entry["module"] for entry in table]
        assert 8 not in module_ids
        assert sorted(module_ids) == expected_ids
        for entry in table:
            ordinals = entry["ordinals"]
            assert ordinals
            assert ordinals == sorted(set(ordinals))
            assert all(0 < ordinal <= 65535 for ordinal in ordinals)
    assert registry["modules"] == sorted(
        genesis_table["modules"], key=lambda entry: entry["module"]
    )
