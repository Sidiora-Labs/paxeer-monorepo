# Retirements contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## kernel-solidity

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.6.488.1</code> | req | 6488.1 |
| <code>acceptance.6.488.1</code> | scenario | Given a throwaway git repository that tracks contracts/X.sol, when assert_retired(["contracts/X.sol"], root) runs, then it raises AssertionError naming contracts/X.sol as tracked |
| <code>acceptance.6.488.1</code> | test | tests/bridge/retired/test_kernel_retirement_harness.py::test_harness_reports_a_tracked_retired_file (planned) |
| <code>acceptance.6.488.1</code> | qualification | UNRUN |
| <code>req.6488</code> | ac_1 | assert_retired fails naming the path when a retired file is still tracked by git (planned). |
| <code>acceptance.6.488.2</code> | req | 6488.2 |
| <code>acceptance.6.488.2</code> | scenario | Given a throwaway repository where tests/solidity/A.t.sol imports ../../contracts/X.sol and contracts/X.sol is untracked, when assert_retired runs, then it raises AssertionError naming tests/solidity/A.t.sol (planned fixture line 1) |
| <code>acceptance.6.488.2</code> | test | tests/bridge/retired/test_kernel_retirement_harness.py::test_harness_reports_a_live_import_outside_the_retired_set (planned) |
| <code>acceptance.6.488.2</code> | qualification | UNRUN |
| <code>req.6488</code> | ac_2 | assert_retired fails naming the referring file and line when a tracked file outside the retired set imports the retired path by a relative import (planned). |
| <code>acceptance.6.488.3</code> | req | 6488.3 |
| <code>acceptance.6.488.3</code> | scenario | Given a throwaway repository where only docs/a.md and spec/b.kvx name contracts/X.sol and the file is untracked, when assert_retired runs, then it returns without error |
| <code>acceptance.6.488.3</code> | test | tests/bridge/retired/test_kernel_retirement_harness.py::test_harness_ignores_documentation_references (planned) |
| <code>acceptance.6.488.3</code> | qualification | UNRUN |
| <code>req.6488</code> | ac_3 | assert_retired passes when the only remaining references are under docs/, spec/ or in *.md files (planned). |
| <code>capability.6.51.1</code> | title | Write the tracked-file and live-reference retirement harness |
| <code>capability.6.51.1</code> | reqs | 6488.1<br>6488.2<br>6488.3 |
| <code>resolution.17.738</code> | title | decision.649 binds: Kernel Solidity retirement is proven by tracked-file and live-reference checks |
| <code>resolution.17.738</code> | applies_to | decision.649<br>task.6.51.1 |
| <code>resolution.17.738</code> | source | decision.649 in fragments/J36/decision.kvx; tasks from affects |
