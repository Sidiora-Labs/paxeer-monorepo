import { createHash } from 'node:crypto';
import { decodeContenthash, namehash, normalize, parseManifest } from '@paxeer/names';
import { createPublicClient, http, type Address } from 'viem';
import { resolverAbi } from './publish.ts';

export type VerifyOptions = {
  name: string;
  rpcUrl: string;
  resolver: Address;
  publicBaseUrl: string;
};

export type Mismatch = { path: string; expected: string; actual: string | null };

export async function verify({
  name,
  rpcUrl,
  resolver,
  publicBaseUrl,
}: VerifyOptions): Promise<Mismatch[]> {
  const client = createPublicClient({ transport: http(rpcUrl) });
  const siteName = normalize(name);
  const contenthash = await client.readContract({
    address: resolver,
    abi: resolverAbi,
    functionName: 'contenthash',
    args: [namehash(siteName)],
  });
  const decoded = decodeContenthash(contenthash);
  if (!decoded) throw new Error('Name has no supported contenthash');
  const baseUrl = publicBaseUrl.replace(/\/+$/, '');
  const manifestPath = `manifests/${decoded.digest.slice(2)}.json`;
  const response = await fetch(`${baseUrl}/${manifestPath}`);
  if (!response.ok) throw new Error(`Cannot fetch manifest: HTTP ${response.status}`);
  const bytes = new Uint8Array(await response.arrayBuffer());
  const actual = createHash('sha256').update(bytes).digest('hex');
  if (actual !== decoded.digest.slice(2)) {
    return [{ path: manifestPath, expected: decoded.digest.slice(2), actual }];
  }
  const manifest = parseManifest(bytes);
  if (manifest.name !== siteName) throw new Error('Manifest name differs from the requested name');
  const mismatches: Mismatch[] = [];
  for (const [path, entry] of Object.entries(manifest.entries)) {
    const object = await fetch(`${baseUrl}/objects/${entry.sha256.toLowerCase()}`);
    if (!object.ok) {
      mismatches.push({ path, expected: entry.sha256.toLowerCase(), actual: null });
      continue;
    }
    const objectBytes = new Uint8Array(await object.arrayBuffer());
    const actual = createHash('sha256').update(objectBytes).digest('hex');
    if (actual !== entry.sha256.toLowerCase() || objectBytes.length !== entry.size) {
      mismatches.push({ path, expected: entry.sha256.toLowerCase(), actual });
    }
  }
  return mismatches;
}
