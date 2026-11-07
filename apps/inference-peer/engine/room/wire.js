// Vendored from https://github.com/Nehanth/pooled/blob/45b5ee275a8754a37fce4a6ce174f31e4a216ef4/room/wire.js (MIT; see LICENSE-ENGINE).
// Wire format for activations between peers: f16 packing, binary/base64 frames, NaN checks.
import { f32ToF16, f16ToF32 } from "../engine/gguf.js";

// Any NaN or Inf anywhere. (It used to sample every 97th element; the 27B's residual stream has
// ~3 "massive" channels, so an Inf there slipped through ~96% of the time, and WGSL's finite-math
// assumption can turn it into finite garbage one hop later.) x - x is 0 only for finite x.
export function badF32(a) {
  for (let i = 0; i < a.length; i++) if (a[i] - a[i] !== 0) return true;
  return false;
}

export function f32ToB64(f) {
  const u = new Uint8Array(f.buffer, f.byteOffset, f.byteLength);
  let s = "";
  for (let i = 0; i < u.length; i += 0x8000) s += String.fromCharCode.apply(null, u.subarray(i, i + 0x8000));
  return btoa(s);
}

export const WIRE_F16 = true;

// Largest |x| of the last frame packed (telemetry: how close each boundary gets to f16's range).
export const wireStats = { lastMax: 0 };
// Beyond this an activation would become Inf in f16 (65504 is the largest finite value).
export const F16_MAX = 65504;
export class WireRangeError extends Error {}

export function packF16(f) {
  const out = new Uint16Array(f.length);
  let mx = 0;
  for (let i = 0; i < f.length; i++) {
    const v = f[i];
    const a = v < 0 ? -v : v;
    if (a > mx) mx = a;
    out[i] = f32ToF16(v);
  }
  wireStats.lastMax = mx;
  // fail loudly instead of sending Inf, which downstream kernels may turn into silent garbage
  if (!(mx <= F16_MAX)) throw new WireRangeError(`an activation reached ${mx.toExponential(2)}, beyond f16's range (${F16_MAX}): this model cannot be split at this boundary with the f16 wire`);
  return out;
}

// f16 -> f32 through a 65,536-entry table built from f16ToF32 itself (exact by construction):
// every hop unpacks every frame, and the per-element math cost ~1.8 ms per 8-column verify
// frame on a laptop CPU, several times that on a phone. 256 KB, built on first use.
let F16_LUT = null;
function f16Table() {
  if (!F16_LUT) { F16_LUT = new Float32Array(65536); for (let h = 0; h < 65536; h++) F16_LUT[h] = f16ToF32(h); }
  return F16_LUT;
}
export function unpackF16(u) {
  const T = f16Table(), out = new Float32Array(u.length);
  for (let i = 0; i < u.length; i++) out[i] = T[u[i]];
  return out;
}

export function asU16(x) {
  if (x instanceof Uint16Array) return x;
  if (x instanceof ArrayBuffer) return new Uint16Array(x);
  if (ArrayBuffer.isView(x)) { const u = new Uint8Array(x.buffer, x.byteOffset, x.byteLength); return new Uint16Array(u.slice().buffer); }
  throw new Error("bad f16 payload");
}

// The floats per column of a frame: the engine's hDim when it has one (Qwen3.8-Flash-Next sends its 4-stream
// residual, 4 x 2560 = 10240 floats, 20 KB per token in f16), else its hidden size. 0 when the engine says neither.
export const wireDim = (engine) => engine?.dims?.hDim || engine?.dims?.dim || 0;
// The token ids a host's frame carries for the devices past it (packWire): only for an engine that asks
// (engine.frameIds: Qwen3.8-Flash-Next, whose n-gram layer may be on the first worker), else none
export const idsFor = (engine, ids) => (engine?.frameIds ? ids : null);

// ids (optional): the frame's token ids, one per column, after the values. A device whose layers read the tokens
// themselves (Qwen3.8-Flash-Next's n-gram layer: engine.frameIds) gets them this way. Each id is two u16 (low word
// first) in an f16 frame, one u32 bit pattern in an f32 one, so the transport carries them as payload: no header
// field, no protocol change. splitWire tells them apart by the payload's length.
export function packWire(f, ids = null) {
  if (!ids?.length) return WIRE_F16 ? { data: packF16(f), enc: "f16" } : { data: f };
  const n = ids.length;
  if (WIRE_F16) {
    const out = new Uint16Array(f.length + 2 * n);
    out.set(packF16(f));
    for (let k = 0; k < n; k++) { const t = idOf(ids[k]); out[f.length + 2 * k] = t & 0xffff; out[f.length + 2 * k + 1] = t >>> 16; }
    return { data: out, enc: "f16" };
  }
  const out = new Float32Array(f.length + n);
  out.set(f);
  new Uint32Array(out.buffer, f.length * 4, n).set(Array.from(ids, idOf));
  return { data: out };
}
const idOf = (t) => { if (!(Number.isInteger(t) && t >= 0 && t <= 0xffffffff)) throw new Error(`not a token id: ${t}`); return t; };

export function unpackWire(d) { return d.enc === "f16" ? unpackF16(asU16(d.data)) : asF32(d.data); }

// A frame of n columns of `width` floats -> { h: Float32Array(n * width), ids: Uint32Array(n) | null }: the token ids
// when the sender added them (packWire), else null. Any other length (or no width: 0) is all hidden state, as
// unpackWire reads it.
export function splitWire(d, n, width) {
  if (!(width > 0)) return { h: unpackWire(d), ids: null };
  if (d.enc === "f16") {
    const u = asU16(d.data), m = n * width;
    if (u.length !== m + 2 * n) return { h: unpackF16(u), ids: null };
    const ids = new Uint32Array(n);
    for (let k = 0; k < n; k++) ids[k] = u[m + 2 * k] | (u[m + 2 * k + 1] << 16);
    return { h: unpackF16(u.subarray(0, m)), ids };
  }
  const f = asF32(d.data), m = n * width;
  if (f.length !== m + n) return { h: f, ids: null };
  return { h: f.slice(0, m), ids: new Uint32Array(f.buffer.slice(f.byteOffset + m * 4, f.byteOffset + (m + n) * 4)) };
}

export function asF32(x) {
  if (typeof x === "string") return b64ToF32(x);
  if (x instanceof Float32Array) return x;
  if (x instanceof ArrayBuffer) return new Float32Array(x);
  if (ArrayBuffer.isView(x)) {
    const u = new Uint8Array(x.buffer, x.byteOffset, x.byteLength);
    return new Float32Array(u.slice().buffer);   // copy: alignment-safe
  }
  throw new Error("unrecognized hidden-state payload");
}

export function b64ToF32(b) {
  const s = atob(b);
  const u = new Uint8Array(s.length);
  for (let i = 0; i < s.length; i++) u[i] = s.charCodeAt(i);
  return new Float32Array(u.buffer);
}
