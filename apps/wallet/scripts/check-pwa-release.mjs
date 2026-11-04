import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { createServer } from 'vite';

const appDir = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');

export function checkManifest(manifest, { PWA_BASE_PATH, SHELL_URL }) {
  const problems = [];
  for (const key of [
    'id',
    'name',
    'short_name',
    'start_url',
    'scope',
    'background_color',
    'theme_color',
  ]) {
    if (typeof manifest[key] !== 'string' || manifest[key].length === 0) {
      problems.push(`missing ${key}`);
    }
  }
  for (const key of ['id', 'start_url', 'scope']) {
    if (manifest[key] !== SHELL_URL) problems.push(`${key} is not ${SHELL_URL}`);
  }
  if (manifest.display !== 'standalone') problems.push('display is not standalone');
  const icons = Array.isArray(manifest.icons) ? manifest.icons : [];
  for (const [size, purpose] of [
    ['192x192', 'any'],
    ['512x512', 'any'],
    ['192x192', 'maskable'],
    ['512x512', 'maskable'],
  ]) {
    if (!icons.some((icon) => icon.sizes === size && icon.purpose === purpose)) {
      problems.push(`no ${purpose} ${size} icon`);
    }
  }
  const publicDir = path.join(appDir, 'public');
  let checked = 0;
  for (const icon of icons) {
    if (typeof icon.src !== 'string' || !icon.src.startsWith(`${PWA_BASE_PATH}/`)) {
      problems.push('icon is outside the PWA mount');
      continue;
    }
    const relative = icon.src.slice(PWA_BASE_PATH.length + 1);
    const file = path.resolve(publicDir, relative);
    if (!file.startsWith(`${publicDir}${path.sep}`)) {
      problems.push('icon escapes public assets');
      continue;
    }
    let bytes;
    try {
      bytes = readFileSync(file);
    } catch {
      problems.push(`unreadable ${file}`);
      continue;
    }
    if (
      bytes.length < 24 ||
      !bytes.subarray(0, 8).equals(Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]))
    ) {
      problems.push(`${file} is not a PNG`);
      continue;
    }
    const size = `${bytes.readUInt32BE(16)}x${bytes.readUInt32BE(20)}`;
    if (size !== icon.sizes) problems.push(`${file} is ${size}, declared ${icon.sizes}`);
    checked += 1;
  }
  if (problems.length > 0) throw new Error(problems.join('; '));
  return `${checked} icons present at their declared sizes under ${PWA_BASE_PATH}`;
}

export function checkRegistration({ PWA_BASE_PATH, SERVICE_WORKER_URL, SHELL_URL }) {
  if (SHELL_URL !== `${PWA_BASE_PATH}/` || SERVICE_WORKER_URL !== `${PWA_BASE_PATH}/sw.js`) {
    throw new Error('Service worker URL and scope disagree with the PWA mount');
  }
  const registration = readFileSync(path.join(appDir, 'src/pwa/register.ts'), 'utf8');
  const provider = readFileSync(path.join(appDir, 'src/providers/PWAProvider.tsx'), 'utf8');
  const build = readFileSync(path.join(appDir, 'src/pwa/build.ts'), 'utf8');
  const importContract = /import\s*\{\s*SERVICE_WORKER_URL\s*\}\s*from\s*['"]\.\/caching['"]/;
  const registerContract =
    /container\.register\(SERVICE_WORKER_URL,\s*\{\s*scope:\s*['"]([^'"]+)['"],\s*updateViaCache:\s*['"]none['"]\s*\}\)/;
  if (
    !importContract.test(registration) ||
    registration.match(registerContract)?.[1] !== SHELL_URL
  ) {
    throw new Error('Registration does not consume the exported worker URL and mounted scope');
  }
  if (
    !/registerServiceWorker\(container\s*,/.test(provider) ||
    !/fileName:\s*\(\)\s*=>\s*['"]sw\.js['"]/.test(build)
  ) {
    throw new Error('Worker producer or provider registration is missing');
  }
  return `PWAProvider registers ${SERVICE_WORKER_URL} with scope ${SHELL_URL}`;
}

async function main() {
  const mode = process.argv[2];
  if (!['manifest', 'registration'].includes(mode))
    throw new Error('Expected manifest or registration');
  const loader = await createServer({
    configFile: false,
    root: appDir,
    publicDir: false,
    logLevel: 'error',
    appType: 'custom',
    server: { middlewareMode: true, hmr: false, ws: false },
    optimizeDeps: { noDiscovery: true, include: [] },
  });
  try {
    const contract = await loader.ssrLoadModule('/src/pwa/caching.ts');
    console.log(
      mode === 'manifest'
        ? checkManifest(
            JSON.parse(readFileSync(path.join(appDir, 'public/manifest.json'), 'utf8')),
            contract
          )
        : checkRegistration(contract)
    );
  } finally {
    await loader.close();
  }
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  main().catch((error) => {
    console.error(error.message);
    process.exitCode = 1;
  });
}
