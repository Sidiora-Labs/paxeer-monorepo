// Vendored from https://github.com/Nehanth/pooled/blob/45b5ee275a8754a37fce4a6ce174f31e4a216ef4/engine/expert_opfs.js (MIT; see LICENSE-ENGINE).
// Expert offload's disk tier in a browser tab (roadmap/32-ssd-expert-offload.md step 8): a MoE's routed experts live
// in the tab's origin-private file system (OPFS) and a miss is read from there into the GPU's expert cache
// (engine/expert_store.js disk layers). The tab then needs memory only for the non-expert weights and the cache, so a
// 36 GB Mac can run a model whose experts are bigger than its memory.
//
// Three parts, all here:
//   layout    expertLayout(G, layers): one file per disk layer, L012.bin, holding that layer's experts already
//             converted to the engine's layout (engine/expert_disk.js readPart: the bytes a parked layer would hold).
//             Expert e is one record at e * recBytes: gate qs | gate scales | up qs | up scales | down qs | down
//             scales (ExpertStore.SLICES order), padded to 4 KiB. So a miss is one read of one contiguous range, not
//             three reads from three stacked GGUF tensors (#338: one expert-sized read beats three part-sized ones),
//             and nothing is converted on the hot path.
//   ingest    ingestExperts({ layout, readAt, store }): copy the experts out of the GGUF (readAt: HTTP ranges, a
//             user-picked File, a room's weight source) into those files, converting on the way. Resumable: a
//             manifest records how many experts of each layer are on disk, saved at least every checkpointBytes
//             after the data is flushed; a restart truncates a layer back to its last checkpoint and goes on from
//             there. Files grow by writes (no big truncate up front: a fresh origin's quota grows with what is
//             written, LocalMind's ingest note), and no precheck against estimate().quota either (ingestStorage:
//             persist() and a warning, never a refusal; out of room, a write fails and the error says how far it
//             got). opfsExpertStore() is the OPFS writer: a worker with a read-write sync access handle per file,
//             and the manifest through createWritable (atomic on close).
//   reader    OpfsExpertReader: ExpertStore's reader interface (readPart(info, e) -> { qs, sc }) over the files.
//             Dedicated workers (sync access handles exist only there) each open every layer file with
//             createSyncAccessHandle({ mode: "read-only" }), which several workers may hold on one file at once
//             (tests/bench/opfs_read.js: cold reads scale to ~5.5 GB/s with 8 workers on the GB10's NVMe). A Chrome
//             without read-only handles gets each file opened by one worker (layer % workers) instead.
//             ExpertStore reads all of a cut's misses at once (_readMany: Promise.all), so they are batched in a
//             microtask and spread over the least busy workers: queue depth = the worker count, from the moment the
//             router's selection is known. The three readPart calls of one expert share one record read, and a cut
//             with fewer misses than workers stripes each record over the idle ones.
//
// Browser-safe and Deno-checkable: OPFS and Worker are touched only inside functions that a tab calls. The same file
// is the worker script (new Worker(import.meta.url, { type: "module" })).
import { expertDisk, expertGeometry } from "./expert_disk.js";

export const EXPERT_OPFS_VERSION = 1;
export const EXPERT_OPFS_DIR = "pooled-experts";
const PARTS = ["gate", "up", "down"];
const ALIGN = 4096;
const NAME_RE = /^blk\.(\d+)\.ffn_(gate|up|down)_exps\.weight$/;
const G_ = /** @type {any} */ (globalThis);
const IS_WORKER = typeof G_.WorkerGlobalScope !== "undefined" && G_.self instanceof G_.WorkerGlobalScope;

export const expertFileName = (layer) => `L${String(layer).padStart(3, "0")}.bin`;
const fnv = (s) => { let x = 0x811c9dc5; for (let i = 0; i < s.length; i++) x = Math.imul(x ^ s.charCodeAt(i), 0x01000193) >>> 0; return x.toString(16).padStart(8, "0"); };

// ------------------------------------------------------------------ layout

// The model's identity for the OPFS copy: general.name plus a hash of where and what every stacked expert tensor is
// (another quant or another layout of the file gets another directory) and, when given, contentSig
// (expertContentSig: a few bytes of each expert tensor, so a re-upload with the same layout but other weights gets
// another directory too, never a stale copy).
const expertNames = (G) => Object.keys(G.tensors).filter((n) => NAME_RE.test(n)).sort();
export function expertModelKey(G, contentSig = "") {
  const sig = expertNames(G).map((n) => { const t = G.tensors[n]; return `${n}:${t.ggmlType}:${t.shape.join("x")}:${t.shard || 0}:${t.byteOffset}`; }).join(";");
  if (!sig) throw new Error("expert OPFS: the model has no stacked expert tensors");
  const name = String(G.meta?.["general.name"] || G.meta?.["general.architecture"] || "model").replace(/[^A-Za-z0-9._-]+/g, "-").slice(0, 48);
  return `${name}-${fnv(sig)}` + (contentSig ? `-${contentSig}` : "");
}

// A cheap content signature of the experts: FNV-1a over the first and last `bytes` of every stacked expert tensor (all
// layers, read in parallel: 6 small reads per layer). Callers that make or open an OPFS copy pass it to expertLayout,
// so fixed weights at the same offsets do not reuse the old copy. readAt(shard, byteOffset, byteLength) as for ingest.
export async function expertContentSig(G, readAt, { bytes = 4096 } = {}) {
  const names = expertNames(G);
  if (!names.length) throw new Error("expert OPFS: the model has no stacked expert tensors");
  const reads = await Promise.all(names.flatMap((n) => {
    const t = G.tensors[n], k = Math.min(bytes, t.byteLength);
    return [readAt(t.shard || 0, t.byteOffset, k), readAt(t.shard || 0, t.byteOffset + t.byteLength - k, k)];
  }));
  let x = 0x811c9dc5;
  for (const b of reads) for (let i = 0; i < b.length; i++) x = Math.imul(x ^ b[i], 0x01000193) >>> 0;
  return x.toString(16).padStart(8, "0");
}

// G: a parsed (or merged split) GGUF header; layers: GGUF block indices (the trunk's layer indices);
// -> { version, key, layers: { [l]: { file, nExp, recBytes, slices[6], offs[6], parts: { gate|up|down: info } } }, totalBytes }
// contentSig: expertContentSig(G, readAt) (a tab should always pass it; tests on synthetic headers may not)
export function expertLayout(G, layers, { contentSig = "" } = {}) {
  const out = { version: EXPERT_OPFS_VERSION, key: expertModelKey(G, contentSig), layers: {}, totalBytes: 0 };
  for (const l of layers) {
    const parts = {}, slices = [];
    let nExp = 0;
    for (const p of PARTS) {
      const name = `blk.${l}.ffn_${p}_exps.weight`, t = G.tensors[name];
      if (!t) throw new Error(`expert OPFS: no ${name} in the model`);
      const info = { name, shape: [...t.shape], ggmlType: t.ggmlType, byteOffset: t.byteOffset, byteLength: t.byteLength, shard: t.shard || 0 };
      const g = expertGeometry(info);
      if (nExp && g.nExp !== nExp) throw new Error(`expert OPFS: layer ${l} has ${nExp} gate experts and ${g.nExp} ${p} experts`);
      nExp = g.nExp;
      // engine/expert_store.js attach(): qs bytes, then f16 scales, per expert
      slices.push(g.rows * g.cols / (g.kind === "q4" ? 2 : 1), g.rows * g.cols / 32 * 2);
      parts[p] = info;
    }
    const offs = [];
    let o = 0;
    for (const n of slices) { offs.push(o); o += n; }
    const recBytes = Math.ceil(o / ALIGN) * ALIGN;
    out.layers[l] = { file: expertFileName(l), nExp, recBytes, slices, offs, parts };
    out.totalBytes += nExp * recBytes;
  }
  return out;
}

// the layer and part of a stacked expert tensor's entry (ExpertStore passes the GGUF entry as info)
export function expertPartOf(info) {
  const m = NAME_RE.exec(info?.name || "");
  if (!m) throw new Error(`expert OPFS: ${info?.name} is not a stacked expert tensor`);
  return { layer: +m[1], part: m[2] };
}

// ------------------------------------------------------------------ ingest

// Copy the experts of layout's layers out of the GGUF into per-layer record files.
//   readAt(shard, byteOffset, byteLength) -> Promise<Uint8Array>   the GGUF
//   store: { manifest() -> Promise<object|null>, saveManifest(m), open(layer, keepBytes), write(layer, at, bytes),
//            flush(layer), close(layer) }   (opfsExpertStore below; tests use one in memory)
//   chunkExperts: experts per read (three range reads of chunkExperts experts each); readAhead: chunks being read while
//   one converts; inflight: chunks written but not yet acknowledged; checkpointBytes: the most that a crash can lose
//   (the manifest is saved after each)
//   onProgress({ doneBytes, totalBytes, layer, skippedBytes })
//   estimate: () -> Promise<{ quota, usage }> (navigator.storage.estimate), read only when a write runs out of room,
//   for the error's message
//   signal: an AbortSignal (the load stopped): no new chunk is read once it fires, the reads in flight are aborted
//   (readAt's 4th argument { signal }), and the ingest throws its reason. The manifest is only ever saved after a
//   flush, so it still says what the last checkpoint had, and the next run resumes there
// -> { totalBytes, wroteBytes, skippedBytes, layers }
// Out of room (a QuotaExceededError from a write, a flush or the manifest) it throws ingestQuotaError's error: how much
// this run wrote, how much is on disk up to the last checkpoint, how much the copy needs and the estimate. The manifest
// still says only what was flushed before the last checkpoint, so ingested() is false and the next run resumes there.
export async function ingestExperts({ layout, readAt, store, chunkExperts = 16, readAhead = 4, inflight = 4, checkpointBytes = 256 * 2 ** 20, onProgress = null, estimate = null, signal = null }) {
  const stopped = () => { if (signal?.aborted) throw signal.reason ?? Object.assign(new Error("the copy was stopped"), { name: "AbortError" }); };
  stopped();
  let m = await store.manifest();
  if (!m || m.version !== layout.version || m.key !== layout.key) m = { version: layout.version, key: layout.key, layers: {} };
  const ids = Object.keys(layout.layers).map(Number);
  const totalBytes = layout.totalBytes;
  let doneBytes = 0, wroteBytes = 0;
  for (const l of ids) {
    const L = layout.layers[l], e = m.layers[l];
    if (e && e.nExp === L.nExp && e.recBytes === L.recBytes) doneBytes += Math.min(e.done, L.nExp) * L.recBytes;
  }
  const skippedBytes = doneBytes;
  onProgress?.({ doneBytes, totalBytes, layer: -1, skippedBytes });
  let pend = [], savedBytes = skippedBytes;   // savedBytes: what the saved manifest says is on disk
  try {
    for (const l of ids) {
      const L = layout.layers[l];
      let ent = m.layers[l];
      if (!ent || ent.nExp !== L.nExp || ent.recBytes !== L.recBytes) ent = m.layers[l] = { nExp: L.nExp, recBytes: L.recBytes, done: 0 };
      if (ent.done >= L.nExp) continue;
      await store.open(l, ent.done * L.recBytes);
      const geo = PARTS.map((p) => expertGeometry(L.parts[p]));
      const readChunk = async (e0, n) => Promise.all(PARTS.map(async (p, i) => {
        const info = L.parts[p], len = n * geo[i].expBytes, off = info.byteOffset + e0 * geo[i].expBytes;
        const b = await (signal ? readAt(info.shard || 0, off, len, { signal }) : readAt(info.shard || 0, off, len));
        if (b.byteLength < len) throw new Error(`expert OPFS ingest: short read of ${info.name} experts ${e0}..${e0 + n - 1} (${b.byteLength} of ${len} B)`);
        return { off, b };
      }));
      let since = 0, e0 = ent.done, eRead = e0;
      const reads = [];   // the next readAhead chunks, being read while this one converts
      const more = () => {
        while (reads.length < Math.max(1, readAhead) && eRead < L.nExp && !signal?.aborted) {
          const p = readChunk(eRead, Math.min(chunkExperts, L.nExp - eRead));
          p.catch(() => {});   // (a failure surfaces where it is awaited)
          reads.push(p); eRead += chunkExperts;
        }
      };
      while (e0 < L.nExp) {
        stopped();
        more();
        const n = Math.min(chunkExperts, L.nExp - e0), raw = await reads.shift();
        stopped();
        const e1 = e0 + n;
        more();
        const rec = new Uint8Array(n * L.recBytes);
        for (let i = 0; i < PARTS.length; i++) {
          const { off, b } = raw[i];
          const conv = expertDisk({ readAt: async (_s, o, len) => b.subarray(o - off, o - off + len) });
          for (let k = 0; k < n; k++) {
            const { qs, sc } = await conv.readPart(L.parts[PARTS[i]], e0 + k);
            if (qs.byteLength !== L.slices[2 * i] || sc.byteLength !== L.slices[2 * i + 1]) throw new Error(`expert OPFS ingest: layer ${l} ${PARTS[i]} expert ${e0 + k} converted to ${qs.byteLength} / ${sc.byteLength} B`);
            rec.set(qs, k * L.recBytes + L.offs[2 * i]); rec.set(sc, k * L.recBytes + L.offs[2 * i + 1]);
          }
        }
        const nb = rec.byteLength;   // (the OPFS store transfers rec's buffer to its worker: read the size first)
        pend.push(store.write(l, e0 * L.recBytes, rec));
        while (pend.length > inflight) await pend.shift();
        since += nb; wroteBytes += nb; doneBytes += nb;
        e0 = e1;
        if (since >= checkpointBytes || e0 === L.nExp) {   // data on disk first, then the manifest that says so
          await Promise.all(pend); pend = [];
          await store.flush(l);
          const was = ent.done;
          ent.done = e0;
          try { await store.saveManifest(m); } catch (e) { ent.done = was; throw e; }
          savedBytes += (e0 - was) * L.recBytes;
          since = 0;
        }
        onProgress?.({ doneBytes, totalBytes, layer: l, skippedBytes });
      }
      await store.close(l);
    }
  } catch (e) {
    await Promise.allSettled(pend);   // (writes still in flight fail too: not unhandled)
    if (!isQuotaError(e)) throw e;
    let est = null;
    try { est = await estimate?.(); } catch { /* unknown */ }
    throw ingestQuotaError(e, { wroteBytes, savedBytes, totalBytes, estimate: est });
  }
  return { totalBytes, wroteBytes, skippedBytes, layers: ids };
}

// ------------------------------------------------------------------ storage room

// Do not precheck an ingest against navigator.storage.estimate().quota. Chrome reports a fresh origin's quota as
// about usage + 10.7 GB whatever the disk has free (Chrome 155 on macOS: 10.0-10.7 GiB with 159 GB free), and the
// quota grows as the origin writes: 48 GiB written past the reported quota went through (docs/bench-log.md,
// 2026-10-11). A precheck refuses the 35B (17.5 GiB) and Flash (~72-105 GB) copies that would fit. Write, and fail
// on a real write error (ingestExperts: QuotaExceededError -> ingestQuotaError). The same holds for any other OPFS
// copy (a room loader's): use ingestStorage for the warning and the record, never to refuse.
export const isQuotaError = (e) => e?.name === "QuotaExceededError" || /quota/i.test(e?.message || "");
const gib = (b) => (b >= 2 ** 30 ? (b / 2 ** 30).toFixed(1) + " GiB" : (b / 2 ** 20).toFixed(1) + " MiB");

// Before an ingest, best effort: ask for persistent storage (Chrome grants or denies it without a prompt; denied, or
// headless, the copy can be evicted under storage pressure) and read the estimate. Never throws.
//   storage: navigator.storage (tests pass a stand-in) -> { persisted, quota, usage, warning } (warning: a line to log
//   when the reported quota is below what the copy needs; the ingest goes on anyway)
export async function ingestStorage(totalBytes, { storage = G_.navigator?.storage } = {}) {
  let persisted = false, quota = null, usage = null;
  try { persisted = (await storage?.persist?.()) === true; } catch { /* best effort */ }
  try { const e = await storage?.estimate?.(); quota = e?.quota ?? null; usage = e?.usage ?? null; } catch { /* unknown */ }
  const warning = quota != null && quota - (usage || 0) < totalBytes
    ? `OPFS: estimate() reports ${gib(quota)} quota, ${gib(usage || 0)} used, the copy needs ${gib(totalBytes)}; going on (Chrome's quota grows as an origin writes; a real shortage fails the write)`
    : null;
  return { persisted, quota, usage, warning };
}

// The error for an ingest that ran out of room: what was written and what was needed, the estimate, the cause.
//   wroteBytes: this run; savedBytes: on disk per the saved manifest (earlier runs + this run's checkpoints);
//   totalBytes: the copy; estimate: { quota, usage } or null
export function ingestQuotaError(cause, { wroteBytes, savedBytes, totalBytes, estimate = null }) {
  const est = estimate?.quota != null ? `estimate() quota ${gib(estimate.quota)}, usage ${gib(estimate.usage || 0)}` : "estimate() unknown";
  const e = new Error(`expert OPFS ingest: out of storage after writing ${gib(wroteBytes)} this run; ${gib(savedBytes)} of the ${gib(totalBytes)} copy is on disk to the last checkpoint, ${gib(totalBytes - savedBytes)} still needed (${est}). Free disk space and load again: the ingest resumes from the checkpoint. (${cause?.message || cause})`, { cause });
  e.name = "QuotaExceededError";
  Object.assign(e, { wroteBytes, savedBytes, totalBytes, neededBytes: totalBytes - savedBytes, estimate });
  return e;
}

// whether the manifest has every expert of these layers (a reader can open them)
export function ingested(manifest, layout) {
  if (!manifest || manifest.version !== layout.version || manifest.key !== layout.key) return false;
  return Object.entries(layout.layers).every(([l, L]) => { const e = manifest.layers?.[l]; return e && e.nExp === L.nExp && e.recBytes === L.recBytes && e.done >= L.nExp; });
}

// ------------------------------------------------------------------ worker side
if (IS_WORKER) {
  const rh = new Map(), wh = new Map();   // file name -> read handle / read-write handle
  let queue = Promise.resolve();
  const dirOf = async (path, create) => { let d = await G_.navigator.storage.getDirectory(); for (const p of path) d = await d.getDirectoryHandle(p, { create }); return d; };
  G_.self.onmessage = (ev) => { queue = queue.then(() => handle(ev.data)); };
  const reply = (m, t) => G_.self.postMessage(m, t || []);
  const handle = async (m) => {
    try {
      if (m.op === "ropen") {   // every file, read-only (shared) or read-write (exclusive: this worker owns them)
        const d = await dirOf(m.dir, false), sizes = [];
        for (const name of m.names) {
          const fh = await d.getFileHandle(name);
          const h = m.mode === "read-only" ? await fh.createSyncAccessHandle({ mode: "read-only" }) : await fh.createSyncAccessHandle();
          rh.set(name, h); sizes.push(h.getSize());
        }
        reply({ id: m.id, ok: true, sizes });
      } else if (m.op === "read") {   // [[name, at, len], ...] -> one ArrayBuffer each, transferred
        const bufs = [];
        for (const [name, at, len] of m.reqs) {
          const h = rh.get(name);
          if (!h) throw new Error(`expert OPFS: ${name} is not open in this worker`);
          const b = new Uint8Array(len), got = h.read(b, { at });
          if (got !== len) throw new Error(`expert OPFS: short read of ${name} at ${at} (${got} of ${len} B)`);
          bufs.push(b.buffer);
        }
        reply({ id: m.id, ok: true, bufs }, bufs);
      } else if (m.op === "wopen") {
        const d = await dirOf(m.dir, true);
        const h = await (await d.getFileHandle(m.name, { create: true })).createSyncAccessHandle();
        if (h.getSize() > m.keep) h.truncate(m.keep);   // back to the last checkpoint (a shrink)
        wh.set(m.name, h);
        reply({ id: m.id, ok: true, size: h.getSize() });
      } else if (m.op === "write") {
        const h = wh.get(m.name), b = new Uint8Array(m.buf);
        const n = h.write(b, { at: m.at });
        if (n !== b.byteLength) throw new Error(`expert OPFS: short write to ${m.name} at ${m.at} (${n} of ${b.byteLength} B)`);
        reply({ id: m.id, ok: true });
      } else if (m.op === "flush") {
        wh.get(m.name).flush();
        reply({ id: m.id, ok: true });
      } else if (m.op === "wclose") {
        const h = wh.get(m.name);
        if (h) { h.flush(); h.close(); wh.delete(m.name); }
        reply({ id: m.id, ok: true });
      } else if (m.op === "close") {
        for (const h of [...rh.values(), ...wh.values()]) { try { h.close(); } catch {} }
        rh.clear(); wh.clear();
        reply({ id: m.id, ok: true });
      }
    } catch (e) {
      reply({ id: m.id, ok: false, error: String(e?.message || e), name: e?.name });
    }
  };
}

// ------------------------------------------------------------------ main-thread side

class WorkerRPC {
  constructor(url, n) {
    this.seq = 0; this.wait = new Map();
    this.ws = Array.from({ length: n }, () => new G_.Worker(url, { type: "module" }));
    this.busy = new Array(n).fill(0);
    for (const w of this.ws) w.onmessage = (ev) => {
      const p = this.wait.get(ev.data.id);
      if (!p) return;
      this.wait.delete(ev.data.id);
      if (ev.data.ok) p.resolve(ev.data);
      else p.reject(Object.assign(new Error(ev.data.error), { name: ev.data.name || "Error" }));
    };
  }
  // weight: what the message adds to the worker's queue (busy: outstanding reads per worker)
  call(w, msg, transfer = [], weight = 1) {
    const id = ++this.seq;
    this.busy[w] += weight;
    return new Promise((resolve, reject) => { this.wait.set(id, { resolve, reject }); this.ws[w].postMessage({ ...msg, id }, transfer); })
      .finally(() => { this.busy[w] -= weight; });
  }
  async close() {
    await Promise.all(this.ws.map((_, w) => this.call(w, { op: "close" }).catch(() => {})));
    for (const w of this.ws) w.terminate();
    this.ws = [];
  }
}

async function opfsDir(path, create) {
  let d = await G_.navigator.storage.getDirectory();
  for (const p of path) d = await d.getDirectoryHandle(p, { create });
  return d;
}
export async function readManifest(dir) {
  try {
    const f = await (await (await opfsDir(dir, false)).getFileHandle("manifest.json")).getFile();
    return JSON.parse(await f.text());
  } catch { return null; }
}

// Save a copy's manifest (dir: its OPFS path): through a writable, atomic on close
export async function saveExpertManifest(dir, m) {
  const w = await (await (await opfsDir(dir, true)).getFileHandle("manifest.json", { create: true })).createWritable();
  await w.write(JSON.stringify(m)); await w.close();
}

// The OPFS writer for ingestExperts: one worker, a read-write sync access handle per layer file while it is written.
// dir: the path from the OPFS root (default [EXPERT_OPFS_DIR, layout.key]). Asks for persistent storage (Chrome grants
// or denies it without a prompt; denied, the copy can be evicted under storage pressure) -> the store, with persisted.
export async function opfsExpertStore(layout, { dir = [EXPERT_OPFS_DIR, layout.key], url = import.meta.url } = {}) {
  const persisted = await G_.navigator.storage?.persist?.().catch(() => false) ?? false;
  await opfsDir(dir, true);
  const rpc = new WorkerRPC(url, 1), name = (l) => layout.layers[l].file;
  return {
    dir, persisted,
    manifest: () => readManifest(dir),
    saveManifest: (m) => saveExpertManifest(dir, m),
    open: (l, keep) => rpc.call(0, { op: "wopen", dir, name: name(l), keep }),
    write: (l, at, bytes) => { const buf = bytes.byteOffset === 0 && bytes.byteLength === bytes.buffer.byteLength ? bytes.buffer : bytes.slice().buffer; return rpc.call(0, { op: "write", name: name(l), at, buf }, [buf]); },
    flush: (l) => rpc.call(0, { op: "flush", name: name(l) }),
    close: (l) => rpc.call(0, { op: "wclose", name: name(l) }),
    shutdown: () => rpc.close(),
  };
}

// Remove a model's OPFS copy (a settings page's "free the space"; tests)
export async function removeExperts(key, { root = EXPERT_OPFS_DIR } = {}) {
  try { await (await opfsDir([root], false)).removeEntry(key, { recursive: true }); return true; } catch { return false; }
}

// ExpertStore's disk reader over the OPFS copy (engine/expert_store.js option disk: { layers, reader }).
export class OpfsExpertReader {
  constructor(layout, rpc, files, shared) {
    this.layout = layout; this.rpc = rpc; this.files = files; this.shared = shared;
    this.inflight = new Map();   // layer * 2^20 + e -> Promise<Uint8Array> (one record read for the three parts)
    this.pending = [];
    this.stats = { reads: 0, bytes: 0, batches: 0, maxBatch: 0 };
    this.stripeBytes = 256 * 1024;
  }
  // layout: expertLayout() of the disk layers; workers: readers (4-8: #338's cold reads scale to 8); mode: "auto" (read-only
  // shared handles, else one owner per file), "shared" or "owned"; stripeBytes: the smallest stripe a record is split
  // into when a cut has fewer misses than workers (0: never split)
  static async open(layout, { dir = [EXPERT_OPFS_DIR, layout.key], workers = 4, mode = "auto", stripeBytes = 256 * 1024, url = import.meta.url } = {}) {
    const m = await readManifest(dir);
    if (!ingested(m, layout)) throw new Error(`expert OPFS: ${dir.join("/")} does not hold every expert of layers ${Object.keys(layout.layers).join(",")} (ingestExperts first)`);
    const ids = Object.keys(layout.layers).map(Number), names = ids.map((l) => layout.layers[l].file);
    const W = Math.max(1, workers);
    const tryOpen = async (shared) => {
      const rpc = new WorkerRPC(url, W);
      try {
        const res = await Promise.all(rpc.ws.map((_, w) => {
          const mine = shared ? names : ids.filter((_, i) => i % W === w).map((l) => layout.layers[l].file);
          return rpc.call(w, { op: "ropen", dir, names: mine, mode: shared ? "read-only" : "readwrite" }).then((r) => [mine, r.sizes]);
        }));
        for (const [mine, sizes] of res) mine.forEach((n, i) => {
          const l = ids[names.indexOf(n)], want = layout.layers[l].nExp * layout.layers[l].recBytes;
          if (sizes[i] < want) throw new Error(`expert OPFS: ${n} is ${sizes[i]} B, layer ${l} needs ${want}`);
        });
        return rpc;
      } catch (e) { await rpc.close(); throw e; }
    };
    let rpc = null, shared = mode !== "owned";
    if (shared) {
      try { rpc = await tryOpen(true); } catch (e) {
        // a Chrome without read-only sync handles locks the file at the second open
        if (mode === "shared" || !/NoModificationAllowed|InvalidState|TypeError/.test(e.name)) throw e;
        shared = false;
      }
    }
    if (!rpc) rpc = await tryOpen(false);
    const r = new OpfsExpertReader(layout, rpc, names, shared);
    r.stripeBytes = stripeBytes;
    return r;
  }
  get workers() { return this.rpc.ws.length; }
  // what one expert read costs on disk: its whole record (ExpertStore's diskBytes counts this, not the GGUF's bytes)
  expertBytes(layer) { return this.layout.layers[layer]?.recBytes ?? 0; }
  _record(layer, e) {
    const k = layer * 2 ** 20 + e;
    let p = this.inflight.get(k);
    if (p) return p;
    const L = this.layout.layers[layer];
    if (!L) throw new Error(`expert OPFS: layer ${layer} is not in the copy`);
    if (!(e >= 0 && e < L.nExp)) throw new Error(`expert OPFS: expert ${e} of ${L.nExp}`);
    p = new Promise((resolve, reject) => {
      if (!this.pending.length) queueMicrotask(() => this._flush());
      this.pending.push({ layer, e, L, resolve, reject });
    });
    this.inflight.set(k, p);
    p.then(() => this.inflight.delete(k), () => this.inflight.delete(k));
    return p;
  }
  // a microtask's worth of requests (one cut's misses): spread over the workers, one message per worker
  _flush() {
    const reqs = this.pending; this.pending = [];
    if (!reqs.length) return;
    const W = this.workers, per = Array.from({ length: W }, () => []), load = this.rpc.busy.slice();
    // jobs: [request, byte offset in its record, length]. A cut with fewer misses than workers (most decode cuts: one
    // or two) splits each record into 4 KiB-aligned stripes, one per idle worker, so a lone miss is read at queue
    // depth W instead of 1 (stripes of at least stripeBytes)
    const k = this.shared && this.stripeBytes > 0 ? Math.max(1, Math.floor(W / reqs.length)) : 1;
    const jobs = [];
    for (const r of reqs) {
      const n = Math.min(k, Math.max(1, Math.floor(r.L.recBytes / this.stripeBytes))), step = Math.ceil(r.L.recBytes / n / 4096) * 4096;
      r.left = 0;
      for (let o = 0; o < r.L.recBytes; o += step) { jobs.push([r, o, Math.min(step, r.L.recBytes - o)]); r.left++; }
      if (r.left > 1) r.buf = new Uint8Array(r.L.recBytes);
    }
    for (const j of jobs) {
      let w;
      if (this.shared) { w = 0; for (let i = 1; i < W; i++) if (load[i] < load[w]) w = i; load[w]++; }
      else w = this.files.indexOf(j[0].L.file) % W;
      per[w].push(j);
    }
    this.stats.batches++; this.stats.maxBatch = Math.max(this.stats.maxBatch, reqs.length);
    per.forEach((js, w) => {
      if (!js.length) return;
      this.rpc.call(w, { op: "read", reqs: js.map(([r, o, n]) => [r.L.file, r.e * r.L.recBytes + o, n]) }, [], js.length).then((res) => {
        js.forEach(([r, o], i) => {
          const b = new Uint8Array(res.bufs[i]);
          if (r.buf) r.buf.set(b, o);
          if (--r.left) return;
          this.stats.reads++; this.stats.bytes += r.L.recBytes;
          r.resolve(r.buf || b);
        });
      }, (err) => js.forEach(([r]) => r.reject(err)));
    });
  }
  // one expert's whole record (its six slices at layout.layers[layer].offs; room/flashdisk.js reads a resident layer's
  // experts this way, every part from one read) -> Promise<Uint8Array>
  record(layer, e) { return this._record(layer, e); }
  // ExpertStore's reader interface: expert e of a stacked tensor -> { qs, sc } in the engine's layout
  async readPart(info, e) {
    const { layer, part } = expertPartOf(info), L = this.layout.layers[layer];
    const rec = await this._record(layer, e), j = PARTS.indexOf(part) * 2;
    return { qs: rec.subarray(L.offs[j], L.offs[j] + L.slices[j]), sc: rec.subarray(L.offs[j + 1], L.offs[j + 1] + L.slices[j + 1]) };
  }
  close() { return this.rpc.close(); }
}

// The expert cache's GPU budget on a unified-memory device in disk mode (a Mac, the GB10): the pools and the prefill
// region come out of the same memory the OS page cache serves the OPFS reads from, and on Apple silicon Metal buffers
// are wired (flash-moe: a 21 GB Metal LRU ran at 1.99 tok/s, no cache at 5.74, because the reads lost the page
// cache). So this is an upper bound, not a default: at most `frac` of memory, and the Metal working set (~75% of
// memory) less the resident weights and `headroomBytes`. Nothing calls it by default; the Mac runbook
// (tests/bench/tab_disk.mjs) sweeps the cache up from 2 GiB and treats this bound as one point of the sweep.
//   memoryBytes: the device's memory in bytes (a "36 GB" Mac has 36 GiB; a tab cannot read it: navigator.deviceMemory
//   caps at 8, so the page asks). -> bytes (36 GiB, 2 GiB resident: 19.8 GiB)
export function diskCacheBytes({ memoryBytes, residentBytes = 0, frac = 0.55, headroomBytes = 2 * 2 ** 30 }) {
  if (!(memoryBytes > 0)) throw new Error("diskCacheBytes: memoryBytes is required");
  return Math.max(0, Math.floor(Math.min(frac * memoryBytes, 0.75 * memoryBytes - residentBytes - headroomBytes)));
}
