import { sha256 } from '@noble/hashes/sha2.js';
import { bytesToHex, utf8ToBytes } from '@noble/hashes/utils.js';
import { expect, test } from 'vitest';
import { canonicalManifest, manifestDigest, parseManifest, type Manifest } from '../src/index.js';

function manifest_digest_is_canonical_and_order_independent() {
  const html = utf8ToBytes('<h1>Paxeer</h1>');
  const css = utf8ToBytes('body{color:red}');
  const first: Manifest = {
    version: 1,
    name: 'example.pax',
    entries: {
      '/index.html': { sha256: bytesToHex(sha256(html)), size: html.length, type: 'text/html' },
      '/assets/site.css': { sha256: bytesToHex(sha256(css)), size: css.length, type: 'text/css' },
    },
  };
  const second: Manifest = {
    entries: {
      '/assets/site.css': { type: 'text/css', size: css.length, sha256: bytesToHex(sha256(css)) },
      '/index.html': { type: 'text/html', size: html.length, sha256: bytesToHex(sha256(html)) },
    },
    name: 'example.pax',
    version: 1,
  };
  const canonical = canonicalManifest(first);
  expect(canonical).toBe(canonicalManifest(second));
  expect(canonical).toBe(
    `{"entries":{"/assets/site.css":{"sha256":"${bytesToHex(sha256(css))}","size":${css.length},"type":"text/css"},"/index.html":{"sha256":"${bytesToHex(sha256(html))}","size":${html.length},"type":"text/html"}},"name":"example.pax","version":1}`
  );
  expect(manifestDigest(first)).toBe(manifestDigest(second));
  expect(manifestDigest(first)).toBe(`0x${bytesToHex(sha256(utf8ToBytes(canonical)))}`);
  expect(parseManifest(utf8ToBytes(canonical))).toEqual(first);
  expect(parseManifest(canonical)).toEqual(first);
  const changedHtml = html.slice();
  changedHtml[4] = changedHtml[4]! ^ 1;
  const changed: Manifest = {
    ...first,
    entries: {
      ...first.entries,
      '/index.html': { ...first.entries['/index.html']!, sha256: bytesToHex(sha256(changedHtml)) },
    },
  };
  expect(manifestDigest(changed)).not.toBe(manifestDigest(first));
  for (const path of ['/../secret', '/assets/../secret', '/assets/..', 'index.html']) {
    const invalid = { ...first, entries: { [path]: first.entries['/index.html'] } };
    expect(() => parseManifest(JSON.stringify(invalid)), path).toThrow(TypeError);
  }
  for (const sha of ['a'.repeat(63), 'a'.repeat(65), 'g'.repeat(64), `0x${'a'.repeat(64)}`]) {
    expect(() =>
      parseManifest(
        JSON.stringify({
          ...first,
          entries: { '/index.html': { ...first.entries['/index.html'], sha256: sha } },
        })
      )
    ).toThrow(TypeError);
  }
  for (const size of [-1, 0.5, Number.MAX_SAFE_INTEGER + 1, '14', null]) {
    expect(() =>
      parseManifest(
        JSON.stringify({
          ...first,
          entries: { '/index.html': { ...first.entries['/index.html'], size } },
        })
      )
    ).toThrow(TypeError);
  }
  for (const invalid of [
    null,
    [],
    {},
    { ...first, version: 2 },
    { ...first, name: 7 },
    { ...first, entries: [] },
    { ...first, entries: { '/': null } },
    { ...first, entries: { '/': { ...first.entries['/index.html'], type: 7 } } },
  ]) {
    expect(() => parseManifest(JSON.stringify(invalid))).toThrow(TypeError);
  }
  expect(() => parseManifest('{')).toThrow();
  expect(() => parseManifest(new Uint8Array([0xff]))).toThrow();
}

test(
  manifest_digest_is_canonical_and_order_independent,
  manifest_digest_is_canonical_and_order_independent
);
