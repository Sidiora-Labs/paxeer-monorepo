// Vendored from https://github.com/Nehanth/pooled/blob/45b5ee275a8754a37fce4a6ce174f31e4a216ef4/engine/preset.js (MIT; see LICENSE-ENGINE).
// The room's engine settings, in one place. room.js builds every Qwen 3.5/3.6 engine with these, and
// the benchmarks and profilers import the same functions, so a bench number comes from the settings
// the room actually runs (issue #82).
//
// Every switch is read from a query string with the room's own names (?draftvocab=0, ?kv=q8, ...):
// the room passes location.search, the Deno scripts pass ROOM_FLAGS (tests/load_model.js roomFlags),
// the bench pages their own query string. No flags: the room's defaults.
//
//   roomQwen35Options(flags)   options for Qwen35Engine.create (everything but device, weights, range)
//   roomEngineFlags(flags)     engine properties the room sets after create (draft-cache refill etc.)
//   applyRoomFlags(eng, flags) sets those on an engine, returns it

// 16 batch columns: prefill passes go through the row-stationary GEMM (docs/research/prefill-gemm-v2.md).
// Speculative verifies are <= 8 columns and drop to the 8- or 4-column GEMV twins automatically, so the
// generated stream is unchanged.
export const ROOM_BATCH_COLS = 16;
// draft over the first N vocabulary rows (engine/qwen35.js draftVocab): the head is the biggest matrix a
// draft reads, and on English prose and code only 1-2.5% of tokens lie above 65536
// (benchmarks/draftvocab_coverage.js)
export const ROOM_DRAFT_VOCAB = 65536;

const params = (flags) => (flags instanceof URLSearchParams ? flags : new URLSearchParams(flags ?? ""));

export function roomQwen35Options(flags) {
  const q = params(flags), off = (k) => q.get(k) === "0";
  const dv = q.get("draftvocab");
  const gpuSample = !off("gpusample");
  return {
    batchCols: ROOM_BATCH_COLS, coopRowsB: 1,
    // ?draftvocab=N: draft over the first N vocabulary rows only; ?draftvocab=0: full head always. For other
    // scripts (Chinese ~84% above 65536) the engine falls back to the full head by itself (draftVocabAuto);
    // ?dvauto=0: the small head always. Drafts only, the output never changes.
    draftVocab: dv === null ? ROOM_DRAFT_VOCAB : parseInt(dv, 10) || 0,
    draftVocabAuto: !off("dvauto"),
    // the K drafts of a speculative step, its verify and its LM head in one submit; ?draftchain=0 turns it off
    draftChain: !off("draftchain"),
    // ?specfuse=0: speculative verify as separate trunk / head submits (A/B; same output bits)
    specFuse: !off("specfuse"),
    // a chain host's share of each lap in one submit (engine headAhead / _hostTrunkFused); ?hostfuse=0 keeps
    // the separate submits for A/B (same output bits)
    hostFuse: !off("hostfuse"),
    // ?fuse=0: the unfused kernels (attention glue, DeltaNet delta + gated norm, batched attention) for A/B
    // timing; both give the same bits, so devices may differ
    ...(off("fuse") ? { attnGlue: false, dnFuse: false, attnMC: false } : {}),
    // ?layerfuse=0|1: decode layer fusion (engine layerFuse: fewer dispatches per token, same bits) off / on;
    // unset: the engine's per-adapter default (on for NVIDIA, Apple and Deno)
    ...(q.has("layerfuse") ? { layerFuse: q.get("layerfuse") === "0" ? false : q.get("layerfuse") === "1" ? true : q.get("layerfuse") } : {}),
    // ?kv=q8: int8 KV cache (~56% of f16's memory) for long contexts; changes the numerics a little
    kvQ8: q.get("kv") === "q8",
    // ?moefuse=0: the unfused MoE FFN kernels (A/B). The fused path (the default) gives different MoE bits,
    // so every device of a room should run the same setting; ?moednrows=1|2|4 tunes it
    moeFuse: !off("moefuse"),
    moeDnRows: parseInt(q.get("moednrows"), 10) || 1,
    // ?attndecode=v1: decode / verify attention on attn_flash instead of the split-K attn_dec (engine default
    // "v2"; different numerics, so every device of a room should run the same setting)
    ...(q.get("attndecode") ? { attnDecode: q.get("attndecode") } : {}),
    // Prefill options (attnPrefillTile, prefillUbatch, moeGroupPrefill) are deliberately not set: every device
    // takes the engine's defaults, so host and workers agree.
    // GPU sampling, on by default (?gpusample=0: off): argmax / top-k of the head in the same submit.
    // ?argmaxwide=0|1 (default: same as gpusample): the draft argmax as the two-stage multi-workgroup kernel.
    gpuSample,
    argmaxWide: (q.get("argmaxwide") ?? (gpuSample ? "1" : "0")) === "1",
  };
}

export function roomEngineFlags(flags) {
  const q = params(flags), off = (k) => q.get(k) === "0";
  return {
    // ?mtpbatch=0: one draft-cache row per submit, for A/B
    mtpBatchFill: !off("mtpbatch"),
    // after a verify: the draft-cache refill as one batched pass (?mtprefill=0: one submit per row) and the
    // next step's first draft run in that same pass (?predraft=0: off). Drafts only.
    mtpBatchRefill: !off("mtprefill"),
    mtpPreDraft: !off("predraft"),
  };
}

export function applyRoomFlags(eng, flags) {
  if (eng) Object.assign(eng, roomEngineFlags(flags));
  return eng;
}
