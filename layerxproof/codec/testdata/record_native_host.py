"""Refresh the byte-exact node creation/reset fixture from the production host."""

from pathlib import Path


def main():
    root = Path(__file__).resolve().parents[3]
    source = (root / "modules/lxphost/host.go").read_text()
    preamble = source.split("/*", 1)[1].split("*/", 1)[0]
    preamble = "\n".join(
        line for line in preamble.split("\n")
        if not line.strip().startswith("#cgo ")
    )
    preamble = preamble.split("static void *lxphost_accounts", 1)[0]
    (root / "layerxproof/codec/internal/nativefixture/lxphost_native.h").write_text(
        preamble
    )


if __name__ == "__main__":
    main()
