import importlib.util
import pathlib


path = pathlib.Path(__file__).resolve().parents[1] / 'scripts/generate.py'
spec = importlib.util.spec_from_file_location('sdk_generate', path)
generate = importlib.util.module_from_spec(spec)
spec.loader.exec_module(generate)


def test_repository_root_is_monorepo():
    root = generate.repository_root()
    assert root / 'sdk/launchpad/packages/sdk/scripts/generate.py' == path
    assert (root / 'spec/workflow.kvx').is_file()
    assert (root / 'tools/launchpad/production/interfaces/codegen.py').is_file()


def test_default_contracts_use_launchpad_spec_prefix():
    root = path.parents[5]
    for major in (1, 2, 3):
        assert generate.default_contracts(major) == root / (
            'launchpad/spec/production-system/integration/contracts/v' + str(major)
        )
        package = 'sdk' if major == 1 else 'sdk-v' + str(major)
        assert generate.expected_package(major) == root / 'sdk/launchpad/packages' / package
