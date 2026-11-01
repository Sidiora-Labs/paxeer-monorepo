from pathlib import Path

from programme.decisions import Decision, load_index
from programme.kvx import load

ROOT = Path(__file__).resolve().parents[4]
SPEC = load(ROOT / "spec/layerx-merge/spec.kvx")
HEAD_BASIS_REVISION = "7fd177486f7544342d1b9a7a97f6fc8053f99396"

# The task's exact path scope places the renderer here rather than decisions.py.
HEAD_BASES = {
    12: (
        "The carried admission limit is 16 signers (spec/layerx-merge/spec.kvx:117). "
        "The observation's NetworkID comes from the EVM chain id "
        "(modules/xweb/keeper/fulfil.go:72)."
    ),
    13: (
        "The reference guest crates are declared in "
        "programs/crates/layerx-programs-interpreter/Cargo.toml:2, "
        "programs/crates/layerx-programs-market/Cargo.toml:2 and "
        "programs/crates/layerx-programs-ai-market/Cargo.toml:2."
    ),
    14: (
        "executeSponsored is defined at contracts/src/BatchCallAndSponsor.sol:135; "
        "the gas station encodes that batch at "
        "services/interop/crates/layerx-gas-station/src/tx.rs:245."
    ),
    15: (
        "ReceiptKeys is the governed list of pinned receipt keys; primary and "
        "standby host key ranges may overlap "
        "(modules/sequencer/keeper/receipt_keys.go:77)."
    ),
    16: (
        "The addr binding refusal registry is at "
        "precompiles/common/lxp_errors.go:303; the carried application-level "
        "rotation decision is spec/layerx-merge/spec.kvx:145."
    ),
    17: "services/agent/schema/agent-api/v1.kvx:3 declares major = 2.",
    18: (
        "SetLeaderPin is at consensus/internal/consensus/types/round_state.go:201 "
        "and LeaderPin.Leader at :451; BeginBlock passes votes to slashing at "
        "node/legacyabci/begin_block.go:61."
    ),
    19: (
        "DeliverTx is at node/abci.go:190 and DeliverTxBatch at node/abci.go:251. "
        "The carried rollout policy is spec/layerx-merge/spec.kvx:166."
    ),
    20: (
        "MergeAddressTable lists all eight agent addresses at "
        "precompiles/setup.go:71; node/precompiles.go:20 imports all eight "
        "constructor packages. newMergePrecompiles builds registered "
        "constructors at precompiles/setup.go:134 and mergePrecompileGate "
        "applies the v7.0 module gates at node/upgrades.go:553. "
        "The gateway's orob route is services/wallet/gateway/src/precompiles.ts:18."
    ),
    21: (
        "layerxAnchor stays registered at precompiles/setup.go:212. Its executor "
        "refuses every method after StoreDeleteUpgrade, and refuses writes at "
        "the merge upgrade (precompiles/layerxanchor/layerxanchor.go:184)."
    ),
    22: (
        "The four addresses are listed at precompiles/setup.go:71. Their "
        "constructors register in precompiles/layerxagent/orob/merge.go:16, "
        "precompiles/layerxagent/clearing/merge.go:16, "
        "precompiles/layerxagent/oracle/merge.go:19 and "
        "precompiles/layerxagent/pofq/merge.go:16, imported by "
        "node/precompiles.go:20. computeClearing is declared at "
        "precompiles/layerxagent/clearing/clearing.go:22; activation gates are "
        "at node/upgrades.go:561."
    ),
}

REWRITTEN_VALUES = {
    20: (
        "layerxctl stays an operator EVM client and the program registry service "
        "a read cache. All eight agent functions 0x0901-0x0908 have constructors "
        "registered through production node imports and are built from "
        "MergeAddressTable. Serving them remains subject to the v7.0 activation "
        "and module gates (decision.1706; task.17.1.4)."
    ),
    22: (
        "orob (0x0901), clearing (0x0902), oracle aggregator (0x0903) and pofq "
        "(0x0904) are implemented under precompiles/layerxagent, and their merge "
        "constructors are registered through production node imports. The "
        "v7.0 gate requires layerxagent for all four and also layerxexchange "
        "for oracle (decision.1706; task.17.1.4)."
    ),
}


def render_record(entry: Decision) -> str:
    """Render one carry-forward entry independently of the Markdown on disk."""
    identifier = int(Path(entry["record"]).stem.removeprefix("D"))
    decision = SPEC[f"decision.{identifier}"]
    if entry["carry"] == "rewritten":
        value = REWRITTEN_VALUES[identifier]
    else:
        value = decision["value"]
    affects = decision["affects"]
    assert isinstance(affects, list)
    lines = [
        f"# D{identifier}: {decision['title']}",
        "",
        f"Carry status: **{entry['carry']}**.",
        "",
        "Carry-forward index: "
        f"`programme/data/decisions.kvx` `[decision.{identifier}]`.",
        "",
        "## Value",
        "",
        str(value),
        "",
        "## HEAD basis",
        "",
        f"Source revision: `{HEAD_BASIS_REVISION}`.",
        "",
        HEAD_BASES[identifier],
        "",
        "## Bound tasks",
        "",
        *(f"- `task.{task}`" for task in entry["binds"]),
        "",
        "## Affects",
        "",
        *(f"- {target}" for target in affects),
        "",
        "[Decision index](INDEX.md)",
        "",
    ]
    return "\n".join(lines)


def test_records_12_to_22_match_index() -> None:
    index = load_index()
    identifiers = set(range(12, 23))
    assert set(HEAD_BASES) == identifiers
    assert identifiers <= index.keys()
    assert {
        identifier
        for identifier in identifiers
        if index[identifier]["carry"] == "rewritten"
    } == set(REWRITTEN_VALUES)

    for identifier in sorted(identifiers):
        entry = index[identifier]
        decision = SPEC[f"decision.{identifier}"]
        assert entry["record"] == f"spec/layerx-merge/docs/decisions/D{identifier}.md"
        assert entry["carry"] == decision["carry"]
        assert entry["binds"]
        assert all(f"task.{task}" in SPEC for task in entry["binds"])
        if entry["carry"] == "rewritten":
            assert REWRITTEN_VALUES[identifier] != decision["value"]
        expected = render_record(entry)
        assert (ROOT / entry["record"]).read_bytes() == expected.encode("utf-8"), (
            f"D{identifier} differs from its carry-forward index rendering"
        )
