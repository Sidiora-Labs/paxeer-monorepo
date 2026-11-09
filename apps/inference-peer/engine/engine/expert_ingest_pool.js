// Vendored from https://github.com/Nehanth/pooled/blob/45b5ee275a8754a37fce4a6ce174f31e4a216ef4/engine/expert_ingest_pool.js (MIT; see LICENSE-ENGINE).
// The experts' copy into OPFS (engine/expert_opfs.js ingestExperts) in a pool of dedicated workers, off the main
// thread. Each worker copies whole layers, one at a time: it fetches the layer's expert ranges itself (HTTP range
// requests, Hugging Face's redirect followed once: engine/ngram.js httpReadAt resolve), converts them and writes the
// layer's file through its own sync access handle (one writer per file, as OPFS wants). The main thread only hands
// out layers, sums the progress and saves the manifest.
//
// Why: on the main thread the copy was the tab's wall. Converting Flash's down experts (Q4_1 -> Q8) and moving every
// byte through one thread held the save to ~235 MB/s alone and ~168 MB/s next to the n-gram copy on the GB10, and it
// could not run next to the GPU step (it took that step from 144 to 10 MB/s: room/flashload.js). Here it scales with
// the workers, and its requests are as many as the workers keep in flight (workers x readAhead x 3).
//
// The worker reuses ingestExperts on a one-layer layout, so the resume rules are the same: a worker flushes its file
// and then says how many experts are on disk (a checkpoint, at least every checkpointBytes); the main thread puts that
// in the manifest (saved one at a time, atomically: createWritable), so the manifest never says more than is on disk.
// A restart truncates each layer back to its last checkpoint and goes on from there.
//
// The same file is the worker script and, imported on the main thread, the pool (ingestExpertsPool).
//
// Messages, main -> worker: { op: "layer", layer, L, version, key, dir, done (experts on disk at its checkpoint),
// urls, chunkExperts, readAhead, checkpointBytes, resolve }; { op: "abort" }.
// worker -> main: { op: "progress", layer, bytes } (bytes this job wrote, at most ~5 a second); { op: "ckpt", layer,
// done }; then { op: "done", layer } or { op: "error", layer, name, message, wrote } (sent once its file is closed).
import { ingestExperts, ingestQuotaError, isQuotaError } from "./expert_opfs.js";
import { httpReadAt } from "./ngram.js";

const G = /** @type {any} */ (globalThis);
const IS_WORKER = typeof G.WorkerGlobalScope !== "undefined" && G.self instanceof G.WorkerGlobalScope;
const abortError = () => Object.assign(new Error("the copy was stopped"), { name: "AbortError" });

// One layer's copy, the worker's job (exported for the tests, which run it without a worker).
//   job: the "layer" message; env: { getDirectory() (the OPFS root), fetch, post(msg), now(), signal, direct (the
//   worker's resolved URLs, kept from one layer to the next: httpReadAt's direct) }
// -> Promise (rejects with the copy's error, .wrote set)
export async function copyLayer(job, env) {
  const { layer, L, dir, urls } = job;
  const now = env.now || (() => Date.now());
  const layout = { version: job.version, key: job.key, layers: { [layer]: L }, totalBytes: L.nExp * L.recBytes };
  const readAt = httpReadAt(urls, { fetch: (u, init) => env.fetch(u, { ...init, cache: "no-store" }), resolve: job.resolve !== false, direct: env.direct });
  let h = null, wrote = false, sent = 0, bytes = 0;
  const store = {
    manifest: async () => ({ version: job.version, key: job.key, layers: { [layer]: { nExp: L.nExp, recBytes: L.recBytes, done: job.done || 0 } } }),
    saveManifest: async (m) => { env.post({ op: "ckpt", layer, done: m.layers[layer].done }); },
    async open(_l, keep) {
      let d = await env.getDirectory();
      for (const p of dir) d = await d.getDirectoryHandle(p, { create: true });
      h = await (await d.getFileHandle(L.file, { create: true })).createSyncAccessHandle();
      if ((await h.getSize()) > keep) await h.truncate(keep);   // back to the last checkpoint (await: older Safari)
    },
    async write(_l, at, b) {
      const n = await h.write(b, { at });
      if (n !== b.byteLength) throw new Error(`expert OPFS: short write to ${L.file} at ${at} (${n} of ${b.byteLength} B)`);
      wrote = true;
    },
    flush: async () => { await h.flush(); },
    close: async () => { const x = h; h = null; if (x) { await x.flush(); await x.close(); } },
  };
  try {
    await ingestExperts({ layout, readAt, store, chunkExperts: job.chunkExperts || 8, readAhead: job.readAhead || 3, inflight: 1,
      checkpointBytes: job.checkpointBytes || 256 * 2 ** 20, signal: env.signal,
      onProgress: (p) => {
        bytes = p.doneBytes - p.skippedBytes;
        const t = now();
        if (t - sent >= 200) { sent = t; env.post({ op: "progress", layer, bytes }); }
      } });
    env.post({ op: "progress", layer, bytes });
  } catch (e) {
    throw Object.assign(e instanceof Error ? e : new Error(String(e)), { wrote });
  } finally {
    if (h) { try { await h.close(); } catch { /* closed */ } }
  }
}

// ---------------------------------------------------------------- worker side
if (IS_WORKER) {
  let ac = null;
  const direct = new Map();
  // (set after the imports ran: engine/expert_opfs.js, imported above, installs its own reader/writer handler in a
  // worker; this one replaces it)
  G.self.onmessage = (ev) => {
    const m = ev.data || {};
    if (m.op === "abort") { ac?.abort(abortError()); return; }
    if (m.op !== "layer") return;
    ac = new AbortController();
    copyLayer(m, { getDirectory: () => G.navigator.storage.getDirectory(), fetch: (u, i) => G.fetch(u, i), post: (x) => G.self.postMessage(x), signal: ac.signal, direct })
      .then(() => G.self.postMessage({ op: "done", layer: m.layer }),
        (e) => G.self.postMessage({ op: "error", layer: m.layer, name: e?.name || "Error", message: String(e?.message || e), wrote: !!e?.wrote }));
  };
}

// ---------------------------------------------------------------- main-thread side
// ingestExperts' copy in `workers` workers. layout: expertLayout(); urls: the GGUF's files (by shard: a tensor's
// shard indexes them); dir: the copy's OPFS path; store: { manifest(), saveManifest(m) } (the manifest only: the
// workers write the layer files). chunkExperts / readAhead: per worker (requests in flight: workers x readAhead x 3);
// resolve: follow Hugging Face's redirect once per file and worker. onProgress, estimate, signal as ingestExperts'.
// make(url) -> a Worker (tests). -> { totalBytes, wroteBytes, skippedBytes, layers }.
// A failure stops the other workers (their reads aborted, their files closed) and rejects with the first error, its
// .wrote false when no worker wrote a byte (no module workers or sync access handles here: the caller may copy on the
// main thread instead). Out of room it rejects with ingestQuotaError's error, as ingestExperts does.
export async function ingestExpertsPool({ layout, urls, dir, store, workers = 4, chunkExperts = 8, readAhead = 3, checkpointBytes = 256 * 2 ** 20,
  resolve = true, onProgress = null, estimate = null, signal = null, make = null, url = import.meta.url, closeWaitMs = 3000 }) {
  if (signal?.aborted) throw signal.reason ?? abortError();
  let m = await store.manifest();
  if (!m || m.version !== layout.version || m.key !== layout.key) m = { version: layout.version, key: layout.key, layers: {} };
  const ids = Object.keys(layout.layers).map(Number), totalBytes = layout.totalBytes;
  let skippedBytes = 0;
  const queue = [];
  for (const l of ids) {
    const L = layout.layers[l];
    let e = m.layers[l];
    if (!e || e.nExp !== L.nExp || e.recBytes !== L.recBytes) e = m.layers[l] = { nExp: L.nExp, recBytes: L.recBytes, done: 0 };
    e.done = Math.min(e.done, L.nExp);
    skippedBytes += e.done * L.recBytes;
    if (e.done < L.nExp) queue.push(l);
  }
  let savedBytes = skippedBytes;
  const cur = new Map();   // layer -> bytes this run wrote to it
  const doneNow = () => skippedBytes + [...cur.values()].reduce((a, b) => a + b, 0);
  onProgress?.({ doneBytes: skippedBytes, totalBytes, layer: -1, skippedBytes });
  if (!queue.length) return { totalBytes, wroteBytes: 0, skippedBytes, layers: ids };

  // the manifest, one save at a time; a failed save fails the copy
  let saving = Promise.resolve(), saveErr = null;
  const save = () => { saving = saving.then(() => saveErr ? null : store.saveManifest(m)).catch((e) => { saveErr ||= e; }); return saving; };

  const W = Math.max(1, Math.min(workers | 0 || 1, queue.length));
  const ws = [];
  let failed = null, wrote = false;
  const jobs = new Map();   // worker index -> { layer, resolve, reject }
  const fail = (e) => {
    if (failed) return;
    failed = e;
    for (const w of ws) { try { w.postMessage({ op: "abort" }); } catch { /* gone */ } }
  };
  const onAbort = () => fail(signal.reason ?? abortError());
  signal?.addEventListener?.("abort", onAbort, { once: true });
  const make_ = make || ((u) => new G.Worker(u, { type: "module" }));
  try {
    for (let i = 0; i < W; i++) {
      let w;
      try { w = make_(url); } catch (e) { throw Object.assign(e instanceof Error ? e : new Error(String(e)), { wrote: false }); }
      w.onmessage = (ev) => {
        const x = ev.data || {}, j = jobs.get(i);
        if (x.op === "progress") { cur.set(x.layer, x.bytes); if (x.bytes > 0) wrote = true; onProgress?.({ doneBytes: doneNow(), totalBytes, layer: x.layer, skippedBytes }); }
        else if (x.op === "ckpt") {
          const e = m.layers[x.layer], L = layout.layers[x.layer];
          if (x.done > e.done) { savedBytes += (x.done - e.done) * L.recBytes; e.done = x.done; save(); }
        } else if (x.op === "done") { jobs.delete(i); j?.resolve(); }
        else if (x.op === "error") {
          jobs.delete(i);
          if (x.wrote) wrote = true;
          j?.reject(Object.assign(new Error(x.message), { name: x.name || "Error", wrote: !!x.wrote }));
        }
      };
      w.onerror = (ev) => {
        ev?.preventDefault?.();
        const j = jobs.get(i); jobs.delete(i);
        const e = new Error(`the expert copy worker failed: ${ev?.message || "it could not start"}`);
        if (j) j.reject(e); else fail(e);
      };
      ws.push(w);
    }
    const runner = async (i) => {
      while (queue.length && !failed) {
        const l = queue.shift(), L = layout.layers[l];
        try {
          await new Promise((res, rej) => {
            jobs.set(i, { layer: l, resolve: res, reject: rej });
            ws[i].postMessage({ op: "layer", layer: l, L, version: layout.version, key: layout.key, dir: [...dir], done: m.layers[l].done, urls: [...urls],
              chunkExperts, readAhead, checkpointBytes, resolve });
          });
        } catch (e) { fail(e); throw e; }
        // (the worker's last checkpoint is the layer's end: its done is nExp now)
      }
    };
    // allSettled: every worker has closed its file before the copy says it is over
    // (a worker that does not answer the abort within closeWaitMs is given up on: terminate() below closes its file)
    let over = false, timer = null;
    const settled = await Promise.race([
      Promise.allSettled(ws.map((_, i) => runner(i))),
      new Promise((r) => { const t = () => { if (over) return; timer = setTimeout(failed ? r : t, failed ? closeWaitMs : 200); }; t(); }),
    ]);
    over = true; clearTimeout(timer);
    if (!settled) for (const [, j] of jobs) j.reject(failed);
    await saving;
    if (failed) throw failed;
    if (saveErr) throw saveErr;
    const wroteBytes = doneNow() - skippedBytes;
    onProgress?.({ doneBytes: doneNow(), totalBytes, layer: -1, skippedBytes });
    return { totalBytes, wroteBytes, skippedBytes, layers: ids };
  } catch (e0) {
    await saving;
    const e = failed || e0;
    if (e && typeof e === "object") e.wrote = wrote || !!e.wrote;
    if (!isQuotaError(e)) throw e;
    let est = null;
    try { est = await estimate?.(); } catch { /* unknown */ }
    throw Object.assign(ingestQuotaError(e, { wroteBytes: doneNow() - skippedBytes, savedBytes, totalBytes, estimate: est }), { wrote: true });
  } finally {
    signal?.removeEventListener?.("abort", onAbort);
    for (const w of ws) { try { w.terminate(); } catch { /* gone */ } }
  }
}
