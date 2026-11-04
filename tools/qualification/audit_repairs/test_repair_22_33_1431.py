"""Offline config-drift proof; trace deployed settings through production sources.

No Rust compiler, running cluster, secret material or live SMTP endpoint is used.
Runtime delivery, TLS trust and mailbox receipt remain deployment qualification.
"""

import ast
from pathlib import Path
import re
import tomllib

import yaml


ROOT = Path(__file__).resolve().parents[3]
IDENTITY = ROOT / "services/platform/hosted/identity"
RECORD = Path(__file__).with_name("repair_22_33_1431.kvx")


def source(name):
    return (IDENTITY / name).read_text(encoding="utf-8")


def resources():
    documents = list(yaml.safe_load_all(source("deployment.yaml")))
    assert all(isinstance(document, dict) for document in documents)
    indexed = {(d["kind"], d["metadata"]["name"]): d for d in documents}
    assert len(indexed) == len(documents), "duplicate Kubernetes resource"
    assert all(d["metadata"]["namespace"] == "layerx-testnet" for d in documents)
    return indexed


def deployment():
    return resources()["Deployment", "layerx-identity"]


def environment():
    containers = deployment()["spec"]["template"]["spec"]["containers"]
    assert len(containers) == 1 and containers[0]["name"] == "identity"
    env = containers[0]["env"]
    values = {entry["name"]: entry for entry in env}
    assert len(values) == len(env), "duplicate environment name"
    return values


def function(text, name):
    """Select a top-level Rust function up to the next top-level declaration."""
    pattern = r"(?m)^(?:pub )?fn " + re.escape(name) + r"\b"
    match = re.search(pattern, text)
    assert match, name
    remaining = text[match.start():]
    boundary = re.search(r"(?m)^}(?:\r?\n|$)", remaining)
    assert boundary, name
    return remaining[:boundary.end()]


def require(text, *facts):
    for fact in facts:
        assert fact in text, fact


def ordered(text, *facts):
    offset = 0
    for fact in facts:
        position = text.find(fact, offset)
        assert position >= 0, fact
        offset = position + len(fact)


def test_identity_smtp_required_secret_and_sender_reach_the_real_parser():
    env = environment()
    assert env["LAYERX_IDENTITY_SMTP_URL"] == {
        "name": "LAYERX_IDENTITY_SMTP_URL",
        "valueFrom": {"secretKeyRef": {
            "name": "layerx-identity-smtp", "key": "url", "optional": False,
        }},
    }
    assert env["LAYERX_IDENTITY_SMTP_FROM"] == {
        "name": "LAYERX_IDENTITY_SMTP_FROM",
        "valueFrom": {"configMapKeyRef": {
            "name": "layerx-identity-mail", "key": "from", "optional": False,
        }},
    }
    configured = function(source("src/main.rs"), "config")
    ordered(
        configured, 'env::var("LAYERX_IDENTITY_SMTP_URL")',
        "Ok(url) => Some(Smtp::parse(", "&url,",
        'env::var("LAYERX_IDENTITY_SMTP_FROM").map_err(',
        '"LAYERX_IDENTITY_SMTP_FROM is required with LAYERX_IDENTITY_SMTP_URL"',
        ")?),",
    )
    require(
        source("src/mailer.rs"), 'url.strip_prefix("smtps://")',
        "if tls { 465 } else { 25 }", "Some(_) if !tls =>",
        '"LAYERX_IDENTITY_SMTP_URL credentials require smtps://"',
        "Some((percent_decode(user)?, percent_decode(password)?))",
        "if !valid_address(from)",
    )


def test_identity_smtp_service_and_egress_are_scoped_to_the_tls_relay():
    docs = resources()
    pod = deployment()["spec"]["template"]["metadata"]["labels"]
    policy = docs["NetworkPolicy", "layerx-identity-egress"]["spec"]
    assert policy["podSelector"] == {"matchLabels": {"app": pod["app"]}}
    assert policy["policyTypes"] == ["Egress"]
    assert policy["egress"] == [
        {
            "to": [{"namespaceSelector": {}}],
            "ports": [{"protocol": "UDP", "port": 53},
                      {"protocol": "TCP", "port": 53}],
        },
        {
            "to": [{"podSelector": {"matchLabels": {"app": "layerx-identity-smtp"}}}],
            "ports": [{"protocol": "TCP", "port": 465}],
        },
    ]
    relay = docs["Service", "layerx-identity-smtp"]
    assert relay["spec"] == {
        "selector": policy["egress"][1]["to"][0]["podSelector"]["matchLabels"],
        "ports": [{"name": "smtps", "protocol": "TCP", "port": 465, "targetPort": 465}],
    }
    # No second policy may silently broaden egress for the identity pod.
    egress_policies = [
        d for d in docs.values() if d["kind"] == "NetworkPolicy"
        and "Egress" in d["spec"].get("policyTypes", [])
    ]
    assert egress_policies == [docs["NetworkPolicy", "layerx-identity-egress"]]
    require(
        source("mail-delivery.md"),
        "smtps://<encoded-user>:<encoded-password>@"
        "layerx-identity-smtp.layerx-testnet.svc.cluster.local:465",
        "app: layerx-identity-smtp", "implicit-TLS listener on TCP 465",
        "trusted\nby the identity image's native trust store",
    )


def test_identity_smtp_producer_and_consumer_share_the_persistent_outbox():
    env = environment()
    state = Path(env["LAYERX_IDENTITY_STATE_DIR"]["value"])
    outbox = Path(env["LAYERX_IDENTITY_SIGNUP_OUTBOX_DIR"]["value"])
    assert outbox == state / "signup-outbox"
    spec = deployment()["spec"]
    assert spec["replicas"] == 1 and spec["strategy"] == {"type": "Recreate"}
    pod = spec["template"]["spec"]
    mounts = {m["name"]: m for m in pod["containers"][0]["volumeMounts"]}
    assert mounts["state"] == {"name": "state", "mountPath": str(state)}
    volumes = {v["name"]: v for v in pod["volumes"]}
    claim = volumes["state"]["persistentVolumeClaim"]["claimName"]
    assert claim == "layerx-identity-state"
    assert resources()["PersistentVolumeClaim", claim]["spec"]["accessModes"] == ["ReadWriteOnce"]
    main = source("src/main.rs")
    require(function(main, "config"),
            'env::var("LAYERX_IDENTITY_SIGNUP_OUTBOX_DIR")',
            'state_dir.join("signup-outbox"), PathBuf::from')
    startup = function(main, "platform_identity")
    ordered(startup, "Store::open(&config.state_dir)?", "verification::reconcile(",
            "&config.outbox_dir", "TcpListener::bind(config.listen)")
    ordered(startup, "let smtp = config.smtp.take();", "thread::spawn(move || loop {",
            "verification::reconcile(", "&mail_shared.config.outbox_dir",
            "smtp.drain_confirmed(&mail_shared.config.outbox_dir, |digest|",
            ".delivery_accepted(digest)")


def test_identity_smtp_signup_payload_reaches_delivery_before_acknowledgement():
    main = source("src/main.rs")
    require(function(main, "route"), '"/v1/signup" => start_signup(shared, request)',
            "if service != Service::Provisioning")
    ordered(function(main, "start_signup"), "verification::start(",
            "&shared.config.outbox_dir", "email: &body.email", "&token,",
            '"pending": true')
    verification = source("src/verification.rs")
    ordered(function(verification, "start"), '"kind": "signup_verification"',
            '"email": email', '"token": token', '"expires_at": expires_at',
            "desk.start_with_delivery(", "publish(outbox, &digest, &delivery, key)")
    require(function(verification, "stage"), "key.open(&delivery.sealed_message)?",
            'published: outbox.join(format!("{digest}.json"))', ".mode(0o600)",
            "file.write_all(&message)", "file.sync_all()")
    mailer = source("src/mailer.rs")
    ordered(mailer, "struct Verification {", "kind: String", "email: String",
            "token: String", "expires_at: u64")
    ordered(mailer, "let verification: Verification = serde_json::from_slice(&raw)",
            'verification.kind != "signup_verification"', "self.send(&verification)",
            "result?;", "accepted(digest)?;", "fs::remove_file(&path)",
            "fs::File::open(outbox)", "f.sync_all()")
    ordered(mailer, "TcpStream::connect((self.host.as_str(), self.port))",
            "if self.tls {", "native_tls::TlsConnector::new()",
            ".connect(&self.host, stream)", "self.dialogue(BufReader::new(stream), verification)")
    ordered(mailer, "fn dialogue<", "expect(&mut session, 220)?",
            "EHLO layerx-identity", "AUTH PLAIN", "MAIL FROM:<{}>", "self.from",
            "RCPT TO:<{}>", "verification.email", 'b"DATA\\r\\n", 354',
            "token = verification.token.as_str()", "expires = verification.expires_at",
            "command(&mut session, message.as_bytes(), 250)?", 'b"QUIT\\r\\n", 221')


def test_identity_smtp_consumer_has_an_explicit_failure_and_operator_disposition():
    main = source("src/main.rs")
    require(main, "const MAILER_INTERVAL: Duration = Duration::from_secs(5);")
    ordered(function(main, "platform_identity"), "smtp.drain_confirmed(",
            "if let Err(error) = result", 'eprintln!("layerx-identity mailer failed: {error}")',
            "thread::sleep(MAILER_INTERVAL)")
    readiness = function(main, "readiness")
    require(readiness, "store.probe_writable()")
    assert "smtp" not in readiness.lower()
    probe = deployment()["spec"]["template"]["spec"]["containers"][0]["readinessProbe"]
    assert probe["httpGet"] == {"scheme": "HTTPS", "path": "/readyz", "port": "https"}
    require(
        source("mail-delivery.md"), "Domain 8 / fm-platform", "every 5 seconds",
        "`/readyz` checks the identity store, not SMTP delivery",
        "oldest unexpired queued message exceeds 60 seconds",
        "pause new signup at\nthe provisioning caller",
        "let the built-in consumer retry", "do not manually remove queue files",
        "`/v1/signup`", "`/v1/signup/verify`", "operator-controlled mailbox",
        "temporarily refuse relay acceptance", "confirm retry delivery",
        "does not execute this deployed signup-to-delivery check",
    )


def test_identity_smtp_repair_record_names_proofs_limits_and_external_wiring():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    finding = record["finding"]["identity-smtp-deployment"]
    audit = tomllib.loads((ROOT / "spec/layerx-merge/programme/audit/records/22.30.80.kvx")
                         .read_text(encoding="utf-8"))
    original = audit["finding"]["identity-smtp-deployment"]
    for key in ("file", "class", "severity", "proposal", "owner_domain", "interface"):
        assert finding[key] == original[key]
    tests = {node.name for node in ast.parse(Path(__file__).read_text()).body
             if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")}
    assert set(finding["tests"]) == tests - {
        "test_identity_smtp_repair_record_names_proofs_limits_and_external_wiring",
    }
    task = record["repair"]
    assert task["task"] == "22.33.1431"
    require(task["baseline"], "No pre-change verification was run", "No baseline failures are claimed")
    require(task["review_only"], "not executed", "mailbox receipt", "TLS trust")
    require(task["evidence"], "/out/run/gates.jsonl", "revision", "command", "exit", "log")
    assert len(task["wiring_requests"]) == 1
    require(task["wiring_requests"][0], "tools/bringup", "domain 8", "relay",
            "Secret", "ConfigMap", "monitor", "signup-to-delivery")
