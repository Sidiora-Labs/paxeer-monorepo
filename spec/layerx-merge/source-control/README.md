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
