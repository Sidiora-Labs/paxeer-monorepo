"""Shared source boundary: generated state and secrets are never source inputs.

Locks/manifests remain included; dependencies/tool binaries are separately bound
by the frozen interface bundle and actual executed-check receipt hashes.
"""
from pathlib import PurePosixPath
SOURCE_POLICY_VERSION='qualification-source-v1'
IGNORED_DIRS=frozenset({'.git','.codegraph','node_modules','.next','.gradle','.turbo','.cache','__pycache__','.pytest_cache','coverage','evidence','playwright-report','test-results','dist','.venv','venv','secrets','.secrets','.aws','.ssh','.gnupg'})
IGNORED_FILES=frozenset({'coverage.out','.coverage','.DS_Store','tsconfig.tsbuildinfo'})
def ignored(relative):
    parts=PurePosixPath(str(relative)).parts
    # Keep the isolated fixture roots, with the same exact exceptions at the
    # live monorepo roots. Nested output/secret directories remain excluded.
    coverage_roots = {('protocol', 'coverage'), ('sdk', 'launchpad', 'protocol', 'coverage')}
    secrets_roots = {('shared', 'secrets'), ('sdk', 'launchpad', 'shared', 'secrets')}
    for root in secrets_roots:
        if parts[:len(root)] == root and len(parts) > len(root):
            if len(parts) != len(root) + 1 or not parts[-1].endswith('.go'):
                return True
    return any((p in IGNORED_DIRS and parts[:i+1] not in coverage_roots | secrets_roots) or p=='.env' or p.startswith('.env.') for i,p in enumerate(parts)) or bool(parts and (parts[-1] in IGNORED_FILES or parts[-1].endswith(('.pyc','.prof','.test','.tsbuildinfo','.pem','.key','.p12','.pfx'))))
