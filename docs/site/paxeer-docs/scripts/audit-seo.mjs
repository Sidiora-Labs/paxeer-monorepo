#!/usr/bin/env node
// Adapted from the MIT-licensed Paxeer Docs v3 SEO audit; see THIRD_PARTY_NOTICES.md.
import { appendFile, readFile } from 'node:fs/promises';
import { extname, resolve, sep } from 'node:path';

const HELP = `Audit rendered public pages and crawler files without dependencies.

Usage:
  node scripts/audit-seo.mjs --base-url http://127.0.0.1:3000 --site-url https://docs.example.com
  node scripts/audit-seo.mjs --dir out --site-url https://docs.example.com

Options:
  --base-url URL   Read a running site's sitemap and rendered HTML over HTTP.
  --dir PATH      Read an existing static export (exclusive with --base-url).
  --site-url URL  Canonical site origin; defaults to SITE_URL, VITE_SITE_URL,
                  NEXT_PUBLIC_SITE_URL, or the static export sitemap origin.
  --exclude PATH  Exclude a route prefix; repeat as needed. /api, /search and
                  framework assets are excluded from page and sitemap coverage.
  --help          Print this help.

Checks: sitemap, title, description, canonical URL, Open Graph, Twitter cards,
one H1, content heading order, public local links/anchors, robots.txt, llms.txt
coverage and llms-full.txt. Only the supplied base URL or directory is read;
external links and remote image hosts are never crawled.

Errors fail the audit. Title (>60) and description (outside 50–160 characters)
lengths, heading skips and missing sitemap coverage are warnings by default.
SEO_STRICT=1 also fails on warnings. GITHUB_ACTIONS=true emits CI annotations
and appends a summary to GITHUB_STEP_SUMMARY when provided.
`;

const args = process.argv.slice(2);
if (args.includes('--help') || args.includes('-h')) {
  console.log(HELP);
  process.exit(0);
}

const options = { exclude: ['/api', '/search', '/_next', '/_build'] };
for (let i = 0; i < args.length; i++) {
  const name = args[i];
  if (!['--base-url', '--dir', '--site-url', '--exclude'].includes(name) || !args[i + 1] || args[i + 1].startsWith('--')) {
    console.error(`Invalid or incomplete option: ${name}. Use --help.`);
    process.exit(1);
  }
  const value = args[++i];
  if (name === '--exclude') options.exclude.push(value);
  else options[name.slice(2)] = value;
}

const findings = [];
const cache = new Map();
const pages = new Map();
const links = [];
let site;
let base;
let directory;
const add = (level, path, message) => findings.push({ level, path, message });
const error = (path, message) => add('error', path, message);
const warn = (path, message) => add('warning', path, message);
const normalize = (path) => path === '/' ? '/' : path.replace(/\/+$/, '');
const excluded = (path) => options.exclude.some((prefix) => path === prefix || path.startsWith(`${prefix}/`));
const isPage = (path) => !excluded(path) && (!extname(path) || extname(path) === '.html');
const entities = { amp: '&', quot: '"', apos: "'", lt: '<', gt: '>', nbsp: ' ', ndash: '–', mdash: '—', hellip: '…', rsquo: '’', lsquo: '‘', rdquo: '”', ldquo: '“' };
function decode(value) {
  return value.replace(/&(#x[0-9a-f]+|#\d+|[a-z]+);/gi, (raw, code) => {
    if (!code.startsWith('#')) return entities[code.toLowerCase()] ?? raw;
    const number = parseInt(code.slice(code[1].toLowerCase() === 'x' ? 2 : 1), code[1].toLowerCase() === 'x' ? 16 : 10);
    return number > 0 && number <= 0x10ffff ? String.fromCodePoint(number) : raw;
  });
}
const text = (html) => decode(html.replace(/<[^>]*>/g, ' ')).replace(/\s+/g, ' ').trim();
function attrs(raw) {
  const result = {};
  for (const match of raw.matchAll(/([^\s=/>]+)(?:\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]+)))?/g)) {
    result[match[1].toLowerCase()] = decode(match[2] ?? match[3] ?? match[4] ?? '');
  }
  return result;
}
function absolute(value, path, label) {
  try {
    const url = new URL(value);
    if (!['http:', 'https:'].includes(url.protocol)) throw new Error();
    return url;
  } catch {
    error(path, `${label} must be an absolute HTTP(S) URL: ${value || '(missing)'}`);
    return null;
  }
}
function samePage(a, b) {
  return a.origin === b.origin && normalize(a.pathname) === normalize(b.pathname) && a.search === b.search && a.hash === b.hash;
}
function localUrl(href, path) {
  try {
    const url = new URL(href, new URL(path, site));
    if (url.origin !== site.origin && url.origin !== base?.origin) return null;
    return url;
  } catch { return null; }
}

async function readResource(path) {
  const key = new URL(path, site).pathname;
  if (!cache.has(key)) cache.set(key, (async () => {
    if (directory) {
      let decoded;
      try { decoded = decodeURIComponent(key); } catch { return { status: 400, body: '' }; }
      const candidate = resolve(directory, `.${decoded}`);
      if (candidate !== directory && !candidate.startsWith(`${directory}${sep}`)) return { status: 403, body: '' };
      const choices = extname(candidate) ? [candidate] : [candidate, `${candidate}.html`, resolve(candidate, 'index.html')];
      for (const filename of choices) {
        try {
          const bytes = await readFile(filename);
          return { status: 200, body: bytes.toString('utf8'), filename };
        } catch (cause) {
          if (!['ENOENT', 'EISDIR', 'ENOTDIR'].includes(cause.code)) throw cause;
        }
      }
      return { status: 404, body: '' };
    }
    let url = new URL(key, base);
    for (let hops = 0; hops < 6; hops++) {
      const response = await fetch(url, { redirect: 'manual', signal: AbortSignal.timeout(15000) });
      if (response.status >= 300 && response.status < 400 && response.headers.has('location')) {
        const target = new URL(response.headers.get('location'), url);
        await response.body?.cancel();
        if (target.origin !== base.origin) return { status: response.status, body: '', issue: `Redirect leaves the supplied base URL (${target.origin})` };
        url = target;
        continue;
      }
      return { status: response.status, body: await response.text(), type: response.headers.get('content-type'), robots: response.headers.get('x-robots-tag') };
    }
    return { status: 310, body: '', issue: 'Too many redirects' };
  })().catch((cause) => ({ status: 0, body: '', issue: cause.message })));
  return cache.get(key);
}

async function requireResource(path) {
  const resource = await readResource(path);
  if (resource.status !== 200) error(path, `Expected HTTP 200 or an existing export file; got ${resource.status}${resource.issue ? ` (${resource.issue})` : ''}`);
  return resource;
}

async function sitemap(path = '/sitemap.xml', visited = new Set()) {
  if (visited.has(path)) return;
  visited.add(path);
  const resource = await requireResource(path);
  if (resource.status !== 200) return;
  const nested = /<sitemapindex\b/i.test(resource.body);
  if (!nested && !/<urlset\b/i.test(resource.body)) {
    error(path, 'Expected a sitemap urlset or sitemapindex XML document');
    return;
  }
  const locations = [...resource.body.matchAll(/<loc\b[^>]*>([\s\S]*?)<\/loc>/gi)];
  if (!locations.length) error(path, 'Sitemap has no URLs');
  for (const match of locations) {
    const url = absolute(decode(match[1].trim()), path, 'Sitemap location');
    if (!url) continue;
    if (url.origin !== site.origin || url.search || url.hash) {
      error(path, `Sitemap URL must use ${site.origin} with no query or fragment: ${url.href}`);
      continue;
    }
    if (nested) {
      if (visited.size >= 50) { error(path, 'Sitemap index exceeds the 50-file audit limit'); break; }
      await sitemap(url.pathname, visited);
    } else if (!excluded(url.pathname)) {
      const key = normalize(url.pathname);
      if (pages.has(key)) error(path, `Duplicate sitemap page: ${key}`);
      else pages.set(key, { url, ids: new Set() });
    }
  }
}

function inspectHtml(path, resource) {
  const clean = resource.body.replace(/<!--[\s\S]*?-->/g, '').replace(/<(script|style|template)\b[^>]*>[\s\S]*?<\/\1\s*>/gi, '');
  const head = clean.match(/<head\b[^>]*>([\s\S]*?)<\/head>/i)?.[1];
  if (!head) { error(path, 'No rendered HTML head was found'); return; }
  const titles = [...head.matchAll(/<title\b[^>]*>([\s\S]*?)<\/title>/gi)].map((match) => text(match[1]));
  if (titles.length !== 1 || !titles[0]) error(path, `Expected exactly one nonempty title; found ${titles.length}`);
  if (titles[0]?.length > 60) warn(path, `Title is ${titles[0].length} characters (recommended maximum 60)`);
  const meta = new Map();
  const canonical = [];
  for (const match of head.matchAll(/<(meta|link)\b((?:"[^"]*"|'[^']*'|[^'">])*)>/gi)) {
    const a = attrs(match[2]);
    if (match[1].toLowerCase() === 'link' && a.rel?.toLowerCase().split(/\s+/).includes('canonical')) canonical.push(a.href);
    if (match[1].toLowerCase() === 'meta') {
      const key = (a.name ?? a.property ?? '').toLowerCase();
      if (meta.has(key) && key && meta.get(key) !== a.content) warn(path, `Conflicting duplicate metadata: ${key}`);
      meta.set(key, a.content ?? '');
    }
  }
  const description = meta.get('description')?.trim();
  if (!description) error(path, 'Missing meta description');
  else if (description.length < 50 || description.length > 160) warn(path, `Description is ${description.length} characters (recommended 50–160)`);
  if (canonical.length !== 1) error(path, `Expected one canonical link; found ${canonical.length}`);
  else {
    const url = absolute(canonical[0], path, 'Canonical link');
    if (url && !samePage(url, pages.get(path).url)) error(path, `Canonical ${url.href} does not match sitemap URL ${pages.get(path).url.href}`);
  }
  for (const key of ['og:title', 'og:description', 'og:type', 'og:url', 'og:image', 'twitter:card', 'twitter:title', 'twitter:description', 'twitter:image']) {
    if (!meta.get(key)?.trim()) error(path, `Missing ${key} metadata`);
  }
  if (meta.get('og:url')) {
    const url = absolute(meta.get('og:url'), path, 'og:url');
    if (url && !samePage(url, pages.get(path).url)) error(path, 'og:url does not match the sitemap page URL');
  }
  for (const key of ['og:image', 'twitter:image']) {
    if (!meta.get(key)) continue;
    const url = absolute(meta.get(key), path, key);
    if (url?.origin === site.origin) links.push({ from: path, url, kind: key });
  }
  if (meta.get('twitter:card') && !['summary', 'summary_large_image', 'app', 'player'].includes(meta.get('twitter:card'))) error(path, 'Invalid twitter:card value');
  if (/\b(noindex|none)\b/i.test(`${meta.get('robots') ?? ''} ${meta.get('googlebot') ?? ''} ${resource.robots ?? ''}`)) error(path, 'Sitemap page is marked noindex');
  const headings = [];
  const stack = [];
  const voids = new Set(['area', 'base', 'br', 'col', 'embed', 'hr', 'img', 'input', 'link', 'meta', 'param', 'source', 'track', 'wbr']);
  let h1s = 0;
  for (const match of clean.matchAll(/<(\/?)([a-z][\w:-]*)\b((?:"[^"]*"|'[^']*'|[^'">])*)>/gi)) {
    const [, close, rawTag, rawAttrs] = match;
    const tag = rawTag.toLowerCase();
    if (close) {
      const index = stack.findLastIndex((item) => item.tag === tag);
      if (index !== -1) stack.length = index;
      continue;
    }
    const a = attrs(rawAttrs);
    const ignored = stack.some((item) => item.ignored) || ['nav', 'aside', 'footer'].includes(tag) || a.role === 'navigation' || 'hidden' in a || a['aria-hidden'] === 'true';
    if (a.id) pages.get(path).ids.add(a.id);
    if (/^h[1-6]$/.test(tag)) {
      if (tag === 'h1') h1s++;
      if (!ignored && stack.some((item) => item.tag === 'main' || item.tag === 'article')) headings.push(Number(tag[1]));
    }
    if (tag === 'a' && a.href && !('download' in a)) {
      const url = localUrl(a.href, pages.get(path).url.pathname);
      if (url && (!excluded(url.pathname) || extname(url.pathname) === '.md')) links.push({ from: path, url, kind: 'link' });
    }
    if (!voids.has(tag) && !rawAttrs.trimEnd().endsWith('/')) stack.push({ tag, ignored });
  }
  if (h1s !== 1) error(path, `Expected one rendered H1; found ${h1s}`);
  let previous = 1;
  for (const level of headings) {
    if (level > previous + 1) warn(path, `Content headings skip from H${previous || 1} to H${level}`);
    previous = level;
  }
}

async function inspectRobots() {
  const resource = await requireResource('/robots.txt');
  if (resource.status !== 200) return;
  if (/<html\b/i.test(resource.body)) { error('/robots.txt', 'Received HTML instead of robots directives'); return; }
  const expected = new URL('/sitemap.xml', site).href;
  const declared = [...resource.body.matchAll(/^\s*Sitemap:\s*(\S+)/gim)].map((match) => match[1]);
  if (!declared.includes(expected)) error('/robots.txt', `Missing absolute Sitemap: ${expected}`);
  let agents = [];
  let hasRules = false;
  const rules = [];
  for (const line of resource.body.split('\n')) {
    const match = line.replace(/#.*/, '').match(/^\s*([\w-]+):\s*(.*?)\s*$/);
    if (!match) continue;
    const key = match[1].toLowerCase();
    if (key === 'user-agent') {
      if (hasRules) { agents = []; hasRules = false; }
      agents.push(match[2].toLowerCase());
    } else if (['allow', 'disallow'].includes(key)) {
      hasRules = true;
      if (agents.includes('*') && match[2]) rules.push({ allow: key === 'allow', path: match[2] });
    }
  }
  for (const path of pages.keys()) {
    const matches = rules.filter((rule) => {
      const end = rule.path.endsWith('$');
      const pattern = (end ? rule.path.slice(0, -1) : rule.path).split('*').map((part) => part.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')).join('.*');
      return new RegExp(`^${pattern}${end ? '$' : ''}`).test(path);
    }).sort((a, b) => b.path.length - a.path.length || Number(b.allow) - Number(a.allow));
    if (matches[0] && !matches[0].allow) error('/robots.txt', `Public sitemap page ${path} is disallowed for User-agent: *`);
  }
}

async function inspectLlms() {
  const coverage = new Set();
  const rawSources = [];
  for (const path of ['/llms.txt', '/llms-full.txt']) {
    const resource = await requireResource(path);
    if (resource.status !== 200) continue;
    if (!/^#\s+\S/m.test(resource.body) || /<!doctype html|<html\b/i.test(resource.body)) error(path, 'Expected a Markdown document with a title');
    if (path === '/llms-full.txt') {
      for (const pair of resource.body.matchAll(/^URL:\s*(\S+)\s*\n+Source:\s*(\S+)/gm)) {
        const page = localUrl(pair[1], path);
        const source = localUrl(pair[2], path);
        if (page && source) rawSources.push({ page: normalize(page.pathname), source: normalize(source.pathname) });
      }
    }
    for (const match of resource.body.matchAll(/\[[^\]\n]*\]\((<?[^\s)]+>?)(?:\s+"[^"]*")?\)/g)) {
      const href = match[1].replace(/^<|>$/g, '');
      const url = localUrl(href, path);
      if (!url || (excluded(url.pathname) && extname(url.pathname) !== '.md')) continue;
      if (path === '/llms.txt') coverage.add(normalize(url.pathname));
      links.push({ from: path, url, kind: 'llms' });
    }
  }
  for (const pair of rawSources) if (coverage.has(pair.source)) coverage.add(pair.page);
  for (const path of pages.keys()) {
    if (!['/', '/docs'].includes(path) && !coverage.has(path)) warn('/llms.txt', `Sitemap documentation page is absent from the LLM index: ${path}`);
  }
}

async function inspectLinks() {
  const seen = new Set();
  for (const { from, url, kind } of links) {
    const path = normalize(url.pathname);
    const key = `${from}:${url.pathname}${url.hash}:${kind}`;
    if (seen.has(key)) continue;
    seen.add(key);
    const target = pages.get(path);
    if (target) {
      if (url.hash && !url.hash.startsWith('#:~:text=')) {
        let id;
        try { id = decodeURIComponent(url.hash.slice(1)); } catch { id = url.hash.slice(1); }
        if (!target.ids.has(id)) error(from, `Broken local heading link: ${url.pathname}${url.hash}`);
      }
    } else {
      const resource = await readResource(url.pathname);
      if (resource.status !== 200) error(from, `Broken local ${kind}: ${url.pathname} (${resource.status}${resource.issue ? `; ${resource.issue}` : ''})`);
      else if (isPage(path) && kind === 'link') warn(from, `Public page link is missing from sitemap: ${path}`);
    }
  }
}

function escapeAnnotation(value, property = false) {
  let result = value.replaceAll('%', '%25').replaceAll('\r', '%0D').replaceAll('\n', '%0A');
  if (property) result = result.replaceAll(':', '%3A').replaceAll(',', '%2C');
  return result;
}
async function report() {
  findings.sort((a, b) => a.level.localeCompare(b.level) || a.path.localeCompare(b.path) || a.message.localeCompare(b.message));
  const unique = findings.filter((item, i) => i === 0 || JSON.stringify(item) !== JSON.stringify(findings[i - 1]));
  for (const { level, path, message } of unique) {
    if (process.env.GITHUB_ACTIONS === 'true') {
      console.log(`::${level} title=${escapeAnnotation(`SEO ${path}`, true)}::${escapeAnnotation(message)}`);
    } else console.log(`${level === 'error' ? 'ERROR' : 'WARN '} ${path}  ${message}`);
  }
  const errors = unique.filter((item) => item.level === 'error').length;
  const warnings = unique.length - errors;
  const summary = `${errors} error(s), ${warnings} warning(s) across ${pages.size} sitemap pages; ${cache.size} local resources checked.`;
  console.log(summary);
  if (process.env.GITHUB_ACTIONS === 'true' && process.env.GITHUB_STEP_SUMMARY) await appendFile(process.env.GITHUB_STEP_SUMMARY, `### SEO and structure audit\n\n${summary}\n\nMode: ${options.dir ? 'static export' : 'rendered HTTP'}; canonical origin: ${site?.origin ?? '(invalid)'}.\n`);
  process.exitCode = errors || (process.env.SEO_STRICT === '1' && warnings) ? 1 : 0;
}

async function main() {
  if (Boolean(options.dir) === Boolean(options['base-url'])) throw new Error('Supply exactly one of --base-url or --dir; use --help for examples.');
  let configuredSite = options['site-url'] || process.env.SITE_URL || process.env.VITE_SITE_URL || process.env.NEXT_PUBLIC_SITE_URL;
  if (!configuredSite && options.dir) {
    const xml = await readFile(resolve(options.dir, 'sitemap.xml'), 'utf8');
    const firstLocation = xml.match(/<loc\b[^>]*>([\s\S]*?)<\/loc>/i)?.[1];
    if (firstLocation) configuredSite = new URL(decode(firstLocation.trim())).origin;
  }
  site = new URL(configuredSite || '');
  if (!['http:', 'https:'].includes(site.protocol) || site.pathname !== '/' || site.search || site.hash || site.username || site.password) throw new Error('--site-url must be an HTTP(S) origin without a path, credentials, query or fragment.');
  if (options.dir) directory = resolve(options.dir);
  else {
    base = new URL(options['base-url']);
    if (!['http:', 'https:'].includes(base.protocol) || base.pathname !== '/' || base.search || base.hash || base.username || base.password) throw new Error('--base-url must be an HTTP(S) origin without a path, credentials, query or fragment.');
  }
  console.log(`Auditing ${directory ?? base.origin} against ${site.origin}`);
  await sitemap();
  if (!pages.size) error('/sitemap.xml', 'No public pages were discovered');
  for (const [path] of pages) {
    const resource = await requireResource(path);
    if (resource.status === 200) inspectHtml(path, resource);
  }
  await inspectRobots();
  await inspectLlms();
  await inspectLinks();
}

await main().catch((cause) => error('(audit)', cause.message));
await report();
