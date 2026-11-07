# Bridge signer inventory

Render `BRIDGE_SIGNER_KEYS` separately for each relayer before deploying its
manifest. From the repository root, for instance 2:

```sh
python3 services/interop/deploy/bridge-relayer/render-signer-keys.py --instance 2
```

The output is a single `BRIDGE_SIGNER_KEYS=<JSON>` line suitable for
`fly secrets import --app paxeer-bridge-relayer-2`. Use the same instance number
in the renderer, app name and `relayer-2.toml` deployment manifest. Repeat for
instances 1 through 5; do not share the rendered inventory between apps.

The renderer checks the selected manifest's app, attestor index, five-member
threshold-three policy, attestor key environment setting, secret mount and
inventory destination. It checks that every inventory key path has exactly one
signer key mount. It reads only the public TOML and JSON files here, never key
contents. Output contains paths and signing policies, not private material.

`signer-keys.json` is the policy source and matches instance 1. Rendering changes
only the attestor path to the selected instance's mount; submitter and fee-payer
policies remain intact. All instances mount the rendered `BRIDGE_SIGNER_KEYS` at
`/etc/layerx/bridge-signer/keys.json`, replacing the image's instance-1 inventory.
Startup requires the key mount to match the attestor index and requires the
inventory's single `bridge/attestor` entry to name that exact mount.
