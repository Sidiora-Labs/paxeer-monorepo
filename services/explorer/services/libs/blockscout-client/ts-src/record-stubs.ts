import fs from 'fs-extra';
import fetch from 'node-fetch';
import path from 'path';

export async function recordResponses(
  restOrigin: string,
  sourceRevision: string,
  outputDir: string,
  paths: string[],
): Promise<void> {
  const origin = new URL(restOrigin);
  if (
    !['http:', 'https:'].includes(origin.protocol) ||
    !['127.0.0.1', '[::1]'].includes(origin.hostname) ||
    origin.pathname !== '/' || origin.search || origin.hash ||
    origin.username || origin.password
  ) {
    throw new Error('Supply an explicit loopback REST origin without credentials or paths');
  }
  if (!/^[0-9a-f]{40}$/.test(sourceRevision)) {
    throw new Error('Supply the full source backend revision');
  }
  for (const requestPath of paths) {
    if (!requestPath.startsWith('/api/') || requestPath.includes('#')) {
      throw new Error('Only mounted /api/ request paths and queries are accepted');
    }
    const url = new URL(requestPath, origin);
    if (url.origin !== origin.origin) {
      throw new Error('Recording cannot leave the explicit local origin');
    }
    const response = await fetch(url, { redirect: 'error', timeout: 30000, size: 8 * 1024 * 1024 });
    const body = await response.text();
    const contentType = response.headers.get('content-type');
    const recording = {
      metadata: { source_revision: sourceRevision, rest_origin: origin.origin },
      request: { method: 'GET', url: url.pathname + url.search },
      response: {
        status: response.status,
        body,
        headers: contentType === null ? {} : { 'Content-Type': contentType },
      },
    };
    const destination = path.join(outputDir, `${encodeURIComponent(requestPath)}.json`);
    await fs.outputJson(destination, recording, { spaces: 2, flag: 'wx' });
  }
}

async function main(): Promise<void> {
  const args = process.argv.slice(2);
  if (args.length < 3) {
    throw new Error('usage: record-stubs REST_ORIGIN SOURCE_REVISION OUTPUT_DIR [PATH ...]');
  }
  const [origin, revision, output] = args;
  const paths = args.length > 3 ? args.slice(3) : ['/api/health', '/api/v2/blocks'];
  await recordResponses(origin, revision, output, paths);
}

if (require.main === module) {
  main().catch((error: Error) => {
    console.error(error);
    process.exitCode = 1;
  });
}
