from pathlib import Path
import tomllib


def test_gateway_local_harness_has_no_lifecycle_test():
    tests = Path(__file__).resolve().parents[1] / "gateway" / "tests"
    local = tests / "local"
    assert not local.exists(), "Retired gateway local harness remains"
    assert not local.is_symlink(), "Retired gateway local harness symlink remains"
    manifest = tomllib.loads((tests.parent / 'Cargo.toml').read_text(encoding='utf-8'))
    for target in manifest.get('test', []):
        assert target.get('name') != 'lifecycle', 'Retired lifecycle test is declared'
        assert not target.get('path', '').startswith('tests/local/'), 'Retired local test is wired'

    for name in (
        "lifecycle.rs",
        "events.rs",
        "funding.rs",
        "withdrawal.rs",
        "registry_runtime.rs",
        "build.rs",
    ):
        path = local / name
        assert not path.exists(), f"Retired lifecycle harness file remains: {name}"
        assert not path.is_symlink(), f"Retired lifecycle harness symlink remains: {name}"

    pay2 = tests.parents[3] / 'agent/qualification/pay2'
    consumer = tomllib.loads((pay2 / 'Cargo.toml').read_text())
    assert {'name': 'pay2e', 'path': 'pay2e_test.rs'} in consumer['test']
    for name in ('required.rs', 'pay2e_test.rs', 'funding.rs', 'sequence.rs'):
        assert (pay2 / name).is_file(), f'Maintained funded qualification input is missing: {name}'
    assert 'pub trait Required<T>' in (pay2 / 'required.rs').read_text()
    for source in (pay2 / 'Cargo.toml', pay2 / 'build.rs'):
        assert 'gateway/tests/local' not in source.read_text()

    for name in ("retired_client_modules.rs", "retired_client_modules_w2.rs"):
        path = tests / name
        assert not path.exists(), f"Retired lifecycle module scan remains: {name}"
        assert not path.is_symlink(), f"Retired lifecycle module scan symlink remains: {name}"
