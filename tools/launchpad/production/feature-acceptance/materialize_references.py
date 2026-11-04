"""Materialize declaration lineage from the tracked merged specification."""
import importlib.util
import json
import pathlib

DIRECTORY = pathlib.Path(__file__).resolve().parent


def module(name, filename):
    spec = importlib.util.spec_from_file_location(name, DIRECTORY / filename)
    result = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(result)
    return result


def write(name, value):
    (DIRECTORY / name).write_text(json.dumps(value, indent=2) + '\n')


def main():
    generator = module('inventory_generator', 'build_inventory.py')
    references = generator.reference_policy()
    specification = generator.specification()
    inventory = generator.build()
    write('inventory.json', inventory)
    write('inventory-spec-references.json', generator.inventory_references(inventory))
    for phase in (2, 3, 4):
        name = f'phase-{phase}-scenario-manifest.json'
        manifest = json.loads((DIRECTORY / name).read_text())
        manifest['verify_cmd'] = ('python3 tools/launchpad/production/qualify.py --task ' +
                                  manifest['task'] + ' --spec ' + references.SPEC_PATH +
                                  ' --require-current-source')
        manifest['spec_reference'] = references.reference(specification, 'task.' + manifest['task'])
        write(name, manifest)
    policy = module('feature_policy', 'policy.py')
    name = 'final-integrated-gate-readiness.json'
    manifest = json.loads((DIRECTORY / name).read_text())
    for task, gate in policy.required_release_gates(generator.SPEC).items():
        manifest['mandatory_gates'][task] = dict(verify_cmd=gate['command'],
                                                requires=gate['requires'], kind=gate['kind'],
                                                reference=gate['reference'])
    manifest['verify_cmd'] = ('python3 tools/launchpad/production/qualify.py --task ' +
                              manifest['task'] + ' --spec ' + references.SPEC_PATH +
                              ' --require-current-source')
    manifest['spec_reference'] = references.reference(specification, 'task.' + manifest['task'])
    manifest['graph_scope'] = 'Historical gate declarations; retired references are not mandatory tasks of the merged graph and cannot establish qualification'
    write(name, manifest)


if __name__ == '__main__':
    main()
