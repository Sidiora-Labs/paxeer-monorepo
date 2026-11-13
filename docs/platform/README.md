# Developer documentation site

This directory is the source of the static developer documentation site for
Paxeer X Network (`site.kvx` names it "LayerX Developer
Documentation").

`build/build_site.py` renders `site.kvx` and the pages under `content/` into a static site under `site/`. It has no dependencies beyond Python 3. Run it from this directory:

```sh
python3 build/build_site.py ../..
python3 build/build_site.py --check ../..
```

The first form writes the reference Markdown, extracted sample blocks and
static site. The second compares the generated reference Markdown and sample
blocks with their inputs, validates capability labels, and checks the internal
links in a freshly rendered in-memory site. Run the write form to publish that
rendering to `site/`.
Link checking ignores query strings and fragments; it validates file targets,
not heading anchors or remote URL availability.

The write form additionally emits `site/search.js` (a self-contained search
index and widget, no network assets), `site/manifest.json` (every generated
page with its byte length and SHA-256 digest) and `site/link-check.json` (the
number of internal links checked and any problem found). Repository references
use immutable source URLs so they resolve when the site is hosted alone.

## What is generated

`content/reference/human-api.md`, `content/reference/agent-api.md` and `content/reference/errors.md` are generated from `services/human/schema/human-api` and `services/agent/schema/agent-api`. `content/reference/enforcement.md` is generated from `capabilities.kvx`, and `content/reference/samples.md` from `samples.kvx`. None of the five is hand-edited; the build overwrites them.

Every other page under `content/` is written by hand and carries an
`Enforced by` table. Capability rows use `protocol`, `agent-layer`, `service` or
`hosted-surface`; the builder checks each matched capability title and layer
against `capabilities.kvx` and requires every declared capability to appear on
at least one page.

`beta-environment.md` sits outside `content/` and is never rewritten.

Public wiki pages for payments (wallet, faucet, send, token, 402), the
OpenRPC method list, Asset encodings, and commitment levels live under
[`docs/wiki/`](../wiki/Home.md).

`make platform-test-docs` from the repository root runs the documentation
check together with the reference-application and sample checks.

## Samples

`samples.kvx` declares every sample directory in [`services/platform/examples/samples/`](../../services/platform/examples/samples/). A code fence in a page carrying `sample=<id>` is filled from that sample's entry file; `file=` selects a different file in the directory, and `region=` selects a `layerx:begin <name>` / `layerx:end <name>` block within it. The fence language must match the language the sample declares.

Each sample declares a `measured_region` and a `maximum_integration_lines` budget. The build counts the non-blank lines between that region's markers and fails when any sample exceeds its budget, writing the counts to `site/measurements.json`.

Samples are real programs. Each entry in `samples.kvx` declares its `run` command and, in `requires`, the toolchain and the `LAYERX_*` environment inputs it reads.
