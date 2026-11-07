// Vendored from https://github.com/Nehanth/pooled/blob/45b5ee275a8754a37fce4a6ce174f31e4a216ef4/engine/stage_ring.js (MIT; see LICENSE-ENGINE).
// A ring of mapped upload buffers (MAP_WRITE | COPY_SRC) for expert offload's disk tier (engine/expert_store.js
// option disk.ring): the bytes of an expert read from disk are copied into a mapped slot on the CPU, the slot is
// unmapped, and the store's plan() encodes copyBufferToBuffer from it into the pool, in the same encoder as the
// kernels that use the experts. That is in place of one queue.writeBuffer per slice, which in a browser goes through
// Dawn's 4 MiB upload ring and the renderer's command stream, one write at a time (roadmap/32 step 8; prior art:
// diskformer.js's expert-stream.js uploads through a mapped ring of 16).
//
// One slot holds one expert's six slices back to back (put()). A round is what one cut reads:
//   recycle()  the slots sealed in earlier rounds are mapped again (mapAsync, not awaited: a slot comes back when
//              the GPU is done with the copies from it). The caller's contract: every encoder that copies from
//              them has been submitted by now (ExpertStore.prepare: the cut before this one was submitted before
//              this cut's selection was read back).
//   put(arrs)  copy the arrays into a free mapped slot -> { buf, offs } (each array's offset in buf), or null when
//              no slot is free (the caller then writes the bytes with queue.writeBuffer: correct, only slower)
//   seal()     unmap the slots filled this round; they can be copied from in encoders submitted from here on
//   waitFree() a prefill region fill, which streams a whole layer through the ring: after its seal + submit +
//              recycle, wait for the GPU to hand a slot back
// Only waitFree() waits on the GPU: a ring too small for a decode round makes some experts take the writeBuffer path.
export class StageRing {
  // slotBytes: the largest put() (a multiple of 4); slots: how many
  constructor(device, { slotBytes, slots = 32 }) {
    if (!(slotBytes > 0) || slotBytes % 4) throw new Error(`StageRing: slot size ${slotBytes} is not a positive multiple of 4`);
    this.device = device; this.slotBytes = slotBytes;
    const usage = GPUBufferUsage.MAP_WRITE | GPUBufferUsage.COPY_SRC;
    // state: "free" (mapped, empty), "filled" (mapped, written this round), "sealed" (unmapped, copies may read it),
    // "mapping" (mapAsync pending)
    this.slots = Array.from({ length: slots }, () => ({ buf: device.createBuffer({ size: slotBytes, usage, mappedAtCreation: true }), state: "free" }));
    this.stats = { puts: 0, full: 0, bytes: 0 };
  }
  get bytes() { return this.slots.length * this.slotBytes; }
  recycle() {
    for (const s of this.slots) {
      if (s.state !== "sealed") continue;
      s.state = "mapping";
      s.ready = s.buf.mapAsync(GPUMapMode.WRITE).then(() => { if (s.state === "mapping") s.state = "free"; }, () => { s.state = "lost"; });
    }
  }
  // until a slot is free, or nothing is being mapped (then nothing would free one: the caller seals, submits and
  // recycles first)
  async waitFree() {
    while (!this.slots.some((x) => x.state === "free")) {
      const m = this.slots.filter((x) => x.state === "mapping");
      if (!m.length) return false;
      await Promise.race(m.map((x) => x.ready));
    }
    return true;
  }
  put(arrs) {
    const n = arrs.reduce((a, b) => a + b.byteLength, 0);
    if (n > this.slotBytes) throw new Error(`StageRing: ${n} B do not fit a ${this.slotBytes} B slot`);
    const s = this.slots.find((x) => x.state === "free");
    if (!s) { this.stats.full++; return null; }
    const view = new Uint8Array(s.buf.getMappedRange(0, this.slotBytes)), offs = [];
    let o = 0;
    for (const a of arrs) { view.set(a, o); offs.push(o); o += a.byteLength; }
    s.state = "filled";
    this.stats.puts++; this.stats.bytes += n;
    return { buf: s.buf, offs };
  }
  seal() {
    for (const s of this.slots) if (s.state === "filled") { s.buf.unmap(); s.state = "sealed"; }
  }
  destroy() { for (const s of this.slots) s.buf.destroy(); this.slots = []; }
}
