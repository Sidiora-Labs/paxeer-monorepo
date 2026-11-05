import ast
import re
import subprocess
from pathlib import Path

from programme import decisions
from programme.kvx import load

ROOT = Path(__file__).resolve().parents[4]
PROGRAMME = ROOT / "spec/layerx-merge/programme"
SPEC_PATH = ROOT / "spec/layerx-merge/spec.kvx"
CITATION = re.compile(r"(?:([\w./-]+\.[A-Za-z]+))?:(\d+)(?:-(\d+))?")
BASIS_CITATIONS = (
    (1, "modules/lxphost/ctx.go", 1, "cgo"),
    (1, "modules/lxphost/ctx.go", 50, 'import "C"'),
    (1, "src/modules/programs/registration.c", 155, "lxp_module_ctx"),
    (1, "precompiles/perps/perps.go", 25, "PerpsAddress"),
    (1, "modules/layerxexchange/types/ids.go", 15, "ExchangeAddress"),
    (2, "include/layerx/lxp_genesis.h", 148, "subaccount_kind"),
    (2, "include/layerx/lxp_genesis.h", 149, "parent_account_id"),
    (2, "modules/perps/keeper/account.go", 20, "GetAccount"),
    (2, "modules/perps/keeper/account.go", 51, "Deposit"),
    (3, "include/layerx/lxp_authority.h", 12, "LXP_AUTHORITY_OWNER"),
    (3, "include/layerx/lxp_authority.h", 19, "LXP_AUTHORITY_KIND_TIMELOCK"),
    (3, "include/layerx/lxp_authority.h", 13, "LXP_AUTHORITY_SESSION_KEY"),
    (4, "node/upgrades.go", 457, "mergeUpgradePlan"),
    (5, "node/upgrades.go", 419, "activationStoreUpgrades"),
    (5, "node/upgrades.go", 421, "Added"),
    (6, "precompiles/common/precompiles.go", 108, "WithLXPReasons"),
    (6, "precompiles/common/precompiles.go", 131, "RevertReason"),
    (7, "include/layerx/lxp_batch.h", 32, "timestamp_ms"),
    (7, "modules/evm/keeper/params.go", 310, "GetMaxFeeTokenRateAge"),
    (8, "include/layerx/lx_perps.h", 27, "LX_PERPS_BOOK_CAPACITY"),
    (8, "include/layerx/lx_perps.h", 34, "LX_PERPS_DISPATCH_MAX_FILLS"),
    (8, "modules/spot/types/keys.go", 14, "PECORAddress"),
    (8, "modules/spot/types/keys.go", 26, "PECORVaultAddress"),
    (8, "modules/launchpad/keeper/swap.go", 32, "FeeRecorded"),
    (9, "src/modules/programs/deploy.c", 239, "lxp_programs_metering_schedule_current"),
    (9, "src/modules/programs/deploy.c", 223, "validate_wasm"),
    (10, "services/interop/specs/conformance/x402/README.md", 9, "PaymentPayload"),
    (11, "modules/xweb/keeper/program_fulfil.go", 191, "splitProgramFee"),
    (11, "modules/xweb/keeper/program_denom.go", 16, "ProgramFeeDenom"),
    (11, "modules/xweb/keeper/request.go", 59, "SendCoins"),
    (12, "modules/xweb/types/observation.go", 11, "ObservationMaxSignatures"),
    (12, "modules/xweb/keeper/program_fulfil.go", 41, "ObservationMaxSignatures"),
    (12, "modules/xweb/keeper/program_fulfil.go", 47, "ObservationMaxSignatures"),
    (12, "modules/xweb/types/state.go", 213, "MaxAttestorSetSize"),
    (12, "modules/xweb/types/keys.go", 21, "MaxAttestors"),
    (12, "modules/xweb/keeper/fulfil.go", 21, "MaxAttestors"),
    (12, "modules/xweb/keeper/program_fulfil.go", 124, "KernelNetworkId"),
    (12, "modules/xweb/types/state.go", 106, "KernelNetworkId"),
    (
        13,
        "programs/crates/layerx-programs-interpreter/Cargo.toml",
        2,
        "layerx-programs-interpreter",
    ),
    (
        13,
        "programs/crates/layerx-programs-market/Cargo.toml",
        2,
        "layerx-programs-market",
    ),
    (
        13,
        "programs/crates/layerx-programs-ai-market/Cargo.toml",
        2,
        "layerx-programs-ai-market",
    ),
    (14, "contracts/src/BatchCallAndSponsor.sol", 135, "executeSponsored"),
    (
        14,
        "services/interop/crates/layerx-gas-station/src/tx.rs",
        245,
        "executeSponsored",
    ),
    (15, "modules/sequencer/keeper/receipt_keys.go", 77, "ReceiptKeys"),
    (16, "precompiles/common/lxp_errors.go", 303, "LXP_ADDR_PANIC"),
    (16, "spec/layerx-merge/spec.kvx", 147, "Application level"),
    (17, "services/agent/schema/agent-api/v1.kvx", 3, "major = 2"),
    (18, "consensus/internal/consensus/types/round_state.go", 201, "SetLeaderPin"),
    (18, "consensus/internal/consensus/types/round_state.go", 451, "LeaderPin) Leader"),
    (18, "node/legacyabci/begin_block.go", 61, "slashing.BeginBlocker"),
    (19, "node/abci.go", 190, "DeliverTx("),
    (19, "node/abci.go", 251, "DeliverTxBatch("),
    (19, "spec/layerx-merge/spec.kvx", 168, "OCC micro-batches"),
    (20, "precompiles/setup.go", 66, "MergeAddressTable"),
    (20, "node/precompiles.go", 20, "layerxagent/clearing"),
    (20, "precompiles/setup.go", 134, "newMergePrecompiles"),
    (20, "node/upgrades.go", 553, "mergePrecompileGate"),
    (20, "services/wallet/gateway/src/precompiles.ts", 18, "orob"),
    (21, "precompiles/setup.go", 212, "layerxanchor"),
    (21, "precompiles/layerxanchor/layerxanchor.go", 184, "StoreDeleteUpgrade"),
    (22, "precompiles/setup.go", 66, "MergeAddressTable"),
    (22, "precompiles/layerxagent/orob/merge.go", 16, "RegisterMergeConstructor"),
    (22, "precompiles/layerxagent/clearing/merge.go", 16, "RegisterMergeConstructor"),
    (22, "precompiles/layerxagent/oracle/merge.go", 19, "RegisterMergeConstructor"),
    (22, "precompiles/layerxagent/pofq/merge.go", 16, "RegisterMergeConstructor"),
    (22, "node/precompiles.go", 20, "layerxagent/clearing"),
    (22, "precompiles/layerxagent/clearing/clearing.go", 22, "computeClearing"),
    (22, "node/upgrades.go", 561, "layerxagenttypes.ModuleName"),
    (23, "precompiles/common/precompiles.go", 308, "DynamicGasPrecompile"),
    (23, "precompiles/common/precompiles.go", 108, "WithLXPReasons"),
    (
        24,
        "src/modules/service/lx_service_dispute.c",
        87,
        "lx_service_dispute_resolve_execute",
    ),
    (24, "src/modules/service/lx_service_dispute.c", 114, "escrow_link.arbiter"),
    (24, "src/modules/service/lx_service_dispute.c", 115, "authority->principal"),
    (
        24,
        "src/modules/service/lx_service_dispute.c",
        116,
        "LXP_ERR_UNAUTHORIZED_ARBITER",
    ),
    (25, "precompiles/layerxexchange/layerxexchange.go", 52, "WithdrawMarginMethod"),
    (25, "modules/perps/keeper/account.go", 20, "GetAccount"),
    (25, "modules/perps/keeper/account.go", 81, "Withdraw("),
    (26, "modules/xweb/keeper/fulfil.go", 17, "verifySignatures"),
    (27, "services/interop/specs/conformance/x402/README.md", 9, "PaymentPayload"),
    (28, "modules/xweb/types/errors.go", 43, "LXP_XWEB_UNAUTHORIZED"),
    (29, "precompiles/setup.go", 76, "streams"),
    (29, "include/layerx/lx_stream.h", 26, "LX_STREAM_ID_PAYLOAD_BYTES"),
    (30, "precompiles/common/precompiles.go", 467, "DefaultGasCost"),
    (30, "precompiles/perps/perps.go", 2, "fixed gas"),
    (31, "precompiles/layerxagent/clearing/clearing.go", 22, "computeClearing"),
    (32, "precompiles/pricefeed/pricefeed.go", 23, "PriceFeedAddress"),
    (32, "precompiles/perps/switch_test.go", 711, "pushOracle"),
    (33, "modules/xweb/keeper/fulfil.go", 17, "verifySignatures"),
)


def test_records_render_from_programme() -> None:
    index = decisions.load_index()
    spec = load(SPEC_PATH)
    for identifier in range(1, 34):
        entry = index[identifier]
        record = ROOT / entry["record"]
        rendered = decisions.render_record(entry, spec)
        assert record.read_bytes() == rendered.encode("utf-8"), record
        title = spec[f"decision.{identifier}"]["title"]
        assert rendered.startswith(f"# D{identifier}: {title}\n")
        assert f"Carry status: `{entry['carry']}`." in rendered
        assert (
            f"## Code basis\n\nBasis revision: `{entry['basis_revision']}`.\n\n"
            f"{entry['basis']}\n\n"
        ) in rendered
        value = (
            entry["value"]
            if "value" in entry
            else spec[f"decision.{identifier}"]["value"]
        )
        assert f"## Value\n\n{value}\n\n" in rendered
        headings = [line for line in rendered.splitlines() if line.startswith("## ")]
        assert headings == [
            "## Value",
            "## Code basis",
            "## Affects",
            "## Bound tasks",
        ]
        assert rendered.endswith("[Decision index](INDEX.md)\n")

    definitions = []
    for module in PROGRAMME.rglob("*.py"):
        tree = ast.parse(module.read_text(encoding="utf-8"))
        for node in ast.walk(tree):
            if (
                isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef))
                and node.name == "render_record"
            ):
                definitions.append(module)
    assert definitions == [PROGRAMME / "decisions.py"]


def test_basis_citations_resolve() -> None:
    index = decisions.load_index()
    cited = set()
    for identifier in range(1, 34):
        entry = index[identifier]
        assert re.fullmatch(r"[0-9a-f]{40}", entry["basis_revision"])
        matches = list(CITATION.finditer(entry["basis"]))
        assert matches, f"D{identifier} has no code citations"
        path = None
        for match in matches:
            path = match[1] or path
            assert path is not None, f"D{identifier} has a citation without a path"
            start = int(match[2])
            end = int(match[3] or match[2])
            assert 1 <= start <= end
            cited.update(
                (identifier, path, line) for line in range(start, end + 1)
            )
    assert {identifier for identifier, _, _, _ in BASIS_CITATIONS} == set(range(1, 34))
    assert cited == {
        (identifier, path, line) for identifier, path, line, _ in BASIS_CITATIONS
    }

    sources = {}
    for identifier, path, line, symbol in BASIS_CITATIONS:
        revision = index[identifier]["basis_revision"]
        key = (revision, path)
        if key not in sources:
            sources[key] = subprocess.run(
                ["git", "show", f"{revision}:{path}"],
                cwd=ROOT,
                check=True,
                capture_output=True,
                text=True,
            ).stdout.splitlines()
        lines = sources[key]
        assert 1 <= line <= len(lines), (identifier, revision, path, line)
        assert symbol in lines[line - 1], (identifier, revision, path, line, symbol)

    basis = index[24]["basis"]
    assert (
        "already refuses every caller other than the linked escrow's arbiter" in basis
    )
    assert "LXP_ERR_UNAUTHORIZED_ARBITER" in basis
    assert "src/modules/service/lx_service_dispute.c:114-116" in basis
    assert "ruling is in force" in basis
