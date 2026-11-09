import { spawnSync } from 'node:child_process';
import { appendFileSync, copyFileSync, mkdirSync, mkdtempSync, readFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

import { expect, test } from 'vitest';

const sourceDir = dirname(fileURLToPath(import.meta.url));
const explorerDir = resolve(sourceDir, '../../..');
const repositoryDir = resolve(explorerDir, '../..');
const baseEnvs = {
  NEXT_PUBLIC_APP_PROTOCOL: 'https',
  NEXT_PUBLIC_APP_HOST: 'localhost',
  NEXT_PUBLIC_APP_PORT: '3000',
  NEXT_PUBLIC_APP_ENV: 'production',
  NEXT_PUBLIC_API_HOST: 'localhost',
  NEXT_PUBLIC_API_WEBSOCKET_PROTOCOL: 'wss',
  NEXT_PUBLIC_NETWORK_NAME: 'Paxeer',
  NEXT_PUBLIC_NETWORK_ID: '125',
};

function loadPaxeerPreset(directory: string): Record<string, string> {
  const presetDir = join(directory, 'configs/envs');
  mkdirSync(presetDir, { recursive: true });
  copyFileSync(join(explorerDir, 'configs/envs/paxeer-x.env'), join(presetDir, '.env.paxeer-x'));

  const entrypoint = readFileSync(join(explorerDir, 'deploy/scripts/entrypoint.sh'), 'utf8');
  const downloadOffset = entrypoint.indexOf('\n# Download external assets');
  expect(downloadOffset).toBeGreaterThan(0);
  const exported = spawnSync('bash', [ '-c', `${ entrypoint.slice(0, downloadOffset) }\nenv -0` ], {
    cwd: directory,
    env: { PATH: process.env.PATH, ...baseEnvs, ENVS_PRESET: 'paxeer-x' },
    encoding: 'utf8',
    timeout: 10_000,
  });
  expect(exported.status, exported.stdout + exported.stderr).toBe(0);

  return Object.fromEntries(exported.stdout.split('\0')
    .filter(entry => entry.startsWith('NEXT_PUBLIC_'))
    .map(entry => {
      const separator = entry.indexOf('=');
      return [ entry.slice(0, separator), entry.slice(separator + 1) ];
    }));
}

function explorerAssistantStartupValidation() {
  const directory = mkdtempSync(join(tmpdir(), 'explorer-assistant-'));
  try {
    mkdirSync(join(directory, 'envs-validator'));
    const validator = join(directory, 'envs-validator/index.mjs');
    copyFileSync(join(sourceDir, 'dist/index.js'), validator);

    const registry = spawnSync('bash', [
      join(explorerDir, 'deploy/scripts/collect_envs.sh'),
      join(repositoryDir, 'docs/explorer/ENVS.md'),
    ], {
      cwd: directory,
      env: { PATH: process.env.PATH },
      encoding: 'utf8',
      timeout: 10_000,
    });
    expect(registry.status, registry.stdout + registry.stderr).toBe(0);
    expect(readFileSync(join(directory, '.env.registry'), 'utf8')).toContain('NEXT_PUBLIC_ASSISTANT_ENABLED=__');

    const validate = (appEnvs: Record<string, string>) => spawnSync(process.execPath, [ validator ], {
      cwd: tmpdir(),
      env: { PATH: process.env.PATH, ...appEnvs },
      encoding: 'utf8',
      timeout: 10_000,
    });

    for (const enabled of [ 'true', 'false' ]) {
      const validation = validate({ ...baseEnvs, NEXT_PUBLIC_ASSISTANT_ENABLED: enabled });
      expect(validation.status, validation.stdout + validation.stderr).toBe(0);
      expect(validation.stdout).toContain('🌀 Validating ENV variables values...');
      expect(validation.stdout).toContain('👍 All good!');
    }

    const invalid = validate({ ...baseEnvs, NEXT_PUBLIC_ASSISTANT_ENABLED: 'invalid' });
    expect(invalid.status, invalid.stdout + invalid.stderr).toBe(1);
    expect(invalid.stdout).toContain('ENVs validation failed');
    expect(invalid.stdout).toContain('NEXT_PUBLIC_ASSISTANT_ENABLED must be a `boolean` type');

    const unknownKey = 'NEXT_PUBLIC_UNRELATED_UNKNOWN_SETTING';
    appendFileSync(join(directory, '.env.registry'), `${ unknownKey }=__\n`);
    const unknown = validate({ ...baseEnvs, NEXT_PUBLIC_ASSISTANT_ENABLED: 'false', [unknownKey]: 'true' });
    expect(unknown.status, unknown.stdout + unknown.stderr).toBe(1);
    expect(unknown.stdout).toContain('ENVs validation failed');
    expect(unknown.stdout).toContain(`Unknown ENV variables were provided: ${ unknownKey }`);

    const preset = loadPaxeerPreset(directory);
    expect(preset.NEXT_PUBLIC_APP_HOST).toBe(baseEnvs.NEXT_PUBLIC_APP_HOST);
    expect(preset.NEXT_PUBLIC_APP_ENV).toBe('production');
    expect(preset.NEXT_PUBLIC_NETWORK_NAME).toBe('Paxeer X Network');
    expect(preset.NEXT_PUBLIC_ASSISTANT_ENABLED).toBe('false');
    expect(preset.NEXT_PUBLIC_FOOTER_LINKS).toContain('/configs/paxeer-x/footer.json');
    const assetsDir = join(directory, 'public/assets/configs');
    mkdirSync(assetsDir, { recursive: true });
    copyFileSync(join(explorerDir, 'configs/paxeer-x/footer.json'), join(assetsDir, 'footer_links.json'));

    const validation = validate(preset);
    expect(validation.status, validation.stdout + validation.stderr).toBe(0);
    expect(validation.stdout).toContain('🌀 Validating ENV variables values...');
    expect(validation.stdout).toContain('👍 All good!');
  } finally {
    rmSync(directory, { recursive: true, force: true });
  }
}

test(explorerAssistantStartupValidation);
