import { spawnSync } from 'node:child_process';
import { copyFileSync, existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

import { afterEach, describe, expect, test } from 'vitest';

const sourceDir = dirname(fileURLToPath(import.meta.url));
const directories: Array<string> = [];
const baseEnvs = {
  NEXT_PUBLIC_APP_HOST: 'localhost',
  NEXT_PUBLIC_API_HOST: 'localhost',
  NEXT_PUBLIC_NETWORK_NAME: 'Paxeer',
  NEXT_PUBLIC_NETWORK_ID: '125',
};

function createRuntime(configEnvs: Record<string, string> = {}) {
  const directory = mkdtempSync(join(tmpdir(), 'explorer-external-json-'));
  directories.push(directory);
  mkdirSync(join(directory, 'envs-validator'));
  copyFileSync(join(sourceDir, 'dist/index.js'), join(directory, 'envs-validator/index.mjs'));

  const appEnvs = { ...baseEnvs, ...configEnvs };
  writeFileSync(join(directory, '.env'), '');
  writeFileSync(join(directory, '.env.registry'), Object.keys(appEnvs).map(name => `${ name }=`).join('\n'));

  const env = { PATH: process.env.PATH, ...appEnvs };
  const download = () => spawnSync('bash', [ resolve(sourceDir, '../../scripts/download_assets.sh'), './public/assets/configs' ], {
    cwd: directory,
    env,
    encoding: 'utf8',
    timeout: 10_000,
  });
  const validate = () => spawnSync(process.execPath, [ join(directory, 'envs-validator/index.mjs') ], {
    cwd: tmpdir(),
    env,
    encoding: 'utf8',
    timeout: 10_000,
  });

  return { directory, download, validate };
}

afterEach(() => {
  directories.splice(0).forEach(directory => rmSync(directory, { recursive: true, force: true }));
});

describe('external JSON validation filesystem layout', () => {
  test('validates inline footer JSON written by the real downloader without an explorer directory', () => {
    const runtime = createRuntime({ NEXT_PUBLIC_FOOTER_LINKS: '[]' });
    const download = runtime.download();
    expect(download.status, download.stdout + download.stderr).toBe(0);
    expect(readFileSync(join(runtime.directory, 'public/assets/configs/footer_links.json'), 'utf8').trim()).toBe('[]');
    expect(existsSync(join(runtime.directory, 'public/explorer'))).toBe(false);

    const validation = runtime.validate();
    expect(validation.status, validation.stdout + validation.stderr).toBe(0);
    expect(validation.stdout).toContain('👍 All good!');
  });

  test('validates all eight external JSON settings in the downloader layout', () => {
    const fixtures = join(sourceDir, 'test/assets/configs');
    const runtime = createRuntime({
      NEXT_PUBLIC_FEATURED_NETWORKS: `file://${ fixtures }/featured_networks.json`,
      NEXT_PUBLIC_MARKETPLACE_ENABLED: 'true',
      NEXT_PUBLIC_MARKETPLACE_SUBMIT_FORM: 'https://example.com/submit',
      NEXT_PUBLIC_MARKETPLACE_CONFIG_URL: `file://${ fixtures }/marketplace_config.json`,
      NEXT_PUBLIC_MARKETPLACE_CATEGORIES_URL: `file://${ fixtures }/marketplace_categories.json`,
      NEXT_PUBLIC_MARKETPLACE_GRAPH_LINKS_URL: '{}',
      NEXT_PUBLIC_FOOTER_LINKS: `file://${ fixtures }/footer_links.json`,
      NEXT_PUBLIC_ADDRESS_3RD_PARTY_WIDGETS_CONFIG_URL: `file://${ fixtures }/address_3rd_party_widgets_config.json`,
      NEXT_PUBLIC_ZETACHAIN_SERVICE_API_HOST: 'https://example.com/zetachain',
      NEXT_PUBLIC_ZETACHAIN_SERVICE_CHAINS_CONFIG_URL: `file://${ fixtures }/zetachain_service_chains_config.json`,
      NEXT_PUBLIC_HOMEPAGE_HIGHLIGHTS_CONFIG: `file://${ fixtures }/homepage_highlights_config.json`,
    });
    const download = runtime.download();
    expect(download.status, download.stdout + download.stderr).toBe(0);
    expect(existsSync(join(runtime.directory, 'public/explorer'))).toBe(false);

    const validation = runtime.validate();
    expect(validation.status, validation.stdout + validation.stderr).toBe(0);
  });

  test('rejects a missing downloaded file even when the browser-prefixed directory contains it', () => {
    const runtime = createRuntime({ NEXT_PUBLIC_FOOTER_LINKS: '[]' });
    const browserDirectory = join(runtime.directory, 'public/explorer/assets/configs');
    mkdirSync(browserDirectory, { recursive: true });
    writeFileSync(join(browserDirectory, 'footer_links.json'), '[]');

    const validation = runtime.validate();
    expect(validation.status).toBe(1);
    expect(validation.stdout).toContain('Unable to read file: ./public/assets/configs/footer_links.json');
    expect(validation.stderr).toContain('ENOENT');
  });

  test.each([ '[broken', '[{"title":"Invalid group","links":[{"text":"Missing URL"}]}]' ])(
    'still rejects malformed or schema-invalid downloaded JSON: %s',
    (content) => {
      const runtime = createRuntime({ NEXT_PUBLIC_FOOTER_LINKS: content });
      const download = runtime.download();
      expect(download.status, download.stdout + download.stderr).toBe(0);

      const validation = runtime.validate();
      expect(validation.status).toBe(1);
      expect(validation.stdout).toContain('ENVs validation failed');
      expect(validation.stdout).toContain('NEXT_PUBLIC_FOOTER_LINKS');
    },
  );

  test('does not require asset files when external JSON settings are absent', () => {
    const runtime = createRuntime();
    expect(existsSync(join(runtime.directory, 'public'))).toBe(false);

    const validation = runtime.validate();
    expect(validation.status, validation.stdout + validation.stderr).toBe(0);
  });
});
