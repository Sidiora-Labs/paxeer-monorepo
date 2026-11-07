// Vendored from https://github.com/Nehanth/pooled/blob/45b5ee275a8754a37fce4a6ce174f31e4a216ef4/engine/expert_t1.js (MIT; see LICENSE-ENGINE).
// Expert offload's RAM tier for disk layers ("T1", roadmap/32-ssd-expert-offload.md step 3): a cache of experts read
// from the file, each in a slot buffer of its own (MAP_WRITE | COPY_SRC: on a discrete GPU, Dawn puts these in the
// upload heap, system RAM, and the copy engine pulls a slot into VRAM at PCIe speed, as it does a parked layer). A
// pool miss on a disk layer is served from here with six copyBufferToBuffer, and only a T1 miss reads the file.
//
// A slot is in one of four states:
//   free     mapped, empty: the CPU may write an expert into it (the "mapped-free reserve")
//   full     unmapped, holds one expert (layer, e): a copy source for the GPU
//   mapping  evicted, mapAsync(WRITE) pending: nothing may use it until it is free again
//   (none)   not created yet: slots are made on first use, mapped at creation, up to cap
// A mapped buffer may not be used by a submitted command buffer, and mapAsync on a buffer that submitted work still
// reads waits for that work. So the CPU writes only into free slots and unmaps them at once (commit), and eviction
// (taking full slots back to free) happens only in flush(), which the store calls once everything encoded so far has
// been submitted (ExpertStore.prepare / loadLayerAsync / loadExpertsAsync: their callers submit first). plan() never
// evicts: a slot it copies from stays full until the next flush, so nothing the step uses is evicted under it.
// When no free slot is left (a burst of misses faster than mapAsync returns), the expert goes by queue.writeBuffer
// instead (ExpertStore's PR 1 path) and is not cached.
//
// Replacement: segmented LRU with admit-on-second-miss (prior art: 20% probationary, 80% protected; an expert is
// cached only the second time it misses, so a burst of one-off experts, a long prompt's tail, does not evict the hot
// decode set). A first miss goes into a ghost list (keys only) instead; while the cache still has slots it never
// used, every miss is admitted. A hit in probation moves the expert to protected; protected overflowing demotes its
// least recent entry back to probation. The victim is probation's least recent, else protected's.
//
// Keys are layer * nExp + e. The slot's six slices sit at the offsets ExpertStore passes (each 256-byte aligned).
const MAP_SLOT = () => GPUBufferUsage.MAP_WRITE | GPUBufferUsage.COPY_SRC;

export class ExpertT1 {
  // bytes: the RAM the cache may take (a promise, like a pledge: never more). slotBytes: one slot (the largest disk
  // layer's expert, its slices 256-aligned). reserve: free slots flush() keeps ready (mapped) for the next misses.
  constructor(device, { bytes, slotBytes, reserve = 16, protectedShare = 0.8 }) {
    if (!(slotBytes > 0)) throw new Error("ExpertT1: slotBytes must be positive");
    this.device = device;
    this.slotBytes = Math.ceil(slotBytes / 256) * 256;
    this.cap = Math.max(0, Math.floor((bytes > 0 ? bytes : 0) / this.slotBytes));
    this.reserve = Math.max(0, Math.min(reserve, Math.floor(this.cap / 4)));
    this.protCap = Math.max(1, Math.floor(this.cap * protectedShare));
    this.slots = [];             // { buf, state, key, view }
    this.of = new Map();         // key -> slot index (full slots)
    this.prob = new Map();       // key -> true, least recent first (Map order)
    this.prot = new Map();
    this.ghost = new Map();      // keys that missed once, least recent first; at most cap of them
    this.freeIdx = [];           // free (mapped) slot indices
    this.pending = 0;            // slots in "mapping"
    this.stats = this._zero();
  }
  _zero() { return { hits: 0, admits: 0, skips: 0, noSlot: 0, evicts: 0 }; }
  get bytes() { return this.slots.length * this.slotBytes; }
  get full() { return this.of.size; }

  // the full slot holding key, or -1. touch: count it as a use (SLRU promotion)
  find(key, touch = true) {
    const s = this.of.get(key);
    if (s === undefined) return -1;
    if (touch) {
      this.stats.hits++;
      if (this.prob.has(key)) { this.prob.delete(key); this._protect(key); }
      else { this.prot.delete(key); this.prot.set(key, true); }
    }
    return s;
  }
  _protect(key) {
    this.prot.set(key, true);
    while (this.prot.size > this.protCap) {   // demote protected's least recent to probation's most recent
      const k = this.prot.keys().next().value;
      this.prot.delete(k); this.prob.set(k, true);
    }
  }
  // Whether a miss of key should be cached (admit-on-second-miss): yes while the cache has never-used room, or when
  // key missed before (it is in the ghost list); else it is remembered as a first miss. The ghost entry stays until
  // commit() caches key, so a second miss that finds no free slot (take() -> null) keeps its credit for the next one.
  wants(key) {
    if (this.cap === 0) return false;
    if (this.slots.length < this.cap || this.ghost.has(key)) return true;
    this.ghost.delete(key); this.ghost.set(key, true);
    if (this.ghost.size > this.cap) this.ghost.delete(this.ghost.keys().next().value);
    this.stats.skips++;
    return false;
  }
  // A free slot to write key into: -> { s, view } (view: the mapped slot, slotBytes long), or null when none is free
  // and none can be made (the caller falls back to writeBuffer). Call commit(s) once written.
  take(key) {
    if (this.of.has(key)) throw new Error(`ExpertT1: ${key} is already cached`);
    let s = this.freeIdx.pop();
    if (s === undefined && this.slots.length < this.cap) {
      const buf = this.device.createBuffer({ size: this.slotBytes, usage: MAP_SLOT(), mappedAtCreation: true });
      s = this.slots.push({ buf, state: "free", key: -1, view: new Uint8Array(buf.getMappedRange()) }) - 1;
    }
    if (s === undefined) { this.stats.noSlot++; return null; }
    const S = this.slots[s];
    if (S.state !== "free" || !S.view) throw new Error(`ExpertT1: slot ${s} is ${S.state}, not free`);
    return { s, view: S.view };
  }
  // the slot written: unmap it (from here the GPU may copy from it) and cache key there, most recent in probation
  commit(s, key) {
    const S = this.slots[s];
    S.buf.unmap(); S.view = null; S.state = "full"; S.key = key;
    this.of.set(key, s); this.prob.set(key, true); this.ghost.delete(key);
    this.stats.admits++;
    return S.buf;
  }
  buf(s) { return this.slots[s].buf; }

  // After everything encoded so far is submitted: evict down to `reserve` free slots (counting those mapping) once the
  // cache is at its cap, and start their maps. A map that fails drops the slot (the cache is one smaller).
  flush() {
    if (this.slots.length < this.cap) return 0;
    let n = 0;
    while (this.freeIdx.length + this.pending < this.reserve) {
      const key = this.prob.size ? this.prob.keys().next().value : this.prot.size ? this.prot.keys().next().value : undefined;
      if (key === undefined) break;
      this.prob.delete(key); this.prot.delete(key);
      const s = this.of.get(key), S = this.slots[s];
      this.of.delete(key);
      S.state = "mapping"; S.key = -1; this.pending++; n++;
      this.stats.evicts++;
      // a slot destroy() already killed is no longer counted in pending (destroy() zeroes it)
      S.buf.mapAsync(GPUMapMode.WRITE).then(() => {
        if (S.state !== "mapping") return;
        this.pending--;
        S.view = new Uint8Array(S.buf.getMappedRange()); S.state = "free"; this.freeIdx.push(s);
      }, () => { if (S.state === "mapping") { this.pending--; S.state = "dead"; } });
    }
    return n;
  }
  // free slots, slots mapping (for tests and the summary)
  counts() {
    const c = { free: 0, full: 0, mapping: 0, dead: 0 };
    for (const S of this.slots) c[S.state]++;
    return c;
  }
  destroy() {
    for (const S of this.slots) { S.state = "dead"; try { S.buf.destroy(); } catch {} }
    this.slots = []; this.of.clear(); this.prob.clear(); this.prot.clear(); this.ghost.clear(); this.freeIdx = []; this.pending = 0;
  }
}
