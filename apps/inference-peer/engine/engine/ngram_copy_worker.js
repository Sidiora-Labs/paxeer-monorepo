// Vendored from https://github.com/Nehanth/pooled/blob/45b5ee275a8754a37fce4a6ce174f31e4a216ef4/engine/ngram_copy_worker.js (MIT; see LICENSE-ENGINE).
// The n-gram table's copy into OPFS (room/flashdisk.js ingestNgram), off the main thread: a dedicated worker fetches
// the table's rows by HTTP range (low priority, never the HTTP cache) and writes each head's file through a
// FileSystemSyncAccessHandle. On the main thread the same copy (createWritable) took ~0.9 of a core and halved the
// GPU step's speed when the two ran together; here it runs next to the GPU step and the experts' copy.
//
// The same file is the worker script and, imported on the main thread, its client (copyHeadsInWorker). The worker's
// job is copyHeads, which the tests run without a worker.
//
// A head's file is written in order, from byte 0 (truncated first): it reaches its full size only with its last
// chunk, so "the file is head h's size" (ingestNgram's test) still means "head h is all there". A copy that stops
// part way leaves the file short, and the next load copies that head again.
//
// Messages, main -> worker: { op: "copy", cfg, urls, dir, heads, parallel, chunkRows?, readAhead?, resolve? } (cfg:
// { shard, byteOffset, offsets, vocab, rowBytes }); { op: "parallel", n } (heads at a time from now on); { op: "abort" }.
// worker -> main: { op: "progress", done } (bytes, at most ~4 a second), { op: "head", h }, then one of
// { op: "done" } / { op: "error", name, message, wrote } (wrote: whether any byte reached a file), sent only once every
// file it opened is closed.
import { splitHeads, httpReadAt, ngramFileName } from "./ngram.js";

const G = /** @type {any} */ (globalThis);
const IS_WORKER = typeof G.WorkerGlobalScope !== "undefined" && G.self instanceof G.WorkerGlobalScope;
// A head is read in chunks of CHUNK_ROWS rows (11.25 MiB of Flash's 90-byte rows), READ_AHEAD of them in flight: one
// 5.6 MiB read at a time paid a round trip per chunk and copied a head at ~9 MB/s over Hugging Face
export const CHUNK_ROWS = 1 << 17, READ_AHEAD = 4;
const abortError = () => Object.assign(new Error("the copy was stopped"), { name: "AbortError" });

// The copy itself. job: { cfg, urls, dir, heads, parallel }. env: { getDirectory() (the OPFS root), fetch, post(msg),
// now() }. -> a control { parallel(n), abort(), done: Promise<void> } (done rejects with the first error, its
// .wrote set). Each head is fetched and written in order; `parallel` heads at a time (read before each head starts)
export function copyHeads(job, env) {
  const { cfg, urls, dir, heads } = job;
  let par = Math.max(1, job.parallel | 0 || 1), failed = null, wrote = false, sent = 0, done = 0;
  const now = env.now || (() => Date.now());
  const ac = new AbortController(), open = new Set(), cur = new Map();
  const fetchLow = (u, init) => env.fetch(u, { ...init, cache: "no-store", priority: "low" });
  // (resolve: Hugging Face's 302 to its CDN followed once per file, not once per chunk: engine/ngram.js httpReadAt)
  const readAt = httpReadAt(urls, { fetch: fetchLow, resolve: job.resolve !== false });
  const chunkRows = job.chunkRows || CHUNK_ROWS, readAhead = job.readAhead || READ_AHEAD;
  const read = (f, o, n) => { if (failed) throw failed; return readAt(f, o, n, { signal: ac.signal }); };
  const progress = (force = false) => {
    const t = now();
    if (force || t - sent >= 250) { sent = t; env.post({ op: "progress", done }); }
  };
  const one = async (d, h) => {
    const fh = await d.getFileHandle(ngramFileName(h), { create: true });
    const sh = await fh.createSyncAccessHandle();
    open.add(sh);
    try {
      await sh.truncate(0);
      await splitHeads(cfg, read, async (_h, bytes, at) => {
        if (failed) throw failed;
        await sh.write(bytes, { at });   // (await: older Safari's handle returns promises)
        wrote = true;
      }, { heads: [h], chunkRows, readAhead, onProgress: (n) => { cur.set(h, n); done = [...cur.values()].reduce((a, b) => a + b, 0); progress(); } });
      if (failed) throw failed;
      await sh.flush();
    } finally {
      open.delete(sh);
      try { await sh.close(); } catch { /* closed */ }
    }
    env.post({ op: "head", h });
  };
  const run = async () => {
    let d = await env.getDirectory();
    for (const p of dir) d = await d.getDirectoryHandle(p, { create: true });
    const queue = heads.slice();
    const runner = async (k) => {
      while (queue.length && !failed) {
        if (k >= par) { await new Promise((r) => setTimeout(r, 100)); continue; }
        const h = queue.shift();
        try { await one(d, h); } catch (e) { if (!failed) { failed = e; ac.abort(); } throw e; }
      }
    };
    // allSettled: every head has stopped (and closed its file) before the copy says it is over
    const max = Math.max(par, job.maxParallel || 2);
    await Promise.allSettled(Array.from({ length: Math.min(max, queue.length) }, (_, k) => runner(k)));
    progress(true);
    if (failed) throw Object.assign(failed, { wrote });
  };
  const done$ = run().catch((e) => { throw Object.assign(e instanceof Error ? e : new Error(String(e)), { wrote }); });
  done$.catch(() => {});
  return {
    parallel(n) { par = Math.max(1, n | 0 || 1); },
    abort() { if (!failed) { failed = abortError(); ac.abort(); } },
    done: done$,
    get wrote() { return wrote; },
  };
}

// ---------------------------------------------------------------- worker side
if (IS_WORKER) {
  let job = null;
  G.self.onmessage = (ev) => {
    const m = ev.data || {};
    if (m.op === "copy" && !job) {
      job = copyHeads(m, { getDirectory: () => G.navigator.storage.getDirectory(), fetch: (u, i) => G.fetch(u, i), post: (x) => G.self.postMessage(x) });
      job.done.then(() => G.self.postMessage({ op: "done" }),
        (e) => G.self.postMessage({ op: "error", name: e?.name || "Error", message: String(e?.message || e), wrote: !!e?.wrote }));
    } else if (m.op === "parallel") job?.parallel(m.n);
    else if (m.op === "abort") job?.abort();
  };
}

// ---------------------------------------------------------------- main-thread side
// Copy `heads` of the table in a worker. cfg: the table's config (engine/ngram.js ngramConfig); urls: the GGUF's files
// (by shard); dir: the OPFS path. parallel: a number, or a function read every 250 ms (heads at a time). signal aborts
// it (the worker stops, closes its files, then it is terminated; at most ~2 s). onProgress(bytes done of these heads).
// make(url) -> a Worker (tests). chunkRows / readAhead: rows a read and reads in flight per head (CHUNK_ROWS,
// READ_AHEAD); cdnResolve: follow the file's redirect once (engine/ngram.js httpReadAt resolve). -> Promise<bytes> when every head is there. A failure rejects with the worker's error
// and .wrote: false when nothing reached a file (the caller may copy another way), true otherwise.
export function copyHeadsInWorker(cfg, { urls, dir, heads, parallel = 2, signal = null, onProgress = () => {}, make = null, url = import.meta.url, closeWaitMs = 2000,
  chunkRows = CHUNK_ROWS, readAhead = READ_AHEAD, cdnResolve = true } = {}) {
  const par = typeof parallel === "function" ? parallel : () => parallel;
  const total = heads.reduce((a, h) => a + cfg.vocab[h] * cfg.rowBytes, 0);
  return new Promise((resolve, reject) => {
    let w;
    try { w = make ? make(url) : new G.Worker(url, { type: "module" }); }
    catch (e) { reject(Object.assign(e instanceof Error ? e : new Error(String(e)), { wrote: false })); return; }
    let over = false, got = false, last = par(), tick = null, stopT = null;
    const end = (fn) => {
      if (over) return;
      over = true;
      clearInterval(tick); clearTimeout(stopT);
      signal?.removeEventListener?.("abort", onAbort);
      try { w.terminate(); } catch { /* gone */ }
      fn();
    };
    const onAbort = () => {
      try { w.postMessage({ op: "abort" }); } catch { /* gone */ }
      // the worker answers once its files are closed; one that does not is terminated (its handles go with it)
      stopT = setTimeout(() => end(() => reject(Object.assign(signal.reason ?? abortError(), { wrote: got }))), closeWaitMs);
    };
    w.onmessage = (ev) => {
      const m = ev.data || {};
      if (m.op === "progress") { if (m.done > 0) got = true; onProgress(m.done, total); }
      else if (m.op === "done") end(() => resolve(total));
      else if (m.op === "error") {
        const e = signal?.aborted ? (signal.reason ?? abortError()) : Object.assign(new Error(m.message), { name: m.name || "Error" });
        end(() => reject(Object.assign(e, { wrote: !!m.wrote || got })));
      }
    };
    w.onerror = (ev) => {
      ev?.preventDefault?.();
      end(() => reject(Object.assign(new Error(`the n-gram copy worker failed: ${ev?.message || "it could not start"}`), { wrote: got })));
    };
    if (signal?.aborted) { onAbort(); return; }
    signal?.addEventListener?.("abort", onAbort, { once: true });
    const plain = { shard: cfg.shard, byteOffset: cfg.byteOffset, offsets: [...cfg.offsets], vocab: [...cfg.vocab], rowBytes: cfg.rowBytes };
    w.postMessage({ op: "copy", cfg: plain, urls: [...urls], dir: [...dir], heads: [...heads], parallel: last, maxParallel: 2, chunkRows, readAhead, resolve: cdnResolve });
    tick = setInterval(() => { const n = par(); if (n !== last) { last = n; try { w.postMessage({ op: "parallel", n }); } catch { /* gone */ } } }, 250);
  });
}
