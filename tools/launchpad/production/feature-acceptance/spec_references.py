"""Reconcile historical declarations without promoting them to current gates."""
import importlib.util
import pathlib
import re

ROOT = pathlib.Path(__file__).resolve().parents[4]
SPEC_PATH = 'spec/layerx-merge/spec.kvx'
RETIREMENT_TITLE = 'The removed launchpad production-system spec tree is retired'
LEGACY_SECTION = re.compile(r'^(task\.(0\.[123]|[1-4]\.(0[1-9]|[1-4][0-9]|50|99)|9\.1)|req\.D[0-9]{2}_[0-9]{3}|coverage\.(PF(0[1-9]|[12][0-9]|30)|OC(0[1-9]|1[0-9]|2[0-5])))$')


def load_spec(source=None):
    source = pathlib.Path(source) if source is not None else ROOT / SPEC_PATH
    if source.is_symlink() or source.resolve() != ROOT / SPEC_PATH:
        raise ValueError('current root specification required')
    module_spec = importlib.util.spec_from_file_location(
        'feature_acceptance_kvx', ROOT / 'spec/layerx-merge/programme/kvx.py')
    module = importlib.util.module_from_spec(module_spec)
    module_spec.loader.exec_module(module)
    return module.load(source)


def reference(specification, section):
    if section in specification:
        return {'spec': SPEC_PATH, 'section': section, 'class': 'current-reference',
                'reason': 'Declared in ' + SPEC_PATH}
    if not LEGACY_SECTION.fullmatch(section):
        raise ValueError('unresolved current specification reference: ' + section)
    retirement = specification.get('decision.1803', {})
    if not retirement.get('title', '').startswith(RETIREMENT_TITLE) or 'sdk/launchpad/protocol/contracts/v1/' not in retirement.get('value', ''):
        raise ValueError('missing retirement authority for ' + section)
    return {'spec': SPEC_PATH, 'section': section, 'class': 'retired-reference',
            'reason': 'decision.1803 retires the launchpad production-system specification; ' + section +
                      ' is absent from ' + SPEC_PATH}

