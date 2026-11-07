#!/usr/bin/env python3
"""Provision real replication mTLS for the production committed-fixture recorder."""

import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import tomllib


def openssl(*args, input=None):
    return subprocess.run(
        ["openssl", *map(str, args)], input=input, check=True, stdout=subprocess.PIPE
    ).stdout


def certificates(directory):
    ca = directory / "ca.pem"
    ca_key = directory / "ca.key"
    openssl(
        "req", "-x509", "-newkey", "ed25519", "-nodes", "-days", "1",
        "-subj", "/CN=committed-fixture-root", "-addext", "basicConstraints=critical,CA:TRUE",
        "-addext", "keyUsage=critical,keyCertSign", "-keyout", ca_key, "-out", ca,
    )
    for serial, role in enumerate(["primary", "standby"], 2):
        key = directory / f"{role}.key"
        csr = directory / f"{role}.csr"
        certificate = directory / f"{role}.pem"
        extensions = directory / f"{role}.ext"
        extensions.write_text(
            f"subjectAltName=DNS:{role}.test\n"
            "extendedKeyUsage=serverAuth,clientAuth\n"
            "keyUsage=critical,digitalSignature\n"
        )
        openssl(
            "req", "-new", "-newkey", "ed25519", "-nodes", "-subj", f"/CN={role}.test",
            "-keyout", key, "-out", csr,
        )
        openssl(
            "x509", "-req", "-in", csr, "-CA", ca, "-CAkey", ca_key,
            "-set_serial", serial, "-days", "1", "-extfile", extensions, "-out", certificate,
        )
        public_key = openssl("x509", "-in", certificate, "-pubkey", "-noout")
        der = openssl("pkey", "-pubin", "-outform", "DER", input=public_key)
        (directory / f"{role}.pin").write_text(hashlib.sha256(der).hexdigest())
        key.chmod(0o600)
    ca_key.chmod(0o600)


def record(node):
    root = Path(__file__).resolve().parents[6]
    crate = Path(__file__).resolve().parents[2]
    with tempfile.TemporaryDirectory(prefix="repair223355-tls-") as temporary:
        directory = Path(temporary)
        certificates(directory)
        environment = os.environ.copy()
        environment["REPAIR223355_PAXD"] = str(node.resolve(strict=True))
        environment["REPAIR223355_TLS"] = str(directory)
        subprocess.run(
            [
                "go", "run", "-p", "4", "./daemon/committed-fixture", "record",
                "--paxd", str(Path(__file__).resolve()),
                "--out", str(crate / "tests/fixtures/committed"),
                "--vector", "vector-v4-committed",
            ],
            cwd=root, env=environment, check=True,
        )


def launch(arguments):
    node = os.environ["REPAIR223355_PAXD"]
    if arguments[0] == "start":
        home = Path(arguments[arguments.index("--home") + 1])
        with (home / "config/app.toml").open("rb") as config:
            role = tomllib.load(config)["sequencer"]["role"]
        assert role in ("primary", "standby")
        peer = "standby" if role == "primary" else "primary"
        directory = Path(os.environ["REPAIR223355_TLS"])
        options = {
            "replication-tls-cert": directory / f"{role}.pem",
            "replication-tls-key": directory / f"{role}.key",
            "replication-tls-ca": directory / "ca.pem",
            "replication-peer-name": f"{peer}.test",
            "replication-peer-spki": (directory / f"{peer}.pin").read_text(),
        }
        # The recorder's prime step also reads app.toml directly between starts.
        config = home / "config/app.toml"
        prefix, section = config.read_text().split("[sequencer]", 1)
        body, separator, suffix = section.partition("\n[")
        retained = [
            line for line in body.splitlines()
            if line.split("=", 1)[0].strip() not in options
        ]
        entries = [f"{name} = {json.dumps(str(value))}" for name, value in options.items()]
        config.write_text(
            prefix + "[sequencer]\n" + "\n".join(retained + entries) + "\n"
            + separator + suffix
        )
        for option, value in options.items():
            arguments.extend([f"--sequencer.{option}", str(value)])
    os.execv(node, [node, *arguments])


if __name__ == "__main__":
    if sys.argv[1] == "--record":
        record(Path(sys.argv[2]))
    else:
        launch(sys.argv[1:])
