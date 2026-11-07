// Vendored from https://github.com/Nehanth/pooled/blob/45b5ee275a8754a37fce4a6ce174f31e4a216ef4/room/plan.js (MIT; see LICENSE-ENGINE).
// Layer placement and the model ladder. DOM-free so it can be unit tested.

// Deal L layers over devices in proportion to what each can hold. caps: bytes each device offers
// for layers (host first; the host's cap already has the embedding, head and draft block taken
// out). Every device gets at least one layer; rounding leftovers go to the largest remainders.
// max (optional): the most layers each device may hold (its pledge in whole layers, room/plan.js
// layerCaps). With it no device ever gets more than its max: what a full device cannot take goes to
// the others with room, a device with max 0 holds nothing, and when the maxes cannot hold all L
// layers the deal is not made (`short` says how many layers are missing; the room must not start).
// Returns { assigned: [count per device], ranges: [[lo, hi) per device], short }.
export function planSplit(L, caps, max = null) {
  // a cap that is not a positive number (a malformed pledge) counts as nothing; when nobody
  // offers anything the layers are dealt evenly instead of dividing by zero
  caps = caps.map((c) => (Number.isFinite(c) && c > 0 ? c : 0));
  if (max) return splitWithin(L, caps, max.map((m) => (Number.isFinite(m) && m > 0 ? Math.floor(m) : 0)));
  const top = Math.max(0, ...caps);
  if (top > 0) caps = caps.map((c) => c / top);   // only the ratios matter; keeps L * c finite
  let totalCap = caps.reduce((s, c) => s + c, 0);
  if (!(totalCap > 0)) { caps = caps.map(() => 1); totalCap = caps.length; }
  const assigned = caps.map((c) => Math.floor(L * c / totalCap));
  const fracs = caps.map((c, i) => ({ i, f: L * c / totalCap - assigned[i] })).sort((a, b) => b.f - a.f);
  const rem = L - assigned.reduce((a, b) => a + b, 0);
  for (let k = 0; k < rem; k++) assigned[fracs[k % fracs.length].i]++;
  // the host first (it runs the embedding and the head, so it must hold a layer too), and only
  // take a layer from a device that keeps one: with fewer layers than devices the last ones wait
  for (let i = 0; i < assigned.length; i++)
    if (assigned[i] === 0) { const j = assigned.indexOf(Math.max(...assigned)); if (assigned[j] > 1 || (i === 0 && L > 0)) { assigned[j]--; assigned[i]++; } }
  return { assigned, ranges: rangesOf(assigned), short: 0 };
}
const rangesOf = (assigned) => { const r = []; let acc = 0; for (const a of assigned) { r.push([acc, acc + a]); acc += a; } return r; };

// planSplit with a most-layers-per-device: in proportion to caps, each device held to its max, the
// excess dealt again over the devices with room (in proportion too), rounding leftovers to the
// largest remainders among devices with room. The host keeps at least one layer; a device whose
// max is 0 holds none; the others get one each while there are layers enough.
function splitWithin(L, caps, max) {
  const n = caps.length, assigned = new Array(n).fill(0);
  if (!n || L <= 0) return { assigned, ranges: rangesOf(assigned), short: Math.max(0, L) };
  const room = max.reduce((s, m) => s + m, 0);
  if (max[0] < 1 || room < L) return { assigned, ranges: rangesOf(assigned), short: Math.max(L - room, max[0] < 1 ? 1 : 0) };
  // weights: the byte caps' ratios, or even when none is given; a device with room but no weight
  // (a malformed cap next to a whole max) still counts a little so it can take an overflow
  const top = Math.max(0, ...caps);
  const w = caps.map((c, i) => (max[i] > 0 ? (top > 0 ? Math.max(c / top, 1e-9) : 1) : 0));
  let left = L;
  // water-fill by weight: every round deals what is left over the devices still below their max
  for (let guard = 0; left > 0 && guard < n + 2; guard++) {
    const open = [...Array(n).keys()].filter((i) => assigned[i] < max[i]);
    const tw = open.reduce((s, i) => s + w[i], 0);
    const want = open.map((i) => ({ i, x: left * w[i] / tw }));
    let dealt = 0;
    for (const { i, x } of want) { const t = Math.min(Math.floor(x), max[i] - assigned[i]); assigned[i] += t; dealt += t; }
    left -= dealt;
    if (left > 0) {
      // rounding leftovers: largest remainders among devices that still have room
      const fr = want.filter(({ i }) => assigned[i] < max[i]).map(({ i, x }) => ({ i, f: x - Math.floor(x) })).sort((a, b) => b.f - a.f || a.i - b.i);
      for (const { i } of fr) { if (left <= 0) break; assigned[i]++; left--; }
    }
  }
  // then, as planSplit does, every device with room holds a layer (the host first), taken from the
  // device holding the most while it keeps one: with fewer layers than devices the last ones wait
  for (let i = 0; i < n; i++)
    if (assigned[i] === 0 && max[i] > 0) { const j = assigned.indexOf(Math.max(...assigned)); if (assigned[j] > 1 || (i === 0 && j !== 0)) { assigned[j]--; assigned[i]++; } }
  return { assigned, ranges: rangesOf(assigned), short: 0 };
}

// ---------------- layer costs: one size for every layer, or each layer its own ----------------
// Every function below that takes `layerBytes` takes either a number (every layer costs that: the Qwen 3 / 3.5 / 3.6
// rooms, the tests' toy models) or per-layer bytes (an array or Float64Array of L: Qwen3.8-Flash-Next, room/models.js
// flashLayerCost: its layers 0-5 are heavier, its QSA layers carry K / V that grow with the context, its DeltaNet layers
// a fixed state). With per-layer bytes, what n layers cost depends on where they are, as what they park does
// (offloadAt): `pos` says it the same way:
//   a number: the range ends there (exact)
//   { lo, fixed: true }: the range starts at lo (the host's starts at 0: exact)
//   { lo } or null: it starts at lo or later (not dealt yet): the heaviest n consecutive layers that could be its range
// So a device's cap in layers is exact for the host and, for every other device, holds for whichever range the deal
// then gives it (every range past the host's starts at layer 1 or later): no range is ever moved after the deal, and
// each device's `held` is its range's exact sum. A number gives exactly the arithmetic of one size per layer.
// costOf(layerBytes) -> { uniform, L, span(lo, hi), at(n, pos), fitN(bytes, pos), avg, off, resident(n, m, pos, E) }:
//   at(n, pos): what n layers at pos cost; fitN(bytes, pos): the most layers at pos within bytes; avg: a layer's
//   average; off: per-layer costs' offVram / offRam as costs of their own (room/models.js flashLayerCost), else null;
//   resident(n, m, pos, E): what n layers at pos keep on the GPU with the experts of their last m offloaded: each
//   offloaded layer's own experts when the costs carry them (flashLayerCost's experts: Flash's layers 6-47 have 52 MB
//   fewer than layers 0-5), else E each (expertSizes: a profile's E)
const costCache = new WeakMap();
const HOST_POS = { lo: 0, fixed: true }, PEER_POS = { lo: 1 };
export function costOf(layerBytes) {
  if (layerBytes && typeof layerBytes === "object") {
    let C = costCache.get(layerBytes);
    if (!C) costCache.set(layerBytes, C = perLayerCost(layerBytes));
    return C;
  }
  const b = +layerBytes || 0;
  return { uniform: true, L: Infinity, span: (lo, hi) => (hi - lo) * b, at: (n) => n * b, fitN: (bytes) => (b > 0 && bytes > 0 ? Math.floor(bytes / b + 1e-9) : 0), avg: b, off: null,
    resident: (n, m, pos, E) => n * b - m * E };
}
// c.once / c.onceAt (optional): bytes a device keeps once when its range holds any layer flagged in onceAt (Flash: the QSA
// score scratch, flashLayerCost), counted in every range's sum that holds one
function perLayerCost(c, withOff = true) {
  const L = c.length, pre = new Float64Array(L + 1), worstMemo = new Map();
  for (let i = 0; i < L; i++) pre[i + 1] = pre[i] + (+c[i] || 0);
  const once = c.once > 0 && c.onceAt?.length === L ? c.once : 0, opre = new Uint16Array(L + 1);
  if (once) for (let i = 0; i < L; i++) opre[i + 1] = opre[i] + (c.onceAt[i] ? 1 : 0);
  const sum = (lo, hi) => pre[hi] - pre[lo] + (once && hi > lo && opre[hi] > opre[lo] ? once : 0);
  const span = (lo, hi) => (lo < 0 || hi > L || hi < lo ? Infinity : sum(lo, hi));
  const worst = (n, lo0 = 0) => {
    if (n <= 0) return 0;
    if (lo0 + n > L) return Infinity;
    const k = n * (L + 1) + lo0;
    let w = worstMemo.get(k);
    if (w === undefined) { w = 0; for (let s = lo0; s + n <= L; s++) w = Math.max(w, sum(s, s + n)); worstMemo.set(k, w); }
    return w;
  };
  const at = (n, pos = null) => {
    if (!(n > 0)) return 0;
    const hi = typeof pos === "number" ? pos : pos?.fixed ? (pos.lo || 0) + n : null;
    return hi != null ? span(hi - n, hi) : worst(n, pos?.lo || 0);
  };
  const fitN = (bytes, pos = null) => { let n = 0; while (n < L && at(n + 1, pos) <= bytes + 1e-6) n++; return n; };
  const off = withOff && c.offVram && c.offRam ? { vram: perLayerCost(c.offVram, false), ram: perLayerCost(c.offRam, false) } : null;
  // the experts' prefix sums (per-layer costs that carry them), and the worst resident part of n layers starting at lo0
  // or later with their last m offloaded
  const ex = withOff && c.experts?.length === L ? Float64Array.from(c.experts) : null, epre = new Float64Array(L + 1), resMemo = new Map();
  if (ex) for (let i = 0; i < L; i++) epre[i + 1] = epre[i] + ex[i];
  const resident = (n, m, pos, E) => {
    if (!ex) return at(n, pos) - m * E;
    const hi = typeof pos === "number" ? pos : pos?.fixed ? (pos.lo || 0) + n : null;
    if (hi != null) return span(hi - n, hi) - (epre[hi] - epre[hi - m]);
    const lo0 = pos?.lo || 0;
    if (lo0 + n > L) return Infinity;
    const k = (n * (L + 1) + m) * (L + 1) + lo0;
    let w = resMemo.get(k);
    if (w === undefined) { w = -Infinity; for (let s = lo0; s + n <= L; s++) w = Math.max(w, sum(s, s + n) - (epre[s + n] - epre[s + n - m])); resMemo.set(k, w); }
    return w;
  };
  return { uniform: false, L, span, at, fitN, avg: L ? sum(0, L) / L : 0, off, resident };
}
// The layers' total bytes: L x a number, or the per-layer bytes' sum
export const layersTotal = (layerBytes, L) => (layerBytes && typeof layerBytes === "object" ? costOf(layerBytes).span(0, layerBytes.length) : L * (+layerBytes || 0));

// How many whole layers each device can hold within its pledge. pledges: bytes each device lends
// (host first); layerBytes: one layer with its share of the KV cache at the room's context, or each layer's own
// (costOf: the host's from layer 0, every other device's the heaviest range it could be dealt);
// hostBytes: what the model host holds besides its layers (the embedding, the output head, the
// draft block: room/models.js hostHeldBytes). A pledge is a promise: nobody is dealt more than this.
// With per-layer costs the last device that can hold a layer is sized on the range ending at layer L, exactly: every
// deal (planSplit, planForSpeed, dealOffload, placeNgram) keeps the host-first order, so when it holds layers its
// range is the tail. A device between them keeps the heaviest window (it could start anywhere past the host's).
export function layerCaps(pledges, layerBytes, hostBytes = 0) {
  const C = costOf(layerBytes);
  const b = pledges.map((p, i) => (Number.isFinite(p) && p > 0 ? p : 0) - (i === 0 ? hostBytes : 0));
  const caps = b.map((x, i) => (C.avg > 0 && x > 0 ? C.fitN(x, i === 0 ? HOST_POS : PEER_POS) : 0));
  const t = lastPeer(b, C);
  if (t) caps[t.i] = t.n;
  return caps;
}
// The last device past the host that holds a layer of the tail, and how many it holds there ({ i, n }); null with one
// size per layer (where a range is does not matter) or when none can
function lastPeer(b, C) {
  if (C.uniform || !(C.avg > 0)) return null;
  for (let i = b.length - 1; i >= 1; i--) { const n = b[i] > 0 ? C.fitN(b[i], C.L) : 0; if (n >= 1) return { i, n }; }
  return null;
}

// Whether a room's pledges hold a model, and if not by how much they fall short. Same arguments as
// layerCaps plus L. Returns { fits, caps, missing, short, raise }: caps in layers per device,
// missing layers, short = the fewest extra bytes one device could add to make it fit (0 when it
// fits), raise[i] = the extra bytes device i alone would have to add (Infinity when it alone can't:
// the host must hold the embedding and head plus a layer before anyone else's layers count).
// off (optional, expert offload): { expertBytes, ram: [bytes per device, host first], disk?: [bool per device] }. When
// the pledges alone fall short, a device with RAM to park experts in counts with what it holds that way (offloadCap);
// the result then fits with offload: true, caps in layers with offload and residentCaps the pledge-only ones. Only
// when that falls short too do the devices with a disk tier (off.disk) count with the layers they hold with some
// experts left in the file (disk: true, offloadCaps: the caps without disk): disk is the last resort.
export function roomFit(L, pledges, layerBytes, hostBytes = 0, off = null) {
  const fit = residentFit(L, pledges, layerBytes, hostBytes);
  if (fit.fits || !offloadOn(off)) return fit;
  const holds = (caps) => caps[0] >= 1 && caps.reduce((s, c) => s + c, 0) >= L;
  const caps = offloadCaps(L, pledges, layerBytes, hostBytes, off);
  if (holds(caps)) return { fits: true, caps, missing: 0, short: 0, raise: caps.map(() => 0), offload: true, residentCaps: fit.caps };
  if (!diskOn(off)) return fit;
  const dcaps = offloadCaps(L, pledges, layerBytes, hostBytes, off, true);
  if (!holds(dcaps)) return fit;
  return { fits: true, caps: dcaps, missing: 0, short: 0, raise: dcaps.map(() => 0), offload: true, disk: true, residentCaps: fit.caps, offloadCaps: caps };
}
function residentFit(L, pledges, layerBytes, hostBytes) {
  const caps = layerCaps(pledges, layerBytes, hostBytes);
  const has = caps.reduce((s, c) => s + c, 0);
  const hostOk = caps[0] >= 1;
  const missing = Math.max(0, L - has, hostOk ? 0 : 1);
  if (!missing) return { fits: true, caps, missing: 0, short: 0, raise: caps.map(() => 0) };
  const p = (i) => (Number.isFinite(pledges[i]) && pledges[i] > 0 ? pledges[i] : 0);
  const others = has - caps[0], C = costOf(layerBytes);
  // the last device holding layers is sized on the tail (layerCaps)
  const last = C.uniform ? -1 : caps.reduce((j, c, i) => (i > 0 && c >= 1 ? i : j), -1);
  const raise = caps.map((c, i) => {
    if (i === 0) return hostBytes + C.at(Math.max(1, L - others), HOST_POS) - p(0);
    if (!hostOk) return Infinity;
    return C.at(c + (L - has), i === last ? C.L : PEER_POS) - p(i);
  }).map((b) => (b === Infinity ? b : Math.max(1, Math.ceil(b))));
  return { fits: false, caps, missing, short: Math.min(...raise), raise };
}

// ---------------- expert offload (engine/expert_store.js; room-node devices with a discrete GPU) ----------------
// A MoE layer's routed experts are most of its bytes (35B: 470 of 498 MB; 122B: 1.41 of 1.49 GB). A device that
// offloads keeps the experts of some of its layers in system RAM ("parked") and copies the ones a token picks into
// a VRAM cache. Its pledge (VRAM) then pays for: its layers less the parked experts, plus the cache: one whole
// layer's experts for prefill (the region) and at least OFFLOAD_MIN_SLOTS experts per offloaded layer. Its RAM
// (meta.ramGB) pays for the parked experts. Neither is ever exceeded: a pledge is a promise (#291), and so is ramGB.
// A device offloads only what its pledge can't hold, the fewest layers it can (each costs a GPU round trip per
// token), and the rest of its pledge becomes the cache.
export const OFFLOAD_MIN_SLOTS = 16;
export const OFFLOAD_EXPERTS = 256;   // routed experts per layer (Qwen3.5 / 3.6 MoE): the cache's unit is 1/256 layer
const offloadOn = (off) => !!off && off.expertBytes > 0 && (off.ram || []).some((r) => r > 0);
// The disk tier (roadmap/32-ssd-expert-offload.md; engine/expert_store.js disk layers, engine/expert_t1.js): a device
// that can (a room node with a discrete GPU, `--disk`, the model's files on its own disk: room/pledge.js diskFor)
// may leave the experts of the last layers of its offloaded range in the GGUF, read when a token picks them, through a
// RAM cache (T1) of at least DISK_RAM_MIN. Its VRAM pays for the same pool and region as a parked layer's; its RAM
// for the parked layers, the spare and T1. So disk lifts the RAM limit, not the VRAM one: the layers a device holds
// are bounded by its pledge alone. It costs disk reads per token, so a deal uses it only when parking can't hold the
// model (roomFit), each device parks all its RAM allows first, and the fewest layers go to disk.
// 2 GiB: the 3-tier simulator's T1 for the 122B and Flash rooms (docs/bench-log.md 2026-10-11, tests/sim/tiersim.js).
export const DISK_RAM_MIN = 2 * 2 ** 30;
const diskOn = (off) => offloadOn(off) && (off.disk || []).some(Boolean);
// A disk-only device (off.diskOnly[i]: a browser tab with expert offload's OPFS tier, engine/expert_opfs.js; the page
// sets meta.tabDisk, room/pledge.js tabDiskGB): its GPU shares the computer's memory, so it never parks experts in "RAM"
// (that is the memory its pledge already lends): every layer it offloads reads its experts from disk, and its RAM
// (meta.tabDisk.ramGB) pays only for the spare and T1, which has no minimum there (the 3-tier simulator: an inclusive
// T1 under a bigger VRAM pool gains under 3%, docs/bench-log.md 2026-10-11). It counts only in roomFit's disk pass, never as a device that parks. Which
// layers it offloads is chosen by tabDiskCost (the fewest m starve every pool: see offloadPlan), not the fewest.
const diskOnlyAt = (off, i) => !!off?.diskOnly?.[i];
// The pool's hit rate by the fraction of a layer's experts it caches: Qwen3.8-Flash-Next's recorded routing (decode,
// chat / code / multiturn, the per-layer LRU of ExpertStore.plan; docs/bench-log.md 2026-10-11), linear between the
// points. An estimate for any MoE: it only ranks offload plans (tabDiskCost), it promises nothing.
const HIT_CURVE = [[0, 0], [0.0625, 0.591], [0.125, 0.713], [0.17, 0.777], [0.25, 0.854], [0.27, 0.867], [0.5, 0.94], [1, 1]];
export function poolHitRate(frac) {
  const f = Math.max(0, Math.min(1, frac));
  for (let i = 1; i < HIT_CURVE.length; i++) {
    const [f1, h1] = HIT_CURVE[i], [f0, h0] = HIT_CURVE[i - 1];
    if (f <= f1) return h0 + (h1 - h0) * (f - f0) / (f1 - f0);
  }
  return 1;
}
// A disk-only device's per-token cost (ms, modelled) of offloading m of its layers with `slots` experts cached each:
// a cut per offloaded layer (TAB_CUT_MS: 0.27 ms on Metal, #351) and the misses each read from OPFS (one expert's
// record at TAB_DISK_GBPS, below the Mac's ~10 GB/s cold reads: a miss also waits for its upload). K: experts a token
// picks. Offloading one more layer frees a whole layer's experts for the pools, so on a pledge well under the model a
// deal that offloads every layer (Mac alone: 48 layers, ~140 slots, ~87% hits) beats the fewest (36 layers at 16 slots,
// ~48%) by about 3x in misses.
export const TAB_CUT_MS = 0.3, TAB_DISK_GBPS = 6;
export function tabDiskCost(m, slots, nExp, K, perExpBytes) {
  const miss = m * K * (1 - poolHitRate(slots / nExp));
  return m * TAB_CUT_MS + miss * perExpBytes / (TAB_DISK_GBPS * 1e6);
}
// The experts' sizes, ExpertStore's way (engine/expert_store.js). ex: room/models.js expertsOf's profile
// ({ E, nExp, slices: per layer, one expert's six parked slices }), or a number: every layer's experts that many bytes
// in the file and parked alike (an estimate; the tests' toy models).
//   E: the VRAM a layer's estimate (layerBytes) frees when its experts are parked
//   park(lo, hi): RAM the experts of layers [lo, hi) park (ExpertStore.parkedBytes)
//   region(lo, hi): the prefill region ExpertStore allocates for them (the largest of each slice, all experts)
//   ramOf(park, m): the RAM ExpertStore's buffers take to park that many bytes of m layers: the bytes, plus at most
//     one expert slice left at the end of each 1 GiB buffer (a slice never spans two), 256-byte alignment per
//     segment, and the last buffer's 4 KiB of slack (ExpertStore._alloc). The deal holds this to the device's RAM.
// Layers differ: the 35B's layers 0-4 and the 122B's 0-5 have Q4_1 down experts, which park as Q8 (25% more).
//   spare(n): what a device that offloads n layers needs past them and its cache (offloadSpare); 0 with a number
//
// What else grows on a device that offloads. It holds its whole pledge (its layers' resident part, then the cache),
// so anything the engine allocates past its layers went over the pledge: on the 122B room the RTX 5070 PC (10 GiB
// pledged, layers 15-47 offloaded) reached 11.5 of its 12.2 GB of VRAM after a 12k-token prompt (prefill buffers,
// speculation's rollback state, the prompt's checkpoints), and WDDM moved what did not fit into system RAM: the
// process grew from 43 to 49-51 GB with 6.7-8 GB of the 63.5 GB left free. So the deal keeps room for it:
//   vram: the engine's working buffers (profile work), each layer's rollback state for speculation (state) and the
//     host's checkpoints: OFFLOAD_CKPT_SLOTS copies of every layer's recurrent state (state each, whatever the prompt's
//     length) and the KV rows of OFFLOAD_CKPT_TOKENS positions across them (kvPos each), all out of the cache;
//   ram: OFFLOAD_RAM_BASE (the process itself: JS, Dawn, upload staging) and one long checkpoint's copy (a pinned
//     prefix goes to disk through RAM, packages/room-node/ckptdisk.js), on top of the parked experts, within what the
//     device lends.
// The 122B on that PC (33 layers): 2.0 GiB of VRAM (its cache 7.15 -> 5.17 GiB, 36 -> 23 experts per layer) and 1.9 GiB
// of RAM kept. Replayed there with the deal's numbers (8 checkpoints, a 12k-token prompt): main's cache spilled 6 GB
// into RAM (free 14.3 -> 8.4 GB) and decode slowed 23 -> 14.5 tok/s; with the spare VRAM peaked at 10.2 GiB, no spill.
export const OFFLOAD_CKPT_SLOTS = 8;           // a room node host keeps 4 answer + 4 pinned checkpoints (ckpt.js CKPT_DEFAULTS)
export const OFFLOAD_CKPT_TOKENS = 32768;      // their KV rows: a 16k-token agent prompt pinned, and its turn checkpoint
export const OFFLOAD_RAM_BASE = 1.5 * 2 ** 30; // the PC's process past its parked experts: 1.6 GB
// C, pos (optional): the deal's layer costs (costOf) and where the n layers are. A profile with byLayer
// (Qwen3.8-Flash-Next: room/models.js flashOffload) takes its checkpoints' part from them when they have it
// (flashLayerCost's offVram / offRam: the shared flashCkptReserve, counted once), else from kvPos x ckptTokens.
function offloadSpare(p, n, C = null, pos = null) {
  if (!(p.work > 0)) return { vram: 0, ram: 0 };
  const st = p.state || 0;
  if (p.byLayer && C?.off) return { vram: p.work + n * st + C.off.vram.at(n, pos), ram: OFFLOAD_RAM_BASE + C.off.ram.at(n, pos) };
  // ckptTokens: a model whose context is shorter keeps fewer KV rows in its checkpoints (Qwen3.8-Flash-Next: 8 x 2048)
  const kv = p.kvPos || 0, T = p.ckptTokens > 0 ? p.ckptTokens : OFFLOAD_CKPT_TOKENS;
  return { vram: p.work + n * (st * (1 + OFFLOAD_CKPT_SLOTS) + kv * T),
    ram: OFFLOAD_RAM_BASE + n * (st + kv * T / 2) };
}
export { offloadSpare as offloadSpareOf };
const exCache = new WeakMap();
function expertSizes(ex, nExp = OFFLOAD_EXPERTS) {
  if (ex && typeof ex === "object") {
    let X = exCache.get(ex);
    if (X) return X;
    const N = ex.nExp || nExp, sl = ex.slices || [], L = sl.length;
    const pre = [0]; sl.forEach((a, i) => { pre[i + 1] = pre[i] + a.reduce((x, y) => x + y, 0) * N; });
    const maxSlice = Math.max(0, ...sl.flat());
    const bad = (lo, hi) => !(lo >= 0 && hi <= L && hi >= lo);
    // region: false (Qwen3.8-Flash-Next's engine builds its store without one): none is set aside. K: experts a token picks
    X = { E: +ex.E || 0, nExp: N, L, K: +ex.K || 8, spare: (n, C, pos) => offloadSpare(ex, n, C, pos),
      ramOf: (park, m) => { const bufs = Math.ceil(park / 2 ** 30) + 1; return park + bufs * maxSlice + (6 * m + bufs) * 256 + 4096; },
      park: (lo, hi) => (bad(lo, hi) ? Infinity : pre[hi] - pre[lo]),
      region: (lo, hi) => {
        if (bad(lo, hi)) return Infinity;
        if (ex.region === false) return 0;
        let r = 0;
        for (let j = 0; j < 6; j++) { let m = 0; for (let l = lo; l < hi; l++) m = Math.max(m, sl[l][j]); r += m * N; }
        return r;
      } };
    exCache.set(ex, X);
    return X;
  }
  const E = +ex || 0;
  return { E, nExp, L: Infinity, K: 8, spare: () => ({ vram: 0, ram: 0 }), ramOf: (park) => park, park: (lo, hi) => (hi - lo) * E, region: () => E };
}
// Where a device's layers are (offloadNeed's pos): its last m layers are the ones offloaded, so what they park depends
// on where its range ends.
//   a number: the range ends there (the deal has dealt it: exact)
//   { lo, fixed: true }: the range starts at lo (the host's starts at 0), so n layers end at lo + n (exact)
//   { lo } or null: it starts at lo or later (not dealt yet): the worst range of n layers that does (its last m layers
//     the m that park the most, the region of every layer they could be)
// Every device after the host starts past the host's resident layers (dealOffload gives the host at least those),
// so a device's caps from { lo: that } hold for whatever range the deal then gives it.
// d: of the m offloaded layers, the last d stay on disk (the disk tier): they take pool slots and the region like the
// others, no parked RAM, and the device keeps DISK_RAM_MIN for T1.
// -> { park, ramNeed, region, perExp: one expert of each offloaded layer (ExpertStore.build's perExp), resident }
// tMin: T1's least size when d > 0 (DISK_RAM_MIN; 0 on a disk-only device)
// layerBytes: a number or per-layer bytes (costOf): the n layers' resident part is theirs at pos.
function offloadAt(X, n, m, layerBytes, pos, d = 0, tMin = DISK_RAM_MIN) {
  let all, park, region;
  const hi = typeof pos === "number" ? pos : pos?.fixed ? (pos.lo || 0) + n : null;
  if (hi != null) { all = X.park(hi - m, hi); park = d ? X.park(hi - m, hi - d) : all; region = X.region(hi - m, hi); }
  else if (!Number.isFinite(X.L)) { all = X.park(0, m); park = d ? X.park(0, m - d) : all; region = X.region(0, m); }
  else {
    const s0 = Math.max(0, (pos?.lo || 0) + n - m);
    all = park = s0 + m > X.L ? Infinity : 0;
    for (let s = s0; s + m <= X.L; s++) { all = Math.max(all, X.park(s, s + m)); if (d) park = Math.max(park, X.park(s, s + m - d)); }
    if (!d) park = all;
    region = X.region(Math.min(s0, X.L), X.L);
  }
  const C = costOf(layerBytes), sp = X.spare(n, C, pos);
  const ramNeed = (m - d > 0 ? X.ramOf(park, m - d) : 0) + sp.ram + (d > 0 ? tMin : 0);
  return { park, ramNeed, ramSpare: sp.ram, vramSpare: sp.vram, region, perExp: all / X.nExp, resident: C.resident(n, m, pos, X.E) + sp.vram };
}
// the VRAM cache n layers with m offloaded leave in `bytes`, and the slots per offloaded layer it holds after the
// region (ExpertStore.build: P = floor((vramBytes - region) / perExp), at most nExp)
function cacheAt(a, bytes, nExp) {
  const vram = Math.floor(bytes - a.resident + 1e-6);
  return { vram, slots: Math.min(nExp, Math.floor((vram - a.region) / a.perExp)) };
}
// The fewest of n layers to offload so they fit `bytes` of VRAM (the pledge, less the host's own tensors) and
// `ram` bytes of RAM: 0 when they fit without, -1 when they can't fit at all. ex: expertSizes's (a profile or a
// number); pos: where the device's range is (offloadAt).
export function offloadNeed(n, bytes, ram, layerBytes, ex, minSlots = OFFLOAD_MIN_SLOTS, nExp = OFFLOAD_EXPERTS, pos = null) {
  if (!(n > 0)) return 0;
  const C = costOf(layerBytes);
  if (C.at(n, pos) <= bytes + 1e-6) return 0;
  const X = expertSizes(ex, nExp);
  if (!(X.E > 0) || !(X.E < C.avg)) return -1;
  // n layers, m offloaded: their resident part + the region + minSlots experts per offloaded layer <= bytes; parked <= ram
  for (let m = 1; m <= n; m++) {
    const a = offloadAt(X, n, m, layerBytes, pos);
    if (!(a.ramNeed <= ram + 1e-6)) return -1;   // (parking more layers only takes more RAM)
    if (cacheAt(a, bytes, X.nExp).slots >= minSlots) return m;
  }
  return -1;
}
// offloadNeed with the disk tier: the fewest of n layers to offload for the VRAM, and of those the fewest to leave on
// disk for the RAM. -> { m, d } ({ m: 0, d: 0 } when they fit without offload; d: 0 when parking alone holds them,
// with offloadNeed's m), or null when they can't fit at all (the VRAM, or no RAM for the spare and T1).
// diskOnly (a disk-only device: diskOnlyAt): every offloaded layer is a disk layer ({ m, d: m }) and T1 has no minimum.
export function offloadNeedDisk(n, bytes, ram, layerBytes, ex, minSlots = OFFLOAD_MIN_SLOTS, nExp = OFFLOAD_EXPERTS, pos = null, diskOnly = false) {
  const C = costOf(layerBytes);
  if (!(n > 0) || C.at(n, pos) <= bytes + 1e-6) return { m: 0, d: 0 };
  const X = expertSizes(ex, nExp);
  if (!(X.E > 0) || !(X.E < C.avg)) return null;
  const fits = (m, d) => offloadAt(X, n, m, layerBytes, pos, d, diskOnly ? 0 : DISK_RAM_MIN).ramNeed <= ram + 1e-6;
  for (let m = 1; m <= n; m++) {
    if (cacheAt(offloadAt(X, n, m, layerBytes, pos), bytes, X.nExp).slots < minSlots) continue;   // (d leaves the VRAM as it is)
    if (diskOnly) return fits(m, m) ? { m, d: m } : null;
    if (fits(m, 0)) return { m, d: 0 };
    if (!fits(m, m)) return null;   // not even the spare and T1: more layers don't help
    let lo = 1, hi = m;   // the fewest on disk: past d = 0 (no T1), RAM need falls as d grows
    while (lo < hi) { const mid = (lo + hi) >> 1; if (fits(m, mid)) hi = mid; else lo = mid + 1; }
    return { m, d: lo };
  }
  return null;
}
// The most layers (up to L) a device holds with offload (disk: with the disk tier too). bytes: VRAM for layers (pledge
// less host tensors); pos: where its range starts (offloadAt).
export function offloadCap(L, bytes, ram, layerBytes, ex, minSlots = OFFLOAD_MIN_SLOTS, pos = null, disk = false, diskOnly = false) {
  const C = costOf(layerBytes);
  let n = C.avg > 0 && bytes > 0 ? Math.min(L, C.fitN(bytes, pos)) : 0;
  const can = disk ? (k) => offloadNeedDisk(k, bytes, ram, layerBytes, ex, minSlots, OFFLOAD_EXPERTS, pos, diskOnly) !== null
    : (k) => offloadNeed(k, bytes, ram, layerBytes, ex, minSlots, OFFLOAD_EXPERTS, pos) >= 0;
  while (n < L && can(n + 1)) n++;
  return n;
}
const vramFor = (pledges, hostBytes, i) => (Number.isFinite(pledges[i]) && pledges[i] > 0 ? pledges[i] : 0) - (i === 0 ? hostBytes : 0);
const offEx = (off) => off.experts || off.expertBytes;
// layers each device holds with offload where it can (off.ram[i] > 0), within its pledge otherwise; disk: with the disk
// tier where it has one (off.disk[i]). The host's range starts at layer 0; every other device's after the host's
// resident layers (dealOffload).
// A disk-only device counts only in the disk pass, and only with its disk tier for this model (off.disk[i]).
export function offloadCaps(L, pledges, layerBytes, hostBytes, off, disk = false) {
  const rc = layerCaps(pledges, layerBytes, hostBytes);
  return pledges.map((_, i) => (off.ram?.[i] > 0 && (!diskOnlyAt(off, i) || (disk && !!off.disk?.[i]))
    ? Math.max(rc[i], offloadCap(L, vramFor(pledges, hostBytes, i), off.ram[i], layerBytes, offEx(off), off.minSlots, i === 0 ? { lo: 0, fixed: true } : { lo: Math.min(rc[0], L) }, disk && !!off.disk?.[i], diskOnlyAt(off, i)))
    : rc[i]));
}
// What one device's n layers look like with offload: null when they fit its pledge as they are, else
// { layers: m (the last m of its range are offloaded), vramBytes: its expert cache (region + slots: the rest of the
// pledge after vramSpare), ramBytes: the experts parked, ramSpare / vramSpare: what it keeps free past them
// (offloadSpare), slots: experts cached per offloaded layer }. With a profile and the range's
// end (pos: a number, as dealOffload passes), ramBytes and slots are ExpertStore's own: it parks exactly ramBytes and
// sizes P = slots from vramBytes.
// m: the fewest layers that leave each offloaded layer OFFLOAD_GOOD_SLOTS cached experts, else as many as RAM
// allows. Offloading one more layer frees its experts but adds a GPU round trip per token (~0.4 ms on the RTX 5070);
// the cache's hit rate falls fast below a quarter of the experts (35B on the 5070: 109 slots 90% hits, 50 slots 70%,
// docs/bench-log.md 2026-10-02), so the cache's size wins until then.
// disk (the device has a disk tier): when parking can't hold the n layers, the last diskLayers of the m stay in the file
// (offloadNeedDisk: the fewest m, then the fewest on disk) and the plan also has diskLayers and diskRamBytes: the
// RAM for T1, all that is left past the parked experts and the spare (at least DISK_RAM_MIN). When parking holds
// them, the plan is the one without disk, field for field.
// The disk plan keeps the fewest m and does not step up toward OFFLOAD_GOOD_SLOTS, so its pools are small (19 slots
// per layer for the 122B tab(28) + pc(10, --ram 24 --disk); 20 for the 35B on pc(10, --ram 8 --disk)). With the RAM
// full, every layer the step-up adds goes to disk, and T1's experts spread over more layers. Measured on the GB10
// (35B, pc(10, 8, disk), Dawn, warm page cache, docs/bench-log.md 2026-10-11): stepping up to 40 layers / 65 slots
// took pool hits from 43% to 80% (147 -> 65 misses per token) but T1 hits fell from 69 to 7 per token and disk reads
// rose from 35.5 to 40.8 per token; decode was within the shared GPU's noise (8.9-12.3 vs 9.9-10.3 tok/s). Disk reads
// are the slowest tier on an NVMe PC, so the fewest m stays until a cold-cache run on the PC says otherwise.
export const OFFLOAD_GOOD_SLOTS = 64;
// diskOnly (a disk-only device, with disk): every offloaded layer reads from disk, T1 is all its RAM past the spare,
// and m is the one tabDiskCost rates cheapest from the fewest that fit up to all n (not the fewest: on a unified-memory
// tab a layer kept resident costs a whole layer's worth of pool slots in every offloaded layer).
export function offloadPlan(n, bytes, ram, layerBytes, ex, minSlots = OFFLOAD_MIN_SLOTS, nExp = OFFLOAD_EXPERTS, pos = null, disk = false, diskOnly = false) {
  if (disk && diskOnly) {
    const nd = offloadNeedDisk(n, bytes, ram, layerBytes, ex, minSlots, nExp, pos, true);
    if (!nd || nd.m <= 0) return null;
    const X = expertSizes(ex, nExp);
    let best = null;
    for (let m = nd.m; m <= n; m++) {
      const a = offloadAt(X, n, m, layerBytes, pos, m, 0), c = cacheAt(a, bytes, X.nExp);
      if (c.slots < minSlots || !(a.ramNeed <= ram + 1e-6)) continue;
      const cost = tabDiskCost(m, c.slots, X.nExp, X.K, a.perExp);
      if (!best || cost < best.cost - 1e-9) best = { m, a, c, cost };
    }
    if (!best) return null;
    const t1 = Math.max(0, Math.floor((ram - best.a.ramNeed) / 2 ** 20) * 2 ** 20);
    return { layers: best.m, vramBytes: best.c.vram, ramBytes: 0, ramSpare: best.a.ramSpare, vramSpare: best.a.vramSpare, slots: best.c.slots,
      diskLayers: best.m, diskRamBytes: t1, diskOnly: true };
  }
  if (disk) {
    const nd = offloadNeedDisk(n, bytes, ram, layerBytes, ex, minSlots, nExp, pos);
    if (!nd || nd.m <= 0) return null;
    if (nd.d > 0) {
      const X = expertSizes(ex, nExp), a = offloadAt(X, n, nd.m, layerBytes, pos, nd.d), c = cacheAt(a, bytes, X.nExp);
      const t1 = Math.floor((ram - (a.ramNeed - DISK_RAM_MIN)) / 2 ** 20) * 2 ** 20;
      return { layers: nd.m, vramBytes: c.vram, ramBytes: a.park, ramSpare: a.ramSpare, vramSpare: a.vramSpare, slots: c.slots, diskLayers: nd.d, diskRamBytes: t1 };
    }
  }
  const m0 = offloadNeed(n, bytes, ram, layerBytes, ex, minSlots, nExp, pos);
  if (m0 <= 0) return null;
  const X = expertSizes(ex, nExp);
  const at = (m) => { const a = offloadAt(X, n, m, layerBytes, pos); return { ...a, ...cacheAt(a, bytes, X.nExp) }; };
  let m = m0, best = at(m0);
  for (let k = m0 + 1; k <= n && best.slots < OFFLOAD_GOOD_SLOTS; k++) {
    const a = at(k);
    if (!(a.ramNeed <= ram + 1e-6)) break;
    if (a.slots >= minSlots) { m = k; best = a; }
  }
  return { layers: m, vramBytes: best.vram, ramBytes: best.park, ramSpare: best.ramSpare, vramSpare: best.vramSpare, slots: best.slots };
}

// Speculative decoding (the draft head's MTP drafts, lookup drafts) in a room whose deal has a device offloading
// experts. A verify pass runs K+1 tokens through every layer, and on an offloaded layer their distinct experts
// (up to 8 per token, ~30 for K=3) must all be in the slot pool: a verify copies several times the experts one plain
// token does, and copies don't overlap compute on the offloading GPU. Measured on the 122B room (Mac Studio +
// RTX 5070 with layers 15-47 offloaded): plain 17-18 tok/s, speculation 12.7-13.5 on prose and 9.3-11.6 on a
// 12k-token agent turn; only a short, highly predictable code answer gained. So speculation is off by default
// while the deal offloads, on otherwise (a resident room: unchanged).
//   offloads: the deal's offload per device (dealRoom's `offload`, or a name -> offload map), or a boolean
//   force: true / false to override (room node POOLED_OFFLOAD_SPEC, room page ?offspec=), null for the default
export const dealOffloads = (offloads) => !!offloads && (typeof offloads === "boolean" ? offloads : Object.values(offloads).some(Boolean));
export function specWithOffload(offloads, force = null) {
  if (force === true || force === false) return force;
  return !dealOffloads(offloads);
}
// "1" / "on" / "true" -> true, "0" / "off" / "false" -> false, anything else -> null (the default)
export function parseForce(v) {
  const s = String(v ?? "").trim().toLowerCase();
  return ["1", "on", "true", "yes"].includes(s) ? true : ["0", "off", "false", "no"].includes(s) ? false : null;
}

// The deal when the pledges alone can't hold the model and offload can (roomFit's offload: true): every device
// that does not offload holds what its pledge holds (all of it: the room is short without offload), and the
// devices that offload take the rest, each from its pledge-only layers up to its cap with offload, in proportion
// to the extra each can take. Phones are left out when the computers hold the model (as dealRoom does).
// With the disk tier (fit.disk): every device first holds what it holds without it (fit.offloadCaps: its pledge, then
// parking), and only the rest goes to the devices with a disk tier, in proportion to the extra it gives each.
function dealOffload({ L, layerBytes, hostBytes, pledges, phone, phoneLayers, off, fit }) {
  const n = pledges.length, rc = fit.residentCaps, oc = fit.caps, out = {}, base = fit.disk ? fit.offloadCaps : rc;
  const canOff = (i) => off.ram?.[i] > 0 && (!fit.disk || !!off.disk?.[i]) && oc[i] > base[i];
  let idx = [...Array(n).keys()].filter((i) => i === 0 || oc[i] >= 1);
  for (let i = 1; i < n; i++) if (oc[i] < 1) out[i] = "small";
  if (!phoneLayers) {
    const comp = idx.reduce((s, i) => s + (i === 0 || !phone?.[i] ? oc[i] : 0), 0);
    if (comp >= L) { idx.filter((i) => i > 0 && phone?.[i]).forEach((i) => { out[i] = "unneeded"; }); idx = idx.filter((i) => i === 0 || !phone?.[i]); }
  }
  const a = new Map(idx.map((i) => [i, base[i]]));
  let left = L - [...a.values()].reduce((s, x) => s + x, 0);
  const offIdx = idx.filter(canOff), extra = new Map(offIdx.map((i) => [i, oc[i] - base[i]]));
  const tot = offIdx.reduce((s, i) => s + extra.get(i), 0);
  if (left <= 0 || left > tot) return null;   // (left <= 0: the pledges hold it after all; roomFit says so first)
  // in proportion to the extra each can take, largest remainders first, never past a cap
  const want = offIdx.map((i) => ({ i, x: left * extra.get(i) / tot }));
  for (const { i, x } of want) { const t = Math.min(Math.floor(x), extra.get(i)); a.set(i, a.get(i) + t); left -= t; }
  want.sort((p, q) => (q.x - Math.floor(q.x)) - (p.x - Math.floor(p.x)) || p.i - q.i);
  for (let guard = 0; left > 0 && guard < 4 * n + 4; guard++) for (const { i } of want) { if (left <= 0) break; if (a.get(i) < oc[i]) { a.set(i, a.get(i) + 1); left--; } }
  if (left > 0) return null;
  const used = idx.filter((i) => i === 0 || a.get(i) > 0);
  idx.forEach((i) => { if (!used.includes(i)) out[i] = "unneeded"; });
  const assigned = used.map((i) => a.get(i)), ranges = rangesOf(assigned);
  const offload = used.map((i, k) => {
    if (!(off.ram?.[i] > 0)) return null;
    const dk = !!fit.disk && !!off.disk?.[i];
    if (diskOnlyAt(off, i) && !dk) return null;   // (a disk-only device never parks: within its pledge, checked below)
    const p = offloadPlan(assigned[k], vramFor(pledges, hostBytes, i), off.ram[i], layerBytes, offEx(off), off.minSlots, OFFLOAD_EXPERTS, ranges[k][1], dk, diskOnlyAt(off, i));
    return p && { ...p, lo: ranges[k][1] - p.layers, hi: ranges[k][1], ...(p.diskLayers > 0 ? { diskLo: ranges[k][1] - p.diskLayers } : {}) };
  });
  // (the caps hold for any range the deal gives: offloadCaps. Should a device still be dealt past its pledge without
  // a plan, the deal fails rather than break a promise)
  if (used.some((i, k) => assigned[k] > rc[i] && !offload[k])) return null;
  // a device that offloads holds its whole pledge (its layers' resident part, then the cache); its RAM apart
  const C = costOf(layerBytes);
  const held = used.map((i, k) => (offload[k] ? vramFor(pledges, hostBytes, i) : C.span(ranges[k][0], ranges[k][1])) + (i === 0 ? hostBytes : 0));
  return { fit, used, assigned, ranges, held, out, offload };
}
// ---------------- a disk-only tab host + a room node that parks: the split by modelled speed ----------------
// One answer's tokens run through the devices one after another, so a token costs the sum of each device's part, and
// the split is worth choosing per layer. dealOffload gives a disk-only tab (the Mac in the Flash demo) only what its
// pledge holds before disk and the node the rest: with the Mac lending 20 GB the RTX 5070 node (--gb 10 --ram 43
// --disk) got layers 14-47, 31 of them parked with 26 experts cached each (56-60% hits, ~180 copies from RAM a
// token, one GPU round trip per parked layer): 54-70 ms a token there against 12-18 ms for the Mac's 14 layers, and
// at a 12 GB lend the node also read 3 layers' experts from its disk. Here the host's share k is the one whose
// modelled cost (tabCost + nodeCost) is lowest among the shares both devices can hold (offloadPlan, as dealOffload
// plans them). Only for that room shape (two devices: the host a disk-only tab, the other a node that parks); every
// other deal is dealOffload's. off.hostLayers (the page's ?hostlayers=, tests) sets k; off.speedSplit === false
// (?flashsplit=0) keeps dealOffload's.
// The constants (ms a token) are measured on the demo's machines (Mac Studio tab, RTX 5070 node; PR notes):
//   TAB_LAYER_MS: a tab layer (14 layers 12-18 ms, the embedding and n-gram rows included); TAB_CUT2_MS: an offloaded
//     layer's cut; TAB_MISS_MS: an expert read from OPFS into the pool (the Mac alone: ~100 reads a token for ~65 ms)
//   NODE_LAYER_MS: a node layer (6 resident layers 4.7 ms a step); NODE_CUT_MS: a parked layer's cut (6 cuts +1.1 ms);
//     NODE_MISS_MS: an expert copied from parked RAM (2.76 MB at ~30 GB/s); NODE_DISK_MISS_MS: one read from the
//     node's model file (~3 GB/s cold)
// The model only ranks splits; SPLIT_GAIN keeps dealOffload's unless another is clearly cheaper.
export const TAB_LAYER_MS = 1.0, TAB_CUT2_MS = 0.3, TAB_MISS_MS = 0.67;
export const SPLIT_GAIN = 0.1;
export const NODE_LAYER_MS = 0.78, NODE_CUT_MS = 0.18, NODE_MISS_MS = 0.09, NODE_DISK_MISS_MS = 0.9;
export function tabCost(n, plan, X) {
  if (!plan) return n * TAB_LAYER_MS;
  return n * TAB_LAYER_MS + plan.layers * TAB_CUT2_MS + plan.layers * X.K * (1 - poolHitRate(plan.slots / X.nExp)) * TAB_MISS_MS;
}
export function nodeCost(n, plan, X) {
  if (!plan) return n * NODE_LAYER_MS;
  const missPer = X.K * (1 - poolHitRate(plan.slots / X.nExp)), d = plan.diskLayers || 0;
  return n * NODE_LAYER_MS + plan.layers * NODE_CUT_MS + missPer * ((plan.layers - d) * NODE_MISS_MS + d * NODE_DISK_MISS_MS);
}
function splitBySpeed(d, { L, layerBytes, hostBytes, pledges, off }) {
  if (off.speedSplit === false || d.used.length !== 2) return null;
  const [h, w] = d.used;
  if (h !== 0 || !diskOnlyAt(off, h) || !off.disk?.[h] || diskOnlyAt(off, w) || !(off.ram?.[w] > 0)) return null;
  const ex = offEx(off), X0 = expertSizes(ex, OFFLOAD_EXPERTS);
  const X = { nExp: X0.nExp, K: X0.K, perExp: Number.isFinite(X0.L) ? X0.park(0, X0.L) / X0.L / X0.nExp : X0.E / X0.nExp };
  const vh = vramFor(pledges, hostBytes, h), vw = vramFor(pledges, hostBytes, w);
  // -> { k, hp, wp, cost } for the host holding layers [0, k), or null when one of them can't
  // (layerBytes: one size, or since #380 each layer's own costs: costOf, as dealOffload; a plain k * layerBytes was NaN there)
  const C = costOf(layerBytes);
  const at = (k) => {
    const nw = L - k;
    if (k < 1 || nw < 1) return null;
    const hb = C.span(0, k), wb = C.span(k, L);
    const hp = hb <= vh + 1e-6 ? null : offloadPlan(k, vh, off.ram[h], layerBytes, ex, off.minSlots, OFFLOAD_EXPERTS, k, true, true);
    if (hb > vh + 1e-6 && !hp) return null;
    const wp = wb <= vw + 1e-6 ? null : offloadPlan(nw, vw, off.ram[w], layerBytes, ex, off.minSlots, OFFLOAD_EXPERTS, L, !!off.disk?.[w], false);
    if (wb > vw + 1e-6 && !wp) return null;
    return { k, hp, wp, cost: tabCost(k, hp, X) + nodeCost(nw, wp, X) };
  };
  let best = null;
  if (Number.isInteger(off.hostLayers)) best = at(off.hostLayers);
  else for (let k = 1; k < L; k++) { const c = at(k); if (c && (!best || c.cost < best.cost - 1e-9)) best = c; }
  if (!best) return null;
  // the model is coarse (a few ms either way at the 20 GB demo split, where the measured best was dealOffload's own):
  // keep dealOffload's split unless another is modelled at least SPLIT_GAIN cheaper (or the share is forced)
  const was = at(d.assigned[0]);
  if (!Number.isInteger(off.hostLayers) && was && best.cost > was.cost * (1 - SPLIT_GAIN)) return null;
  const assigned = [best.k, L - best.k], ranges = rangesOf(assigned);
  const plan = (p, k) => p && { ...p, lo: ranges[k][1] - p.layers, hi: ranges[k][1], ...(p.diskLayers > 0 ? { diskLo: ranges[k][1] - p.diskLayers } : {}) };
  const offload = [plan(best.hp, 0), plan(best.wp, 1)];
  const held = [offload[0] ? vh : C.span(0, best.k), offload[1] ? vw : C.span(best.k, L)].map((b, k) => b + (d.used[k] === 0 ? hostBytes : 0));
  return { ...d, assigned, ranges, held, offload, speed: { k: best.k, ms: +best.cost.toFixed(1), was: d.assigned[0] } };
}

// What a deal keeps on each device's disk, bytes per device index (host first, as pledges): the routed experts of the
// layers it leaves on disk ([diskLo, hi): expBytes each) and the n-gram table on the device dealt its layer. A room
// node reads them from its own model files; a tab copies them to its disk (room/flashdisk.js), so this is what a tab
// stores. n: how many devices; a device dealt nothing stores 0.
export function diskShares(deal, n, expBytes) {
  const out = new Array(n).fill(0);
  (deal?.used || []).forEach((i, k) => {
    const o = deal.offload?.[k];
    if (o && Number.isInteger(o.diskLo)) out[i] += (o.hi - o.diskLo) * expBytes;
    if (deal.ngram?.at === k) out[i] += deal.ngram.bytes;
  });
  return out;
}
// What this room can run, smallest model first: [{ key, need, ok, short }]. short is how many
// more GB the room needs for that model (0 when it fits).
export function ladder(needGB, pledgedGB) {
  return Object.entries(needGB)
    .sort((a, b) => a[1] - b[1])
    .map(([key, need]) => ({ key, need, ok: pledgedGB >= need, short: Math.max(0, +(need - pledgedGB).toFixed(1)) }));
}

// The largest model that fits the room's pledges, or the smallest one when nothing fits yet.
export function bestFit(needGB, pledgedGB) {
  const l = ladder(needGB, pledgedGB);
  const fits = l.filter((x) => x.ok);
  return (fits.length ? fits[fits.length - 1] : l[0]).key;
}

// Room code from a join link: /r/ABCD, /room/ABCD, ?code=ABCD or #ABCD. Codes use the room's
// 30-letter alphabet (no I, L, O, U, 0, 1); anything else is ignored. Returns "" when absent.
export function codeFromLocation(pathname = "", search = "", hash = "") {
  const ok = (c) => /^[A-HJKMNP-TV-Z2-9]{4,6}$/.test(c) ? c : "";
  const m = /\/(?:r|room)\/([A-Za-z0-9]{4,6})\/?$/.exec(pathname);
  if (m) return ok(m[1].toUpperCase());
  const q = new URLSearchParams(search).get("code");
  if (q) return ok(q.trim().toUpperCase());
  const h = hash.replace(/^#/, "");
  if (h && h !== "debug") return ok(h.toUpperCase());
  return "";
}

// Deal L layers for speed instead of by memory. A token's lap costs every device's compute for
// its layers plus one network hop per device in the chain, so: fill the fastest devices first,
// each up to what it can hold, and leave out devices that are not needed. caps: layers each
// device can hold (host first); msPerLayer: measured compute per layer (null or missing = not
// measured yet: then fewest hops wins, biggest devices first). phone: devices that are phones;
// until a phone is measured it counts as PHONE_COST times slower than an unmeasured computer, so
// computers fill first. The host always keeps at least one layer (it holds the embedding and the
// head anyway). No device gets more than its cap: when the caps cannot hold all L layers (or the
// host's cap is under one layer) nothing is dealt and `short` says how many layers are missing.
// Returns planSplit's shape plus `used`: the device indices that hold layers, in the original order.
export const PHONE_COST = 20;   // one iPhone layer ~ 15-25 GB10 layers (docs/bench-log.md, device matrix)
export function planForSpeed(L, caps, msPerLayer = [], phone = []) {
  caps = caps.map((c) => (Number.isFinite(c) && c > 0 ? c : 0));   // a malformed cap holds nothing
  msPerLayer = msPerLayer || [];
  const n = caps.length;
  const known = msPerLayer.filter((x) => x > 0);
  const fallback = known.length ? Math.max(...known) * 1.5 : 1;   // unmeasured: assume slower than any measured device
  const cost = caps.map((_, i) => msPerLayer[i] > 0 ? msPerLayer[i] : fallback * (phone?.[i] ? PHONE_COST : 1));
  const order = [...caps.keys()].sort((a, b) => cost[a] - cost[b] || (a === 0) * -1 + (b === 0) || caps[b] - caps[a] || a - b);
  const assigned = new Array(n).fill(0);
  assigned[0] = 1;
  let left = L - 1;
  for (const i of order) {
    if (left <= 0) break;
    const take = Math.min(left, Math.max(0, Math.floor(caps[i]) - assigned[i]));
    assigned[i] += take; left -= take;
  }
  // nobody can hold the rest: the room is short. A pledge is a promise, so nothing is spread past
  // anyone's cap (this used to spill the rest over everyone by capacity): the caller must not start
  // and says how short the room is (roomFit). The host needs its one layer within its cap too.
  const short = Math.max(0, left) + (caps[0] < 1 && L > 0 ? 1 : 0);
  if (short || L <= 0) assigned.fill(0);
  return { assigned, ranges: rangesOf(assigned), short, used: assigned.map((a, i) => (a > 0 ? i : -1)).filter((i) => i >= 0) };
}

// A phone: its own flag, or a phone user agent (the meta a device sends when it joins).
export function isPhoneMeta(m) {
  return !!(m?.phone || /^(iPhone|Android)$/.test(m?.ua || ""));
}

// Phones hold layers only when the computers cannot. One layer on a phone costs as much as 15-25
// layers on a desktop GPU plus two Wi-Fi hops per token (docs/bench-log.md, device matrix), so a
// room whose computers can hold the whole model runs faster with its phones as ask-only guests.
// capsLayers: layers each device can hold (host first); phone: which devices are phones. The host
// always counts as a computer (it holds the embedding and the head either way). Returns the
// device indices (never 0) to leave out: every phone when the others hold all L layers, else none.
export function phonesToLeaveOut(L, capsLayers, phone) {
  const others = capsLayers.reduce((s, c, i) => s + (i === 0 || !phone[i] ? Math.max(0, Math.floor(c)) : 0), 0);
  return others >= L ? capsLayers.map((_, i) => i).filter((i) => i > 0 && phone[i]) : [];
}

// The device that becomes the model host when someone presses Start. The model host holds the
// embedding, the head and the draft block, samples every token and runs the Code agent, so it is
// the strongest device, whoever pressed Start: a device with WebGPU first, then a computer before
// a phone (a phone's lent memory says little about its GPU, and a phone tab sleeps with its
// screen), then the most memory lent, then the lowest id so every screen picks the same one.
// Then speed: decode is bound by memory bandwidth and the host's extra work sits on every token's
// path, so a device of the same kind whose GPU copies memory clearly faster (meta.gbps, measured at
// load by room/gpuspeed.js, at least SPEED_EDGE times the memory pick's) and that lends at least
// half as much memory hosts instead: the fastest such device, then memory, then id. A device that
// did not measure (an older tab, or a probe that failed) leaves the pick by memory as it is, and so
// does a device with the same GPU as the memory pick (meta.gpu, e.g. two tabs on one machine or two
// identical machines): the same GPU copies at the same speed, so a gap there is load at probe time.
// devices: [{ id, meta: { webgpu, contribGB, phone, ua, gbps, gpu } }]. Returns an id (null for none).
export const SPEED_EDGE = 1.5;
export function pickModelHost(devices) {
  const gb = (m) => (m?.webgpu ? +m?.contribGB || 0 : 0);
  const key = (d) => [d.meta?.webgpu ? 1 : 0, isPhoneMeta(d.meta) ? 0 : 1, gb(d.meta)];
  // > 0 when a is the better pick by kind, then memory, then the lower id
  const cmp = (a, b) => { const x = key(a), y = key(b); return x[0] - y[0] || x[1] - y[1] || x[2] - y[2] || (a.id < b.id ? 1 : -1); };
  const ds = devices.filter((d) => d?.id);
  let best = null;
  for (const d of ds) if (!best || cmp(d, best) > 0) best = d;
  if (!best) return null;
  const bps = (d) => +d.meta?.gbps || 0, kb = key(best);
  if (!kb[0] || !(bps(best) > 0)) return best.id;
  let fast = null;
  for (const d of ds) {
    const k = key(d);
    if (k[0] !== kb[0] || k[1] !== kb[1] || k[2] < 0.5 * kb[2] || bps(d) < SPEED_EDGE * bps(best)) continue;
    if (d !== best && d.meta?.gpu && d.meta.gpu === best.meta?.gpu) continue;   // same GPU: the gap is noise
    if (!fast || bps(d) > bps(fast) || (bps(d) === bps(fast) && cmp(d, fast) > 0)) fast = d;
  }
  return (fast || best).id;
}

// GB for the screen, rounded up to a tenth so "short by" never undersells what is missing
export const gbUp = (bytes) => Math.ceil(bytes / 2 ** 30 * 10 - 1e-9) / 10;

// The least memory a room's devices must pledge for a model, by the planner's own fit check: the smallest total of
// these pledges, scaled together, at which fitsAt(pledgesScaled) still holds (roomFit, with whatever offload and disk
// tier the devices offer). For a model whose experts stream from disk (room/models.js DISK_COPY) that is the GPU
// memory it needs, far below its size; the picker shows it as the model's need, so its number and its "fits" agree.
// pledges: bytes, one per device; fitsAt(pledges) -> bool. -> { bytes, scaled } (scaled: the pledges at that total),
// or null when no total up to maxScale times these pledges fits (or nobody pledges)
export function minFitPledge(pledges, fitsAt, { maxScale = 16, iters = 24 } = {}) {
  const total = pledges.reduce((a, b) => a + (b > 0 ? b : 0), 0);
  if (!(total > 0)) return null;
  const at = (k) => pledges.map((b) => b * k);
  let lo = 0, hi = 1;
  if (!fitsAt(at(1))) {
    lo = 1;
    while (hi < maxScale && !fitsAt(at(hi * 2))) { lo = hi * 2; hi *= 2; }
    if (hi >= maxScale) return null;
    hi *= 2;
  }
  for (let i = 0; i < iters; i++) { const mid = (lo + hi) / 2; if (fitsAt(at(mid))) hi = mid; else lo = mid; }
  return { bytes: total * hi, scaled: at(hi) };
}

// The one sentence a room that is short for a model shows under Start (and the reason a start stops).
// label: the model's short name; fit: roomFit's result (host first); names: the devices in the same
// order; spareGB: how much more each device could still lend (0 for a phone at its cap). Names up to
// two devices that could close the gap alone, the host's raise first when it is the smaller one. fix: what to do, in
// the words of where it shows (the CLI's "raise a pledge" by default; the page passes PAGE_SHORT_FIX)
export const PAGE_SHORT_FIX = "Invite a device, or give more GPU memory on the right";
export function shortNote(label, fit, names = [], spareGB = [], fix = "Add a device or raise a pledge") {
  if (fit.fits) return "";
  // the pledges hold it, but no device that can take the n-gram table's layer has the disk for it (dealRoom ngram)
  if (fit.ngram) return `${label} needs ${gbUp(fit.ngram.bytes)} GB of free disk for its n-gram table on the device that holds layer ${fit.ngram.layer}, and the devices that could hold that layer don't have it. Free disk space there or add a device that has the room.`;
  const who = givers(fit, spareGB).slice(0, 2).map(({ i, b }) => `${names[i] || "a device"} could give ${gbUp(b)} GB more`);
  const head = `This room is ${gbUp(shortBy(fit, spareGB))} GB short for ${label}`;
  const host = fit.caps[0] < 1 ? `; the model host (${names[0] || "the biggest device"}) needs room for the embedding, the output head and a layer` : "";
  return `${head}${host}. ${fix}${who.length ? `: ${who.join(", or ")}` : ""}.`;
}
// the devices that could close the gap alone, fewest bytes first: [{ i, b }]
const givers = (fit, spareGB) => fit.raise.map((b, i) => ({ i, b })).filter(({ i, b }) => b < Infinity && gbUp(b) <= (spareGB[i] || 0) + 1e-9)
  .sort((a, b) => a.b - b.b || a.i - b.i);
// How short a room is, in bytes, for the screen: what the device that can give the least more would
// have to add (so "0.8 GB short" and "Laptop could give 0.8 GB more" agree), or when no device can,
// the smallest raise anyone would need. 0 when it fits.
export function shortBy(fit, spareGB = []) {
  if (fit.fits) return 0;
  const g = givers(fit, spareGB);
  return g.length ? g[0].b : fit.short;
}

// The whole deal the model host makes at Start, from bytes: which devices hold which layers, within
// every pledge. L layers of layerBytes each (with their KV cache at the room's context); hostBytes
// what the host holds besides its layers (room/models.js hostHeldBytes); pledges: bytes each device
// lends, host first; mode: "speed" (planForSpeed: fastest first, ms per layer where measured) or
// "memory" (planSplit in proportion to pledges, phones left out while the computers can hold the
// model unless phoneLayers); phone: which devices are phones.
// Returns { fit, used: device indices that hold layers (host first, in order), assigned and ranges
// for those devices, held: bytes each device in `used` holds, out: { index: "small" | "unneeded" } }.
// When the pledges cannot hold the model nothing is dealt: fit.fits is false (fit.short, fit.raise).
// off (optional): roomFit's expert offload ({ expertBytes, ram, disk? }). Only when the pledges alone can't hold the
// model does a device offload (dealOffload); the result then also has `offload`: per device in `used`, null or
// { layers, lo, hi, vramBytes, ramBytes } (offloadPlan, with the range of its layers to offload), and with the disk
// tier (only when parking can't hold the model) diskLo (layers [diskLo, hi) stay in the file) and diskRamBytes (T1).
// ngram (optional, Qwen3.8-Flash-Next): the n-gram table's placement (ngramFor, room/pledge.js): the device dealt its
// layer must have the table on its own disk (placeNgram). The result then also has `ngram`.
// layerCost (optional): each layer's own bytes (room/models.js flashLayerCost), in place of layerBytes: every check
// and every range of the deal (caps, offload, the n-gram move, held) is then on those exact costs (costOf).
export function dealRoom({ L, layerBytes, layerCost = null, hostBytes = 0, pledges, mode = "memory", ms = [], phone = [], phoneLayers = false, off = null, ngram = null }) {
  if (layerCost) { layerBytes = layerCost; if (layerCost.length !== L) throw new Error(`dealRoom: ${layerCost.length} layer costs for ${L} layers`); }
  const d = dealLayers({ L, layerBytes, hostBytes, pledges, mode, ms, phone, phoneLayers, off });
  return ngram && d.used.length ? placeNgram(d, ngram, { layerBytes, hostBytes }) : d;
}
function dealLayers({ L, layerBytes, hostBytes, pledges, mode, ms, phone, phoneLayers, off }) {
  const fit = roomFit(L, pledges, layerBytes, hostBytes, off);
  const none = { fit, used: [], assigned: [], ranges: [], held: [], out: {} };
  if (!fit.fits) return none;
  if (fit.offload) {
    const d = dealOffload({ L, layerBytes, hostBytes, pledges, phone, phoneLayers, off, fit });
    return (d && splitBySpeed(d, { L, layerBytes, hostBytes, pledges, off })) || d || { ...none, fit: { ...fit, fits: false } };
  }
  const out = {};
  // a device whose pledge is under one layer holds none (the host always has room: fit says so)
  let idx = fit.caps.map((c, i) => i).filter((i) => i === 0 || fit.caps[i] >= 1);
  fit.caps.forEach((c, i) => { if (i > 0 && c < 1) out[i] = "small"; });
  let assigned, ranges;
  if (mode === "speed") {
    const sp = planForSpeed(L, idx.map((i) => fit.caps[i]), idx.map((i) => ms?.[i]), idx.map((i) => !!phone?.[i]));
    if (sp.short) return none;
    idx.forEach((i, k) => { if (!sp.used.includes(k)) out[i] = "unneeded"; });
    assigned = sp.used.map((k) => sp.assigned[k]); ranges = sp.used.map((k) => sp.ranges[k]);
    idx = sp.used.map((k) => idx[k]);
  } else {
    const off = phoneLayers ? [] : phonesToLeaveOut(L, idx.map((i) => fit.caps[i]), idx.map((i, k) => k > 0 && !!phone?.[i]));
    off.forEach((k) => { out[idx[k]] = "unneeded"; });
    idx = idx.filter((_, k) => !off.includes(k));
    const sp = planSplit(L, idx.map((i) => pledges[i] - (i === 0 ? hostBytes : 0)), idx.map((i) => fit.caps[i]));
    if (sp.short) return none;
    // fewer layers than devices: the ones dealt none are not needed
    const keep = sp.assigned.map((a, k) => (k === 0 || a > 0 ? k : -1)).filter((k) => k >= 0);
    idx.forEach((i, k) => { if (!keep.includes(k)) out[i] = "unneeded"; });
    assigned = keep.map((k) => sp.assigned[k]); ranges = keep.map((k) => sp.ranges[k]);
    idx = keep.map((k) => idx[k]);
  }
  const C = costOf(layerBytes);
  const held = idx.map((i, k) => C.span(ranges[k][0], ranges[k][1]) + (i === 0 ? hostBytes : 0));
  return { fit, used: idx, assigned, ranges, held, out };
}

// The deal for a model host that loads without a load-time check (a room page's tab: room/models.js
// flashTabHostMargin): it keeps `full` past its layers (their upload staging and scratch) while it holds a resident
// layer, `small` when it offloads every layer it holds (their experts are never staged). deal(extra) -> dealRoom's
// result with extra more host bytes. Returns the deal with `full`, or the one with `small` when that has the host
// offloading every layer it holds (so a host that streams all its experts keeps its pool, and one with resident layers
// its staging).
export function dealWithHostMargin(deal, full, small = full) {
  const a = deal(full);
  if (!(small < full)) return a;
  const allOff = (d) => !!(d.fit.fits && d.used.length && d.offload?.[0] && d.offload[0].lo === d.ranges[0][0]);
  if (a.fit.fits && !allOff(a)) return a;
  const b = deal(small);
  return allOff(b) ? b : a;
}

// ---------------- the n-gram table (Qwen3.8-Flash-Next, roadmap/33) ----------------
// Flash's n-gram (PLE) embedding is a 28.8 GB table read a few rows per token (engine/ngram.js NgramTable) by the
// device that runs its layer (layer 1). It never goes to a GPU and is not part of any layer's bytes, so the pledges
// don't pay for it: the deal only has to give that layer to a device with the table on its own disk (a room node's
// model file, or OPFS in a tab). ng: { layer, bytes, disk: [bytes per device, host first: what it can keep the
// table in (the table already there counts as Infinity); null when it did not say] } (room/pledge.js ngramFor).
// A device that did not say is taken at its word (as before the field existed): `checked` is then false.
// When the device dealt the layer can't hold the table, the boundary between the host and the next device moves so
// one that can holds it: the host takes the layer (a device that held only it then holds none), or the next device
// takes every layer of the host's but the first. Only those two, within the pledges (fit.caps) and on a deal
// without offload (an offloading device's plan is made for its range); otherwise the room does not start:
// fit.fits false with fit.ngram = { layer, bytes } (what is missing: a device with that much disk for that layer).
// -> the deal with ngram: { layer, bytes, device (index among the room's devices), at (index in `used`), checked }
export function ngramHolder(ranges, layer) { return ranges.findIndex(([lo, hi]) => layer >= lo && layer < hi); }
export function placeNgram(d, ng, { layerBytes, hostBytes = 0 }) {
  const { layer, bytes } = ng, disk = ng.disk || [];
  const says = (i) => disk[i] != null && !Number.isNaN(+disk[i]);
  const can = (i) => !says(i) || +disk[i] >= bytes;
  const done = (x, k) => ({ ...x, ngram: { layer, bytes, device: x.used[k], at: k, checked: says(x.used[k]) } });
  const k = ngramHolder(d.ranges, layer);
  if (k < 0) return d;   // (no device holds that layer: not a deal this model makes)
  if (can(d.used[k])) return done(d, k);
  const short = { fit: { ...d.fit, fits: false, ngram: { layer, bytes } }, used: [], assigned: [], ranges: [], held: [], out: {} };
  if (d.offload?.some(Boolean) || d.used.length < 2) return short;
  const caps = d.fit.caps, a = d.assigned.slice(), used = d.used.slice(), out = { ...d.out };
  const [lo1] = d.ranges[1];
  if (k === 1 && layer === lo1 && a[1] >= 1 && can(used[0]) && caps[used[0]] >= a[0] + 1) {
    a[0]++; a[1]--;   // the host takes the layer
    if (!a[1]) { out[used[1]] = "unneeded"; used.splice(1, 1); a.splice(1, 1); }
  } else if (k === 0 && layer >= 1 && can(used[1]) && caps[used[1]] >= a[1] + a[0] - 1) {
    a[1] += a[0] - 1; a[0] = 1;   // the next device takes the host's layers past the first
  } else return short;
  const C = costOf(layerBytes), ranges = rangesOf(a), held = used.map((i, j) => C.span(ranges[j][0], ranges[j][1]) + (i === 0 ? hostBytes : 0));
  const x = { ...d, used, assigned: a, ranges, held, out };
  const j = ngramHolder(ranges, layer);
  return j >= 0 && can(used[j]) ? done(x, j) : short;
}
