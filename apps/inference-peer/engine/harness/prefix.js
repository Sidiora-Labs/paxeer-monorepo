// Vendored from https://github.com/Nehanth/pooled/blob/45b5ee275a8754a37fce4a6ce174f31e4a216ef4/harness/prefix.js (MIT; see LICENSE-ENGINE).
// Which saved checkpoint to resume from. A hybrid model's state (DeltaNet) cannot be cut back to
// an arbitrary earlier position, so reuse works at checkpoints: the harness saves the state after
// the system prompt + tool list and after every finished assistant turn, and a new request resumes
// from the longest checkpoint whose tokens are a prefix of the new prompt, prefilling only the rest
// (SGLang / vLLM do the same for hybrid models). DOM-free.
//
// A pinned checkpoint (add(ids, key, { pin: true })) is the one after the system prompt + tools:
// it is never evicted by the limit, so a compaction that rewrites the middle of an agent's prompt
// still resumes after the system prompt instead of prefilling it again (issue #73).
export class PrefixIndex {
  constructor(limit = 64) { this.items = []; this.limit = limit; this.clock = 0; }   // [{ ids, key, t, pin }], t = use order
  add(ids, key, { pin = false } = {}) {
    this.items = this.items.filter((x) => x.key !== key);
    this.items.push({ ids: Array.from(ids), key, t: ++this.clock, pin: !!pin });
    // over the limit: keep the pinned ones, then the most recently used
    if (this.items.length > this.limit) { this.items.sort((a, b) => (b.pin - a.pin) || (b.t - a.t)); this.items.length = this.limit; }
  }
  remove(key) { this.items = this.items.filter((x) => x.key !== key); }
  pinned() { return this.items.filter((x) => x.pin); }
  unpinned() { return this.items.filter((x) => !x.pin); }
  // Longest checkpoint that is a strict prefix of ids (at least one token must be left to run,
  // since the last prompt token has to go through the head). -> { key, n } or null
  best(ids) {
    let best = null;
    for (const x of this.items) {
      const n = x.ids.length;
      if (n >= ids.length || (best && n <= best.n)) continue;
      if (isPrefix(x.ids, ids)) best = { key: x.key, n };
    }
    if (best) this.items.find((x) => x.key === best.key).t = ++this.clock;
    return best;
  }
}

// a is a prefix of b (equal included)
export function isPrefix(a, b) {
  if (a.length > b.length) return false;
  for (let i = 0; i < a.length; i++) if (a[i] !== b[i]) return false;
  return true;
}

// Where to cut a prefill so a checkpoint can be saved at `pin` (the end of the system prompt +
// tools) on the way: `pin` when the caches hold less than it (reused < pin) and at least one
// prompt token follows it (the last one has to go through the head), else 0 (no cut).
export function pinSplit(reused, pin, total) {
  return Number.isInteger(pin) && pin > 0 && pin > reused && pin < total ? pin : 0;
}
