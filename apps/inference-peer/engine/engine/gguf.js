// Vendored from https://github.com/Nehanth/pooled/blob/45b5ee275a8754a37fce4a6ce174f31e4a216ef4/engine/gguf.js (MIT; see LICENSE-ENGINE).
// GGUF loading: header and tensor parsing, tokenizer extraction, Q4_0/Q8_0
// repacking into GPU-friendly layouts (packed nibbles + f16 scales), and
// streaming upload straight into GPU buffers. See docs/models.md.
// Parses GGUF v2/v3 and handles Q4_0, Q4_1, Q5_0, Q5_K, Q6_K, Q8_0, F16, BF16
// and F32 tensors, plus the qwen35 and qwen4exp loaders. The parser returns tensor byte
// ranges so browser peers can range-fetch only their layers.

const T_U8 = 0, T_I8 = 1, T_U16 = 2, T_I16 = 3, T_U32 = 4, T_I32 = 5, T_F32 = 6,
  T_BOOL = 7, T_STR = 8, T_ARR = 9, T_U64 = 10, T_I64 = 11, T_F64 = 12;

// ggml tensor types we support
export const GGML_F32 = 0, GGML_F16 = 1, GGML_Q8_0 = 8;
export const QK8_0 = 32;                 // elems per Q8_0 block
export const Q8_0_BLOCK_BYTES = 34;      // f16 scale + 32 int8

const _f16buf = new Float32Array(1), _f16u32 = new Uint32Array(_f16buf.buffer);
export function f32ToF16(v) {
  // IEEE f32 -> f16 bits, round to nearest with ties rounded up (away from
  // zero in magnitude), not ties-to-even like the GPU's pack2x16float. The scratch views are hoisted:
  // this is called once per element of every wire frame and every quantized block.
  _f16buf[0] = v;
  const x = _f16u32[0];
  const sign = (x >>> 16) & 0x8000;
  let e = (x >>> 23) & 0xff, m = x & 0x7fffff;
  if (e === 0xff) return sign | 0x7c00 | (m ? 0x200 : 0);
  e = e - 127 + 15;
  if (e >= 0x1f) return sign | 0x7c00;
  if (e <= 0) {
    if (e < -10) return sign;
    m = (m | 0x800000) >> (1 - e);
    return sign | ((m + 0x1000) >> 13);
  }
  // '+' not '|': a mantissa that rounds up to 0x400 must CARRY into the
  // exponent. With '|' the carry was dropped whenever the exponent's low bit
  // was already set, silently halving ~0.03% of all values (e.g. -1.99999 -> -1).
  return sign | ((e << 10) + ((m + 0x1000) >> 13));
}

export function f16ToF32(h) {
  const s = (h & 0x8000) ? -1 : 1, e = (h >> 10) & 0x1f, m = h & 0x3ff;
  if (e === 0) return s * m * 2 ** -24;
  if (e === 31) return m ? NaN : s * Infinity;
  return s * (1 + m / 1024) * 2 ** (e - 15);
}

// Parse the GGUF header from an ArrayBuffer that contains AT LEAST the full
// header (metadata + tensor infos). Tensor data may be absent (range-fetch).
// opts.skipTokenizer: walk past tokenizer.* arrays without building 500k JS
// strings (100+ MB of heap a worker shard never uses; phones cannot afford it)
export function parseGGUFHeader(buf, opts = {}) {
  const dv = new DataView(buf);
  let off = 0;
  const u32 = () => { const v = dv.getUint32(off, true); off += 4; return v; };
  const u64 = () => { const v = Number(dv.getBigUint64(off, true)); off += 8; return v; };
  const i64 = () => { const v = Number(dv.getBigInt64(off, true)); off += 8; return v; };
  const f32v = () => { const v = dv.getFloat32(off, true); off += 4; return v; };
  const f64v = () => { const v = dv.getFloat64(off, true); off += 8; return v; };
  const str = (skip) => {
    const n = u64();
    const s = skip ? undefined : new TextDecoder().decode(new Uint8Array(buf, off, n));
    off += n;
    return s;
  };
  const value = (t, skip = false) => {
    switch (t) {
      case T_U8: return dv.getUint8(off++);
      case T_I8: return dv.getInt8(off++);
      case T_U16: { const v = dv.getUint16(off, true); off += 2; return v; }
      case T_I16: { const v = dv.getInt16(off, true); off += 2; return v; }
      case T_U32: return u32();
      case T_I32: { const v = dv.getInt32(off, true); off += 4; return v; }
      case T_F32: return f32v();
      case T_BOOL: return !!dv.getUint8(off++);
      case T_STR: return str(skip);
      case T_U64: return u64();
      case T_I64: return i64();
      case T_F64: return f64v();
      case T_ARR: {
        const et = u32(), n = u64(), out = skip ? undefined : new Array(n);
        for (let i = 0; i < n; i++) { const v = value(et, skip); if (!skip) out[i] = v; }
        return out;
      }
      default: throw new Error("bad gguf value type " + t);
    }
  };

  if (u32() !== 0x46554747) throw new Error("not a GGUF file");
  const version = u32();
  if (version < 2) throw new Error("gguf v1 unsupported");
  const nTensors = u64();
  const nKV = u64();
  const meta = {};
  for (let i = 0; i < nKV; i++) {
    const key = str();
    const skip = !!opts.skipTokenizer && key.startsWith("tokenizer.");
    const v = value(u32(), skip);
    if (!skip) meta[key] = v;
  }
  const infos = [];
  for (let i = 0; i < nTensors; i++) {
    const name = str();
    const nd = u32();
    const dims = [];
    for (let d = 0; d < nd; d++) dims.push(u64());
    const ggmlType = u32();
    const offset = u64();
    // ggml dims are [ne0(=innermost/cols), ne1(rows), ...]; torch order is reversed
    infos.push({ name, shape: dims.slice().reverse(), ggmlType, offset });
  }
  const align = meta["general.alignment"] || 32;
  const dataStart = Math.ceil(off / align) * align;
  const tensors = {};
  for (const t of infos) {
    const n = t.shape.reduce((a, b) => a * b, 1);
    const bytes = ggmlTypeBytes(t.ggmlType, n); // -1 for unsupported types
    tensors[t.name] = { ...t, nElems: n, byteOffset: dataStart + t.offset, byteLength: bytes };
  }
  // the Qwen3.5+ MoE files use their own architecture name (keys like "qwen35moe.block_count");
  // alias them under "qwen35." so the one engine reads both, dense and MoE. Qwen3.8-Flash-Next ("qwen4exp")
  // shares the Qwen3.5 keys too (block_count, ssm.*, expert_*, attention.*); its own keys (hyper_connection.*,
  // attention.indexer.*, ple.*) are aliased along with them but read under their own names (qwen4expConfig).
  const arch = meta["general.architecture"];
  if (typeof arch === "string" && arch !== "qwen35" && (/^qwen3[5-9]/.test(arch) || arch === "qwen4exp"))
    for (const k of Object.keys(meta)) if (k.startsWith(arch + ".")) { const a = "qwen35." + k.slice(arch.length + 1); if (!(a in meta)) meta[a] = meta[k]; }
  return { meta, tensors, dataStart, headerBytes: off };
}

// Dequant a tensor's raw bytes to Float32Array.
export function ggufToF32(info, bytes /* Uint8Array of the tensor's data */) {
  const n = info.nElems;
  if (info.ggmlType === GGML_F32)
    return new Float32Array(bytes.buffer, bytes.byteOffset, n);
  if (info.ggmlType === GGML_F16) {
    const u16 = new Uint16Array(bytes.buffer, bytes.byteOffset, n);
    const out = new Float32Array(n);
    for (let i = 0; i < n; i++) out[i] = f16ToF32(u16[i]);
    return out;
  }
  if (info.ggmlType === GGML_Q8_0) {
    const out = new Float32Array(n);
    const nb = n / QK8_0;
    for (let b = 0; b < nb; b++) {
      const base = b * Q8_0_BLOCK_BYTES;
      const scale = f16ToF32(bytes[base] | (bytes[base + 1] << 8));
      for (let i = 0; i < QK8_0; i++) {
        const q = bytes[base + 2 + i];
        out[b * QK8_0 + i] = scale * (q > 127 ? q - 256 : q);
      }
    }
    return out;
  }
  throw new Error("unsupported ggml type " + info.ggmlType);
}

// Repack Q8_0 for the GPU: {qs: Int8Array (as Uint32-aligned buffer), scales: Float32Array}
export function q8Repack(info, bytes) {
  const n = info.nElems;
  const nb = n / QK8_0;
  const qs = new Uint8Array(n);
  const scales = new Uint32Array(Math.ceil(nb / 2));   // raw f16 scales, 2 per word
  q8RepackInto(bytes, qs, new Uint16Array(scales.buffer), 0, nb);
  return { qs, scales };
}
// q8Repack's loop for blocks [b0, b1) of `bytes` (the tensor's file bytes from its start) into qs (n bytes) and sc16
// (a u16 per block), each from the tensor's start: what a worker thread runs on its share of the blocks
// (packages/room-node/convert.js, all three in shared memory)
export function q8RepackInto(bytes, qs, sc16, b0, b1) {
  if (((bytes.byteOffset | qs.byteOffset) & 1) === 0) {
    // 34-byte blocks keep 2-byte alignment: move u16 words (3x faster than subarray+set per block)
    const src = new Uint16Array(bytes.buffer, bytes.byteOffset, b1 * 17), dst = new Uint16Array(qs.buffer, qs.byteOffset, b1 * 16);
    for (let b = b0, s = b0 * 17, d = b0 * 16; b < b1; b++, s += 17, d += 16) {
      sc16[b] = src[s];
      for (let j = 0; j < 16; j++) dst[d + j] = src[s + 1 + j];
    }
    return;
  }
  for (let b = b0; b < b1; b++) {
    const base = b * Q8_0_BLOCK_BYTES;
    sc16[b] = bytes[base] | (bytes[base + 1] << 8);
    qs.set(bytes.subarray(base + 2, base + 2 + QK8_0), b * QK8_0);
  }
}

// map ggml tensor names -> the engine's internal names for llama/qwen-family models
export function ggmlLayerNames(i) {
  const p = `blk.${i}.`;
  return {
    inNorm: p + "attn_norm.weight",
    q: p + "attn_q.weight", k: p + "attn_k.weight", v: p + "attn_v.weight",
    o: p + "attn_output.weight",
    qNorm: p + "attn_q_norm.weight", kNorm: p + "attn_k_norm.weight", // qwen3 only
    postNorm: p + "ffn_norm.weight",
    gate: p + "ffn_gate.weight", up: p + "ffn_up.weight", down: p + "ffn_down.weight",
  };
}
export const GGML_EMBED = "token_embd.weight";
export const GGML_FINAL_NORM = "output_norm.weight";
export const GGML_OUTPUT = "output.weight"; // absent when embeddings are tied
export const GGML_PLE_TABLE = "per_layer_token_embd.weight"; // qwen4exp's n-gram table: rows only, never an entry

// One tensor's engine entry (repacked for the GPU) from a parsed GGUF header.
// bytesOf: async (info) => Uint8Array of that tensor's data (local slice or
// HTTP range fetch, same contract as the safetensors shard path).
// cpuOnly: never through G.streamEntry (an offloaded expert tensor is parked by the caller, not uploaded)
export async function ggufEntry(G, bytesOf, name, optional, onBytes = () => {}, cpuOnly = false) {
  // Qwen3.8-Flash-Next's n-gram table (28.8 GB) is read 16 rows per token, never converted or uploaded whole
  if (name === GGML_PLE_TABLE) throw new Error(name + " is read by rows (qwen4expPleTable), not loaded as a weight");
  const info0 = G.tensors[name];
  if (!info0) {
    if (optional) return null;
    throw new Error("missing tensor " + name);
  }
  // stacked MoE experts [nExp][dOut][dIn] are an ordinary matrix of nExp * dOut rows
  const info = info0.shape.length === 3 ? { ...info0, shape: [info0.shape[0] * info0.shape[1], info0.shape[2]] } : info0;
  // converted-weights cache (tests/weight_cache.js in Deno/Node, room/convertedcache.js in the browser): a hit returns the exact
  // bytes the conversion below would produce, without reading or converting the tensor
  if (G.entryCache) {
    const hit = await G.entryCache.get(info);
    if (hit) { onBytes(info.byteLength); return hit; }
  }
  // the embedding stays on the CPU too (per-token row lookups), so it takes the normal path
  if (G.streamEntry && !cpuOnly && name !== GGML_EMBED && info.shape.length === 2 && streamable(info.ggmlType)) {
    const e = await G.streamEntry(info);
    if (e) { onBytes(info.byteLength); return e; }
  }
  const bytes = await bytesOf(info);
  onBytes(info.byteLength);
  // G.convert: the same conversion elsewhere (packages/room-node/convert.js: worker threads, off the event loop)
  const e = G.convert ? await G.convert(info, bytes) : convertEntry(info, bytes);
  if (G.entryCache) await G.entryCache.put(info, e);
  return e;
}

// The CPU conversion from a tensor's GGUF bytes to the entry the engine uploads: Q4_0/Q8_0
// repacked (nibbles/int8 + split f16 scales), other quants requantized to Q8, the rest f32.
// Pure and deterministic: the weight cache stores exactly what this returns.
export function convertEntry(info, bytes) {
  if (info.shape.length === 2) {
    if (info.ggmlType === GGML_Q8_0) {
      const { qs, scales } = q8Repack(info, bytes);
      return { kind: "q8", qs, scales, shape: info.shape };
    }
    if (info.ggmlType === GGML_Q4_0) {
      const { qs, scales } = q4Repack(info, bytes);
      return { kind: "q4", qs, scales, shape: info.shape };
    }
    if (info.ggmlType !== GGML_F32 && info.ggmlType !== GGML_F16 && info.ggmlType !== GGML_BF16) {
      const { qs, scales } = requantQ8Streaming(info, bytes);
      return { kind: "q8", qs, scales, shape: info.shape };
    }
  }
  return { kind: "f32", data: dequantF32(info, bytes) };
}

// K-quant -> Q8 without materializing the whole tensor in f32 (a 27B lm_head
// would be a 5 GB allocation, which browser tabs refuse). Chunks of rows.
export function requantQ8Streaming(info, bytes) {
  const [rows, cols] = info.shape;
  const rowBytes = ggmlTypeBytes(info.ggmlType, cols);
  const n = rows * cols;
  const qs = new Uint8Array(n);
  const scales = new Uint32Array(Math.ceil(n / 32 / 2));
  const sc16 = new Uint16Array(scales.buffer);
  if (info.ggmlType === GGML_Q4_1 && n % 32 === 0) { q41ToQ8(bytes, n / 32, qs, sc16); return { qs, scales }; }
  const CH = Math.max(1, Math.floor((8 * 2 ** 20) / (cols * 4))); // ~8MB f32 per chunk
  for (let r0 = 0; r0 < rows; r0 += CH) {
    const rc = Math.min(CH, rows - r0);
    const sub = { ggmlType: info.ggmlType, nElems: rc * cols, shape: [rc, cols] };
    const view = bytes.subarray(r0 * rowBytes, (r0 + rc) * rowBytes);
    const f = dequantF32(sub, view);
    const q = quantizeQ8(f);
    qs.set(q.qs, r0 * cols);
    const nbChunk = (rc * cols) / 32;
    sc16.set(new Uint16Array(q.scales.buffer, 0, nbChunk), (r0 * cols) / 32);
  }
  return { qs, scales };
}

// Q4_1 -> Q8 in one pass, byte for byte what dequantF32 + quantizeQ8 give (tests/unit/quant_test.js checks it): a
// Q4_1 block has only 16 distinct values, fround(d * k + m) for k = 0..15 (dequantF32's f32 store), so each is
// divided and rounded once instead of 32 times, and nothing goes through an f32 array. Flash's down experts are Q4_1:
// the old path converted them at ~62 MB/s on one core (GB10), the main thread's wall in a tab's first save.
let _f16tab = null;   // every f16 bit pattern -> its value (f16ToF32's, exact in an f64)
const _q41v = new Float64Array(16), _q41q = new Uint8Array(16);
function q41ToQ8(bytes, nb, qs, sc16) {
  const T = _f16tab ||= Float64Array.from({ length: 65536 }, (_, h) => f16ToF32(h));
  const v = _q41v, qv = _q41q;
  for (let b = 0; b < nb; b++) {
    const base = b * 20, o = b * 32;
    const d = T[bytes[base] | (bytes[base + 1] << 8)], m = T[bytes[base + 2] | (bytes[base + 3] << 8)];
    if (!Number.isFinite(d) || !Number.isFinite(m)) {   // an inf / NaN scale: the reference path, for its NaN rules
      const q = quantizeQ8(dequantF32({ ggmlType: GGML_Q4_1, nElems: 32 }, bytes.subarray(base, base + 20)));
      qs.set(q.qs, o); sc16[b] = q.scales[0] & 0xFFFF;
      continue;
    }
    for (let k = 0; k < 16; k++) v[k] = Math.fround(d * k + m);
    // the largest |value| among the nibbles used (finite here: no NaN to propagate)
    let used = 0;
    for (let l = 0; l < 16; l++) { const q = bytes[base + 4 + l]; used |= (1 << (q & 0xF)) | (1 << (q >> 4)); }
    let amax = 0;
    for (let k = 0; k < 16; k++) if (used & (1 << k)) { const a = Math.abs(v[k]); if (a > amax) amax = a; }
    const h = f32ToF16(amax / 127 || 1);
    sc16[b] = h;
    const dd = T[h] || 1;
    for (let k = 0; k < 16; k++) { const r = Math.round(v[k] / dd); qv[k] = (r > 127 ? 127 : r < -127 ? -127 : r) & 0xFF; }
    for (let l = 0; l < 16; l++) {
      const q = bytes[base + 4 + l];
      qs[o + l] = qv[q & 0xF];
      qs[o + l + 16] = qv[q >> 4];
    }
  }
}

export async function ggufWeights(G, bytesOf, { lo, hi, hasEmbed, hasHead }, onProgress = () => {}, onEntry = null) {
  let fetched = 0;
  const entry = async (name, optional) => {
    const e = await ggufEntry(G, bytesOf, name, optional, (b) => { fetched += b; onProgress(fetched); });
    if (e && onEntry) onEntry(e, name);
    return e;
  };
  const layers = [];
  for (let i = lo; i < hi; i++) {
    const N = ggmlLayerNames(i);
    layers.push({
      inNorm: await entry(N.inNorm), postNorm: await entry(N.postNorm),
      q: await entry(N.q), k: await entry(N.k), v: await entry(N.v), o: await entry(N.o),
      qNorm: await entry(N.qNorm, true), kNorm: await entry(N.kNorm, true),
      gate: await entry(N.gate), up: await entry(N.up), down: await entry(N.down),
    });
  }
  const out = { layers };
  if (hasEmbed || hasHead) out.embed = await entry(GGML_EMBED);
  if (hasHead) {
    out.finalNorm = await entry(GGML_FINAL_NORM);
    out.head = await entry(GGML_OUTPUT, true); // null when tied
  }
  return out;
}

// Total bytes a shard will download (for progress bars / pledge checks)
export function ggufShardBytes(G, { lo, hi, hasEmbed, hasHead }) {
  let total = 0;
  const add = (n) => { if (G.tensors[n]) total += G.tensors[n].byteLength; };
  for (let i = lo; i < hi; i++) Object.values(ggmlLayerNames(i)).forEach(add);
  if (hasEmbed || hasHead) add(GGML_EMBED);
  if (hasHead) { add(GGML_FINAL_NORM); add(GGML_OUTPUT); }
  return total;
}

// ---------- extended quant formats (Qwen3.8 / Q4_0 file family) ----------
export const GGML_Q4_0 = 2, GGML_Q4_1 = 3, GGML_Q5_0 = 6, GGML_Q5_K = 13, GGML_Q6_K = 14, GGML_BF16 = 30;
export const QK_K = 256;

export function ggmlTypeBytes(type, n) {
  switch (type) {
    case GGML_F32: return n * 4;
    case GGML_F16: return n * 2;
    case GGML_Q4_0: return (n / 32) * 18;
    case GGML_Q4_1: return (n / 32) * 20;
    case GGML_Q5_0: return (n / 32) * 22;
    case GGML_BF16: return n * 2;
    case GGML_Q8_0: return (n / 32) * 34;
    case GGML_Q5_K: return (n / QK_K) * 176;
    case GGML_Q6_K: return (n / QK_K) * 210;
    default: return -1;
  }
}

function getScaleMinK4(j, scales) {
  // ported from ggml get_scale_min_k4
  if (j < 4) return [scales[j] & 63, scales[j + 4] & 63];
  return [
    (scales[j + 4] & 0xF) | ((scales[j - 4] >> 6) << 4),
    (scales[j + 4] >> 4) | ((scales[j] >> 6) << 4),
  ];
}

// Dequantize any supported ggml type to Float32Array (CPU).
export function dequantF32(info, bytes) {
  const n = info.nElems, T = info.ggmlType;
  if (T === GGML_F32 || T === GGML_F16 || T === GGML_Q8_0) return ggufToF32(info, bytes);
  const out = new Float32Array(n);
  if (T === GGML_BF16) {   // bf16 is the top half of an f32: exact
    const u = new Uint32Array(out.buffer);
    for (let i = 0; i < n; i++) u[i] = (bytes[2 * i] | (bytes[2 * i + 1] << 8)) << 16;
    return out;
  }
  if (T === GGML_Q5_0) {   // ggml dequantize_row_q5_0: d f16, qh u32 (5th bits), 16 bytes of nibbles
    const nb = n / 32;
    for (let b = 0; b < nb; b++) {
      const base = b * 22, d = f16ToF32(bytes[base] | (bytes[base + 1] << 8));
      const qh = (bytes[base + 2] | (bytes[base + 3] << 8) | (bytes[base + 4] << 16) | (bytes[base + 5] << 24)) >>> 0;
      for (let j = 0; j < 16; j++) {
        const q = bytes[base + 6 + j], h0 = ((qh >>> j) << 4) & 0x10, h1 = (qh >>> (j + 12)) & 0x10;
        out[b * 32 + j] = d * (((q & 0xF) | h0) - 16);
        out[b * 32 + j + 16] = d * (((q >> 4) | h1) - 16);
      }
    }
    return out;
  }
  if (T === GGML_Q4_0) {
    const nb = n / 32;
    for (let b = 0; b < nb; b++) {
      const base = b * 18;
      const d = f16ToF32(bytes[base] | (bytes[base + 1] << 8));
      for (let l = 0; l < 16; l++) {
        const q = bytes[base + 2 + l];
        out[b * 32 + l] = d * ((q & 0xF) - 8);
        out[b * 32 + l + 16] = d * ((q >> 4) - 8);
      }
    }
    return out;
  }
  if (T === GGML_Q4_1) {
    const nb = n / 32;
    for (let b = 0; b < nb; b++) {
      const base = b * 20;
      const d = f16ToF32(bytes[base] | (bytes[base + 1] << 8));
      const m = f16ToF32(bytes[base + 2] | (bytes[base + 3] << 8));
      for (let l = 0; l < 16; l++) {
        const q = bytes[base + 4 + l];
        out[b * 32 + l] = d * (q & 0xF) + m;
        out[b * 32 + l + 16] = d * (q >> 4) + m;
      }
    }
    return out;
  }
  if (T === GGML_Q5_K) {
    const nb = n / QK_K;
    for (let i = 0; i < nb; i++) {
      const base = i * 176;
      const d = f16ToF32(bytes[base] | (bytes[base + 1] << 8));
      const min = f16ToF32(bytes[base + 2] | (bytes[base + 3] << 8));
      const scales = bytes.subarray(base + 4, base + 16);
      const qh = bytes.subarray(base + 16, base + 48);
      let ql = base + 48;
      let y = i * QK_K, is = 0, u1 = 1, u2 = 2;
      for (let j = 0; j < QK_K; j += 64) {
        const [sc1, m1q] = getScaleMinK4(is + 0, scales);
        const [sc2, m2q] = getScaleMinK4(is + 1, scales);
        const d1 = d * sc1, m1 = min * m1q;
        const d2 = d * sc2, m2 = min * m2q;
        for (let l = 0; l < 32; l++) out[y++] = d1 * ((bytes[ql + l] & 0xF) + (qh[l] & u1 ? 16 : 0)) - m1;
        for (let l = 0; l < 32; l++) out[y++] = d2 * ((bytes[ql + l] >> 4) + (qh[l] & u2 ? 16 : 0)) - m2;
        ql += 32; is += 2; u1 <<= 2; u2 <<= 2;
      }
    }
    return out;
  }
  if (T === GGML_Q6_K) {
    const nb = n / QK_K;
    for (let i = 0; i < nb; i++) {
      const base = i * 210;
      const d = f16ToF32(bytes[base + 208] | (bytes[base + 209] << 8));
      let ql = base, qh = base + 128, sc = base + 192;
      let y = i * QK_K;
      for (let nblk = 0; nblk < QK_K; nblk += 128) {
        for (let l = 0; l < 32; l++) {
          const is = (l / 16) | 0;
          const s = (v) => v > 127 ? v - 256 : v; // int8 scales
          const q1 = ((bytes[ql + l] & 0xF) | (((bytes[qh + l] >> 0) & 3) << 4)) - 32;
          const q2 = ((bytes[ql + l + 32] & 0xF) | (((bytes[qh + l] >> 2) & 3) << 4)) - 32;
          const q3 = ((bytes[ql + l] >> 4) | (((bytes[qh + l] >> 4) & 3) << 4)) - 32;
          const q4 = ((bytes[ql + l + 32] >> 4) | (((bytes[qh + l] >> 6) & 3) << 4)) - 32;
          out[y + l] = d * s(bytes[sc + is]) * q1;
          out[y + l + 32] = d * s(bytes[sc + is + 2]) * q2;
          out[y + l + 64] = d * s(bytes[sc + is + 4]) * q3;
          out[y + l + 96] = d * s(bytes[sc + is + 6]) * q4;
        }
        y += 128; ql += 64; qh += 32; sc += 8;
      }
    }
    return out;
  }
  throw new Error("dequantF32: unsupported type " + T);
}

// Tokenizer straight from GGUF metadata (tokens + merges arrays); returns a
// tokenizer.json-shaped object to pass to makeTokenizer.
export function tokenizerFromGGUF(meta) {
  const tokens = meta["tokenizer.ggml.tokens"];
  const merges = meta["tokenizer.ggml.merges"];
  const tj = {
    model: {
      vocab: Object.fromEntries(tokens.map((t, i) => [t, i])),
      merges,
    },
    // which pre-tokenizer split the model uses (qwen2, qwen35, ...); see preSplitter in tokenizer.js
    pre: meta["tokenizer.ggml.pre"],
  };
  return tj; // caller passes through makeTokenizer-compatible builder
}

// Repack Q4_0 for the GPU: nibbles stay packed (16 bytes/block), scales split out.
export function q4Repack(info, bytes) {
  const n = info.nElems;
  const nb = n / 32;
  const qs = new Uint8Array(n / 2);
  const scales = new Uint32Array(Math.ceil(nb / 2));   // raw f16 scales, 2 per word
  q4RepackInto(bytes, qs, new Uint16Array(scales.buffer), 0, nb);
  return { qs, scales };
}
// q4Repack's loop for blocks [b0, b1) (as q8RepackInto: qs holds 16 bytes a block)
export function q4RepackInto(bytes, qs, sc16, b0, b1) {
  if (((bytes.byteOffset | qs.byteOffset) & 1) === 0) {
    // 18-byte blocks keep 2-byte alignment: move u16 words (3x faster than subarray+set per block;
    // this loop is most of a cold 27B load)
    const src = new Uint16Array(bytes.buffer, bytes.byteOffset, b1 * 9), dst = new Uint16Array(qs.buffer, qs.byteOffset, b1 * 8);
    for (let b = b0, s = b0 * 9, d = b0 * 8; b < b1; b++, s += 9, d += 8) {
      sc16[b] = src[s];
      dst[d] = src[s + 1]; dst[d + 1] = src[s + 2]; dst[d + 2] = src[s + 3]; dst[d + 3] = src[s + 4];
      dst[d + 4] = src[s + 5]; dst[d + 5] = src[s + 6]; dst[d + 6] = src[s + 7]; dst[d + 7] = src[s + 8];
    }
    return;
  }
  for (let b = b0; b < b1; b++) {
    const base = b * 18;
    sc16[b] = bytes[base] | (bytes[base + 1] << 8);
    qs.set(bytes.subarray(base + 2, base + 18), b * 16);
  }
}

// Quantize f32 -> Q8_0-style GPU entry (for K-quant tensors we don't have GPU
// kernels for: dequant on CPU, requant to q8 — tiny extra error, big memory win)
export function quantizeQ8(data) {
  const n = data.length;
  const nb = Math.ceil(n / 32);
  const qs = new Uint8Array(nb * 32);
  const scales = new Uint32Array(Math.ceil(nb / 2));   // packed f16, like the GGUF format itself
  const sc16 = new Uint16Array(scales.buffer);
  for (let b = 0; b < nb; b++) {
    let amax = 0;
    for (let i = b * 32; i < Math.min(n, b * 32 + 32); i++) amax = Math.max(amax, Math.abs(data[i]));
    const h = f32ToF16(amax / 127 || 1);
    sc16[b] = h;
    const d = f16ToF32(h) || 1;
    for (let i = b * 32; i < Math.min(n, b * 32 + 32); i++)
      qs[i] = Math.max(-127, Math.min(127, Math.round(data[i] / d))) & 0xFF;
  }
  return { qs, scales };
}


// ---------- qwen3.5/3.8 (hybrid delta-net) shard loader ----------
export function qwen35LayerNames(i, forceFull = false, moe = false, interval = 4) {
  const p = `blk.${i}.`;
  const isFull = forceFull || i % interval === interval - 1;
  const shared = {
    attnNorm: p + "attn_norm.weight",
    postNorm: p + "post_attention_norm.weight",
    ...(moe ? {
      // routed experts (stacked [nExp][dOut][dIn]) + router, and the always-on shared expert
      router: p + "ffn_gate_inp.weight",
      expGate: p + "ffn_gate_exps.weight", expUp: p + "ffn_up_exps.weight", expDown: p + "ffn_down_exps.weight",
      shGate: p + "ffn_gate_shexp.weight", shUp: p + "ffn_up_shexp.weight", shDown: p + "ffn_down_shexp.weight",
      shRouter: p + "ffn_gate_inp_shexp.weight",
    } : { ffnGate: p + "ffn_gate.weight", ffnUp: p + "ffn_up.weight", ffnDown: p + "ffn_down.weight" }),
  };
  if (isFull) return { ...shared, isFull,
    wq: p + "attn_q.weight", wk: p + "attn_k.weight", wv: p + "attn_v.weight",
    wo: p + "attn_output.weight",
    qNorm: p + "attn_q_norm.weight", kNorm: p + "attn_k_norm.weight" };
  return { ...shared, isFull,
    wqkv: p + "attn_qkv.weight", wz: p + "attn_gate.weight",
    wBeta: p + "ssm_beta.weight", wAlpha: p + "ssm_alpha.weight",
    dtBias: p + "ssm_dt.bias", ssmA: p + "ssm_a",
    conv: p + "ssm_conv1d.weight", ssmNorm: p + "ssm_norm.weight",
    wOut: p + "ssm_out.weight" };
}

// experts: an ExpertStore (engine/expert_store.js): the routed experts of the layers it holds (experts.has(i)) are
// parked in its buffers instead of returned for upload; the layer then carries its placeholder entries.
export async function qwen35Weights(G, bytesOf, { lo, hi, hasEmbed, hasHead, mtp = false, experts = null }, onProgress = () => {}, onEntry = null) {
  let fetched = 0;
  const entry = async (name, optional) => {
    const e = await ggufEntry(G, bytesOf, name, optional, (b) => { fetched += b; onProgress(fetched); });
    if (e && onEntry) onEntry(e, name);
    return e;
  };
  const moe = (G.meta["qwen35.expert_count"] || 0) > 0;
  const interval = G.meta["qwen35.full_attention_interval"] || 4;
  const loadLayer = async (i, forceFull = false) => {
    const N = qwen35LayerNames(i, forceFull, moe, interval);
    const L = { isFull: N.isFull,
      attnNorm: await entry(N.attnNorm), postNorm: await entry(N.postNorm) };
    if (moe) {
      L.moe = true;
      L.router = await entry(N.router);
      if (experts && !forceFull && experts.has(i)) {   // offloaded: converted on the CPU, parked, never uploaded
        const nExp = G.meta["qwen35.expert_count"];
        for (const [k, part] of [["expGate", "gate"], ["expUp", "up"], ["expDown", "down"]]) {
          if (experts.isDisk?.(i)) {   // a disk layer: nothing read now, its experts are read from the file when used
            if (!G.tensors[N[k]]) throw new Error("missing tensor " + N[k]);
            L[k] = experts.attach(i, part, { name: N[k], ...G.tensors[N[k]] }, nExp);
          } else L[k] = experts.park(i, part, await ggufEntry(G, bytesOf, N[k], false, (b) => { fetched += b; onProgress(fetched); }, true), nExp);
        }
      } else { L.expGate = await entry(N.expGate); L.expUp = await entry(N.expUp); L.expDown = await entry(N.expDown); }
      L.shGate = await entry(N.shGate, true); L.shUp = await entry(N.shUp, true); L.shDown = await entry(N.shDown, true);
      L.shRouter = await entry(N.shRouter, true);
    } else {
      L.ffnGate = await entry(N.ffnGate); L.ffnUp = await entry(N.ffnUp); L.ffnDown = await entry(N.ffnDown);
    }
    if (N.isFull) {
      L.wq = await entry(N.wq); L.wk = await entry(N.wk); L.wv = await entry(N.wv);
      L.wo = await entry(N.wo);
      L.qNorm = await entry(N.qNorm); L.kNorm = await entry(N.kNorm);
    } else {
      L.wqkv = await entry(N.wqkv); L.wz = await entry(N.wz);
      L.wBeta = await entry(N.wBeta); L.wAlpha = await entry(N.wAlpha);
      L.dtBias = await entry(N.dtBias); L.ssmA = await entry(N.ssmA);
      L.conv = await entry(N.conv); L.ssmNorm = await entry(N.ssmNorm);
      L.wOut = await entry(N.wOut);
    }
    return L;
  };
  const layers = [];
  for (let i = lo; i < hi; i++) layers.push(await loadLayer(i));
  const out = { layers };
  if (hasEmbed) out.embed = await entry(GGML_EMBED);
  if (hasHead) {
    if (!out.embed) out.embed = await entry(GGML_EMBED);
    out.finalNorm = await entry(GGML_FINAL_NORM);
    out.head = await entry(GGML_OUTPUT, true);
  }
  // multi-token-prediction block (the "nextn" layer after the trunk): a normal
  // full-attention layer plus eh_proj / enorm / hnorm / shared_head_norm.
  if (mtp && hasHead) {
    const N = G.meta["qwen35.block_count"] - 1;
    const p = `blk.${N}.nextn.`;
    if (G.tensors[p + "eh_proj.weight"]) {
      out.mtp = {
        layer: await loadLayer(N, true),
        ehProj: await entry(p + "eh_proj.weight"),
        enorm: await entry(p + "enorm.weight"),
        hnorm: await entry(p + "hnorm.weight"),
        sharedHeadNorm: await entry(p + "shared_head_norm.weight"),
      };
    }
  }
  return out;
}

// qwen35LayerNames for this file (dense or MoE, its full-attention interval)
export function qwen35NamesFor(G, i, forceFull = false) {
  return qwen35LayerNames(i, forceFull, (G.meta["qwen35.expert_count"] || 0) > 0, G.meta["qwen35.full_attention_interval"] || 4);
}
// diskLayers: layers whose routed experts stay in the file (expert offload's disk tier): not counted
export function qwen35ShardBytes(G, { lo, hi, hasEmbed, hasHead, mtp = false, diskLayers = null }) {
  let total = 0;
  const add = (n) => { if (G.tensors[n]) total += G.tensors[n].byteLength; };
  for (let i = lo; i < hi; i++) {
    const N = qwen35NamesFor(G, i), skip = diskLayers?.includes?.(i) || diskLayers?.has?.(i) ? new Set([N.expGate, N.expUp, N.expDown]) : null;
    Object.values(N).forEach((v) => { if (typeof v === "string" && !skip?.has(v)) add(v); });
  }
  if (hasEmbed || hasHead) add(GGML_EMBED);
  if (hasHead) { add(GGML_FINAL_NORM); add(GGML_OUTPUT); }
  if (mtp && hasHead) total += qwen35MtpBytes(G);
  return total;
}
// bytes of the multi-token-prediction block (host only)
export function qwen35MtpBytes(G) {
  const N = G.meta["qwen35.block_count"] - 1;
  const p = `blk.${N}.nextn.`;
  if (!G.tensors[p + "eh_proj.weight"]) return 0;
  let total = 0;
  const add = (n) => { if (G.tensors[n]) total += G.tensors[n].byteLength; };
  Object.values(qwen35NamesFor(G, N, true)).forEach((v) => { if (typeof v === "string") add(v); });
  ["eh_proj", "enorm", "hnorm", "shared_head_norm"].forEach((n) => add(p + n + ".weight"));
  return total;
}


// ---------- Qwen3.8-Flash-Next (llama.cpp arch "qwen4exp") loader ----------
// Not a Qwen3.5 variant: every RMSNorm around the blocks is replaced by a 4-stream hyper-connection (HC) mixer, the
// full-attention layers carry a sparse-attention indexer (QSA), layer 1 adds a gated n-gram embedding (PLE) read from
// a 28.8 GB hash table, and there is no output norm (the last HC mixer feeds the head). The MTP block ships in a
// separate GGUF. llama.cpp's port is the spec: src/models/qwen4exp.cpp (load_arch_tensors, ggml-org master f2cc728).

// The model's shape from a qwen4exp header's metadata. The keys llama.cpp requires are required here too (a missing
// one throws); qsa[i] covers the trunk and the MTP block. -> { L (trunk layers), nextn (MTP blocks in this file),
// dim, hc, hcRank, hDim (the residual's width: hc streams of dim), qsa, ssm, nExp, K, expFF, shFF, indexer, ple }
export function qwen4expConfig(meta) {
  if (meta["general.architecture"] !== "qwen4exp") throw new Error(`not a qwen4exp GGUF (${meta["general.architecture"]})`);
  const key = (k, required = true) => {
    const v = meta["qwen4exp." + k];
    if (v === undefined && required) throw new Error("qwen4exp: missing key qwen4exp." + k);
    return v;
  };
  const all = key("block_count"), nextn = key("nextn_predict_layers", false) || 0, L = all - nextn;
  const dim = key("embedding_length"), hc = key("hyper_connection.count"), hcRank = key("hyper_connection.low_rank");
  // a per-layer key is one value for every layer or an array of exactly one per layer (llama.cpp get_key_or_arr)
  const perLayer = (k, v) => {
    if (!Array.isArray(v)) return Array(all).fill(v);
    if (v.length !== all) throw new Error(`qwen4exp: ${v.length} ${k} for ${all} layers`);
    return v;
  };
  // the layer types as llama.cpp sets them (qwen4exp.cpp:148-156): attention.recurrent_layers when the file has it,
  // else every interval-th trunk layer is full attention (QSA), and so is every MTP block (i >= L)
  const recurrent = key("attention.recurrent_layers", false), interval = key("full_attention_interval", false) ?? 4;
  if (interval === 0) throw new Error("qwen4exp: full_attention_interval is 0");
  const qsa = recurrent !== undefined ? perLayer("recurrent layer flags", recurrent).map((r) => !r)
    : Array.from({ length: all }, (_, i) => i >= L || (i + 1) % interval === 0);
  // the indexer pools its keys in blocks of the compress ratio, one for the whole model (qwen4exp.cpp:62-79): the
  // QSA layers' shared ratio, 0 when the file has none
  const topK = key("attention.indexer.top_k");
  let block = 0;
  for (const r of perLayer("compress ratios", key("attention.compress_ratios", false) ?? 0)) {
    if (!r) continue;
    if (block && r !== block) throw new Error(`qwen4exp: QSA layers must share one compress ratio, got ${block} and ${r}`);
    block = r;
  }
  if (block === 1 || (block && topK % block)) throw new Error(`qwen4exp: compress ratio ${block} does not divide top_k ${topK}`);
  const ssm = { convK: key("ssm.conv_kernel"), stateSize: key("ssm.state_size"), nKHeads: key("ssm.group_count"),
    nVHeads: key("ssm.time_step_rank"), inner: key("ssm.inner_size") };
  let ple = null;
  const pleLayers = key("ple.layers", false);
  if (pleLayers?.length) {
    const ngram = key("ple.ngram_size"), headsPerNgram = key("ple.heads_per_ngram"), nHeads = (ngram - 1) * headsPerNgram;
    const headDim = key("embedding_length_per_layer_input");
    const multipliers = key("ple.layer_multipliers"), offsets = key("ple.head_offsets"), vocab = key("ple.head_vocab_sizes");
    if (multipliers.length !== ngram || offsets.length !== nHeads || vocab.length !== nHeads)
      throw new Error(`qwen4exp: PLE arrays of ${multipliers.length}/${offsets.length}/${vocab.length} for ${ngram}-grams, ${nHeads} heads`);
    // the hash is 64-bit integer math (the multipliers are 45-bit): one rounded multiplier would move every row.
    // The header parser reads u64 as Number, exact below 2^53; past that, refuse rather than hash wrong rows
    if (!multipliers.every(Number.isSafeInteger)) throw new Error("qwen4exp: a PLE multiplier past 2^53");
    if (nHeads * headDim !== dim) throw new Error(`qwen4exp: ${nHeads} PLE heads of ${headDim} make ${nHeads * headDim}, not ${dim}`);
    for (const l of pleLayers) if (qsa[l]) throw new Error(`qwen4exp: PLE layer ${l} is not a DeltaNet layer`);
    ple = { layers: pleLayers, ngram, headsPerNgram, nHeads, headDim, convK: key("ple.conv_kernel"),
      eos: key("ple.eos_token_id"),
      // files written before the key (or with 0) hash image positions as EOS (qwen4exp.cpp:1230)
      image: key("ple.image_token_id", false) || key("ple.eos_token_id"),
      multipliers: multipliers.map(BigInt), offsets, vocab,
      rows: Math.max(...offsets.map((o, h) => o + vocab[h])) };   // the rows the heads index (the table pads past it)
  }
  return {
    L, nextn, dim, hc, hcRank, hDim: hc * dim, eps: key("attention.layer_norm_rms_epsilon"),
    nHeads: key("attention.head_count"), nKVHeads: key("attention.head_count_kv"), headDim: key("attention.key_length"),
    ropeDims: key("rope.dimension_count"), ropeTheta: key("rope.freq_base"), ropeSections: key("rope.dimension_sections"),
    qsa, ssm, nExp: key("expert_count"), K: key("expert_used_count"),
    expFF: key("expert_feed_forward_length"), shFF: key("expert_shared_feed_forward_length"),
    indexer: { nHeads: key("attention.indexer.head_count"), headDim: key("attention.indexer.key_length"),
      topK, block },
    ple,
  };
}

// ggml tensor names -> engine names for one qwen4exp layer. The MoE, attention and DeltaNet names match
// qwen35LayerNames; there are no attn_norm / post_attention_norm. Instead: two HC mixers (before the token mixer,
// before the MoE), the QSA indexer on full-attention layers, and the PLE block on the n-gram layer.
export function qwen4expLayerNames(i, { qsa = false, ple = false } = {}) {
  const p = `blk.${i}.`;
  const N = { isFull: qsa, isPle: ple,
    hcAttnNorm: p + "hc_attn_norm.weight", hcAttnDown: p + "hc_attn_down.weight",
    hcAttnUp: p + "hc_attn_up.weight", hcAttnInject: p + "hc_attn_inject.weight",
    hcFfnNorm: p + "hc_ffn_norm.weight", hcFfnDown: p + "hc_ffn_down.weight",
    hcFfnUp: p + "hc_ffn_up.weight", hcFfnInject: p + "hc_ffn_inject.weight",
    router: p + "ffn_gate_inp.weight",
    expGate: p + "ffn_gate_exps.weight", expUp: p + "ffn_up_exps.weight", expDown: p + "ffn_down_exps.weight",
    shGate: p + "ffn_gate_shexp.weight", shUp: p + "ffn_up_shexp.weight", shDown: p + "ffn_down_shexp.weight",
    shRouter: p + "ffn_gate_inp_shexp.weight",
  };
  if (qsa) Object.assign(N, {
    wq: p + "attn_q.weight", wk: p + "attn_k.weight", wv: p + "attn_v.weight", wo: p + "attn_output.weight",
    qNorm: p + "attn_q_norm.weight", kNorm: p + "attn_k_norm.weight",
    idxQ: p + "indexer.q_proj.weight", idxK: p + "indexer.k_proj.weight",
    idxQNorm: p + "indexer.q_norm.weight", idxKNorm: p + "indexer.k_norm.weight" });
  else Object.assign(N, {
    wqkv: p + "attn_qkv.weight", wz: p + "attn_gate.weight",
    wBeta: p + "ssm_beta.weight", wAlpha: p + "ssm_alpha.weight",
    dtBias: p + "ssm_dt.bias", ssmA: p + "ssm_a",
    conv: p + "ssm_conv1d.weight", ssmNorm: p + "ssm_norm.weight",
    wOut: p + "ssm_out.weight" });
  if (ple) Object.assign(N, {
    pleKey: p + "ple_key.weight", pleValue: p + "ple_value.weight",
    pleNormKey: p + "ple_norm_key.weight", pleNormQuery: p + "ple_norm_query.weight",
    pleNormConv: p + "ple_norm_conv.weight", pleConv: p + "ple_conv1d.weight" });
  return N;
}
// qwen4expLayerNames for layer i of this file (cfg: qwen4expConfig(G.meta), passed when the caller has it)
export function qwen4expNamesFor(G, i, cfg = qwen4expConfig(G.meta)) {
  return qwen4expLayerNames(i, { qsa: cfg.qsa[i], ple: !!cfg.ple?.layers.includes(i) });
}
// the tensor names of a layer qwen4expWeights loads (qsa: false leaves out the indexer)
const QWEN4EXP_INDEXER = new Set(["idxQ", "idxK", "idxQNorm", "idxKNorm"]);
function qwen4expLoadedNames(N, qsa = true) {
  return Object.entries(N).filter(([k, v]) => typeof v === "string" && (qsa || !QWEN4EXP_INDEXER.has(k))).map(([, v]) => v);
}
// the head: the last HC mixer (no inject) stands in for the output norm, then the LM head
export const QWEN4EXP_HEAD = { hcNorm: "output_hc_norm.weight", hcDown: "output_hc_down.weight", hcUp: "output_hc_up.weight" };
// the MTP block's own tensors besides its layer (blk.<L>.nextn.*): eh_proj is [fc_embedding | fc_hidden], hnorm and
// the head mixer are per HC stream
export function qwen4expMtpNames(L) {
  const p = `blk.${L}.nextn.`;
  return { ehProj: p + "eh_proj.weight", enorm: p + "enorm.weight", hnorm: p + "hnorm.weight",
    hcHeadNorm: p + "hc_head_norm.weight", hcHeadDown: p + "hc_head_down.weight", hcHeadUp: p + "hc_head_up.weight" };
}

// Where the n-gram table is, for a row reader (it is never an entry: ggufEntry refuses it). Each of the ple.nHeads
// hash heads owns rows [firstRow, firstRow + rows); a row is rowElems values of ggmlType, rowBytes long.
// shard: which split file holds it (room/models.js mergeSplitHeaders; the caller maps it to that file's URL). -> null
// for a file without one
export function qwen4expPleTable(G, cfg = qwen4expConfig(G.meta)) {
  const t = G.tensors[GGML_PLE_TABLE];
  if (!t || !cfg.ple) return null;
  const [rows, rowElems] = t.shape;
  if (rowElems !== cfg.ple.headDim || rows < cfg.ple.rows)
    throw new Error(`${GGML_PLE_TABLE}: ${rows} rows of ${rowElems}, the heads need ${cfg.ple.rows} of ${cfg.ple.headDim}`);
  return { name: GGML_PLE_TABLE, shard: t.shard || 0, byteOffset: t.byteOffset, byteLength: t.byteLength,
    ggmlType: t.ggmlType, rows, rowElems, rowBytes: ggmlTypeBytes(t.ggmlType, rowElems),
    heads: cfg.ple.offsets.map((firstRow, h) => ({ firstRow, rows: cfg.ple.vocab[h] })) };
}

// The qwen4exp shard loader (qwen35Weights' contract). A layer has qwen35Weights' MoE and token-mixer fields plus
// hcAttn / hcFfn ({ norm, down, up, inject }), indexer ({ q, k, qNorm, kNorm }) on QSA layers and ple ({ key, value,
// normKey, normQuery, normConv, conv }) on the n-gram layer. The head is headHC ({ norm, down, up }) and head; there
// is no finalNorm. pleTable (qwen4expPleTable) is returned with the n-gram layer, for the row reader.
// mtp: the MTP file's parsed header, read through mtpBytesOf; loaded with the head as out.mtp = { layer, ehProj,
// enorm, hnorm, hcHead }. Its tensors go through ggufEntry with the MTP header's own hooks, never G's:
// mtp.entryCache (the converted-weights cache is keyed per file, and token_embd / output exist in both files) and
// mtp.streamEntry (G's reads G's file) are unset unless the caller sets them for the MTP file; mtp.convert defaults
// to G.convert, which only converts bytes. ggml-org's MTP file also carries its own token_embd and output (Q4_0; the trunk's head is
// Q6_K); llama.cpp loading that file as its own model uses them, so mtpOwnHead loads them as mtp.embed / mtp.head.
// experts: an ExpertStore, as in qwen35Weights (its layers' routed experts are parked, not returned for upload). The
// MTP block's experts are parked too when the store has its layer (cfg.L), unlike qwen35Weights, which never parks it.
// qsa: false skips the indexer tensors (layer.indexer = null), for the dense phase that does not run QSA.
export async function qwen4expWeights(G, bytesOf, { lo, hi, hasEmbed, hasHead, mtp = null, mtpBytesOf = null, mtpOwnHead = false,
  experts = null, qsa = true }, onProgress = () => {}, onEntry = null) {
  let fetched = 0;
  const progress = (b) => { fetched += b; onProgress(fetched); };
  const entryOf = (H, read) => async (name, optional) => {
    const e = await ggufEntry(H, read, name, optional, progress);
    if (e && onEntry) onEntry(e, name);
    return e;
  };
  const entry = entryOf(G, bytesOf);
  const cfg = qwen4expConfig(G.meta);
  const loadLayer = async (H, read, N, i, mayPark) => {
    const get = H === G ? entry : entryOf(H, read);
    const group = async (o) => { const r = {}; for (const [k, n] of Object.entries(o)) r[k] = await get(n); return r; };
    const L = { isFull: N.isFull, moe: true,
      hcAttn: await group({ norm: N.hcAttnNorm, down: N.hcAttnDown, up: N.hcAttnUp, inject: N.hcAttnInject }) };
    if (N.isFull) {
      L.wq = await get(N.wq); L.wk = await get(N.wk); L.wv = await get(N.wv); L.wo = await get(N.wo);
      L.qNorm = await get(N.qNorm); L.kNorm = await get(N.kNorm);
      L.indexer = qsa ? await group({ q: N.idxQ, k: N.idxK, qNorm: N.idxQNorm, kNorm: N.idxKNorm }) : null;
    } else {
      L.wqkv = await get(N.wqkv); L.wz = await get(N.wz);
      L.wBeta = await get(N.wBeta); L.wAlpha = await get(N.wAlpha);
      L.dtBias = await get(N.dtBias); L.ssmA = await get(N.ssmA);
      L.conv = await get(N.conv); L.ssmNorm = await get(N.ssmNorm);
      L.wOut = await get(N.wOut);
    }
    if (N.isPle) L.ple = await group({ key: N.pleKey, value: N.pleValue, normKey: N.pleNormKey,
      normQuery: N.pleNormQuery, normConv: N.pleNormConv, conv: N.pleConv });
    L.hcFfn = await group({ norm: N.hcFfnNorm, down: N.hcFfnDown, up: N.hcFfnUp, inject: N.hcFfnInject });
    L.router = await get(N.router);
    if (mayPark && experts && experts.has(i)) {   // offloaded: converted on the CPU, parked, never uploaded
      for (const [k, part] of [["expGate", "gate"], ["expUp", "up"], ["expDown", "down"]]) {
        if (experts.isDisk?.(i)) {   // a disk layer (roadmap 32's disk tier, a store that has one): read when used
          if (!H.tensors[N[k]]) throw new Error("missing tensor " + N[k]);
          L[k] = experts.attach(i, part, { name: N[k], ...H.tensors[N[k]] }, cfg.nExp);
        } else L[k] = experts.park(i, part, await ggufEntry(H, read, N[k], false, progress, true), cfg.nExp);
      }
    } else { L.expGate = await get(N.expGate); L.expUp = await get(N.expUp); L.expDown = await get(N.expDown); }
    L.shGate = await get(N.shGate); L.shUp = await get(N.shUp); L.shDown = await get(N.shDown);
    L.shRouter = await get(N.shRouter);
    return L;
  };
  const layers = [];
  for (let i = lo; i < hi; i++) layers.push(await loadLayer(G, bytesOf, qwen4expNamesFor(G, i, cfg), i, true));
  const out = { layers };
  if (cfg.ple && cfg.ple.layers.some((l) => l >= lo && l < hi)) out.pleTable = qwen4expPleTable(G, cfg);
  if (hasEmbed || hasHead) out.embed = await entry(GGML_EMBED);
  if (hasHead) {
    out.headHC = { norm: await entry(QWEN4EXP_HEAD.hcNorm), down: await entry(QWEN4EXP_HEAD.hcDown), up: await entry(QWEN4EXP_HEAD.hcUp) };
    out.head = await entry(GGML_OUTPUT, true);   // null: tied to the embedding (llama.cpp's fallback; no file does it)
  }
  if (mtp && hasHead) {
    const mc = qwen4expConfig(mtp.meta);
    if (!mc.nextn) throw new Error("the MTP GGUF has no nextn block");
    if (mc.dim !== cfg.dim || mc.hc !== cfg.hc || mc.nExp !== cfg.nExp || mc.L !== cfg.L)
      throw new Error("the MTP GGUF does not match the model");
    const Hm = { meta: mtp.meta, tensors: mtp.tensors, entryCache: mtp.entryCache || null, streamEntry: mtp.streamEntry || null,
      convert: mtp.convert || G.convert || null };
    const read = mtpBytesOf || bytesOf, get = entryOf(Hm, read), M = qwen4expMtpNames(mc.L);
    out.mtp = {
      layer: await loadLayer(Hm, read, qwen4expNamesFor(mtp, mc.L, mc), mc.L, true),
      ehProj: await get(M.ehProj), enorm: await get(M.enorm), hnorm: await get(M.hnorm),
      hcHead: { norm: await get(M.hcHeadNorm), down: await get(M.hcHeadDown), up: await get(M.hcHeadUp) },
    };
    if (mtpOwnHead) { out.mtp.embed = await get(GGML_EMBED); out.mtp.head = await get(GGML_OUTPUT); }
  }
  return out;
}

// File bytes qwen4expWeights reads for a shard (progress bars, pledges). The n-gram table is never part of it.
// mtp: the MTP file's parsed header, counted with the head as qwen4expWeights loads it. qsa: false leaves out the
// indexer, as qwen4expWeights does.
export function qwen4expShardBytes(G, { lo, hi, hasEmbed, hasHead, mtp = null, mtpOwnHead = false, qsa = true }) {
  const cfg = qwen4expConfig(G.meta);
  let total = 0;
  const add = (n) => { if (G.tensors[n]) total += G.tensors[n].byteLength; };
  for (let i = lo; i < hi; i++) qwen4expLoadedNames(qwen4expNamesFor(G, i, cfg), qsa).forEach(add);
  if (hasEmbed || hasHead) add(GGML_EMBED);
  if (hasHead) { Object.values(QWEN4EXP_HEAD).forEach(add); add(GGML_OUTPUT); }
  if (mtp && hasHead) total += qwen4expMtpBytes(mtp, { ownHead: mtpOwnHead, qsa });
  return total;
}
// bytes of the MTP file's block (its layer and nextn tensors; ownHead: its token_embd and output too; qsa: false
// leaves out the indexer)
export function qwen4expMtpBytes(Gm, { ownHead = false, qsa = true } = {}) {
  const mc = qwen4expConfig(Gm.meta);
  if (!mc.nextn) return 0;
  let total = 0;
  const add = (n) => { if (Gm.tensors[n]) total += Gm.tensors[n].byteLength; };
  qwen4expLoadedNames(qwen4expNamesFor(Gm, mc.L, mc), qsa).forEach(add);
  Object.values(qwen4expMtpNames(mc.L)).forEach(add);
  if (ownHead) { add(GGML_EMBED); add(GGML_OUTPUT); }
  return total;
}

// Bytes of the entry convertEntry makes from a tensor (what the engine holds, vs. byteLength, the file's): Q4_0
// stays 4-bit, Q8_0 and every other quant matrix become Q8 (Q4_1, Q5_0, Q6_K widen), floats and vectors become f32.
// Stacked experts count as the one matrix ggufEntry makes of them.
export function entryByteLength(info) {
  const n = info.nElems ?? info.shape.reduce((a, b) => a * b, 1), T = info.ggmlType;
  const scales = Math.ceil(n / 32 / 2) * 4;   // f16 scales, two per u32 word
  if (info.shape.length < 2 || T === GGML_F32 || T === GGML_F16 || T === GGML_BF16) return n * 4;
  return (T === GGML_Q4_0 ? n / 2 : n) + scales;
}


// Upload one weight entry to the GPU right away and drop its CPU copy. Used on
// memory-starved devices (phones): peak RAM becomes one tensor instead of the
// whole shard. Engines pick up `e.gpu` instead of re-uploading.
export function gpuUploadEntry(device, e, keepCpu = false) {
  if (e.gpu) return e.gpu;
  // only the big quantized matrices; f32 tensors (norms, conv, biases) are tiny and the
  // engines read their CPU copy directly when building layers
  if (e.kind !== "q8" && e.kind !== "q4") return null;
  const mk = (data, usage) => {
    // writeBuffer in slices: no full-size mapped staging copy (matters on iOS,
    // where the tab's memory budget is a fraction of the device's)
    const src = ArrayBuffer.isView(data) ? new Uint8Array(data.buffer, data.byteOffset, data.byteLength) : new Uint8Array(data);
    const size = Math.ceil(src.byteLength / 4) * 4;
    const buf = device.createBuffer({ size, usage: usage | GPUBufferUsage.COPY_DST });
    const STEP = 16 * 2 ** 20;
    for (let o = 0; o < src.byteLength; o += STEP) {
      const n = Math.min(STEP, src.byteLength - o);
      const n4 = Math.ceil(n / 4) * 4;
      if (n4 === n) device.queue.writeBuffer(buf, o, src, o, n);
      else { const tail = new Uint8Array(n4); tail.set(src.subarray(o, o + n)); device.queue.writeBuffer(buf, o, tail); }
    }
    return buf;
  };
  if (e.kind === "q8" || e.kind === "q4")
    // COPY_SRC: engines may row-concatenate projections on the GPU at load (Qwen35Engine fuseProj), and
    // the MoE engine repacks the shared expert's gate/up on the GPU (engine/qwen35.js packGU)
    e.gpu = { kind: e.kind, qs: mk(e.qs, GPUBufferUsage.STORAGE | GPUBufferUsage.COPY_SRC), sc: mk(e.scales, GPUBufferUsage.STORAGE | GPUBufferUsage.COPY_SRC) };
  else e.gpu = { kind: "f32", buf: mk(e.data, GPUBufferUsage.STORAGE | GPUBufferUsage.COPY_SRC) };
  if (!keepCpu) e.qs = e.scales = e.data = null;
  return e.gpu;
}


// Types streamEntryToGPU takes: Q4_0 / Q8_0 are repacked as they arrive, the other quants
// (Q4_1, Q5_0, Q5_K, Q6_K) are requantized to Q8 a few rows at a time (streamRequantToGPU).
// F32 / F16 / BF16 stay on the CPU path: they are the small tensors (norms, biases, conv).
export const streamable = (t) => t === GGML_Q8_0 || t === GGML_Q4_0 || t === GGML_Q4_1 || t === GGML_Q5_0 || t === GGML_Q5_K || t === GGML_Q6_K;

// Staging memory for the streamed uploads, reused from tensor to tensor (a shard streams hundreds of
// tensors one after another: without this each one left a few MB of garbage behind). A stream
// borrows one and gives it back when done; concurrent streams simply get their own.
const stagePool = [];
function borrowStage(bytes) {
  const i = stagePool.findIndex((b) => b.byteLength >= bytes);
  return i >= 0 ? stagePool.splice(i, 1)[0] : new ArrayBuffer(bytes);
}
function giveStage(buf) { if (stagePool.length < 2) stagePool.push(buf); }

// Stream a Q4_0 / Q8_0 tensor straight from the network into GPU buffers.
// Nothing tensor-sized ever exists in JS: chunks arrive, whole blocks are
// repacked into a small reused staging area and written out, the rest waits
// for the next chunk. Peak CPU memory ~ one network chunk + staging (a few
// MB) instead of 3x the tensor. This is what keeps an iPhone tab alive.
export async function streamEntryToGPU(device, info, openRange, { pace = 0, staging = 4 * 2 ** 20 } = {}) {
  if (info.ggmlType !== GGML_Q4_0 && info.ggmlType !== GGML_Q8_0) return streamRequantToGPU(device, info, openRange, { pace, staging });
  const q4 = info.ggmlType === GGML_Q4_0;
  const BLK = q4 ? 18 : Q8_0_BLOCK_BYTES;      // bytes per block in the file
  const QSB = q4 ? 16 : QK8_0;                 // quant bytes per block on the GPU
  const nb = info.nElems / 32;
  device.pushErrorScope("out-of-memory");
  const qsBuf = device.createBuffer({ size: Math.ceil(nb * QSB / 4) * 4, usage: GPUBufferUsage.STORAGE | GPUBufferUsage.COPY_DST | GPUBufferUsage.COPY_SRC });
  const scBuf = device.createBuffer({ size: Math.ceil(nb / 2) * 4, usage: GPUBufferUsage.STORAGE | GPUBufferUsage.COPY_DST | GPUBufferUsage.COPY_SRC });
  const oom = await device.popErrorScope();
  if (oom) throw new Error(`GPU out of memory while allocating ${info.name} (${(info.byteLength / 2 ** 20).toFixed(0)} MB): this device pledged more than its GPU can hold`);
  const blocksPerFlush = Math.max(1, Math.floor(staging / QSB));
  const stage = borrowStage(blocksPerFlush * (QSB + 2)), scAt = blocksPerFlush * QSB;
  try {
  const qsStage = new Uint8Array(stage, 0, scAt), qsStage16 = new Uint16Array(stage, 0, scAt / 2);
  const scStage = new Uint16Array(stage, scAt, blocksPerFlush);   // raw f16 scales
  let staged = 0, block = 0;
  const flush = async () => {
    if (!staged) return;
    const first = block - staged;
    // byte views only: WebKit and Chrome disagree on element-vs-byte sizes for typed arrays
    device.queue.writeBuffer(qsBuf, first * QSB, stage, 0, staged * QSB);
    device.queue.writeBuffer(scBuf, first * 2, stage, scAt, staged * 2);
    staged = 0;
    await new Promise((r) => setTimeout(r, 0));      // let WebKit hand the copy to the GPU process before we make more
  };
  const r = await openRange(info);
  if (!r.ok && r.status !== 206) throw new Error("range fetch failed for " + info.name);
  const reader = r.body.getReader();
  let carry = new Uint8Array(0);
  for (;;) {
    const { value, done } = await reader.read();
    if (done) break;
    let buf = value;
    if (carry.length) { const m = new Uint8Array(carry.length + value.length); m.set(carry); m.set(value, carry.length); buf = m; carry = new Uint8Array(0); }
    const whole = Math.floor(buf.length / BLK);
    // blocks are an even number of bytes: when the chunk starts 2-aligned, move u16 words
    const src16 = (buf.byteOffset & 1) === 0 ? new Uint16Array(buf.buffer, buf.byteOffset, whole * BLK / 2) : null;
    for (let b = 0; b < whole; b++) {
      const base = b * BLK;
      if (src16) {
        const s = base >> 1, d = staged * QSB / 2;
        scStage[staged] = src16[s];
        for (let j = 0; j < QSB / 2; j++) qsStage16[d + j] = src16[s + 1 + j];
      } else {
        scStage[staged] = buf[base] | (buf[base + 1] << 8);
        qsStage.set(buf.subarray(base + 2, base + BLK), staged * QSB);
      }
      staged++; block++;
      if (staged === blocksPerFlush) await flush();
    }
    const rest = buf.length - whole * BLK;
    if (rest) carry = buf.slice(whole * BLK);
  }
  await flush();
  if (block !== nb) throw new Error(`short tensor ${info.name}: ${block}/${nb} blocks`);
  } finally { giveStage(stage); }
  if (pace) await new Promise((r) => setTimeout(r, pace));   // give the allocator time to return pages
  return { kind: q4 ? "q4" : "q8", shape: info.shape, gpu: { kind: q4 ? "q4" : "q8", qs: qsBuf, sc: scBuf } };
}

// A K-quant (or Q4_1 / Q5_0) tensor, requantized to Q8 on its way to the GPU. The CPU path
// (convertEntry -> requantQ8Streaming) holds the whole tensor's bytes plus the whole Q8 output
// (a MoE expert-down tensor: 160 MB in, 256 MiB out, both alive at once) and this is what killed
// iPhone tabs on layers 0-4 (#207). Here only one chunk of rows is ever in JS: the rows arrive,
// are dequantized and quantized exactly as requantQ8Streaming does (Q8 blocks are 32 elements
// within a row, so the row chunking never changes a bit), and are written to the GPU buffers.
export async function streamRequantToGPU(device, info, openRange, { pace = 0, staging = 4 * 2 ** 20 } = {}) {
  const [rows, cols] = info.shape;
  const rowBytes = ggmlTypeBytes(info.ggmlType, cols);
  if (!(rowBytes > 0) || cols % 32) throw new Error(`can't stream ${info.name}: type ${info.ggmlType}, ${cols} columns`);
  const n = rows * cols, nb = n / 32;
  device.pushErrorScope("out-of-memory");
  const qsBuf = device.createBuffer({ size: Math.ceil(n / 4) * 4, usage: GPUBufferUsage.STORAGE | GPUBufferUsage.COPY_DST | GPUBufferUsage.COPY_SRC });
  const scBuf = device.createBuffer({ size: Math.ceil(nb / 2) * 4, usage: GPUBufferUsage.STORAGE | GPUBufferUsage.COPY_DST | GPUBufferUsage.COPY_SRC });
  const oom = await device.popErrorScope();
  if (oom) throw new Error(`GPU out of memory while allocating ${info.name} (${(info.byteLength / 2 ** 20).toFixed(0)} MB): this device pledged more than its GPU can hold`);
  // rows per chunk: about `staging` bytes of f32 while converting, an even count so every chunk's
  // scales start on a 4-byte boundary of the scale buffer
  const CH = Math.max(2, Math.floor(staging / (cols * 4)) & ~1);
  const stage = borrowStage(CH * rowBytes);
  const rowBuf = new Uint8Array(stage, 0, CH * rowBytes);   // reused for every chunk
  let fill = 0, row = 0;
  try {
  const flush = async () => {
    const rc = fill / rowBytes;
    if (!rc) return;
    const sub = { ggmlType: info.ggmlType, nElems: rc * cols, shape: [rc, cols] };
    const q = quantizeQ8(dequantF32(sub, rowBuf.subarray(0, fill)));
    device.queue.writeBuffer(qsBuf, row * cols, q.qs.buffer, 0, rc * cols);
    const scBytes = (rc * cols / 32) * 2;           // odd block count (the last chunk): q.scales is padded to a word
    device.queue.writeBuffer(scBuf, (row * cols / 32) * 2, q.scales.buffer, 0, Math.ceil(scBytes / 4) * 4);
    row += rc; fill = 0;
    await new Promise((r) => setTimeout(r, 0));
  };
  const r = await openRange(info);
  if (!r.ok && r.status !== 206) throw new Error("range fetch failed for " + info.name);
  const reader = r.body.getReader();
  let got = 0;
  for (;;) {
    const { value, done } = await reader.read();
    if (done) break;
    for (let o = 0; o < value.length;) {
      const take = Math.min(value.length - o, rowBuf.length - fill, info.byteLength - got);
      if (take <= 0) break;
      rowBuf.set(value.subarray(o, o + take), fill);
      fill += take; o += take; got += take;
      if (fill === rowBuf.length) await flush();
    }
  }
  if (got !== info.byteLength || fill % rowBytes) throw new Error(`short tensor ${info.name}: ${got}/${info.byteLength} bytes`);
  await flush();
  } finally { giveStage(stage); }
  if (pace) await new Promise((r) => setTimeout(r, pace));
  return { kind: "q8", shape: info.shape, gpu: { kind: "q8", qs: qsBuf, sc: scBuf } };
}
