// Vendored from https://github.com/Nehanth/pooled/blob/45b5ee275a8754a37fce4a6ce174f31e4a216ef4/engine/map_kick.js (MIT; see LICENSE-ENGINE).
// mapRead: buffer.mapAsync(READ) that resolves as soon as the GPU is done, in Chrome too.
//
// Chrome's GPU process resolves a pending map when it next checks the device for finished work. If the work behind
// the map was still running when the request arrived, that check is a timer about 2 ms away (tests/bench/cut_rt.js
// on the GB10, Chrome 131 / Vulkan: a readback behind 80 us of GPU work took 2.9 ms median, a ready one 0.1-0.5 ms).
// Every request the GPU process handles also checks the device, so mapRead keeps one cheap request in flight while
// the map is pending: a mapAsync of a 16-byte buffer that has no GPU work, which completes at once and is reissued.
// The same readback then took 0.4 ms. Expert offload reads the router's choice back once per offloaded layer and
// token (engine/qwen35.js _cutResolve), so this is most of a cut's cost in a tab. On Dawn in Node the map resolves
// within ~30 us anyway and the kick changed nothing measurable there. Deno (wgpu, ~12 ms per map) was not measured.
// The engine kicks only offloaded readbacks: on resident decode in Chrome (decode-ahead already hides the readback) a
// review's interleaved A/B measured 2-6 tok/s slower with the kick on the token readback. The CPU and power cost of the
// kick stream (one tiny IPC round trip after another while a map waits) is not measured.
// If a kick map fails (device lost, a destroyed buffer), mapRead stops kicking for good and just awaits the real map.
//
// The kick buffer is never read and holds no data: the mapped bytes, the order of GPU work and the results are
// exactly those of a plain mapAsync.
// Kick by default only in Chrome (and its derivatives): Dawn in Node gained nothing and an interleaved A/B there was 0-2
// tok/s slower with it (inside the shared machine's noise); other browsers are not measured.
export const KICK_DEFAULT = /Chrome\//.test(globalThis.navigator?.userAgent || "");
const KICK = new WeakMap();   // device -> { buf, p: the kick map in flight or null }

export async function mapRead(device, buf, offset = 0, size = undefined, kick = true) {
  const m = size === undefined ? buf.mapAsync(GPUMapMode.READ, offset) : buf.mapAsync(GPUMapMode.READ, offset, size);
  if (!kick) return m;
  let done = false;
  m.then(() => { done = true; }, () => { done = true; });
  let k = KICK.get(device);
  if (!k) KICK.set(device, k = { buf: device.createBuffer({ size: 16, usage: GPUBufferUsage.MAP_READ | GPUBufferUsage.COPY_DST }), p: null });
  while (!done && !k.dead) {
    // one kick at a time (a buffer takes one pending map), shared by every readback of the device
    k.p ||= k.buf.mapAsync(GPUMapMode.READ, 0, 16).then(() => { k.buf.unmap(); k.p = null; }, () => { k.p = null; k.dead = true; });
    await Promise.race([m, k.p]);
  }
  return m;
}
