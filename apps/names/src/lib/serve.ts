import { createHash } from 'node:crypto';
import { parseManifest, type resolveContent } from '@paxeer/names';

export const contentSecurityPolicy =
  "default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; font-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'";

export type ServeDependencies = {
  resolve(name: string): ReturnType<typeof resolveContent>;
  bucketBaseUrl: string;
  fetch: typeof globalThis.fetch;
};

export function nameForHost(host: string, shellDomain: string): string | null {
  const domain = shellDomain.toLowerCase();
  if (!/^[a-z0-9.-]+(?::[0-9]+)?$/.test(domain)) return null;
  const suffix = `.pax.${domain}`;
  const normalized = host.toLowerCase();
  if (!normalized.endsWith(suffix)) return null;
  const label = normalized.slice(0, -suffix.length);
  return /^[a-z0-9][a-z0-9-]{1,61}[a-z0-9]$/.test(label) ? `${label}.pax` : null;
}

function failure(status: number, code: string): Response {
  return Response.json(
    { code },
    {
      status,
      headers: {
        'Cache-Control': 'no-store',
        'Content-Security-Policy': contentSecurityPolicy,
        'X-Content-Type-Options': 'nosniff',
      },
    }
  );
}

function sha256(bytes: Uint8Array): string {
  return createHash('sha256').update(bytes).digest('hex');
}

export async function serve(request: Request, deps: ServeDependencies): Promise<Response> {
  if (request.method !== 'GET' && request.method !== 'HEAD') {
    const response = failure(405, 'method_not_allowed');
    response.headers.set('Allow', 'GET, HEAD');
    return response;
  }
  const url = new URL(request.url);
  const host = request.headers.get('host') ?? url.host;
  const match = /^([a-z0-9][a-z0-9-]{1,61}[a-z0-9])\.pax\.[a-z0-9.-]+(?::[0-9]+)?$/i.exec(host);
  if (!match) return failure(404, 'invalid_site_host');
  const name = `${match[1]!.toLowerCase()}.pax`;
  let path: string;
  try {
    path = decodeURIComponent(url.pathname);
  } catch {
    return failure(400, 'invalid_site_path');
  }
  if (
    path.includes('\\') ||
    path.includes('\0') ||
    path.split('/').some((part) => part === '.' || part === '..')
  ) {
    return failure(400, 'invalid_site_path');
  }
  if (path.endsWith('/')) path += 'index.html';

  let resolution: Awaited<ReturnType<typeof resolveContent>>;
  try {
    resolution = await deps.resolve(name);
  } catch {
    return failure(502, 'resolution_failed');
  }
  if (resolution === null || resolution.digest === null) return failure(404, 'name_has_no_site');
  if (resolution.verified !== true) return failure(502, 'resolution_not_verified');
  if (!/^0x[0-9a-fA-F]{64}$/.test(resolution.digest) || resolution.height <= 0n) {
    return failure(502, 'invalid_resolution');
  }
  const digest = resolution.digest.toLowerCase();
  const baseUrl = deps.bucketBaseUrl.replace(/\/+$/, '');
  try {
    const manifestResponse = await deps.fetch(`${baseUrl}/manifests/${digest.slice(2)}.json`, {
      cache: 'no-store',
      redirect: 'error',
    });
    if (!manifestResponse.ok) return failure(502, 'content_unavailable');
    const manifestBytes = new Uint8Array(await manifestResponse.arrayBuffer());
    if (sha256(manifestBytes) !== digest.slice(2)) return failure(502, 'content_digest_mismatch');
    let manifest;
    try {
      manifest = parseManifest(manifestBytes);
    } catch {
      return failure(502, 'invalid_manifest');
    }
    if (manifest.name !== name) return failure(502, 'manifest_name_mismatch');
    const entry = Object.hasOwn(manifest.entries, path) ? manifest.entries[path] : undefined;
    if (!entry) return failure(404, 'site_path_not_found');
    const objectResponse = await deps.fetch(`${baseUrl}/objects/${entry.sha256.toLowerCase()}`, {
      cache: 'no-store',
      redirect: 'error',
    });
    if (!objectResponse.ok) return failure(502, 'content_unavailable');
    const bytes = await objectResponse.arrayBuffer();
    if (
      sha256(new Uint8Array(bytes)) !== entry.sha256.toLowerCase() ||
      bytes.byteLength !== entry.size
    ) {
      return failure(502, 'content_digest_mismatch');
    }
    return new Response(request.method === 'HEAD' ? null : bytes, {
      headers: {
        'Content-Type': entry.type,
        'Content-Length': String(bytes.byteLength),
        'Cache-Control': 'public, max-age=60',
        'Content-Security-Policy': contentSecurityPolicy,
        'X-Content-Type-Options': 'nosniff',
        'X-PNS-Name': name,
        'X-PNS-Manifest': digest,
        'X-PNS-Height': resolution.height.toString(),
      },
    });
  } catch {
    return failure(502, 'content_unavailable');
  }
}
