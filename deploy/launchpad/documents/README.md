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
Output files are named `<service>.json`, contain only document and digest
variables, and are created with mode `0600`.

