import { createServer } from 'node:http';
import { readFile, stat } from 'node:fs/promises';
import { createReadStream } from 'node:fs';
import { extname, resolve, sep } from 'node:path';
import { pathToFileURL } from 'node:url';
import { Readable } from 'node:stream';
import { createIndex } from './retrieval.mjs';
import { createAssistant } from './assistant.mjs';

const mime = { '.js':'text/javascript', '.css':'text/css', '.html':'text/html; charset=utf-8', '.json':'application/json', '.png':'image/png', '.jpg':'image/jpeg', '.jpeg':'image/jpeg', '.svg':'image/svg+xml', '.webp':'image/webp', '.avif':'image/avif', '.woff2':'font/woff2', '.woff':'font/woff', '.ico':'image/x-icon', '.txt':'text/plain; charset=utf-8', '.md':'text/markdown; charset=utf-8', '.xml':'application/xml', '.pdf':'application/pdf' };
async function fileStat(path) { try { return await stat(path); } catch (error) { if (['ENOENT','ENOTDIR'].includes(error.code)) return null; throw error; } }
export async function serve({ root, kind }) {
  const publicRoot = resolve(root, kind === 'static' ? 'out' : '.output/public');
  const document = JSON.parse(await readFile(resolve(root, '.generated/assistant-docs.json'), 'utf8'));
  if (!document.pages?.length) throw new Error('Build the documentation index before starting the server.');
  const index = createIndex(document), assistant = createAssistant(index);
  const worker = kind === 'static' ? null : (await import(pathToFileURL(resolve(root,'.output/server/index.mjs')).href)).default;
  const port = Number(process.env.PORT || 3000), host = process.env.HOST || '0.0.0.0';
  const configured = Boolean(process.env.CENTRA_GATEWAY_API_KEY || process.env.GATEWAY_ROUTER_API_KEY);
  const server = createServer(async (incoming, outgoing) => {
    try {
      const protocol = incoming.headers['x-forwarded-proto'] === 'https' ? 'https' : 'http';
      const url = new URL(incoming.url || '/', `${protocol}://${incoming.headers.host || 'localhost'}`);
      if (url.pathname === '/api/docs-assistant') return await assistant(incoming, outgoing);
      if (url.pathname === '/healthz') {
        outgoing.writeHead(200, { 'content-type':'application/json', 'cache-control':'no-store', 'X-Robots-Tag':'noindex' });
        return outgoing.end(JSON.stringify({ status:'ok', site:document.name, documents:document.pages.length, assistant:configured, release:process.env.RELEASE_ID || 'local' }));
      }
      if (!['GET','HEAD'].includes(incoming.method)) { outgoing.writeHead(405,{Allow:'GET, HEAD'}); return outgoing.end(); }
      let pathname;
      try { pathname = decodeURIComponent(url.pathname); } catch { outgoing.writeHead(400); return outgoing.end(); }
      let file = resolve(publicRoot, '.' + pathname), info;
      if (file !== publicRoot && !file.startsWith(publicRoot + sep)) { outgoing.writeHead(404); return outgoing.end(); }
      info = await fileStat(file);
      if (kind === 'static' && info?.isDirectory()) {
        if (!pathname.endsWith('/')) { outgoing.writeHead(308,{location:url.pathname+'/'+url.search}); return outgoing.end(); }
        file = resolve(file,'index.html'); info = await fileStat(file);
      }
      if (info?.isFile()) {
        const immutable = /\/(assets|_next\/static)\//.test(pathname);
        outgoing.writeHead(200, { 'content-type':mime[extname(file)] || 'application/octet-stream', 'content-length':info.size, 'cache-control':immutable?'public, max-age=31536000, immutable':'public, max-age=0, must-revalidate' });
        if (incoming.method === 'HEAD') return outgoing.end();
        const stream = createReadStream(file); stream.on('error', () => outgoing.destroy()); outgoing.on('close', () => stream.destroy()); return stream.pipe(outgoing);
      }
      if (worker) {
        const request = new Request(url, { method:incoming.method, headers:incoming.headers });
        const response = await worker.fetch(request, {}, { waitUntil:promise => promise.catch(error=>console.error(error.name)) });
        outgoing.writeHead(response.status,Object.fromEntries(response.headers));
        if (incoming.method === 'HEAD' || !response.body) return outgoing.end();
        const stream = Readable.fromWeb(response.body); stream.on('error', () => outgoing.destroy()); outgoing.on('close', () => stream.destroy()); return stream.pipe(outgoing);
      }
      outgoing.writeHead(404,{ 'content-type':'text/html; charset=utf-8', 'X-Robots-Tag':'noindex' });
      outgoing.end(incoming.method === 'HEAD' ? undefined : await readFile(resolve(publicRoot,'404.html')));
    } catch (error) {
      console.error('Docs server request failed', { type:error.name });
      if (!outgoing.headersSent) outgoing.writeHead(500,{ 'content-type':'text/plain', 'X-Robots-Tag':'noindex' });
      outgoing.end('This request could not be completed. Please try again.');
    }
  });
  server.listen(port,host,()=>console.log(`${document.name}: ${document.pages.length} docs, assistant ${configured?'configured':'unconfigured'}, listening on ${host}:${port}`));
  for (const signal of ['SIGTERM','SIGINT']) process.once(signal,()=>{ server.close(()=>process.exit(0)); setTimeout(()=>process.exit(0),10000).unref(); });
  return server;
}
