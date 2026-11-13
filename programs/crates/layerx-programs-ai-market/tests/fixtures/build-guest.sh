#!/usr/bin/env bash
# Regenerate the committed fixture; qualification only reads it in process.
set -euo pipefail
cd "$(dirname "$0")/../../../.."
fixture_dir=crates/layerx-programs-ai-market/tests/fixtures
fixture_name=ai-market-opt-z
if [[ "${1:-}" == "--aggregation-probe" ]]; then
    export PAXAI_AGGREGATION_PROBE=1
    fixture_name=ai-market-aggregation-baseline
elif [[ $# != 0 ]]; then
    echo "usage: build-guest.sh [--aggregation-probe]" >&2
    exit 1
else
    unset PAXAI_AGGREGATION_PROBE
fi
source "$fixture_dir/guest-recipe.sh"
guest_wasm="${CARGO_TARGET_DIR:-target}/wasm32-unknown-unknown/release/layerx_programs_ai_market.wasm"
optimized_wasm="${guest_wasm%.wasm}.opt-z.wasm"
export PAXAI_CHAIN_DOMAIN=62b6b14734fa769c3e0fbc94de495a3d038be49a2524b1c071d75d0a4eed71f6
paxai_guest_build "$PWD" "$optimized_wasm"
export PAXAI_GUEST_OPT_LEVEL PAXAI_GUEST_CRATE_TYPE PAXAI_GUEST_LLVM_PASSES
export PAXAI_GUEST_POSTPROCESS="wasm-opt ${PAXAI_GUEST_PASSES[*]}"
python3 - "$fixture_dir" "$optimized_wasm" "$fixture_name" <<'PY'
from pathlib import Path
import hashlib
import os
import sys
root = Path(sys.argv[1])
wasm = Path(sys.argv[2]).read_bytes()
fixture_name = sys.argv[3]
if len(wasm) + 104 > 524288:
    raise SystemExit('guest plus deployment envelope exceeds protocol limit')
source = hashlib.sha256()
source_root = Path('crates/layerx-programs-ai-market/src')
for path in sorted(source_root.rglob('*.rs')):
    source.update(path.relative_to(source_root).as_posix().encode())
    source.update(b'\0')
    source.update(path.read_bytes())
    source.update(b'\0')
(root / f'{fixture_name}.hex').write_text(wasm.hex() + '\n')
(root / f'{fixture_name}.kvx').write_text(
    '[guest]\n'
    'source = "programs/crates/layerx-programs-ai-market"\n'
    'profile = "release"\n'
    f'opt_level = "{os.environ["PAXAI_GUEST_OPT_LEVEL"]}"\n'
    f'crate_type = "{os.environ["PAXAI_GUEST_CRATE_TYPE"]}"\n'
    'lto = true\n'
    f'llvm_passes = "{os.environ["PAXAI_GUEST_LLVM_PASSES"]}"\n'
    'optimizer_version = "133"\n'
    f'postprocess = "{os.environ["PAXAI_GUEST_POSTPROCESS"]}"\n'
    'chain_id = 125\n'
    'chain_domain = "62b6b14734fa769c3e0fbc94de495a3d038be49a2524b1c071d75d0a4eed71f6"\n'
    f'source_sha256 = "{source.hexdigest()}"\n'
    f'purpose = "{"aggregation-pass-measurement" if fixture_name.endswith("baseline") else "deployment"}"\n'
    f'wasm_bytes = {len(wasm)}\n'
    f'sha256 = "{hashlib.sha256(wasm).hexdigest()}"\n')
PY
