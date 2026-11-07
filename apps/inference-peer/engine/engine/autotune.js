// Vendored from https://github.com/Nehanth/pooled/blob/45b5ee275a8754a37fce4a6ce174f31e4a216ef4/engine/autotune.js (MIT; see LICENSE-ENGINE).
// Device autotune: time a few cooperative-GEMV shapes on the real GPU at load and keep the winner.
// Times the quantized GEMV (the hot kernel) at a few candidate workgroup
// shapes on synthetic buffers sized like a real layer. Wall-clock around
// onSubmittedWorkDone; never timestamp-query (enabling it alone has measured
// multi-x slowdowns). ~1s total at model load.
import { WGSL } from "./wgsl/base.js";
import { probeUnpack, coopWGSL } from "./wgsl/coop.js";
import { moduleSet } from "./wgsl/prune.js";

export async function autotuneCoop(device, { dIn = 5120, dOut = 17408, kind = "q4" } = {}) {
  const candidates = [[256, 4], [128, 4], [256, 8], [128, 8], [64, 4]];
  const nb = dIn / 32;
  const S = GPUBufferUsage.STORAGE | GPUBufferUsage.COPY_DST;
  const qs = device.createBuffer({ size: dOut * (kind === "q4" ? dIn / 2 : dIn), usage: S });
  const sc = device.createBuffer({ size: Math.ceil(dOut * nb / 2) * 4, usage: S });
  const x = device.createBuffer({ size: dIn * 4, usage: S });
  const y = device.createBuffer({ size: dOut * 4, usage: S });
  const shape = device.createBuffer({ size: 16, usage: GPUBufferUsage.UNIFORM | GPUBufferUsage.COPY_DST });
  device.queue.writeBuffer(shape, 0, new Uint32Array([dOut, dIn, 0, 0]));
  const cfgB = device.createBuffer({ size: 48, usage: GPUBufferUsage.UNIFORM });
  const frameB = device.createBuffer({ size: 16, usage: GPUBufferUsage.UNIFORM });
  const C = GPUShaderStage.COMPUTE;
  const results = [];
  for (const [wg, rows] of candidates) {
    try {
      const entry = kind === "q4" ? "matvec_q4_coop" : "matvec_q8_coop";
      const mod = moduleSet(device, WGSL + coopWGSL(wg, rows, 64, 4, 4, await probeUnpack(device)))(entry);
      const l0 = device.createBindGroupLayout({ entries: [0, 1].map((b) => ({ binding: b, visibility: C, buffer: { type: "uniform" } })) });
      const l1 = device.createBindGroupLayout({ entries: ["read-only-storage", "read-only-storage", "read-only-storage", "storage", "uniform"].map((t, i) => ({ binding: i, visibility: C, buffer: { type: t } })) });
      const pipe = await device.createComputePipelineAsync({
        layout: device.createPipelineLayout({ bindGroupLayouts: [l0, l1] }),
        compute: { module: mod, entryPoint: entry },
      });
      const bg0 = device.createBindGroup({ layout: l0, entries: [{ binding: 0, resource: { buffer: cfgB } }, { binding: 1, resource: { buffer: frameB } }] });
      const bg1 = device.createBindGroup({ layout: l1, entries: [qs, sc, x, y, shape].map((b, i) => ({ binding: i, resource: { buffer: b } })) });
      const run = (n) => {
        const enc = device.createCommandEncoder();
        const pass = enc.beginComputePass();
        pass.setPipeline(pipe); pass.setBindGroup(0, bg0); pass.setBindGroup(1, bg1);
        for (let i = 0; i < n; i++) pass.dispatchWorkgroups(Math.ceil(dOut / rows));
        pass.end();
        device.queue.submit([enc.finish()]);
        return device.queue.onSubmittedWorkDone();
      };
      // a device that needs >20 ms for one of these is a software rasterizer (SwiftShader) or
      // hopeless anyway: timing 500 dispatches would take minutes, so keep the default
      if (!results.length) {
        const t1 = performance.now();
        await run(1);
        if (performance.now() - t1 > 20) { for (const b of [qs, sc, x, y]) b.destroy(); return { wg: 256, rows: 4, results, skipped: "slow device" }; }
      }
      // warm up: GPUs ramp clocks under sustained load; short bursts measure the ramp
      const tw = performance.now();
      while (performance.now() - tw < (results.length ? 40 : 250)) await run(20);
      const t0 = performance.now();
      await run(100);
      results.push({ wg, rows, ms: (performance.now() - t0) / 100 });
    } catch { /* config not supported on this device; skip */ }
  }
  for (const b of [qs, sc, x, y]) b.destroy();
  if (!results.length) return { wg: 256, rows: 4, results };
  results.sort((a, b) => a.ms - b.ms);
  const best = results[0];
  // prefer the default unless a config wins by >3% (noise guard)
  const def = results.find((r) => r.wg === 256 && r.rows === 4);
  const pick = def && def.ms <= best.ms * 1.03 ? def : best;
  return { wg: pick.wg, rows: pick.rows, results };
}
