// Vendored from https://github.com/Nehanth/pooled/blob/45b5ee275a8754a37fce4a6ce174f31e4a216ef4/engine/expert_disk.js (MIT; see LICENSE-ENGINE).
// Expert offload's disk tier (engine/expert_store.js disk layers): one routed expert of a stacked GGUF tensor read
// from the file and converted on its own, when a cut or a prefill frame needs it, instead of the whole layer being
// parked in RAM at load.
//
// A stacked expert tensor [nExp][rows][cols] is nExp row blocks of rows * rowBytes, back to back from its byteOffset,
// so expert e is one contiguous range. The conversion (engine/gguf.js convertEntry: Q4_0 / Q8_0 repacked, other
// quants requantized to Q8) works block by block within a row, so converting one expert's rows gives exactly the
// bytes the whole tensor's conversion has at that expert's offset (tests/unit/expert_disk_test.js checks it).
//
// Browser-safe: no node: imports. The caller injects readAt(shard, byteOffset, byteLength) -> Promise<Uint8Array>
// (tests/load_model.js: node:fs positional reads; a room node: its model source).
import { convertEntry, ggmlTypeBytes, GGML_F32, GGML_F16, GGML_BF16, GGML_Q4_0, GGML_Q8_0 } from "./gguf.js";

// the engine's kind for an expert tensor of this ggml type (convertEntry's mapping); throws for types it keeps as f32
export function expertKind(ggmlType) {
  if (ggmlType === GGML_F32 || ggmlType === GGML_F16 || ggmlType === GGML_BF16) throw new Error(`expert offload: experts of ggml type ${ggmlType} stay f32, not Q4_0 / Q8_0`);
  if (ggmlTypeBytes(ggmlType, 256) < 0) throw new Error(`expert offload: unsupported ggml type ${ggmlType}`);
  return ggmlType === GGML_Q4_0 ? "q4" : "q8";
}

// info: the tensor's header entry (shape [nExp, rows, cols] in torch order, ggmlType, byteOffset, shard)
export function expertGeometry(info) {
  if (info.shape.length !== 3) throw new Error(`expert offload: ${info.name || "tensor"} is not a stacked [nExp][rows][cols] expert tensor`);
  const [nExp, rows, cols] = info.shape;
  const rowBytes = ggmlTypeBytes(info.ggmlType, cols);
  if (!(rowBytes > 0) || rowBytes % 1) throw new Error(`expert offload: ${cols} columns of ggml type ${info.ggmlType} are not whole blocks`);
  return { nExp, rows, cols, rowBytes, expBytes: rows * rowBytes, kind: expertKind(info.ggmlType) };
}

const asBytes = (a) => (a instanceof Uint8Array ? a : new Uint8Array(a.buffer, a.byteOffset, a.byteLength));

export function expertDisk({ readAt }) {
  if (typeof readAt !== "function") throw new Error("expertDisk: readAt(shard, byteOffset, byteLength) is required");
  const stats = { reads: 0, bytes: 0 };
  // expert e of one part -> { qs, sc } (Uint8Arrays in the engine's layout, the slices a parked layer would hold)
  async function readPart(info, e) {
    const g = expertGeometry(info);
    if (!(e >= 0 && e < g.nExp)) throw new Error(`expert offload: expert ${e} of ${g.nExp}`);
    const bytes = await readAt(info.shard || 0, info.byteOffset + e * g.expBytes, g.expBytes);
    if (bytes.byteLength < g.expBytes) throw new Error(`expert offload: short read of ${info.name || "an expert tensor"} expert ${e} (${bytes.byteLength} of ${g.expBytes} B)`);
    stats.reads++; stats.bytes += g.expBytes;
    const c = convertEntry({ name: info.name, ggmlType: info.ggmlType, shape: [g.rows, g.cols], nElems: g.rows * g.cols }, bytes);
    return { qs: asBytes(c.qs), sc: asBytes(c.scales) };
  }
  return { readPart, stats };
}
