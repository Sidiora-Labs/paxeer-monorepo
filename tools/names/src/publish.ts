import { createHash } from 'node:crypto';
import { readdir, readFile } from 'node:fs/promises';
import { extname, join } from 'node:path';
import { HeadObjectCommand, PutObjectCommand, S3Client } from '@aws-sdk/client-s3';
import {
  canonicalManifest,
  encodeContenthash,
  manifestDigest,
  namehash,
  normalize,
  type Manifest,
} from '@paxeer/names';
import {
  createPublicClient,
  createWalletClient,
  defineChain,
  http,
  parseAbi,
  type Address,
  type Hex,
} from 'viem';
import { privateKeyToAccount } from 'viem/accounts';

export const resolverAbi = parseAbi([
  'function setContenthash(bytes32 node, bytes hash)',
  'function contenthash(bytes32 node) view returns (bytes)',
]);

export const immutableCacheControl = 'public, max-age=31536000, immutable';

export type PublishOptions = {
  dir: string;
  name: string;
  bucket: {
    endpoint: string;
    region: string;
    bucket: string;
    publicBaseUrl: string;
    credentials: { accessKeyId: string; secretAccessKey: string; sessionToken?: string };
  };
  rpcUrl: string;
  resolver: Address;
  privateKey: Hex;
};

const contentTypes: Record<string, string> = {
  '.html': 'text/html',
  '.css': 'text/css',
  '.js': 'application/javascript',
  '.json': 'application/json',
  '.svg': 'image/svg+xml',
  '.png': 'image/png',
  '.jpg': 'image/jpeg',
  '.jpeg': 'image/jpeg',
  '.webp': 'image/webp',
  '.woff2': 'font/woff2',
  '.txt': 'text/plain',
  '.wasm': 'application/wasm',
};

function hasStatus(error: unknown, status: number): boolean {
  return (
    typeof error === 'object' &&
    error !== null &&
    '$metadata' in error &&
    typeof error.$metadata === 'object' &&
    error.$metadata !== null &&
    'httpStatusCode' in error.$metadata &&
    error.$metadata.httpStatusCode === status
  );
}

export async function publish({ dir, name, bucket, rpcUrl, resolver, privateKey }: PublishOptions) {
  const siteName = normalize(name);
  const node = namehash(siteName);
  const account = privateKeyToAccount(privateKey);
  const manifest: Manifest = { version: 1, name: siteName, entries: {} };
  const objects = new Map<string, { bytes: Buffer; type: string }>();

  async function walk(directory: string, prefix: string): Promise<void> {
    const entries = await readdir(directory, { withFileTypes: true });
    entries.sort((a, b) => a.name.localeCompare(b.name));
    for (const entry of entries) {
      const path = `${prefix}/${entry.name}`;
      const localPath = join(directory, entry.name);
      if (entry.isDirectory()) {
        await walk(localPath, path);
      } else if (entry.isFile()) {
        const bytes = await readFile(localPath);
        const sha256 = createHash('sha256').update(bytes).digest('hex');
        const type = contentTypes[extname(entry.name).toLowerCase()] ?? 'application/octet-stream';
        manifest.entries[path] = { sha256, size: bytes.length, type };
        objects.set(path, { bytes, type });
      } else {
        throw new Error(`Site contains a non-regular file: ${path}`);
      }
    }
  }

  await walk(dir, '');
  const bytes = Buffer.from(canonicalManifest(manifest));
  const digest = manifestDigest(manifest);
  const s3 = new S3Client({
    endpoint: bucket.endpoint,
    region: bucket.region,
    credentials: bucket.credentials,
    forcePathStyle: true,
  });

  async function upload(key: string, body: Buffer, contentType: string): Promise<boolean> {
    try {
      await s3.send(new HeadObjectCommand({ Bucket: bucket.bucket, Key: key }));
      return false;
    } catch (error) {
      if (!hasStatus(error, 404)) throw error;
    }
    try {
      await s3.send(
        new PutObjectCommand({
          Bucket: bucket.bucket,
          Key: key,
          Body: body,
          ContentType: contentType,
          CacheControl: immutableCacheControl,
          IfNoneMatch: '*',
        })
      );
      return true;
    } catch (error) {
      if (hasStatus(error, 412)) return false;
      throw error;
    }
  }

  let uploaded = 0;
  let skipped = 0;
  try {
    for (const [path, entry] of Object.entries(manifest.entries)) {
      const object = objects.get(path)!;
      if (await upload(`objects/${entry.sha256}`, object.bytes, object.type)) uploaded++;
      else skipped++;
    }
    await upload(`manifests/${digest.slice(2)}.json`, bytes, 'application/json');
  } finally {
    s3.destroy();
  }

  const publicClient = createPublicClient({ transport: http(rpcUrl) });
  const chain = defineChain({
    id: await publicClient.getChainId(),
    name: 'Paxeer',
    nativeCurrency: { name: 'PAX', symbol: 'PAX', decimals: 18 },
    rpcUrls: { default: { http: [rpcUrl] } },
  });
  const walletClient = createWalletClient({ account, chain, transport: http(rpcUrl) });
  const txHash = await walletClient.writeContract({
    address: resolver,
    abi: resolverAbi,
    functionName: 'setContenthash',
    args: [node, encodeContenthash(digest)],
  });
  const receipt = await publicClient.waitForTransactionReceipt({ hash: txHash });
  if (receipt.status !== 'success') throw new Error(`Contenthash transaction reverted: ${txHash}`);
  return { digest, uploaded, skipped, txHash };
}
