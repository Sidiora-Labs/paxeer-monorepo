import importlib.util
import pathlib


def test_examples_paths_use_monorepo_layout(tmp_path):
    package = pathlib.Path(__file__).resolve().parents[1]
    spec = importlib.util.spec_from_file_location(
        'sdk_examples', package / 'scripts/examples.py'
    )
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)

    for root in (package.parents[3], tmp_path):
        contracts, output = module.examples_paths(root)
        assert contracts == (
            root / 'launchpad/spec/production-system/integration/contracts/v2/openapi.json'
        )
        assert output == root / 'sdk/launchpad/packages/sdk/examples'

    assert module.examples_paths(package.parents[3])[1] == package / 'examples'
