"""Finding 008: production Java source facts, without a JVM or network dependency.

Regex vectors exercise the literal extracted from Java using its shared regex
syntax and ASCII case folding. They do not execute the Java component.
"""

import json
from pathlib import Path
import re

import pytest


ROOT = Path(__file__).resolve().parents[3]
SAMPLE = ROOT / "sdk/integrations/android/src/sample/java/com/sidiora/layerx/android/sample"
RECORD = ROOT / "tools/qualification/audit_repairs/repair_22_33_615.kvx"


def source(name):
    return (SAMPLE / name).read_text()


def compact(text):
    return re.sub(r"\s+", " ", text).strip()


def method(text, declaration):
    start = text.index(declaration)
    opening = text.index("{", start)
    tokens = re.compile(
        r'"(?:\\.|[^"\\])*"|\'(?:\\.|[^\'\\])*\'|//[^\n]*|/\*.*?\*/|[{}]',
        re.S,
    )
    depth = 0
    for token in tokens.finditer(text, opening):
        if token.group() == "{":
            depth += 1
        elif token.group() == "}":
            depth -= 1
            if depth == 0:
                return text[opening + 1 : token.start()]
    raise AssertionError(f"Unbalanced Java method: {declaration}")


def loopback_pattern():
    body = method(source("RelayReceiptResolver.java"), "private static boolean isLoopbackHost(")
    match = re.fullmatch(r'return host\.matches\(("(?:\\.|[^"\\])*")\);', compact(body))
    assert match is not None, "The entire validator must use String.matches, without bypasses"
    return json.loads(match[1])


def section(name):
    match = re.search(
        r"(?ms)^\[" + re.escape(name) + r"\]\n(.*?)(?=^\[|\Z)",
        RECORD.read_text(),
    )
    assert match is not None, name
    return {
        key: json.loads(value)
        for key, value in re.findall(r"(?m)^([\w.-]+)\s*=\s*(.+)$", match[1])
    }


def test_finding_008_entire_host_validation_guards_http():
    resolver = source("RelayReceiptResolver.java")
    body = method(resolver, "private static URI requireRelay(")
    assert compact(body) == compact(
        '''
        String scheme = uri.getScheme() == null ? "" : uri.getScheme().toLowerCase(Locale.ROOT);
        String host = uri.getHost();
        if (host == null || host.isEmpty() || uri.getUserInfo() != null
                || uri.getQuery() != null || uri.getFragment() != null) {
            throw MobileIntegrationException.of(MobileIntegrationException.Code.INVALID_CONFIGURATION);
        }
        boolean loopback = isLoopbackHost(host);
        if (scheme.equals("https") || (scheme.equals("http") && loopback)) return uri;
        throw MobileIntegrationException.of(MobileIntegrationException.Code.INVALID_CONFIGURATION);
        '''
    )
    assert "InetAddress" not in resolver
    assert 'startsWith("127.")' not in resolver
    assert loopback_pattern().startswith("(?i:localhost)|::1|\\[::1\\]|127")


@pytest.mark.parametrize("host", ["localhost", "LOCALHOST", "LocalHost", "::1", "[::1]"])
def test_finding_008_supported_loopback_names_and_ipv6(host):
    assert re.fullmatch(loopback_pattern(), host, re.ASCII) is not None


def test_finding_008_all_decimal_octet_boundaries():
    pattern = re.compile(loopback_pattern(), re.ASCII)
    for position in range(1, 4):
        for value in range(256):
            octets = ["127", "0", "0", "1"]
            octets[position] = str(value)
            host = ".".join(octets)
            assert pattern.fullmatch(host) is not None, host
        for value in ("256", "999", "000", "00", "01", "001", "0255", "-1", "+1", "1e2", "0x1", "１"):
            octets = ["127", "0", "0", "1"]
            octets[position] = value
            host = ".".join(octets)
            assert pattern.fullmatch(host) is None, host
    assert pattern.fullmatch("127.255.255.255") is not None
    assert pattern.fullmatch("127.0.0.0") is not None


@pytest.mark.parametrize(
    "host",
    [
        "127.attacker.example",
        "127.0.0.1.attacker.example",
        "127.1.2.3.attacker.example",
        "127.0.0.attacker",
        "localhost.attacker.example",
        "localhost.",
        "127.0.0.1.",
        "127.1",
        "127.0.1",
        "127.0.0.1.2",
        "127..0.1",
        "127.",
        "127",
        "126.255.255.255",
        "128.0.0.1",
        "0.0.0.0",
        "192.168.0.1",
        "2130706433",
        "0x7f000001",
        "0177.0.0.1",
        "[::1].attacker.example",
        "[::1",
        "::1]",
        "::2",
        "[::ffff:192.0.2.1]",
        "[::1%25eth0]",
        " localHost",
        "localhost ",
        "localhoſt",
        "127.0.0.1\n",
        "localhost\x00.attacker.example",
        "",
    ],
)
def test_finding_008_remote_and_malformed_hosts_are_not_loopback(host):
    assert re.fullmatch(loopback_pattern(), host, re.ASCII) is None


def test_finding_008_real_configuration_transport_and_evidence_source_path():
    main = compact(source("ConsoleSampleMain.java"))
    assert compact(
        '''RelayReceiptResolver receipts = new RelayReceiptResolver(
            URI.create(SampleEnvironment.required(environment, "LAYERX_RECEIPT_RELAY_URL")),
            mapper, (int) configuration.requestTimeoutMs());'''
    ) in main
    assert "WalletModel model = new WalletModel(mobile, receipts);" in main
    wallet = compact(source("WalletModel.java"))
    assert 'this.gate = mobile.gate(Objects.requireNonNull(receipts, "receipts"));' in wallet

    resolver = source("RelayReceiptResolver.java")
    constructor = compact(method(resolver, "public RelayReceiptResolver("))
    assert constructor.startswith(
        'this.relayUri = requireRelay(Objects.requireNonNull(relayUri, "relayUri"));'
    )
    assert "private final URI relayUri;" in resolver
    assert resolver.count("this.relayUri =") == 1
    transport = compact(method(resolver, "public ReceiptGate.Evidence resolve("))
    facts = [
        "String prefix = relayUri.toString();",
        'String target = prefix + "/" + URLEncoder.encode(receiptReference, StandardCharsets.UTF_8).replace("+", "%20");',
        "connection = (HttpURLConnection) new URL(target).openConnection();",
        "connection.setInstanceFollowRedirects(false);",
        "int status = connection.getResponseCode();",
        "if (status < 200 || status >= 300)",
        "return evidence(mapper.readTree(encoded));",
    ]
    offsets = [transport.index(fact) for fact in facts]
    assert offsets == sorted(offsets)
    assert "connection.setInstanceFollowRedirects(true)" not in transport
    evidence = compact(method(resolver, "private ReceiptGate.Evidence evidence("))
    for fact in (
        'byte[] rawTransaction = hex(field(payload, "raw_transaction"));',
        'byte[] signedReceipt = hex(field(payload, "signed_receipt"));',
        "return new ReceiptGate.Evidence(rawTransaction, signedReceipt, receiptKey);",
    ):
        assert fact in evidence


def test_finding_008_record_identifies_proofs_limits_and_wiring():
    finding = section("finding.008")
    assert finding["audit"] == "22.30.163"
    assert finding["file"] == str((SAMPLE / "RelayReceiptResolver.java").relative_to(ROOT))
    assert finding["tests"] == [
        "test_finding_008_entire_host_validation_guards_http",
        "test_finding_008_supported_loopback_names_and_ipv6",
        "test_finding_008_all_decimal_octet_boundaries",
        "test_finding_008_remote_and_malformed_hosts_are_not_loopback",
        "test_finding_008_real_configuration_transport_and_evidence_source_path",
        "test_finding_008_record_identifies_proofs_limits_and_wiring",
    ]
    task = section("task.22.33.615")
    assert task["base_revision"] == "04c1e9407e75f7ac92c260452f2232ab07bf3ef1"
    assert "No baseline verification was run" in task["baseline"]
    assert "Java execution" in task["review_only"]
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert task["evidence_log"] == "/out/logs/verify-1.log"
    wiring = section("wiring.1")
    assert wiring["owner_domain"] == 15
    assert wiring["path"] == (
        "sdk/integrations/android/src/main/java/com/sidiora/layerx/android/PublishableConfiguration.java"
    )
    assert "share" in wiring["request"]
