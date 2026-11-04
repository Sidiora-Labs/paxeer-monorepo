import { readFileSync, readdirSync, realpathSync } from 'node:fs';
import { createRequire } from 'node:module';
import { resolve, sep } from 'node:path';
import { pathToFileURL } from 'node:url';
import { expect, test } from 'vitest';

const applicationRoot = resolve(__dirname, '../../..');

function swap_source_has_no_nested_install_manifests() {
  const paths = readdirSync(__dirname, { recursive: true }).map(String);
  expect(
    paths.filter((path) => /(^|[/\\])(package\.json|npm-shrinkwrap\.json)$/.test(path))
  ).toEqual([]);
}

function wallet_manifest_owns_swap_fixture_dependencies() {
  const manifest = JSON.parse(readFileSync(resolve(applicationRoot, 'package.json'), 'utf8'));
  expect(manifest.devDependencies).toMatchObject({
    '@ethereumjs/block': '10.1.3',
    '@ethereumjs/common': '10.1.3',
    '@ethereumjs/tx': '10.1.3',
    '@ethereumjs/util': '10.1.3',
    '@ethereumjs/vm': '10.1.3',
    solc: '0.8.27',
  });
}

async function ethereumjs_vm_import_resolves_from_wallet_application() {
  const applicationRequire = createRequire(resolve(applicationRoot, 'package.json'));
  const entry = applicationRequire.resolve('@ethereumjs/vm');
  const installedPackage = realpathSync(resolve(applicationRoot, 'node_modules/@ethereumjs/vm'));
  expect(entry.startsWith(`${installedPackage}${sep}`)).toBe(true);
  const vm = await import(pathToFileURL(entry).href);
  expect(vm.createVM).toBeTypeOf('function');
  expect(vm.runTx).toBeTypeOf('function');
}

test(swap_source_has_no_nested_install_manifests.name, swap_source_has_no_nested_install_manifests);
test(
  wallet_manifest_owns_swap_fixture_dependencies.name,
  wallet_manifest_owns_swap_fixture_dependencies
);
test(
  ethereumjs_vm_import_resolves_from_wallet_application.name,
  ethereumjs_vm_import_resolves_from_wallet_application
);
