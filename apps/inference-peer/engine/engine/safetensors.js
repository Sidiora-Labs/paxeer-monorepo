// Vendored from https://github.com/Nehanth/pooled/blob/45b5ee275a8754a37fce4a6ce174f31e4a216ef4/engine/safetensors.js (MIT; see LICENSE-ENGINE).
// safetensors parsing and HTTP range-fetching of a layer shard (SmolLM demo path).
import { WGSL } from "./wgsl/base.js";

export function parseSafetensors(arrayBuf) {
  const dv = new DataView(arrayBuf);
  const headerLen = Number(dv.getBigUint64(0, true));
  const header = JSON.parse(new TextDecoder().decode(new Uint8Array(arrayBuf, 8, headerLen)));
  const base = 8 + headerLen;
  const out = {};
  for (const [name, info] of Object.entries(header)) {
    if (name === "__metadata__") continue;
    out[name] = { dtype: info.dtype, shape: info.shape, arrayBuf, byteOffset: base + info.data_offsets[0], byteLength: info.data_offsets[1] - info.data_offsets[0] };
  }
  return out;
}

export function tensorF32(t) {
  const n = t.shape.reduce((a, b) => a * b, 1);
  if (t.dtype === "F32") return new Float32Array(t.arrayBuf, t.byteOffset, n);
  if (t.dtype === "BF16") {
    const u16 = new Uint16Array(t.arrayBuf, t.byteOffset, n);
    const out = new Float32Array(n);
    const u32 = new Uint32Array(out.buffer); // view f32 bits directly
    for (let i = 0; i < n; i++) u32[i] = u16[i] << 16;
    return out;
  }
  throw new Error("unsupported dtype " + t.dtype);
}

export function shardTensorNames(cfg, [lo, hi], hasEmbed, hasHead) {
  const names = [];
  if (hasEmbed || hasHead) names.push("model.embed_tokens.weight");
  if (hasHead) names.push("model.norm.weight");
  const parts = ["input_layernorm.weight", "self_attn.q_proj.weight", "self_attn.k_proj.weight",
    "self_attn.v_proj.weight", "self_attn.o_proj.weight", "post_attention_layernorm.weight",
    "mlp.gate_proj.weight", "mlp.up_proj.weight", "mlp.down_proj.weight"];
  for (let i = lo; i < hi; i++) for (const s of parts) names.push(`model.layers.${i}.${s}`);
  return names;
}

// Fetch only the named tensors via HTTP Range requests (falls back to a full
// download if the server won't do ranges). Returns the same tensor-map shape
// parseSafetensors produces.
export async function fetchModelShard(url, names, onProgress = () => {}) {
  const tryRange = async (from, to) => {
    const r = await fetch(url, { headers: { Range: `bytes=${from}-${to}` } });
    return r.status === 206 ? await r.arrayBuffer() : null;
  };
  let head = null;
  try { head = await tryRange(0, 7); } catch { head = null; }
  if (head) {
    const headerLen = Number(new DataView(head).getBigUint64(0, true));
    const hbuf = await tryRange(8, 8 + headerLen - 1);
    const hjson = JSON.parse(new TextDecoder().decode(new Uint8Array(hbuf)));
    const base = 8 + headerLen;
    const infos = names.map((n) => {
      if (!hjson[n]) throw new Error("tensor not in file: " + n);
      return [n, hjson[n]];
    });
    const totalBytes = infos.reduce((s, [, i]) => s + i.data_offsets[1] - i.data_offsets[0], 0);
    let done = 0;
    const out = {};
    const queue = [...infos];
    await Promise.all(Array.from({ length: 5 }, async () => {
      while (queue.length) {
        const [name, info] = queue.shift();
        const [b0, b1] = info.data_offsets;
        const buf = await tryRange(base + b0, base + b1 - 1);
        if (!buf || buf.byteLength !== b1 - b0) throw new Error("range fetch failed: " + name);
        out[name] = { dtype: info.dtype, shape: info.shape, arrayBuf: buf, byteOffset: 0, byteLength: b1 - b0 };
        done += b1 - b0;
        onProgress(done / totalBytes, done, totalBytes);
      }
    }));
    return out;
  }
  // fallback: whole file
  const r = await fetch(url);
  const total = +r.headers.get("Content-Length") || 0;
  const reader = r.body.getReader();
  const chunks = [];
  let got = 0;
  while (true) {
    const { done, value } = await reader.read();
    if (done) break;
    chunks.push(value);
    got += value.length;
    onProgress(total ? got / total : 0, got, total);
  }
  const buf = new Uint8Array(got);
  let o = 0;
  for (const c of chunks) { buf.set(c, o); o += c.length; }
  const all = parseSafetensors(buf.buffer);
  const out = {};
  for (const n of names) out[n] = all[n];
  return out;
}

export function weightsFromSafetensors(tensors, { lo, hi, hasEmbed, hasHead }) {
  const f32 = (name) => ({ kind: "f32", data: tensorF32(tensors[name]) });
  const layers = [];
  for (let i = lo; i < hi; i++) {
    const p = `model.layers.${i}.`;
    layers.push({
      inNorm: f32(p + "input_layernorm.weight"),
      q: f32(p + "self_attn.q_proj.weight"),
      k: f32(p + "self_attn.k_proj.weight"),
      v: f32(p + "self_attn.v_proj.weight"),
      o: f32(p + "self_attn.o_proj.weight"),
      postNorm: f32(p + "post_attention_layernorm.weight"),
      gate: f32(p + "mlp.gate_proj.weight"),
      up: f32(p + "mlp.up_proj.weight"),
      down: f32(p + "mlp.down_proj.weight"),
    });
  }
  const out = { layers };
  if (hasEmbed || hasHead) out.embed = f32("model.embed_tokens.weight");
  if (hasHead) out.finalNorm = f32("model.norm.weight");
  return out;
}
