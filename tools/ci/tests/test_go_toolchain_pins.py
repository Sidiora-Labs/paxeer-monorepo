from pathlib import Path
import re


def test_go_toolchain_pins_agree_with_go_mod():
    root = Path(__file__).resolve().parents[3]
    directive = re.search(r"^go\s+(\S+)\s*$", (root / "go.mod").read_text(), re.M)
    assert directive is not None, "go.mod must declare a Go version"
    version = directive.group(1)
    cosmovisor = re.search(
        r"^go\s+(\S+)\s*$", (root / "sdk/cosmovisor/go.mod").read_text(), re.M
    )
    assert cosmovisor is not None, "sdk/cosmovisor/go.mod must declare a Go version"
    assert version == cosmovisor.group(1), (
        f"go.mod: Go {version!r} disagrees with sdk/cosmovisor/go.mod "
        f"Go {cosmovisor.group(1)}"
    )
    workflow_pin = re.compile(r"^\s*(?:-\s*)?(go-version|GO_VERSION)\s*:\s*(.*)$")
    workflows = sorted((root / ".github/workflows").glob("*.yml"))
    assert workflows, ".github/workflows must contain workflow files"
    pins_checked = 0

    for path in workflows:
        for number, line in enumerate(path.read_text().splitlines(), 1):
            match = workflow_pin.match(line)
            if match is None:
                continue
            value = match.group(2).split("#", 1)[0].strip().strip("'\"")
            if "${{" in value:
                continue
            assert value == version, (
                f"{path.relative_to(root)}:{number}: {match.group(1)} {value!r} "
                f"disagrees with go.mod Go {version}"
            )
            pins_checked += 1

    assert pins_checked > 0, "No literal workflow Go toolchain pins were checked"

    image_pin = re.compile(
        r"^FROM\s+(?:--platform=\S+\s+)?(?:docker\.io/)?golang:([^\s@]+)",
        re.M | re.I,
    )
    for name in (
        "docker/rpcnode/Dockerfile",
        "docker/localnode/Dockerfile",
        "docker/flyci-controller/Dockerfile",
        "docker/layerx/Dockerfile",
        "docker/kernel/Dockerfile",
        "docker/paxeer/Dockerfile.paxd-node",
    ):
        images = image_pin.findall((root / name).read_text())
        assert images, f"{name}: missing golang build image"
        for tag in images:
            assert tag == f"{version}-bookworm", (
                f"{name}: golang image tag {tag!r} disagrees with go.mod Go {version}"
            )

    name = ".devcontainer/Dockerfile"
    arguments = re.findall(
        r"^ARG\s+GO_VERSION=(\S+)\s*$", (root / name).read_text(), re.M
    )
    assert arguments, f"{name}: missing ARG GO_VERSION"
    for value in arguments:
        assert value == version, (
            f"{name}: ARG GO_VERSION {value!r} disagrees with go.mod Go {version}"
        )

    name = "scripts/ci/xweb-workflow-check.sh"
    fixtures = re.findall(r"go-version:\s*'([^']+)'", (root / name).read_text())
    assert fixtures, f"{name}: missing go-version mutation fixture"
    for value in fixtures:
        assert value == version, (
            f"{name}: go-version fixture {value!r} disagrees with go.mod Go {version}"
        )
