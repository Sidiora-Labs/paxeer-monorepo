// Vendored from https://github.com/Nehanth/pooled/blob/45b5ee275a8754a37fce4a6ce174f31e4a216ef4/engine/ngram_worker.js (MIT; see LICENSE-ENGINE).
// The n-gram table's OPFS reader (engine/ngram.js), in dedicated workers: FileSystemSyncAccessHandle exists only
// in a worker, and one handle is exclusive per file, so each worker opens the split head files it owns and reads
// them synchronously. Several workers give parallel reads (roadmap/33-qwen38-flash.md §4: 16 random reads a token, ~80 µs each
// cold, overlapped with layer 0).
//
// The same file is the worker script and, imported on the main thread, its client:
//   const t = await NgramWorkers.open(cfg, { workers: 4, dir: ["models", "q38flash-ngram"] });
//   const emb = await t.rows(state.push([tok]));   // 2560 floats, like NgramTable.rows
//   t.close();
// Worker w owns heads h with h % workers == w. The handles stay open until close(); while they are, nothing else in
// the origin can open those files (another tab's copy of the table gets NoModificationAllowedError: one table user
// per origin at a time).
import { NgramTable, makeConfig, opfsReadAt, ngramFileName } from "./ngram.js";

const G = /** @type {any} */ (globalThis);
const IS_WORKER = typeof G.WorkerGlobalScope !== "undefined" && G.self instanceof G.WorkerGlobalScope;

// ---------------------------------------------------------------- worker side
if (IS_WORKER) {
  let table = null, handles = [], queue = Promise.resolve();
  // one message at a time: a close that arrived while an open was still awaiting its handles must wait for them,
  // or the handles opened after it stay locked
  G.self.onmessage = (ev) => { queue = queue.then(() => handle(ev.data)); };
  const handle = async (m) => {
    try {
      if (m.op === "open") {
        let d = await G.navigator.storage.getDirectory();
        for (const part of m.dir) d = await d.getDirectoryHandle(part);
        const cfg = makeConfig(m.cfg);
        handles = [];
        for (const h of m.heads) {   // a throw here leaves the opened ones in handles, for the client's close
          const fh = await d.getFileHandle(ngramFileName(h));
          handles[h] = await fh.createSyncAccessHandle();
          const size = handles[h].getSize(), want = cfg.vocab[h] * cfg.rowBytes;
          if (size !== want) throw new Error(`${ngramFileName(h)} is ${size} B, head ${h} needs ${want}`);
        }
        table = new NgramTable(cfg, { readAt: opfsReadAt(handles), layout: "split", cacheRows: m.cacheRows ?? 4096 });
        G.self.postMessage({ id: m.id, ok: true });
      } else if (m.op === "rows") {
        const out = await table.rows(m.rows);
        G.self.postMessage({ id: m.id, ok: true, out, stats: table.stats }, [out.buffer]);
      } else if (m.op === "close") {
        for (const h of handles) h?.close();
        handles = []; table = null;
        G.self.postMessage({ id: m.id, ok: true });
      }
    } catch (e) {
      G.self.postMessage({ id: m.id, ok: false, error: String(e?.message || e), name: e?.name });
    }
  };
}

// ---------------------------------------------------------------- main-thread side
// cfg: a config from ngramConfig / makeConfig. Plain values cross to the workers (BigInts are rebuilt there).
const plain = (c) => ({ offsets: c.offsets, vocab: c.vocab, mult: c.mult, ngramSize: c.ngramSize, perNgram: c.perNgram, width: c.width,
  eos: c.eos, image: c.image, ggmlType: c.ggmlType, nRows: c.nRows });

export class NgramWorkers {
  constructor(cfg, workers) {
    this.cfg = cfg; this.workers = workers; this.seq = 0; this.wait = new Map();
    for (const w of workers) w.onmessage = (ev) => {
      const p = this.wait.get(ev.data.id);
      if (!p) return;
      this.wait.delete(ev.data.id);
      if (ev.data.ok) p.resolve(ev.data);
      else p.reject(Object.assign(new Error(ev.data.error), { name: ev.data.name || "Error" }));
    };
  }
  _call(w, msg, transfer = []) {
    const id = ++this.seq;
    return new Promise((resolve, reject) => { this.wait.set(id, { resolve, reject }); this.workers[w].postMessage({ ...msg, id }, transfer); });
  }
  // dir: the OPFS path of the split files (from the root); workers: how many (1-16); url: this module's URL
  static async open(cfg, { dir, workers = 4, cacheRows = 4096, url = import.meta.url } = {}) {
    const W = Math.max(1, Math.min(workers, cfg.nHeads));
    const ws = Array.from({ length: W }, () => new G.Worker(url, { type: "module" }));
    const t = new NgramWorkers(cfg, ws);
    try {
      await Promise.all(ws.map((_, w) => t._call(w, { op: "open", dir, cfg: plain(cfg), cacheRows,
        heads: [...Array(cfg.nHeads).keys()].filter((h) => h % W === w) })));
    } catch (e) {
      // the workers that did open their heads close them first: a terminated worker's handles are released only
      // when it is collected, and until then the files stay locked
      await t.close();
      throw e;
    }
    return t;
  }
  // the head a global row belongs to
  _head(r) { const o = this.cfg.offsets; for (let h = o.length - 1; h >= 0; h--) if (r >= o[h]) return h; return -1; }
  async rows(ids, out = new Float32Array(ids.length * this.cfg.width)) {
    const W = this.workers.length, width = this.cfg.width;
    const pick = Array.from({ length: W }, () => []);   // per worker: positions in ids
    for (let i = 0; i < ids.length; i++) {
      const h = this._head(ids[i]);
      if (h < 0) throw new Error(`n-gram: row ${ids[i]} is in no head`);
      pick[h % W].push(i);
    }
    await Promise.all(pick.map(async (pos, w) => {
      if (!pos.length) return;
      const r = await this._call(w, { op: "rows", rows: Uint32Array.from(pos, (i) => ids[i]) });
      for (let j = 0; j < pos.length; j++) out.set(r.out.subarray(j * width, (j + 1) * width), pos[j] * width);
    }));
    return out;
  }
  async close() {
    await Promise.all(this.workers.map((_, w) => this._call(w, { op: "close" }).catch(() => {})));
    for (const w of this.workers) w.terminate();
    this.workers = [];
  }
}
