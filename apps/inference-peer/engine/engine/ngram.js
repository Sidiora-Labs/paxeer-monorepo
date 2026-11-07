// Vendored from https://github.com/Nehanth/pooled/blob/45b5ee275a8754a37fce4a6ce174f31e4a216ef4/engine/ngram.js (MIT; see LICENSE-ENGINE).
// N-gram row source for Qwen3.8-Flash-Next (qwen4exp): the 51B-parameter "per layer" n-gram embedding that the
// model injects into its residual at layer 1 (roadmap/33-qwen38-flash.md §1 "N-gram embedding (PLE)", §6 PR 3).
//
// The table, per_layer_token_embd.weight, has 320,001,536 rows of 160 values (28.8 GB at Q4_0, 90 bytes a row).
// It never goes to the GPU and is never converted: it stays on disk and each token reads 16 rows of it, one per hash
// head. Heads 0-7 hash the bigram (token, previous token), heads 8-15 the trigram, and head h owns the contiguous
// rows [head_offsets[h], head_offsets[h] + head_vocab_sizes[h]). The 16 rows, dequantized and laid end to end,
// are the token's 2560-float n-gram embedding.
//
// This file has four parts:
//   ngramConfig    the GGUF keys and the tensor entry, checked
//   NgramState     the hash and the predecessor tokens it needs, with export / import / rollback, so speculation,
//                  sessions and multi-turn chats keep computing the same rows as a straight run would
//   NgramTable     rows -> Float32Array: reads through an injected readAt, a small LRU of hot rows, and a row
//                  dequant that gives exactly engine/gguf.js dequantF32's floats
//   readers        readAt(file, byteOffset, byteLength) -> Promise<Uint8Array> for memory, positional file reads
//                  (Node / Deno, handles injected), OPFS sync access handles (in a worker) and HTTP range requests
//
// Two layouts of the bytes:
//   "tensor"  the table where the GGUF has it: file = the shard, at the tensor's byteOffset (a room node reading the
//             model file, a cold start over HTTP)
//   "split"   one file per hash head, file = h, row r of the head at r * rowBytes (OPFS: a sync access handle is
//             exclusive per file, so 16 files are what lets several workers read at once;
//             roadmap/33-qwen38-flash.md §3)
//
// Indices depend only on tokens, so a decode step can start its read as soon as the token is sampled and let layer 0
// run while it lands. Browser-safe: no node: imports, nothing here touches the GPU.
import { f16ToF32, dequantF32, ggmlTypeBytes, GGML_Q4_0, GGML_Q8_0 } from "./gguf.js";

export const NGRAM_TENSOR = "per_layer_token_embd.weight";
const ARCH = "qwen4exp";

// ------------------------------------------------------------------ config

// G: a parsed (or merged split) GGUF header. Returns everything the hash and the readers need.
export function ngramConfig(G, arch = ARCH) {
  const m = G.meta || {}, k = (s) => m[`${arch}.ple.${s}`];
  const info = G.tensors?.[NGRAM_TENSOR];
  if (!info) throw new Error(`n-gram table: no ${NGRAM_TENSOR} in the GGUF`);
  const offsets = k("head_offsets"), vocab = k("head_vocab_sizes"), mult = k("layer_multipliers");
  const ngramSize = k("ngram_size"), perNgram = k("heads_per_ngram");
  const width = m[`${arch}.embedding_length_per_layer_input`] ?? info.shape[1];
  if (!offsets || !vocab || !mult || !ngramSize || !perNgram) throw new Error(`n-gram table: missing ${arch}.ple.* keys`);
  return makeConfig({ offsets, vocab, mult, ngramSize, perNgram, width, eos: k("eos_token_id"), image: k("image_token_id"),
    ggmlType: info.ggmlType, nRows: info.shape[0], byteOffset: info.byteOffset, shard: info.shard || 0, dim: m[`${arch}.embedding_length`] });
}

// The same from plain values (tests, a synthetic table, a split-file manifest).
export function makeConfig({ offsets, vocab, mult, ngramSize = 3, perNgram = 8, width = 160, eos, image = 0, ggmlType = GGML_Q4_0,
  nRows = null, byteOffset = 0, shard = 0, dim = null }) {
  const nHeads = (ngramSize - 1) * perNgram;
  if (offsets.length !== nHeads || vocab.length !== nHeads) throw new Error(`n-gram table: ${offsets.length} offsets / ${vocab.length} sizes for ${nHeads} heads`);
  if (mult.length !== ngramSize) throw new Error(`n-gram table: ${mult.length} multipliers for n-grams up to ${ngramSize}`);
  // the multipliers are 45-bit: exact as Numbers, but the products need 64-bit arithmetic (BigInt below)
  for (const x of mult) if (!Number.isSafeInteger(x) || x < 0) throw new Error(`n-gram table: multiplier ${x} is not an exact integer`);
  if (eos == null) throw new Error("n-gram table: no eos_token_id");
  // llama.cpp qwen4exp set_input: img_tok = image_token_id != 0 ? image_token_id : eos (a missing key reads as 0)
  if (!image) image = eos;
  if (!(Number.isInteger(image) && image >= 0)) throw new Error(`n-gram table: image_token_id ${image}`);
  const rowBytes = ggmlTypeBytes(ggmlType, width);
  if (!(rowBytes > 0) || rowBytes % 1) throw new Error(`n-gram table: ${width} values of ggml type ${ggmlType} are not whole blocks`);
  const end = offsets[nHeads - 1] + vocab[nHeads - 1];
  if (nRows != null && end > nRows) throw new Error(`n-gram table: heads end at row ${end}, the tensor has ${nRows}`);
  if (dim != null && dim !== nHeads * width) throw new Error(`n-gram table: ${nHeads} x ${width} values is not the hidden size ${dim}`);
  return { offsets: [...offsets], vocab: [...vocab], mult: [...mult], multB: mult.map(BigInt), vocabB: vocab.map(BigInt),
    ngramSize, perNgram, nHeads, width, dim: nHeads * width, eos, image, ggmlType, rowBytes, nRows: nRows ?? end, byteOffset, shard };
}

// ------------------------------------------------------------------ hash

// The 16 global row indices for a token and its two predecessors (already cut at EOS: see NgramState).
// llama.cpp qwen4exp.cpp:1196-1295: mixed = c0*m0 ^ c1*m1 [^ c2*m2] as u64, row = mixed % vocab[h] + offset[h].
// The products are below 2^63 (ids < 2^18, multipliers < 2^45), but a double would round them: BigInt, never Number.
export function ngramRowsOf(cfg, c0, c1, c2, out = new Uint32Array(cfg.nHeads), o = 0) {
  const { multB, vocabB, offsets, perNgram, ngramSize } = cfg;
  let mixed = BigInt(c0) * multB[0];
  const ctx = [c0, c1, c2];
  for (let n = 2; n <= ngramSize; n++) {
    mixed = BigInt.asUintN(64, mixed ^ (BigInt(ctx[n - 1]) * multB[n - 1]));
    for (let g = 0; g < perNgram; g++) {
      const h = (n - 2) * perNgram + g;
      out[o + h] = Number(mixed % vocabB[h]) + offsets[h];
    }
  }
  return out;
}

// ------------------------------------------------------------------ predecessor state

// The tokens before the next one, as the hash sees them: p1 is the previous token, p2 the one before. A predecessor
// that does not exist (sequence start), or that has an EOS at or after it, reads as EOS; the token's own EOS does
// not cut its own context (roadmap/33-qwen38-flash.md §1). Image positions are the caller's: it pushes cfg.image for
// them (the file's image_token_id, or EOS when the key is missing or 0, as llama.cpp's set_input does).
//
// push(tokens) hashes the tokens in order and advances; it returns their rows (n x 16, Uint32Array). A short history
// of states (HIST deep) backs rollback(k): a speculative verify pushes its draft tokens and rolls back the rejected
// ones. export() / import() carry the state across sessions and room checkpoints (a few numbers, not the tokens).
export class NgramState {
  constructor(cfg, { history = 64 } = {}) {
    this.cfg = cfg; this.H = history;
    this.reset();
  }
  reset() {
    const E = this.cfg.eos;
    this.p1 = E; this.p2 = E; this.pos = 0;
    this.hist = new Int32Array(this.H * 2); this.nHist = 0;   // ring of (p1, p2) before each of the last pushes
  }
  // rows for tokens without advancing (a draft's look ahead)
  peek(tokens) { return this._run(tokens, false); }
  push(tokens) { return this._run(tokens, true); }
  _run(tokens, advance) {
    const cfg = this.cfg, E = cfg.eos, n = tokens.length, out = new Uint32Array(n * cfg.nHeads);
    // every token checked before any state moves: a throw leaves the state as it was
    for (let i = 0; i < n; i++) if (!(Number.isInteger(tokens[i]) && tokens[i] >= 0)) throw new Error(`n-gram: bad token ${tokens[i]}`);
    let p1 = this.p1, p2 = this.p2;
    for (let i = 0; i < n; i++) {
      const t = tokens[i];
      ngramRowsOf(cfg, t, p1, p2, out, i * cfg.nHeads);
      if (advance) {
        const s = (this.pos % this.H) * 2;
        this.hist[s] = p1; this.hist[s + 1] = p2;
        this.pos++; this.nHist = Math.min(this.nHist + 1, this.H);
      }
      // the next token's predecessors: an EOS cuts everything older than itself
      p2 = t === E ? E : p1; p1 = t;
    }
    if (advance) { this.p1 = p1; this.p2 = p2; }
    return out;
  }
  // undo the last k pushed tokens (k <= the history depth, and not past an import)
  rollback(k) {
    if (!(Number.isInteger(k) && k >= 0)) throw new Error(`n-gram: rollback(${k})`);
    if (k === 0) return;
    if (k > this.nHist) throw new Error(`n-gram: rollback of ${k} tokens, only ${this.nHist} are kept`);
    const s = ((this.pos - k) % this.H) * 2;
    this.p1 = this.hist[s]; this.p2 = this.hist[s + 1];
    this.pos -= k; this.nHist -= k;
  }
  export() { return { p1: this.p1, p2: this.p2, pos: this.pos }; }
  import(s) {
    if (!s || !Number.isInteger(s.p1) || !Number.isInteger(s.p2) || !Number.isInteger(s.pos)) throw new Error("n-gram: bad state");
    if (s.p1 === this.cfg.eos && s.p2 !== this.cfg.eos) throw new Error("n-gram: state has a predecessor older than an EOS");
    this.reset();
    this.p1 = s.p1; this.p2 = s.p2; this.pos = s.pos;
  }
}

// ------------------------------------------------------------------ row dequant

// f16 bits -> f32, as a table: exactly f16ToF32 (every f16 is an f32), without its per-call branches
let F16 = null;
function f16Table() {
  if (!F16) { F16 = new Float32Array(65536); for (let i = 0; i < 65536; i++) F16[i] = f16ToF32(i); }
  return F16;
}

// One row of cfg.width values from src[so ..] into out[oo ..]. Equal to dequantF32 on the same bytes: the same
// double products (scale x small integer) rounded once to f32 on the store. Q4_0 / Q8_0 inline (what every file we
// load has for this table), anything else through dequantF32 itself.
export function dequantRow(cfg, src, so, out, oo) {
  const T = cfg.ggmlType, n = cfg.width;
  if (T === GGML_Q4_0) {
    const H = f16Table();
    for (let b = 0; b < n / 32; b++) {
      const base = so + b * 18, d = H[src[base] | (src[base + 1] << 8)], y = oo + b * 32;
      for (let l = 0; l < 16; l++) {
        const q = src[base + 2 + l];
        out[y + l] = d * ((q & 0xF) - 8);
        out[y + l + 16] = d * ((q >> 4) - 8);
      }
    }
    return;
  }
  if (T === GGML_Q8_0) {
    const H = f16Table();
    for (let b = 0; b < n / 32; b++) {
      const base = so + b * 34, d = H[src[base] | (src[base + 1] << 8)], y = oo + b * 32;
      for (let i = 0; i < 32; i++) { const q = src[base + 2 + i]; out[y + i] = d * (q > 127 ? q - 256 : q); }
    }
    return;
  }
  // a fresh copy: ggufToF32 views F32 / F16 rows in place and needs them aligned
  out.set(dequantF32({ ggmlType: T, nElems: n, shape: [1, n] }, src.slice(so, so + cfg.rowBytes)), oo);
}

// ------------------------------------------------------------------ the table

// Where a global row lives: [file, byteOffset].
export function rowAddress(cfg, layout, row) {
  if (layout === "tensor") return [cfg.shard, cfg.byteOffset + row * cfg.rowBytes];
  // split: find the head (16 entries; a scan is as fast as anything)
  const { offsets, vocab } = cfg;
  for (let h = cfg.nHeads - 1; h >= 0; h--) if (row >= offsets[h]) {
    if (row - offsets[h] >= vocab[h]) break;
    return [h, (row - offsets[h]) * cfg.rowBytes];
  }
  throw new Error(`n-gram: row ${row} is in no head`);
}

// rows(ids) -> Float32Array of ids.length x width (for a token's 16 rows: its 2560-float embedding).
//   readAt(file, byteOffset, byteLength) -> Promise<Uint8Array>   (the readers below)
//   layout: "split" (file = head) or "tensor" (file = shard, at cfg.byteOffset)
//   cacheRows: raw rows kept in an LRU (hot n-grams, repeats in a prompt). 0 turns it off.
//   inflight: reads issued at once (an HTTP source wants fewer than an NVMe)
export class NgramTable {
  constructor(cfg, { readAt, layout = "split", cacheRows = 8192, inflight = 64 } = {}) {
    if (typeof readAt !== "function") throw new Error("NgramTable: readAt(file, byteOffset, byteLength) is required");
    if (layout !== "split" && layout !== "tensor") throw new Error(`NgramTable: layout ${layout}`);
    this.cfg = cfg; this.readAt = readAt; this.layout = layout; this.cacheRows = cacheRows; this.inflight = Math.max(1, inflight);
    this.lru = new Map();   // row -> Uint8Array(rowBytes), oldest first
    this.stats = { rows: 0, hits: 0, reads: 0, bytes: 0 };
  }
  async rows(ids, out = new Float32Array(ids.length * this.cfg.width)) {
    const cfg = this.cfg, W = cfg.width, RB = cfg.rowBytes;
    if (out.length < ids.length * W) throw new Error("NgramTable.rows: output too small");
    // each distinct row once: a prompt repeats n-grams, and the 16 heads of one token never collide but tokens do
    const need = new Map();   // row -> raw bytes (filled below)
    for (const r of ids) {
      if (!(Number.isInteger(r) && r >= 0 && r < cfg.nRows)) throw new Error(`n-gram: row ${r} out of range`);
      if (need.has(r)) continue;
      const hit = this.lru.get(r);
      if (hit) { this.lru.delete(r); this.lru.set(r, hit); this.stats.hits++; }
      need.set(r, hit || null);
    }
    const miss = [...need.keys()].filter((r) => !need.get(r));
    // failed: once one read throws, the other loops stop issuing reads (Promise.all has already rejected)
    let next = 0, failed = false;
    const worker = async () => {
      try {
        while (!failed && next < miss.length) {
          const r = miss[next++], [file, off] = rowAddress(cfg, this.layout, r);
          const b = await this.readAt(file, off, RB);
          if (!b || b.byteLength < RB) throw new Error(`n-gram: short read of row ${r} (${b?.byteLength ?? 0} of ${RB} B)`);
          this.stats.reads++; this.stats.bytes += RB;
          need.set(r, b.byteLength === RB ? b : b.subarray(0, RB));
        }
      } catch (e) { failed = true; throw e; }
    };
    await Promise.all(Array.from({ length: Math.min(this.inflight, miss.length) }, worker));
    for (let i = 0; i < ids.length; i++) dequantRow(cfg, need.get(ids[i]), 0, out, i * W);
    this.stats.rows += ids.length;
    if (this.cacheRows > 0) for (const r of miss) {
      this.lru.set(r, need.get(r).slice());   // a copy: a reader may hand back a view of a bigger, reused buffer
      if (this.lru.size > this.cacheRows) this.lru.delete(this.lru.keys().next().value);
    }
    return out;
  }
}

// ------------------------------------------------------------------ readers
// Each returns readAt(file, byteOffset, byteLength) -> Promise<Uint8Array>.

// files: Uint8Array per file (tests; a table small enough for memory)
export function memoryReadAt(files) {
  const list = Array.isArray(files) ? files : [files];
  return async (file, off, len) => {
    const f = list[file];
    if (!f) throw new Error(`n-gram: no file ${file}`);
    if (off < 0 || off + len > f.byteLength) throw new Error(`n-gram: read ${off}+${len} past the end of file ${file} (${f.byteLength} B)`);
    return f.subarray(off, off + len);
  };
}

// Positional reads on open files. handles[file] is a Node fs.promises FileHandle (read(buf, 0, len, pos)) or a
// Deno.FsFile; Deno has no positional read on a handle, so its reads are seek + read, one at a time per file.
export function fileReadAt(handles) {
  const chains = new Map();
  return async (file, off, len) => {
    const h = handles[file];
    if (!h) throw new Error(`n-gram: no file ${file}`);
    const buf = new Uint8Array(len);
    if (typeof h.seek !== "function") {   // Node FileHandle
      let got = 0;
      while (got < len) { const { bytesRead } = await h.read(buf, got, len - got, off + got); if (!bytesRead) break; got += bytesRead; }
      return buf.subarray(0, got);
    }
    const run = async () => {
      await h.seek(off, 0);   // Deno.SeekMode.Start
      let got = 0;
      while (got < len) { const n = await h.read(buf.subarray(got)); if (!n) break; got += n; }
      return buf.subarray(0, got);
    };
    const p = (chains.get(file) || Promise.resolve()).then(run, run);
    chains.set(file, p.catch(() => {}));
    return p;
  };
}

// OPFS: FileSystemSyncAccessHandle per file (they exist only in a dedicated worker; see engine/ngram_worker.js).
// The reads are synchronous, so the in-flight limit does nothing here: one worker reads one row at a time, and
// parallelism comes from several workers owning different heads.
export function opfsReadAt(syncHandles) {
  return async (file, off, len) => {
    const h = syncHandles[file];
    if (!h) throw new Error(`n-gram: no OPFS file ${file}`);
    const buf = new Uint8Array(len);
    const got = h.read(buf, { at: off });
    return buf.subarray(0, got);
  };
}

const PLAIN = { url: null, until: 0 };   // (httpReadAt: a file that answered without a redirect)

// HTTP range requests. urls[file] is the file's URL (a split head file, or the GGUF shard for layout "tensor").
// For a cold start only: 16 round trips a token is fine while the OPFS copy fills, not for steady decode.
// A server that ignores Range answers 200 with the whole file (a split head file is 1.8 GB): that is refused before
// the body is read, unless the request starts at 0 and Content-Length says the whole file is no bigger than asked.
// A 206's Content-Range, when the response exposes it (cross-origin it needs Access-Control-Expose-Headers), must
// start where we asked.
// (a call's optional 4th argument { signal } aborts its request: room/flashdisk.js ingestNgram stops the other heads' reads
// on the first failure)
//
// resolve: true follows a file's redirect once and then asks its target directly. Hugging Face answers every
// /resolve/ request with a 302 to a signed CDN URL (us.aws.cdn.hf.co/...?Expires=<s>&Signature=..., valid an hour):
// a copy of 10,000 range requests paid that extra round trip (and a hit on the Hub's resolver) every time. The target
// (Response.url, after the redirect) is kept until 5 min before its Expires (at most ttlMs); a request to it that
// fails (an expired signature: 403; any other status or network error) forgets it and goes through the
// original URL again, once. While a file's first request is out, the others wait for its answer (one redirect per
// file, not one per request in flight). direct: a Map to share what was resolved between readers (a worker's jobs).
// now: a clock (tests).
export function httpReadAt(urls, { fetch: f = globalThis.fetch, headers = {}, resolve = false, ttlMs = 50 * 60e3, now = () => Date.now(), direct = new Map() } = {}) {
  const first = new Map();    // original url -> the request that will tell where it redirects
  // (direct: original url -> { url, until }, or PLAIN: it answered without a redirect, nothing to wait for)
  const keep = (url, r) => {
    if (!resolve || (r.status !== 206 && r.status !== 200)) return;
    if (!r.redirected || !r.url || r.url === url) { if (!direct.has(url)) direct.set(url, PLAIN); return; }
    let until = now() + ttlMs;
    const exp = /[?&]Expires=(\d{9,})/.exec(r.url);
    if (exp) until = Math.min(until, +exp[1] * 1000 - 5 * 60e3);
    if (until > now()) direct.set(url, { url: r.url, until });
  };
  const get = async (url, off, len, signal) => {
    const init = { headers: { ...headers, Range: `bytes=${off}-${off + len - 1}` }, ...(signal ? { signal } : {}) };
    if (resolve && !direct.get(url) && first.has(url)) {
      await first.get(url).catch(() => {});
      if (signal?.aborted) throw signal.reason ?? Object.assign(new Error("aborted"), { name: "AbortError" });
    }
    const d = resolve ? direct.get(url) : null;
    if (d && d !== PLAIN && d.until > now()) {
      try {
        const r = await f(d.url, init);
        if (r.status === 206 || r.status === 200) return r;
        r.body?.cancel?.()?.catch?.(() => {});
      } catch (e) { if (signal?.aborted || e?.name === "AbortError") throw e; }
      if (direct.get(url) === d) direct.delete(url);   // expired or refused: through the original URL again
    }
    const p = f(url, init);
    if (resolve && !direct.has(url) && !first.has(url)) { first.set(url, p); p.then(() => first.delete(url), () => first.delete(url)); }
    const r = await p;
    keep(url, r);
    return r;
  };
  return async (file, off, len, { signal } = {}) => {
    const url = urls[file];
    if (!url) throw new Error(`n-gram: no URL for file ${file}`);
    const r = await get(url, off, len, signal);
    const hdr = (k) => r.headers?.get?.(k) ?? null;
    const refuse = (why) => { r.body?.cancel?.()?.catch?.(() => {}); throw new Error(`n-gram: ${url} ${why}`); };
    if (r.status === 206) {
      const cr = hdr("content-range"), m = cr && /^bytes (\d+)-/.exec(cr);
      if (cr && (!m || +m[1] !== off)) refuse(`answered bytes ${cr} to a range request at ${off}`);
    } else if (r.status === 200) {
      const cl = hdr("content-length");
      if (off !== 0 || cl == null || !(+cl <= len)) refuse(`ignored the range request (200, ${cl ?? "no"} Content-Length for ${len} B at ${off})`);
    } else refuse(`answered ${r.status} to a range request`);
    const b = new Uint8Array(await r.arrayBuffer());
    return b.byteLength > len ? b.subarray(0, len) : b;
  };
}

// ------------------------------------------------------------------ split files

// The name of head h's file in a split copy (OPFS directory, a room node's cache dir)
export const ngramFileName = (h) => `ngram-${String(h).padStart(2, "0")}.bin`;

// Copy the table out of the GGUF into per-head files, in chunks: read through readAt (layout "tensor" addressing:
// file cfg.shard at cfg.byteOffset), write through write(h, bytes, byteOffsetInHeadFile). chunkRows rows a read.
// onProgress(doneBytes, totalBytes). The writer is the caller's (an OPFS writable, a Node file).
// readAhead: reads in flight (the next chunks are asked for while one is written; writes stay in order). One read
// at a time pays a round trip per chunk: over Hugging Face (a 302, then the CDN's first byte) a head copied at
// ~9 MB/s with 5.6 MB chunks. A failed read stops the copy; the reads still in flight are left to settle.
export async function splitHeads(cfg, readAt, write, { chunkRows = 1 << 16, heads = null, onProgress = null, readAhead = 1 } = {}) {
  const RB = cfg.rowBytes, list = heads || [...Array(cfg.nHeads).keys()];
  const total = list.reduce((a, h) => a + cfg.vocab[h] * RB, 0);
  const chunks = [];
  for (const h of list) for (let r = 0; r < cfg.vocab[h]; r += chunkRows) chunks.push([h, r, Math.min(chunkRows, cfg.vocab[h] - r)]);
  const ahead = Math.max(1, readAhead | 0), reads = [];
  let next = 0, done = 0;
  const more = () => {
    while (reads.length < ahead && next < chunks.length) {
      const [h, r, n] = chunks[next++];
      const p = readAt(cfg.shard, cfg.byteOffset + (cfg.offsets[h] + r) * RB, n * RB);
      p.catch?.(() => {});   // (a failure surfaces where it is awaited)
      reads.push(p);
    }
  };
  for (const [h, r, n] of chunks) {
    more();
    const b = await reads.shift();
    if (b.byteLength < n * RB) throw new Error(`n-gram split: short read in head ${h} at row ${r}`);
    more();
    await write(h, b, r * RB);
    done += n * RB;
    onProgress?.(done, total);
  }
  return total;
}
