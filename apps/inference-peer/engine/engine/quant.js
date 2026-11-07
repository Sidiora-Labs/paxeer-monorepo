// Vendored from https://github.com/Nehanth/pooled/blob/45b5ee275a8754a37fce4a6ce174f31e4a216ef4/engine/quant.js (MIT; see LICENSE-ENGINE).
// Engine-side quantization helpers (Q4_0 with f16 block scales) and dequant for self-tests.
import { f16ToF32, f32ToF16 } from "./gguf.js";

export function quantizeQ4(data) {
  const n = data.length, nb = Math.ceil(n / 32);
  const qs = new Uint8Array(nb * 16);
  const scales = new Uint32Array(Math.ceil(nb / 2));
  const sc16 = new Uint16Array(scales.buffer);
  for (let b = 0; b < nb; b++) {
    let amax = 0, maxv = 0;
    for (let i = b * 32; i < Math.min(n, b * 32 + 32); i++) {
      if (Math.abs(data[i]) > amax) { amax = Math.abs(data[i]); maxv = data[i]; }
    }
    const h = f32ToF16(maxv / -8 || 1);
    sc16[b] = h;
    const d = f16ToF32(h);
    for (let j = 0; j < 16; j++) {
      const lo = Math.max(0, Math.min(15, Math.round((data[b * 32 + j] || 0) / d) + 8));
      const hi = Math.max(0, Math.min(15, Math.round((data[b * 32 + j + 16] || 0) / d) + 8));
      qs[b * 16 + j] = lo | (hi << 4);
    }
  }
  return { qs, scales };
}
const scOf = (q, b) => f16ToF32((q.scales[b >> 1] >>> ((b & 1) * 16)) & 0xFFFF);
export function dequantQ4(q, n) {
  const out = new Float32Array(n);
  for (let b = 0; b < n / 32; b++) for (let j = 0; j < 16; j++) {
    const byte = q.qs[b * 16 + j];
    out[b * 32 + j] = scOf(q, b) * ((byte & 0xF) - 8);
    out[b * 32 + j + 16] = scOf(q, b) * ((byte >> 4) - 8);
  }
  return out;
}
export function dequantQ8(q, n) {
  const out = new Float32Array(n);
  for (let i = 0; i < n; i++) { const v = q.qs[i]; out[i] = scOf(q, (i / 32) | 0) * (v > 127 ? v - 256 : v); }
  return out;
}
