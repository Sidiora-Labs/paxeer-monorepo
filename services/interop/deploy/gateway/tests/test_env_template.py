import pathlib
import re


def test_template_names_every_variable_the_service_reads():
    gateway = pathlib.Path(__file__).resolve().parents[1]
    config = gateway.parents[1] / "crates/layerx-interop-service/src/config.rs"
    service_variables = set(
        re.findall(r'"(LAYERX_INTEROP_[A-Z0-9_]+)"', config.read_text())
    )
    # Commented assignments document settings that must remain unset on Railway.
    template_variables = set(
        re.findall(
            r"^\s*(?:#\s*)?(LAYERX_INTEROP_[A-Z0-9_]+)=",
            (gateway / "railway.env.example").read_text(),
            re.MULTILINE,
        )
    )

    assert service_variables, "No interop configuration variables found in config.rs"
    missing = service_variables - template_variables
    assert not missing, f"railway.env.example is missing: {', '.join(sorted(missing))}"
