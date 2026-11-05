"""Keep the Android sample's dependency pins and strict verification aligned."""

import json
from pathlib import Path
import re
import xml.etree.ElementTree as ET

from tools.deps.lockfloor import compare_versions


ROOT = Path(__file__).resolve().parents[3]
ANDROID_POM = "sdk/integrations/android/pom.xml"
SAMPLE = ROOT / "sdk/integrations/android/sample-app"
MAVEN = {"m": "http://maven.apache.org/POM/4.0.0"}
VERIFICATION = {"v": "https://schema.gradle.org/dependency-verification"}


def test_android_sample_dependency_verification():
    pom = ET.parse(ROOT / ANDROID_POM).getroot()
    properties = {
        element.tag.removeprefix("{" + MAVEN["m"] + "}"): element.text
        for element in pom.find("m:properties", MAVEN)
    }
    managed = {}
    binding = {}
    dependencies = [
        (managed, "m:dependencyManagement/m:dependencies/m:dependency"),
        (binding, "m:dependencies/m:dependency"),
    ]
    for versions, path in dependencies:
        for dependency in pom.findall(path, MAVEN):
            coordinate = ":".join(
                dependency.findtext(f"m:{name}", namespaces=MAVEN)
                for name in ("groupId", "artifactId")
            )
            version = dependency.findtext("m:version", namespaces=MAVEN)
            if version is None:
                assert coordinate in managed, f"Missing managed version: {coordinate}"
                version = managed[coordinate]
            if version.startswith("${"):
                version = properties[version[2:-1]]
            versions[coordinate] = version

    floors = json.loads((ROOT / "tools/deps/floors/jvm.json").read_text())[
        "manifests"
    ][ANDROID_POM]
    gradle = (SAMPLE / "app/build.gradle.kts").read_text()
    # Comments cannot satisfy a missing or outdated implementation declaration.
    gradle = re.sub(r"/\*.*?\*/|//[^\n]*", "", gradle, flags=re.DOTALL)
    direct = re.findall(r'implementation\(\s*"([^"\n]+)"\s*\)', gradle)
    selected = {}
    for coordinate in (
        "com.fasterxml.jackson.core:jackson-databind",
        "org.bouncycastle:bcprov-jdk18on",
    ):
        versions = [
            value[len(coordinate) + 1 :]
            for value in direct
            if value.startswith(coordinate + ":")
        ]
        assert versions == [binding[coordinate]], (coordinate, versions)
        selected[coordinate] = versions[0]
        assert compare_versions(versions[0], floors[coordinate]) >= 0, coordinate

    metadata = ET.parse(SAMPLE / "gradle/verification-metadata.xml").getroot()
    namespace = "{" + VERIFICATION["v"] + "}"
    assert metadata.tag == namespace + "verification-metadata"
    assert [element.tag for element in metadata] == [
        namespace + "configuration",
        namespace + "components",
    ]
    configuration = metadata.find("v:configuration", VERIFICATION)
    assert [element.tag for element in configuration] == [
        namespace + "verify-metadata",
        namespace + "verify-signatures",
        namespace + "trusted-artifacts",
    ]
    assert configuration.findtext("v:verify-metadata", namespaces=VERIFICATION) == "true"
    assert configuration.findtext("v:verify-signatures", namespaces=VERIFICATION) == "false"
    trusted = configuration.find("v:trusted-artifacts", VERIFICATION)
    assert len(trusted) == 1
    assert trusted[0].tag == namespace + "trust"
    assert trusted[0].attrib == {
        "group": r"com\.sidiora\.layerx",
        "regex": "true",
        "reason": (
            "Built from this repository and installed into mavenLocal by "
            "make platform-install-local-artifacts"
        ),
    }
    assert len(trusted[0]) == 0
    # Neither global nor per-component exemptions may bypass these checksums.
    assert not any(
        element.tag.removeprefix(namespace)
        in {"ignored-key", "ignored-keys", "trusted-key", "trusted-keys"}
        for element in metadata.iter()
    )

    components = metadata.findall("v:components/v:component", VERIFICATION)
    indexed = {
        (element.get("group"), element.get("name"), element.get("version")): element
        for element in components
    }
    assert len(indexed) == len(components), "Duplicate verification components"
    jackson = selected["com.fasterxml.jackson.core:jackson-databind"]
    bouncycastle = selected["org.bouncycastle:bcprov-jdk18on"]
    # Jackson 2.18.11's published BOM selects core/annotations 2.18.11;
    # its POM parent chain is base -> BOM -> parent 2.18.4 -> oss-parent 69.
    required = [
        (
            "com.fasterxml.jackson.core",
            "jackson-databind",
            jackson,
            ("jar", "pom", "module"),
        ),
        ("com.fasterxml.jackson.core", "jackson-core", jackson, ("jar", "pom", "module")),
        (
            "com.fasterxml.jackson.core",
            "jackson-annotations",
            jackson,
            ("jar", "pom", "module"),
        ),
        ("com.fasterxml.jackson", "jackson-base", jackson, ("pom",)),
        ("com.fasterxml.jackson", "jackson-bom", jackson, ("pom",)),
        ("com.fasterxml.jackson", "jackson-parent", "2.18.4", ("pom",)),
        ("com.fasterxml", "oss-parent", "69", ("pom",)),
        ("org.bouncycastle", "bcprov-jdk18on", bouncycastle, ("jar", "pom")),
    ]
    for group, name, version, extensions in required:
        coordinate = (group, name, version)
        assert coordinate in indexed, f"Missing verification component: {coordinate}"
        component = indexed[coordinate]
        assert set(component.attrib) == {"group", "name", "version"}
        assert all(child.tag == namespace + "artifact" for child in component)
        artifacts = {child.get("name"): child for child in component}
        assert len(artifacts) == len(component), coordinate
        for extension in extensions:
            filename = f"{name}-{version}.{extension}"
            assert filename in artifacts, f"Missing verified artifact: {filename}"
            artifact = artifacts[filename]
            assert artifact.attrib == {"name": filename}
            assert len(artifact) == 1, filename
            checksum = artifact[0]
            assert checksum.tag == namespace + "sha256", filename
            assert re.fullmatch(r"[0-9a-f]{64}", checksum.get("value", "")), filename
            assert set(checksum.attrib) <= {"value", "origin"}, filename
            assert len(checksum) == 0, filename
