import { readdir, readFile } from 'node:fs/promises';
import { dirname, extname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { serve } from '../../../../docs/site/docs-assistant/server/index.mjs';

const directory = dirname(fileURLToPath(import.meta.url));
const root = resolve(directory, '../../../../docs/site/paxeer-docs');
const answer = await readFile(resolve(directory, 'assistant-answer.sse'));
const error = await readFile(resolve(directory, 'assistant-error.sse'));
const invalidIndex = await readFile(resolve(directory, 'search-error.json'));
const searchIndex = await readFile(resolve(root, 'public/search-index.json'));

const contentTypes = { '.css': 'text/css', '.html': 'text/html; charset=utf-8', '.ico': 'image/x-icon', '.js': 'text/javascript', '.json': 'application/json', '.png': 'image/png', '.svg': 'image/svg+xml', '.txt': 'text/plain; charset=utf-8', '.woff': 'font/woff', '.woff2': 'font/woff2' };
const files = new Map();
async function cacheFiles(current, relative = '') {
  for (const entry of await readdir(current, { withFileTypes: true })) {
    const file = join(current, entry.name);
    const name = join(relative, entry.name);
    if (entry.isDirectory()) await cacheFiles(file, name);
    else if (entry.isFile()) files.set(`/${name}`, { body: await readFile(file), type: contentTypes[extname(file)] || 'application/octet-stream' });
  }
}
await cacheFiles(resolve(root, 'out'));

function serveStatic(request, response) {
  const url = new URL(request.url, 'http://localhost:4173');
  let pathname;
  try { pathname = decodeURIComponent(url.pathname); } catch { response.writeHead(400); response.end(); return; }
  if (!pathname.endsWith('/') && files.has(`${pathname}/index.html`)) {
    response.writeHead(308, { location: `${pathname}/${url.search}` });
    response.end();
    return;
  }
  const file = files.get(pathname.endsWith('/') ? `${pathname}index.html` : pathname);
  if (!file) {
    const notFound = files.get('/404.html');
    response.writeHead(404, { 'content-type': 'text/html; charset=utf-8', 'X-Robots-Tag': 'noindex' });
    response.end(request.method === 'HEAD' ? undefined : notFound?.body);
    return;
  }
  response.writeHead(200, { 'content-type': file.type, 'content-length': file.body.byteLength, 'cache-control': 'no-store' });
  response.end(request.method === 'HEAD' ? undefined : file.body);
}

process.env.PORT = '4173';
process.env.HOST = '127.0.0.1';
const server = await serve({ root, kind: 'static' });
const [staticRequest] = server.listeners('request');
server.removeListener('request', staticRequest);

server.on('request', async (request, response) => {
  const url = new URL(request.url, 'http://localhost:4173');
  const page = new URL(request.headers.referer ?? '/', url);
  const scenario = page.searchParams.get('render');
  if (url.pathname === '/search-index.json' && scenario?.startsWith('search-')) {
    const retry = request.headers.cookie?.includes('docs-search-retry=1');
    const body = scenario === 'search-error' || (scenario === 'search-retry' && !retry) ? invalidIndex : searchIndex;
    const send = () => {
      response.writeHead(200, {
        'content-type': 'application/json',
        'cache-control': 'no-store',
        ...(scenario === 'search-retry' ? { 'set-cookie': 'docs-search-retry=1; SameSite=Strict; Path=/' } : {}),
      });
      response.end(body);
    };
    if (scenario === 'search-loading') {
      const timer = setTimeout(send, 8000);
      response.once('close', () => clearTimeout(timer));
    } else send();
    return;
  }
  if (url.pathname === '/api/docs-assistant' && request.method === 'POST') {
    request.resume();
    const retry = request.headers.cookie?.includes('docs-answer-retry=1');
    const body = scenario === 'assistant-error' || (scenario === 'assistant-retry' && !retry) ? error : answer;
    const send = () => {
      response.writeHead(200, {
        'content-type': 'text/event-stream',
        'cache-control': 'no-store',
        ...(scenario === 'assistant-retry' ? { 'set-cookie': 'docs-answer-retry=1; SameSite=Strict; Path=/' } : {}),
      });
      response.end(body);
    };
    if (scenario === 'assistant-loading' || scenario === 'assistant-stopped') {
      const timer = setTimeout(send, 8000);
      response.once('close', () => clearTimeout(timer));
    } else send();
    return;
  }
  return serveStatic(request, response);
});
