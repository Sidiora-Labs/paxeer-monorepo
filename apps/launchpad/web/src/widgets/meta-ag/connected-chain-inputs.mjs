import { createHash, randomUUID } from 'node:crypto';
import { lstatSync, readFileSync, mkdirSync, writeFileSync } from 'node:fs';
import { dirname, resolve, relative } from 'node:path';
import { fileURLToPath } from 'node:url';

const sourceNames = [
  'advanced-provider.ts',
  'action-recovery.ts',
  'best-route-swap.tsx',
  'capability-gates.test.tsx',
  'connected-journeys.test.tsx',
  'domain.test.ts',
  'domain.ts',
  'liquidity-position.tsx',
  'measurement-harness.ts',
  'measurement-harness.test.ts',
  'open-orders-list.tsx',
  'order-form.tsx',
  'run-connected-chain.py',
  'connected-chain-inputs.mjs',
  'token-universe.ts',
  'vitest.l28.config.ts',
  'write-gates.test.tsx',
];

export function digest(path) {
  const info = lstatSync(path);
  if (!info.isFile() || info.isSymbolicLink())
    throw new Error(`Missing regular source/input: ${path}`);
  return createHash('sha256').update(readFileSync(path)).digest('hex');
}

export function prepareConnectedChainInputs({
  fixtureFile,
  fixtureSha256,
  output,
  measure = false,
  timeoutSeconds = 300,
}) {
  if (!Number.isInteger(timeoutSeconds) || timeoutSeconds < 1 || timeoutSeconds > 1800)
    throw new Error('Native execution cap must be1..1800seconds');
  fixtureFile = resolve(fixtureFile);
  if (digest(fixtureFile) !== fixtureSha256)
    throw new Error('Advanced fixture descriptor digest mismatch');
  const fixture = JSON.parse(readFileSync(fixtureFile, 'utf8'));
  if (fixture.realm !== 'private_fixture')
    throw new Error('Only an admitted private fixture may run funded qualification');
  if (
    measure &&
    (!fixture.measurement ||
      !Number.isInteger(fixture.measurement.durationMs) ||
      fixture.measurement.durationMs < 1 ||
      fixture.measurement.durationMs > 120000)
  )
    throw new Error(
      'Measurement requires explicit funded accounts and1..120000ms workload within the preserved240second lifecycle gate',
    );
  const source = dirname(fileURLToPath(import.meta.url));
  const web = resolve(source, '../../..');
  const root = resolve(web, '../../..');
  const files = [
    ...sourceNames.map((name) => resolve(source, name)),
    ...[
      'src/core/clients/meta-ag.ts',
      'src/widgets/trading/transaction-controls/recovery.ts',
      'src/widgets/trading/transaction-controls/domain.ts',
      'src/core/network/deployment.ts',
      'src/core/network/abis/MetaAGRouter.json',
      'src/core/network/abis/MetaAGQuoter.json',
      'src/core/network/abis/PECOROrders.json',
      'vitest.config.ts',
    ].map((name) => resolve(web, name)),
    resolve(root, 'apps/launchpad/pnpm-lock.yaml'),
  ];
  const sources = Object.fromEntries(files.map((path) => [relative(root, path), digest(path)]));
  const manifestFile = resolve(fixture.manifestFile);
  const manifestSha = digest(manifestFile);
  if (manifestSha !== fixture.manifestSha256)
    throw new Error('Advanced deployment manifest digest mismatch');
  const runId = randomUUID();
  mkdirSync(output, { recursive: true, mode: 0o700 });
  const run = resolve(output, runId);
  mkdirSync(run, { mode: 0o700 });
  const sourceManifest = resolve(run, 'source-manifest.json');
  writeFileSync(
    sourceManifest,
    JSON.stringify(
      {
        version: 1,
        runId,
        sourceRoot: root,
        scope: 'lane_and_direct_dependency_sources',
        files: sources,
      },
      null,
      2,
    ) + '\n',
    { mode: 0o600, flag: 'wx' },
  );
  const sourceManifestSha = digest(sourceManifest);
  const command = [
    'pnpm',
    'exec',
    'vitest',
    'run',
    'src/widgets/meta-ag/connected-journeys.test.tsx',
    'src/widgets/meta-ag/domain.test.ts',
    'src/widgets/meta-ag/write-gates.test.tsx',
    'src/widgets/meta-ag/capability-gates.test.tsx',
    'src/widgets/meta-ag/measurement-harness.test.ts',
    '--config',
    'src/widgets/meta-ag/vitest.l28.config.ts',
    '--reporter=json',
    '--outputFile',
    resolve(run, 'native.json'),
  ];
  const environment = {
    L28_CHAIN_FIXTURE_JSON: fixtureFile,
    L28_OBSERVATION_DIRECTORY: run,
    L28_SOURCE_MANIFEST_JSON: sourceManifest,
    L28_MEASURE: measure ? '1' : '0',
    L28_PRIVATE_CACHE_DIRECTORY: resolve(run, 'cache'),
  };
  return {
    root,
    web,
    files,
    sources,
    manifestFile,
    manifestSha,
    runId,
    run,
    sourceManifest,
    sourceManifestSha,
    command,
    environment,
  };
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const input = JSON.parse(readFileSync(0, 'utf8'));
  process.stdout.write(JSON.stringify(prepareConnectedChainInputs(input)) + '\n');
}
