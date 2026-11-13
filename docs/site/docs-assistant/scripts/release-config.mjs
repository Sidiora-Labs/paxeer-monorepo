import { createHash } from 'node:crypto';
import { inflateRawSync } from 'node:zlib';

export const PROJECT_ID = 'gwpvienQ1ylGNoU8HNM53';
export const ENVIRONMENT_ID = 'nDnMCP4WtVnHiWicMeNSf';
export const DOKPLOY_URL = 'https://cloud.centra.ag';
export const MODEL = 'openrouter/sakana/fugu-max';
export const SITES = Object.freeze({
  'learn-centra': { name: 'Learn Centra', origin: 'https://learn.centra.ag', output: '.output', kind: 'worker' },
  'learn-workmates': { name: 'Learn Workmates', origin: 'https://learn.workmates.ag', output: '.output', kind: 'worker' },
  'paxeer-docs': { name: 'Paxeer Docs', origin: 'https://docs.paxeer.app', output: 'out', kind: 'static' },
});

export class ReleaseError extends Error {
  constructor(code, message) { super(message); this.name = 'ReleaseError'; this.code = code; }
}
export function ensure(condition, code, message) { if (!condition) throw new ReleaseError(code, message); }
export const sha256 = data => createHash('sha256').update(data).digest('hex');
export const fingerprint = entries => sha256(JSON.stringify(entries.map(({ path, sha256, bytes }) => ({ path, sha256, bytes })).sort((a, b) => a.path.localeCompare(b.path))));
export function releaseId(value) {
  ensure(typeof value === 'string' && /^[A-Za-z0-9][A-Za-z0-9._-]{0,95}$/.test(value), 'invalid_release', 'Use a release identifier containing 1–96 letters, digits, dots, underscores or hyphens.');
  return value;
}
export function siteConfig(name) {
  ensure(Object.hasOwn(SITES, name), 'invalid_site', 'Select learn-centra, learn-workmates or paxeer-docs.');
  return SITES[name];
}
export function parseArgs(args, values, flags = []) {
  const options = {};
  for (let i = 0; i < args.length; i++) {
    const option = args[i];
    ensure(values.includes(option) || flags.includes(option), 'invalid_argument', 'An unsupported command argument was supplied. Use --help.');
    ensure(!Object.hasOwn(options, option), 'duplicate_argument', 'A command argument was supplied twice.');
    if (flags.includes(option)) options[option] = true;
    else { ensure(args[i + 1] && !args[i + 1].startsWith('--'), 'missing_argument', 'A command argument is missing its value.'); options[option] = args[++i]; }
  }
  return options;
}

function closesQuote(text, quote) {
  for (let i = 0; i < text.length; i++) {
    if (text[i] === '\\') { i++; continue; }
    if (text[i] === quote) return true;
  }
  return false;
}
export function environmentBlocks(text) {
  const lines = String(text || '').split('\n'), blocks = [];
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i], match = line.match(/^\s*(?:export\s+)?([A-Za-z_][A-Za-z0-9_.-]*)\s*=\s*(.*)$/);
    let value = line;
    if (match && ['"', "'", '`'].includes(match[2][0])) {
      const quote = match[2][0]; let closed = closesQuote(match[2].slice(1), quote);
      while (!closed && i + 1 < lines.length) { const next = lines[++i]; value += '\n' + next; closed = closesQuote(next, quote); }
      ensure(closed, 'invalid_existing_environment', 'The existing application environment contains an unterminated quoted value.');
    }
    blocks.push({ key: match?.[1] || null, text: value });
  }
  return blocks;
}
export function mergeEnvironment(existing, managed) {
  const names = Object.keys(managed);
  for (const name of names) {
    ensure(/^[A-Z][A-Z0-9_]*$/.test(name), 'invalid_environment_name', 'A managed environment variable name is invalid.');
    ensure(typeof managed[name] === 'string' && !/[\r\n\0]/.test(managed[name]), 'invalid_environment_value', 'A managed environment value must be a single line.');
  }
  const preserved = environmentBlocks(existing).filter(block => !names.includes(block.key)).map(block => block.text).join('\n');
  const additions = names.map(name => `${name}=${JSON.stringify(managed[name])}`).join('\n');
  return preserved + (preserved.endsWith('\n') || !preserved ? '' : '\n') + additions + '\n';
}
export const environmentNames = text => [...new Set(environmentBlocks(text).map(block => block.key).filter(Boolean))].sort();
export function releaseMetadataFromZip(zip) {
  const end = zip.length - 22;
  ensure(end >= 0 && zip.readUInt32LE(end) === 0x06054b50, 'zip_format', 'The release must use the ZIP format produced by package-release.mjs.');
  const count = zip.readUInt16LE(end + 10), centralSize = zip.readUInt32LE(end + 12); let cursor = zip.readUInt32LE(end + 16), found = null;
  ensure(cursor + centralSize === end, 'zip_format', 'The archive directory is invalid.');
  for (let i = 0; i < count; i++) {
    ensure(cursor + 46 <= end && zip.readUInt32LE(cursor) === 0x02014b50, 'zip_format', 'The archive directory entry is invalid.');
    const nameLength = zip.readUInt16LE(cursor + 28), extraLength = zip.readUInt16LE(cursor + 30), commentLength = zip.readUInt16LE(cursor + 32), next = cursor + 46 + nameLength + extraLength + commentLength;
    ensure(next <= end, 'zip_format', 'The archive directory entry exceeds its bounds.');
    const name = zip.subarray(cursor + 46, cursor + 46 + nameLength).toString('utf8');
    if (name === 'app/release.json') {
      ensure(found === null, 'duplicate_release_metadata', 'The archive contains duplicate release metadata.');
      const offset = zip.readUInt32LE(cursor + 42), size = zip.readUInt32LE(cursor + 20), uncompressed = zip.readUInt32LE(cursor + 24), method = zip.readUInt16LE(cursor + 10);
      ensure(offset + 30 < zip.length && zip.readUInt32LE(offset) === 0x04034b50 && uncompressed < 65536 && size < 65536, 'zip_metadata', 'The release metadata entry is invalid.');
      const start = offset + 30 + zip.readUInt16LE(offset + 26) + zip.readUInt16LE(offset + 28);
      ensure(start + size <= cursor && method === 8, 'zip_metadata', 'The release metadata compression or offset is invalid.');
      const data = inflateRawSync(zip.subarray(start, start + size), { maxOutputLength: 65536 });
      ensure(data.length === uncompressed, 'zip_metadata', 'The release metadata size is invalid.');
      found = JSON.parse(data.toString('utf8'));
    }
    cursor = next;
  }
  ensure(cursor === end && found, 'missing_release_metadata', 'The archive does not contain its release metadata.');
  return found;
}
export function bindArchiveManifest(manifest, zip) {
  const embedded = releaseMetadataFromZip(zip);
  for (const name of ['release', 'site', 'origin', 'sourceSha256', 'buildSha256']) ensure(embedded[name] === manifest[name], 'archive_manifest_mismatch', 'The archive and manifest identify different site, release or content fingerprints.');
}
export function safeFailure(error) {
  return error instanceof ReleaseError ? { code: error.code, message: error.message } : { code: 'operation_failed', message: `Operation failed (${error?.name || 'Error'}); no credential or response body is included.` };
}
