# Native source control: completion programme

This directory carries the native source-control specification of the vendored project
(`addiqate/hyperfide/spec/hyperfide-source-control/`, snapshot 22e546f1) into the layerx-merge
feature so its unfinished work is tracked and qualified here.

- `source-spec.kvx` is the original specification (its `[task.*]` blocks keep their original ids
  and statuses as the vendored project recorded them); `source-qualification.kvx` and
  `provider-accounting.kvx` are its qualification and accounting records; the Markdown files are
  its design documents.
- `audit.kvx` records the review of every original task against the code: verdict, evidence and
  what is missing.
- Each original task found not done becomes a task `22.37.N` in `spec/layerx-merge/spec.kvx`
  (`[task.22.37.N]`, with `source_task = "<original id>"`), worked and qualified under the
  layerx-merge rules; its qualification records land in `spec/layerx-merge/qualification.kvx`.

## Placement and numbering

- Task 22.36.1 moves the vendored project from `addiqate/hyperfide/` to `tools/hyperfide/` and its
  documentation to `docs/hyperfide/`; every `22.37.N` task requires it and names the moved paths.
- The audit (2026-11-06) found 7 of the 34 original tasks done (0.2, 0.3 to 0.7, 3.1), 26 partial and
  one not done (0.14). The 27 unfinished tasks map to `22.37.1` to `22.37.27` in source wave order;
  `audit.kvx` names the mapping in each block's `new_task`.
- Live-provider, model and object-store parts of the original tasks (0.11 to 0.14, 1.1, 6.1) are
  gated in component mode; their live runs are recorded UNRUN until the owner supplies credentials
  (decision 2700 in `spec/layerx-merge/spec.kvx`).
- Migration numbers in the proposals are renumbered after the highest migration of the moved tree
  (0760): 0761 ancestry index, 0762 workspace closure, 0763 workspace attempts, 0764 publication
  credentials, 0765 import legacy maps.
