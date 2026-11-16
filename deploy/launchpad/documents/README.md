# Startup document composer

The composer turns reviewed launchpad values into one protected environment
document per runtime service. It binds every JSON document to its lowercase
SHA-256 digest and emits the document as standard base64 for the service's
`*_DOCUMENT` variable.

```bash
python3 deploy/launchpad/documents/compose.py \
  --values deploy/launchpad/documents/values \
  --out /run/kindle/documents \
  --service api --service api-sockets
```

Use `--examples` instead of `--values` for the checked-in example directory.
The values directory contains `common.json` (or `common.example.json`), an
`expected-schema/` directory with one `<service>.json` object per service, and
optional service modules beside `compose.py` for service-specific documents.
Each expected-schema document is the output of the `launchpad-schema expected`
command. The composer compares its `service` member with the service registry
name while retaining the platform service name in the values filename.
Output files are named `<service>.json`, contain only document, file and digest
variables, and are created with mode `0600`.

For the indexer, receipt and lifecycle documents each have two forms. The default
form composes the document from the indexer values and writes its standard base64
to `INDEXER_RECEIPT_CONFIG_DOCUMENT` or `INDEXER_LIFECYCLE_CONFIG_DOCUMENT`, with
the corresponding `*_SHA256`.

To use files rendered by `services/launchpad/indexer/cmd/indexer-runtime-docs`,
set these pairs in `indexer.json` (or `indexer.example.json`):

```json
{
  "receipt_rendered_file": "rendered/receipt.json",
  "receipt_runtime_path": "/run/launchpad/indexer/receipt.json",
  "lifecycle_rendered_file": "rendered/lifecycle.json",
  "lifecycle_runtime_path": "/run/launchpad/indexer/lifecycle.json"
}
```

These members supplement the other indexer values. A rendered input path is
absolute or relative to the values directory; the runtime path is where the
service reads the mounted file. Supply both members of each pair. Each input
must contain one JSON object. The composer preserves its bytes and emits the
runtime path in `INDEXER_RECEIPT_CONFIG_FILE` or `INDEXER_LIFECYCLE_CONFIG_FILE`
and the SHA-256 of those exact bytes in the corresponding `*_SHA256`, omitting
the `*_DOCUMENT` variable. Mount the same bytes at the runtime path. Receipt and
lifecycle may select their forms independently.

The variable contract's `documentSources` maps each document variable to its
file alternative. The composer and variable apply tool require exactly one
source per document, alongside the required digest. When changing an existing
deployment's form, clear its previous source before applying the new variables.

