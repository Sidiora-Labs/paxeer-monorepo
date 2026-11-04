use layerx_platform_testnet::{
    platform_testnet, PendingRelease, LXP_WIRE_PROTOCOL_VERSION, TESTNET_EVM_CHAIN_ID,
    TESTNET_NETWORK_ID,
};
use std::process::Command;

#[test]
fn testnet_chain_id_configuration_rejects_protocol_id() {
    let mut config = platform_testnet();
    let pending = PendingRelease {
        package_semver: config.package_semver.clone(),
        wire_protocol_version: LXP_WIRE_PROTOCOL_VERSION,
    };
    assert_eq!(config.network_id, 402);
    assert_eq!(TESTNET_NETWORK_ID, 402);
    assert_eq!(config.evm_chain_id, 125);
    assert_eq!(TESTNET_EVM_CHAIN_ID, 125);
    assert_eq!(config.validate(&pending), Ok(()));
    for chain_id in [0, 1, u64::from(TESTNET_NETWORK_ID), u64::MAX] {
        config.evm_chain_id = chain_id;
        assert!(config.validate(&pending).is_err(), "accepted {chain_id}");
    }
    config.evm_chain_id = TESTNET_EVM_CHAIN_ID;
    config.network_id = 125;
    assert!(config.validate(&pending).is_err());
}

#[test]
fn testnet_chain_id_parameters_to_signed_transaction() -> Result<(), Box<dyn std::error::Error>> {
    let output = Command::new("python3")
        .arg("-B")
        .arg("-c")
        .arg(
            r#"
import contextlib
import datetime
import importlib.util
import ipaddress
import json
import os
from pathlib import Path
import socket
import ssl
import subprocess
import sys
import tempfile
import time
import urllib.error
import urllib.request

from cryptography import x509
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import ec, utils
from cryptography.x509.oid import NameOID

binary, crate, package_version, wire_version = sys.argv[1:]
sys.path.insert(0, str(Path(crate) / "tests"))
spec = importlib.util.spec_from_file_location(
    "payment_journey", Path(crate) / "tests/payment-journey.py"
)
payment = importlib.util.module_from_spec(spec)
spec.loader.exec_module(payment)
from layerx_sdk.evm_transaction import decode_signed

def refused(function, *args):
    try:
        function(*args)
    except (AssertionError, SystemExit):
        return
    raise AssertionError("invalid chain identity was accepted")

with tempfile.TemporaryDirectory(prefix="testnet-chain-id-") as directory:
    root = Path(directory)
    key = ec.generate_private_key(ec.SECP256R1())
    name = x509.Name([x509.NameAttribute(NameOID.COMMON_NAME, "localhost")])
    now = datetime.datetime.now(datetime.timezone.utc)
    certificate = (
        x509.CertificateBuilder()
        .subject_name(name)
        .issuer_name(name)
        .public_key(key.public_key())
        .serial_number(x509.random_serial_number())
        .not_valid_before(now - datetime.timedelta(minutes=1))
        .not_valid_after(now + datetime.timedelta(hours=1))
        .add_extension(x509.BasicConstraints(ca=True, path_length=None), critical=True)
        .add_extension(x509.SubjectAlternativeName([
            x509.DNSName("localhost"), x509.IPAddress(ipaddress.ip_address("127.0.0.1"))
        ]), critical=False)
        .sign(key, hashes.SHA256())
    )
    (root / "cert.der").write_bytes(certificate.public_bytes(serialization.Encoding.DER))
    (root / "cert.pem").write_bytes(certificate.public_bytes(serialization.Encoding.PEM))
    (root / "key.der").write_bytes(key.private_bytes(
        serialization.Encoding.DER, serialization.PrivateFormat.PKCS8,
        serialization.NoEncryption()
    ))
    (root / "admin.token").write_text(os.urandom(32).hex())
    os.chmod(root / "key.der", 0o600)
    os.chmod(root / "admin.token", 0o600)
    with socket.socket() as public, socket.socket() as admin:
        public.bind(("127.0.0.1", 0))
        admin.bind(("127.0.0.1", 0))
        public_port = public.getsockname()[1]
        admin_port = admin.getsockname()[1]
    origin = f"https://127.0.0.1:{public_port}"
    environment = dict(os.environ)
    environment.update({
        "LAYERX_PENDING_PACKAGE_SEMVER": package_version,
        "LAYERX_PENDING_WIRE_PROTOCOL_VERSION": wire_version,
        "LAYERX_TESTNET_PUBLIC_LISTEN": f"127.0.0.1:{public_port}",
        "LAYERX_TESTNET_ADMIN_LISTEN": f"127.0.0.1:{admin_port}",
        "LAYERX_TESTNET_TLS_CERT_DER": str(root / "cert.der"),
        "LAYERX_TESTNET_TLS_KEY_DER": str(root / "key.der"),
        "LAYERX_OUTBOUND_CA_DER": str(root / "cert.der"),
        "LAYERX_TESTNET_BACKEND_ADMIN_TOKEN_FILE": str(root / "admin.token"),
        "LAYERX_TESTNET_CONTROL_ADMIN_TOKEN_FILE": str(root / "admin.token"),
        "LAYERX_TESTNET_REDIS_URL": "rediss://127.0.0.1:6379",
    })
    for dependency in (
        "IDENTITY", "FAUCET", "CORE", "CORE_ADMIN", "RECEIPT_AUTHORITY", "GATEWAY", "PAXEER"
    ):
        environment[f"LAYERX_TESTNET_{dependency}_URL"] = origin
    context = ssl.create_default_context(cafile=str(root / "cert.pem"))
    opener = urllib.request.build_opener(
        urllib.request.ProxyHandler({}), urllib.request.HTTPSHandler(context=context)
    )
    with (root / "server.log").open("w+") as log:
        server = subprocess.Popen([binary], env=environment, stdout=log, stderr=log)
        try:
            deadline = time.monotonic() + 10
            while True:
                assert server.poll() is None, "real testnet server exited during startup"
                try:
                    with opener.open(origin + "/v1/parameters", timeout=1) as response:
                        assert response.status == 200
                        parameters = json.load(response)
                    break
                except urllib.error.URLError:
                    assert time.monotonic() < deadline, "real testnet TLS listener did not start"
                    time.sleep(0.02)

            assert parameters["network_id"] == 402
            assert parameters["evm_chain_id"] == 125
            assert parameters["network"] == "layerx-testnet"
            assert parameters["package_semver"] == package_version
            assert parameters["lxp_wire_protocol_version"] == int(wire_version)
            arguments = [
                "--gateway", origin, "--node", origin, "--did", "did:layerx:" + "01" * 32,
                "--destination", "did:layerx:" + "02" * 32,
                "--asset", "uhpx", "--amount", "1", "--sequencer-key", "03" * 32,
                "--ca", str(root / "cert.pem"), "--auth-config", str(root / "session.curl"),
                "--signer", str(root / "signer.key"), "--accounts", str(root / "accounts.json"), "--output", str(root / "payment"),
            ]
            args = payment.parse_args(arguments + ["--evm-chain-id", str(parameters["evm_chain_id"])])
            chain_id = payment.validate_evm_chain_id(args.evm_chain_id, "0x7d")
            signer = ec.generate_private_key(ec.SECP256K1())
            public = signer.public_key().public_numbers()
            source = "0x" + payment.keccak256(
                public.x.to_bytes(32, "big") + public.y.to_bytes(32, "big")
            )[-20:].hex()
            data = payment.encode_call("sendNative", ["string"], ["payment-destination"])
            raw = payment.sign_payment(chain_id, 7, 67000000, 200000, 10**12, data, signer, source)
            transaction, signature = decode_signed(raw)
            assert raw[0] == 2
            assert transaction.chain_id == 125
            assert transaction.chain_id != parameters["network_id"]
            assert transaction.nonce == 7 and transaction.gas_limit == 200000
            assert transaction.max_fee_per_gas == 134000000
            assert transaction.max_priority_fee_per_gas == 0
            assert transaction.to == payment.hex_bytes(payment.BANK, 20)
            assert transaction.value == 10**12 and transaction.data == data
            assert transaction.recover_signer(signature).lower() == source
            signer.public_key().verify(
                utils.encode_dss_signature(
                    int.from_bytes(signature[:32], "big"), int.from_bytes(signature[32:64], "big")
                ), transaction.signing_digest(), ec.ECDSA(utils.Prehashed(hashes.SHA256()))
            )
            for expected, actual in [
                (parameters["network_id"], "0x192"), (125, "0x192"),
                (125, "0x1"), (125, "0x0"), (True, "0x7d"),
                ("125", "0x7d"), (125, "0x07d"), (125, "125"),
            ]:
                refused(payment.validate_evm_chain_id, expected, actual)
            refused(payment.sign_payment, parameters["network_id"], 7, 67000000,
                    200000, 10**12, data, signer, source)
            with contextlib.redirect_stderr(log):
                refused(payment.parse_args, arguments)
                refused(payment.parse_args, arguments + ["--evm-chain-id", "402"])
                refused(payment.parse_args, arguments + ["--network-id", "402"])
            print("testnet-chain-id: real HTTPS parameters -> payment parser -> signed type-2 chain 125")
        except BaseException:
            log.flush()
            log.seek(0)
            print(log.read(), file=sys.stderr)
            raise
        finally:
            server.terminate()
            server.wait(timeout=5)
"#,
        )
        .arg(env!("CARGO_BIN_EXE_layerx-testnet-control"))
        .arg(env!("CARGO_MANIFEST_DIR"))
        .arg(env!("CARGO_PKG_VERSION"))
        .arg(LXP_WIRE_PROTOCOL_VERSION.to_string())
        .output()?;
    assert!(
        output.status.success(),
        "parameters/signing path failed: {}\n{}",
        String::from_utf8_lossy(&output.stdout),
        String::from_utf8_lossy(&output.stderr)
    );
    Ok(())
}
