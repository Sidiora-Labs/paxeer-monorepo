import subprocess
import tempfile
import unittest
from pathlib import Path


class BringupSpecPathsTest(unittest.TestCase):
    def test_docs_names_checks_documentation_policy(self):
        root = Path(__file__).resolve().parents[3]
        with tempfile.TemporaryDirectory() as directory:
            document = Path(directory) / "names.md"
            for text, refusal in (
                ("LAYERX_FAUCET_URL", None),
                ("LAYERX_GATEWAY_FAUCET_URL", None),
                ("LAYERX_SCHEDULED_FAUCET_URL", None),
                ("FAUCET_URL_SUFFIX", None),
                ("1FAUCET_URL", None),
                ("FAUCET_URL1", None),
                ("FAUCET_URL", "unpublished faucet configuration"),
                ("`FAUCET_URL`", "unpublished faucet configuration"),
                ("(FAUCET_URL)", "unpublished faucet configuration"),
                ("FAUCET_URL=value", "unpublished faucet configuration"),
                ("mainnet-beta.paxeer.network", None),
                ("https://<node>.mainnet-beta.paxeer.network", None),
                ("https://unpublished.mainnet-beta.paxeer.network", "unpublished public name"),
                ("https://api15.mainnet-beta.paxeer.network", "unpublished public name"),
                ("http://127.0.0.1:$COMET_RPC_PORT/status", None),
                ("http://127.0.0.1:${COMET_RPC_PORT}/validators?page=1&per_page=100", None),
                ("https://api1.mainnet-beta.paxeer.network:$PORT", "node RPC must use its declared HTTPS root"),
                ("https://api1.mainnet-beta.paxeer.network:${PORT}/", "node RPC must use its declared HTTPS root"),
                ("https://api1.mainnet-beta.paxeer.network:$PORT/rpc", "node RPC must use its declared HTTPS root"),
                ("http://127.0.0.1:invalid/status", "malformed documentation URL"),
                ("http://127.0.0.1:${}/status", "malformed documentation URL"),
            ):
                with self.subTest(text=text):
                    document.write_text(text)
                    result = subprocess.run(
                        ["bringup-inspect", "docs-names",
                         "docs/site/docs/reference/public-rpc.md",
                         "spec/layerx-merge/spec.kvx", str(document)],
                        cwd=root, capture_output=True, text=True, timeout=60,
                        check=False,
                    )
                    if refusal is None:
                        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                    else:
                        self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
                        self.assertIn(refusal, result.stderr)

    def test_docs_names_checks_percent_encoded_public_names(self):
        root = Path(__file__).resolve().parents[3]
        with tempfile.TemporaryDirectory() as directory:
            document = Path(directory) / "names.md"
            for text, accepted in (
                ("100% ready: https%3A%2F%2Fwww.paxeer.network%2Fimage.png", True),
                ("https%3a%2f%2f%77%77%77.paxeer.network%2fimage.png", True),
                ("https%3A%2F%2Funpublished.paxeer.network%2Fimage.png", False),
                ("https%3A%2F%2F%75npublished.paxeer.network%2Fimage.png", False),
            ):
                with self.subTest(text=text):
                    document.write_text(text)
                    result = subprocess.run(
                        ["bringup-inspect", "docs-names",
                         "docs/site/docs/reference/public-rpc.md",
                         "spec/layerx-merge/spec.kvx", str(document)],
                        cwd=root, capture_output=True, text=True, timeout=60,
                        check=False,
                    )
                    if accepted:
                        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                    else:
                        self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
                        self.assertIn("unpublished public name", result.stderr)

    def test_docs_names_reads_existing_spec(self):
        root = Path(__file__).resolve().parents[3]
        docs_names = subprocess.run(
            ["bash", "tools/bringup/docs-names.sh"],
            cwd=root,
            capture_output=True,
            text=True,
            timeout=60,
            check=False,
        )
        check_live = subprocess.run(
            ["bash", "tools/bringup/check-live.sh"],
            cwd=root,
            capture_output=True,
            text=True,
            timeout=60,
            check=False,
        )

        self.assertEqual(
            docs_names.returncode, 0, docs_names.stdout + docs_names.stderr
        )
        usage = check_live.stdout + check_live.stderr
        self.assertEqual(check_live.returncode, 2, usage)
        self.assertIn("usage: tools/bringup/check-live.sh", usage)
        self.assertIn("spec/layerx-merge/spec.kvx", usage)
