import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { test } from 'node:test';
import { fileURLToPath } from 'node:url';
import { PHASE_PRODUCTION_BUILD } from 'next/constants';
import loadConfig from 'next/dist/server/config';
import type { webpack as Webpack } from 'next/dist/compiled/webpack/webpack';

async function productionPwaCleanupUsesCompatibleGlobApi() {
  const appDir = fileURLToPath(new URL('..', import.meta.url));
  const root = path.resolve(appDir, '../..');
  const appRequire = createRequire(path.join(appDir, 'package.json'));
  const pwaRequire = createRequire(appRequire.resolve('next-pwa'));
  const cleanupRequire = createRequire(pwaRequire.resolve('clean-webpack-plugin'));
  const delRequire = createRequire(cleanupRequire.resolve('del'));
  const webpack = pwaRequire('webpack');
  const { CleanWebpackPlugin } = pwaRequire('clean-webpack-plugin');
  const environment = { ...process.env };
  const configPath = appRequire.resolve('./next.config.js');
  const cachedConfig = appRequire.cache[configPath];
  const directory = mkdtempSync(path.join(tmpdir(), 'perps-pwa-cleanup-'));
  let compiler: Webpack.Compiler | undefined;

  try {
    Object.assign(process.env, { NODE_ENV: 'production' });
    delete appRequire.cache[configPath];
    const nextConfig = await loadConfig(PHASE_PRODUCTION_BUILD, appDir);
    assert.ok(nextConfig.webpack, 'production config must expose its webpack hook');
    assert.equal(cleanupRequire('del/package.json').version, '6.1.1');
    assert.match(delRequire('globby/package.json').version, /^11\./);
    assert.equal(typeof cleanupRequire('del').sync, 'function');

    const output = path.join(directory, 'public');
    const entry = path.join(directory, 'entry.js');
    mkdirSync(output);
    writeFileSync(entry, 'globalThis.pwaCleanupCompiled = true;\n');
    const staleFiles = ['sw.js', 'sw.js.map', 'workbox-stale.js', 'workbox-stale.js.map'];
    for (const filename of staleFiles) {
      writeFileSync(path.join(output, filename), `stale ${filename}\n`);
    }
    const unrelated = path.join(output, 'unrelated.js');
    const unrelatedContent = 'globalThis.unrelatedAsset = true;\n';
    writeFileSync(unrelated, unrelatedContent);

    const configuration: Webpack.Configuration = {
      mode: 'production',
      context: directory,
      entry: async () => ({ probe: entry }),
      output: { path: output, filename: '[name].js', publicPath: '/_next/' },
      plugins: [],
      externals: [],
    };
    const configured: Webpack.Configuration = await nextConfig.webpack(configuration, {
      dir: directory,
      dev: false,
      isServer: false,
      buildId: 'pwa-cleanup-regression',
      config: nextConfig,
      defaultLoaders: { babel: { loader: appRequire.resolve('next/dist/build/babel/loader') } },
      totalPages: 0,
      webpack,
    });
    const cleanupPlugins = configured.plugins.filter(
      (plugin: Webpack.WebpackPluginInstance) => plugin instanceof CleanWebpackPlugin,
    );
    assert.equal(cleanupPlugins.length, 1, 'production hook must select one PWA cleanup plugin');
    assert.deepEqual(cleanupPlugins[0].cleanOnceBeforeBuildPatterns, [
      path.join(output, 'workbox-*.js'),
      path.join(output, 'workbox-*.js.map'),
      path.join(output, 'sw.js'),
      path.join(output, 'sw.js.map'),
    ]);

    compiler = webpack({ ...configuration, plugins: cleanupPlugins });
    const stats = await new Promise<Webpack.Stats>((resolve, reject) => {
      compiler.run((error: Error | null | undefined, result: Webpack.Stats) => {
        if (error) reject(error);
        else if (!result) reject(new Error('webpack returned no compilation stats'));
        else resolve(result);
      });
    });
    assert.equal(stats.hasErrors(), false, stats.toString({ all: false, errors: true }));
    assert.ok(stats.compilation.assets['probe.js'], 'minimal entry must be compiled');
    assert.ok(existsSync(path.join(output, 'probe.js')), 'webpack must emit the compiled entry');
    for (const filename of staleFiles) {
      assert.equal(existsSync(path.join(output, filename)), false, `${filename} must be removed`);
    }
    assert.equal(readFileSync(unrelated, 'utf8'), unrelatedContent);

    const floorsPath = path.join(root, 'tools/deps/floors/perps-web.json');
    const floors = JSON.parse(readFileSync(floorsPath, 'utf8'));
    assert.deepEqual(Object.keys(floors.manifests).sort(), [
      'apps/perps/package-lock.json',
      'apps/perps/pnpm-lock.yaml',
    ]);
    const floorCheck = spawnSync('python3', ['tools/deps/lockfloor.py', floorsPath], {
      cwd: root,
      encoding: 'utf8',
    });
    assert.ifError(floorCheck.error);
    assert.equal(floorCheck.status, 0, `${floorCheck.stdout}${floorCheck.stderr}`);
  } finally {
    try {
      if (compiler) {
        await new Promise<void>((resolve, reject) => {
          compiler.close((error: Error | null | undefined) => (error ? reject(error) : resolve()));
        });
      }
    } finally {
      for (const key of Object.keys(process.env)) {
        if (!(key in environment)) Reflect.deleteProperty(process.env, key);
      }
      Object.assign(process.env, environment);
      if (cachedConfig) appRequire.cache[configPath] = cachedConfig;
      else delete appRequire.cache[configPath];
      rmSync(directory, { recursive: true, force: true });
    }
  }
}

test(productionPwaCleanupUsesCompatibleGlobApi);
