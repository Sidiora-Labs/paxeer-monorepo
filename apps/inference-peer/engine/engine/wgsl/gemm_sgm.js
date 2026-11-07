// Vendored from https://github.com/Nehanth/pooled/blob/45b5ee275a8754a37fce4a6ce174f31e4a216ef4/engine/wgsl/gemm_sgm.js (MIT; see LICENSE-ENGINE).
// Prefill GEMM on tensor cores through Chrome's experimental subgroup-matrix WGSL extension
// (candidate C, docs/research/prefill-profile-2026-09.md). Off by default: prefillMath "sgmatrix".
//
// Numerics: operands are rounded to f16 (dequantized weight = f16(f32(q) * scale), activation =
// f16(x)), products are exact and accumulate in f32 on the tensor cores. That is exactly what the
// portable prefillMath "f16" GEMM computes on the ALU (engine/wgsl/gemm.js R16), up to summation
// order, so the effect of f16 operands on the goldens can be measured in Deno, where this
// extension does not exist.
//
// Drop-in for gemm_q4_{dIn}_s{S} / gemm_q8_{dIn}_s{S}: same bindings (qs, sc, xT, partials, shape),
// same grid (ceil(dOut / 128) * S workgroups, 2-D over 32768), same split-K partials layout
// p[split][col][dOut], so the fixed-order gemm_red_s{S} reduce and the pinned GEMM_S table are
// reused unchanged. Needs dOut % TM == 0 (every shape the two models run is a multiple of 128).
//
// Schedule: one workgroup = one subgroup (@workgroup_size = the adapter's maximum subgroup size, so a
// workgroup is never a partial subgroup; if the hardware runs smaller subgroups, each one repeats
// the same MMAs and stores the same values, which is wasteful but correct). Per K stage (KB Q4/Q8
// blocks of 32), the workgroup dequantizes a TM x 32KB weight tile and the 32KB x N activation tile
// to f16 in workgroup memory, then each of the TM/M row slabs runs (32KB/Kc) x (N/Nm) MMAs into
// f32 result matrices that live in registers for the whole K range. Results are stored straight
// into the partials buffer, column-major with stride dOut.
//
// The index formulas are written once (below, as functions over numbers or WGSL expression
// strings) and shared by the WGSL generator and the JavaScript twin sgmGemmJS, which the CPU unit
// test (tests/unit/gemm_sgm_test.js) checks against a float64 reference.

export const SGM_TM = 128;   // default rows per workgroup: equals GEMM_TILE, so the grid is the f32 GEMM's
// Tuning knobs (engine option prefillSgm, Chrome bench ?sgm=JSON): TM rows per workgroup (a multiple of
// the MMA M), KB Q4/Q8 blocks per K stage, PAD f16 elements of padding per weight-tile row (keeps the
// row stride a multiple of 16 bytes and spreads the dequantize stores over the shared-memory banks).
export const SGM_DEFAULT = { TM: SGM_TM, KB: 1, PAD: 8 };

// WebGPU feature names a device needs for the tensor-core path (Chrome/Dawn only, experimental), and
// the ones worth requesting with them when the adapter has them ("subgroups": Dawn may tie the
// subgroup-matrix extension to it).
export const SGM_FEATURES = ["shader-f16", "chromium-experimental-subgroup-matrix"];
export const SGM_FEATURES_OPT = ["subgroups"];
// WGSL spelling of the builtins. "template" (current Dawn, as ONNX Runtime's WebGPU kernels use it):
//   subgroupMatrixLoad<T, row_major>(p, offset, stride), subgroupMatrixStore<col_major>(p, offset, v, stride)
// "bool" (the first Chrome releases of the extension): subgroupMatrixLoad<T>(p, offset, colMajor, stride),
//   subgroupMatrixStore(p, offset, v, colMajor, stride). The engine compiles "template" and falls back to "bool".
export const SGM_SYNTAX = ["template", "bool"];

// Pick a subgroup-matrix configuration usable for f16 x f16 -> f32 with N token columns.
// info: GPUAdapterInfo (device.adapterInfo in Chrome), which lists subgroupMatrixConfigs and the
// subgroup size range. Returns { M, Nm, Kc, SG } or null with a reason.
export function pickSgmConfig(info, { N = 16, KB = 1, TM = SGM_TM } = {}) {
  const cs = info?.subgroupMatrixConfigs;
  if (!cs || !cs.length) return { cfg: null, why: "adapter lists no subgroupMatrixConfigs" };
  const KS = 32 * KB;
  const ok = [...cs].filter((c) => c.componentType === "f16" && c.resultComponentType === "f32"
    && TM % c.M === 0 && N % c.N === 0 && KS % c.K === 0);
  if (!ok.length) return { cfg: null, why: `no f16->f32 config tiles ${TM}x${N}x${KS}: ${JSON.stringify([...cs].map((c) => [c.componentType, c.resultComponentType, c.M, c.N, c.K]))}` };
  // prefer the largest MMA (fewest instructions), then square
  ok.sort((a, b) => (b.M * b.N * b.K - a.M * a.N * a.K) || (Math.abs(a.M - a.N) - Math.abs(b.M - b.N)));
  const SG = info.subgroupMaxSize || info.subgroupMinSize || 32;
  if ((TM * KB) % SG || (KS * N / 4) % SG) return { cfg: null, why: `subgroup size ${SG} does not divide the staging work` };
  const c = ok[0];
  return { cfg: { M: c.M, Nm: c.N, Kc: c.K, SG }, why: info.subgroupMinSize !== info.subgroupMaxSize
    ? `subgroup size ${info.subgroupMinSize}..${info.subgroupMaxSize}: workgroup = ${SG} (a smaller subgroup repeats the MMAs)` : "" };
}

// ---- index math, shared by the WGSL generator and the JS twin ----
// Values are numbers (evaluated) or strings (WGSL u32 expressions).
const isN = (v) => typeof v === "number";
const wl = (v) => (isN(v) ? `${v}u` : `(${v})`);
const add = (...xs) => {
  const n = xs.filter(isN).reduce((a, b) => a + b, 0), s = xs.filter((x) => !isN(x));
  if (!s.length) return n;
  return [...s.map(wl), ...(n ? [`${n}u`] : [])].join(" + ");
};
const mul = (a, b) => (isN(a) && isN(b) ? a * b : a === 0 || b === 0 ? 0 : a === 1 ? b : b === 1 ? a : `${wl(a)} * ${wl(b)}`);

export function sgmPlan({ q8 = false, dIn, S, N = 16, TM = SGM_TM, KB = 1, PAD = 8, M, Nm, Kc, SG }) {
  const nb = dIn / 32, KS = 32 * KB, BPW = nb / S, KSP = KS + PAD;
  if (PAD % 8) throw new Error("gemm sgm: PAD must be a multiple of 8 (16-byte row stride)");
  if (dIn % 64) throw new Error("gemm sgm: dIn must be a multiple of 64 (f16 scales are paired)");
  if (nb % S || BPW % KB) throw new Error(`gemm sgm: S=${S}, KB=${KB} must divide the ${nb} blocks of dIn=${dIn}`);
  if (TM % M || N % Nm || KS % Kc) throw new Error("gemm sgm: the MMA shape must tile TM x N x 32KB");
  if ((TM * KB) % SG || (KS * N / 4) % SG) throw new Error("gemm sgm: SG must divide the staging work");
  const P = {
    q8, dIn, S, N, TM, KB, PAD, M, Nm, Kc, SG, nb, KS, KSP, BPW, nStages: BPW / KB,
    MT: TM / M, NT: N / Nm, KT: KS / Kc, WPT: TM * KB / SG, XPT: KS * N / 4 / SG,
    // workgroup -> (row tile, split); the grid is linearized as wg.y * 32768 + wg.x like the f32 GEMM
    row0: (tile) => mul(tile, TM),
    blk0: (split) => mul(split, BPW),
    // staging item li in [0, TM*KB): tile row li / KB, block li % KB (KB = 1 in the default plan)
    itemRow: (li) => (isN(li) ? Math.floor(li / KB) : `${wl(li)} / ${KB}u`),
    itemBlk: (li) => (isN(li) ? li % KB : `${wl(li)} % ${KB}u`),
    blk: (b0, s, b) => add(b0, mul(s, KB), b),
    // weight words: Q4 one vec4<u32> per (row, block); Q8 two in sequence
    wIdx: (row, blk, half = 0) => (q8 ? add(mul(add(mul(row, nb), blk), 2), half) : add(mul(row, nb), blk)),
    // f16 scale pair word and half (nb is even, so row * nb + blk has blk's parity)
    scIdx: (row, blk) => { const i = add(mul(row, nb), blk); return isN(i) ? i >> 1 : `${wl(i)} >> 1u`; },
    scHalf: (blk) => (isN(blk) ? blk & 1 : `${wl(blk)} & 1u`),
    // workgroup tiles: weights [TM][KSP] row-major (KS used), activations [KS][N] row-major (xT's own order)
    smW: (r, k) => add(mul(r, KSP), k),
    xVec: (blk, li) => add(mul(blk, 32 * N / 4), li),   // first vec4 of a stage = its first block
    leftOff: (m, kk) => m * M * KSP + kk * Kc,
    rightOff: (n, kk) => kk * Kc * N + n * Nm,
    storeOff: (split, n, row0, m, dOut) => add(mul(mul(split, N), dOut), mul(n * Nm, dOut), row0, m * M),
  };
  return P;
}

const rng = (n) => Array.from({ length: n }, (_, i) => i);

// One kernel. Names: gemm_sgm_q4_{dIn}_s{S} / gemm_sgm_q8_{dIn}_s{S}.
function kernel(P, syntax = "template") {
  const T = syntax === "template";
  const load = (ty, arr, off, colMajor, stride) => T ? `subgroupMatrixLoad<${ty}, ${colMajor ? "col_major" : "row_major"}>(&${arr}, ${off}, ${stride})`
    : `subgroupMatrixLoad<${ty}>(&${arr}, ${off}, ${colMajor}, ${stride})`;
  const store = (arr, off, v, colMajor, stride) => T ? `subgroupMatrixStore<${colMajor ? "col_major" : "row_major"}>(&${arr}, ${off}, ${v}, ${stride})`
    : `subgroupMatrixStore(&${arr}, ${off}, ${v}, ${colMajor}, ${stride})`;
  const { q8, dIn, S, N, M, Nm, Kc, SG, KSP, KB, MT, NT, KT, WPT, XPT, nStages } = P;
  const name = `gemm_sgm_${q8 ? "q8" : "q4"}_${dIn}_s${S}`;
  const L = (T) => T === "l" ? `subgroup_matrix_left<f16, ${Kc}, ${M}>` : T === "r" ? `subgroup_matrix_right<f16, ${Nm}, ${Kc}>` : `subgroup_matrix_result<f32, ${Nm}, ${M}>`;
  const stageW = rng(WPT).map((j) => {
    const li = `t + ${j * SG}u`, r = P.itemRow(li), bl = P.blk("b0", "s", P.itemBlk(li));
    const head = `{ let r = ${r}; let row = row0 + r; let blk = ${bl};
      let sv = unpack2x16float(sg_sc[${P.scIdx("row", "blk")}])[${P.scHalf("blk")}];
      let wb = ${P.smW("r", KB > 1 ? mul(P.itemBlk(li), 32) : 0)};`;
    if (!q8) return `${head}
      let w = sg_qs[${P.wIdx("row", "blk")}];
      ${rng(4).map((jj) => `{ let lo = vec4<f16>((vec4<f32>((vec4<u32>(w[${jj}]) >> vec4<u32>(0u, 8u, 16u, 24u)) & vec4<u32>(15u)) - vec4<f32>(8.0)) * sv);
        let hi = vec4<f16>((vec4<f32>((vec4<u32>(w[${jj}]) >> vec4<u32>(4u, 12u, 20u, 28u)) & vec4<u32>(15u)) - vec4<f32>(8.0)) * sv);
        ${rng(4).map((i) => `sg_W[wb + ${4 * jj + i}u] = lo[${i}]; sg_W[wb + ${16 + 4 * jj + i}u] = hi[${i}];`).join(" ")} }`).join("\n      ")}
    }`;
    return `${head}
      ${[0, 1].map((h) => `let w${h} = sg_qs[${P.wIdx("row", "blk", h)}];`).join(" ")}
      ${rng(8).map((wd) => `{ let d = vec4<f16>(vec4<f32>(bitcast<vec4<i32>>(vec4<u32>(w${wd >> 2}[${wd & 3}]) << vec4<u32>(24u, 16u, 8u, 0u)) >> vec4<u32>(24u)) * sv);
        ${rng(4).map((i) => `sg_W[wb + ${4 * wd + i}u] = d[${i}];`).join(" ")} }`).join("\n      ")}
    }`;
  }).join("\n    ");
  const stageX = rng(XPT).map((j) => `{ let li = t + ${j * SG}u; let v = vec4<f16>(clamp(sg_xT[${P.xVec(add("b0", mul("s", KB)), "li")}], vec4<f32>(-65504.0), vec4<f32>(65504.0)));
      ${rng(4).map((c) => `sg_X[li * 4u + ${c}u] = v[${c}];`).join(" ")} }`).join("\n    ");
  const mma = rng(KT).map((kk) => `{
      ${rng(NT).map((n) => `let r${n} = ${load(L("r"), "sg_X", `${P.rightOff(n, kk)}u`, false, `${N}u`)};`).join("\n      ")}
      ${rng(MT).map((m) => `{ let l = ${load(L("l"), "sg_W", `${P.leftOff(m, kk)}u`, false, `${KSP}u`)};
        ${rng(NT).map((n) => `c${m}_${n} = subgroupMatrixMultiplyAccumulate(l, r${n}, c${m}_${n});`).join(" ")} }`).join("\n      ")}
    }`).join("\n    ");
  return `
@compute @workgroup_size(${SG})
fn ${name}(@builtin(workgroup_id) wg: vec3<u32>, @builtin(local_invocation_index) t: u32) {
  let dOut = sg_shape.dOut;
  let wgl = wg.y * 32768u + wg.x;
  let tile = wgl / ${S}u; let split = wgl % ${S}u;
  let row0 = ${P.row0("tile")}; let b0 = ${P.blk0("split")};
  if (row0 >= dOut) { return; }   // a 2-D grid's padding workgroups (workgroup-uniform exit)
  ${rng(MT).map((m) => rng(NT).map((n) => `var c${m}_${n} = ${L("c")}();`).join(" ")).join("\n  ")}
  for (var s: u32 = 0u; s < ${nStages}u; s++) {
    workgroupBarrier();
    ${stageW}
    ${stageX}
    workgroupBarrier();
    ${mma}
  }
  ${rng(MT).map((m) => rng(NT).map((n) => `${store("sg_y", P.storeOff("split", n, "row0", m, "dOut"), `c${m}_${n}`, true, "dOut")};`).join(" ")).join("\n  ")}
}`;
}

// A standalone shader module (it needs its own `enable` directives, so it is never concatenated
// into the engine's main module: a compile failure here cannot take the f32 path down with it).
// pairs / pairs8: [[dIn, S], ...] for Q4 / Q8 weights. cfg: from pickSgmConfig.
export function gemmSgmWGSL({ N = 16, TM = SGM_TM, KB = 1, PAD = 8, cfg, pairs = [], pairs8 = [], syntax = "template" }) {
  const { M, Nm, Kc, SG } = cfg;
  const ks = [];
  const seen = new Set();
  for (const [list, q8] of [[pairs, false], [pairs8, true]]) for (const [dIn, S] of list) {
    const key = `${q8}:${dIn}:${S}`; if (seen.has(key)) continue; seen.add(key);
    ks.push(kernel(sgmPlan({ q8, dIn, S, N, TM, KB, PAD, M, Nm, Kc, SG }), syntax));
  }
  // every matrix operand here is workgroup-uniform (workgroup id, uniform dOut, constants), so the
  // uniformity diagnostic is only switched off in case an older compiler cannot prove it
  return /* wgsl */ `enable f16;
enable chromium_experimental_subgroup_matrix;
diagnostic(off, chromium.subgroup_matrix_uniformity);
// ---- tensor-core prefill GEMM (engine/wgsl/gemm_sgm.js, ${syntax} builtins): TM=${TM}, N=${N}, KB=${KB}, PAD=${PAD}, MMA ${M}x${Nm}x${Kc}, subgroup ${SG} ----
struct SgShape { dOut: u32, dIn: u32, xs4: u32, ys: u32 };
@group(1) @binding(0) var<storage, read> sg_qs: array<vec4<u32>>;
@group(1) @binding(1) var<storage, read> sg_sc: array<u32>;
@group(1) @binding(2) var<storage, read> sg_xT: array<vec4<f32>>;
@group(1) @binding(3) var<storage, read_write> sg_y: array<f32>;
@group(1) @binding(4) var<uniform> sg_shape: SgShape;
var<workgroup> sg_W: array<f16, ${TM * (32 * KB + PAD)}>;
var<workgroup> sg_X: array<f16, ${32 * KB * N}>;
${ks.join("\n")}
`;
}

// ---- JavaScript twin (for the CPU unit test) ----
// Runs one kernel over its whole grid with the same plan and index functions, emulating the
// subgroup-matrix builtins as specified (Load/Store: element (r, c) at offset + r * stride + c
// row-major, offset + c * stride + r column-major). f16 rounding via Math.f16round, f32 via
// Math.fround; the MMA accumulates in float64 (hardware accumulation order is unspecified).
// qs: Uint32Array (engine layout), sc: Uint32Array (f16 pairs), xT: Float32Array [dIn][N],
// y: Float64Array partials [S][N][dOut] (untouched cells stay as they were).
export function sgmGemmJS(P, { qs, sc, xT, y, dOut, f16round }) {
  const { N, TM, KB, M, Nm, Kc, SG, KS, KSP, MT, NT, KT, WPT, XPT, nStages, S, q8 } = P;
  const f32 = Math.fround, h = (v) => f16round(v);
  const f16s = (w) => { const b = w & 0xffff; const s = b & 0x8000 ? -1 : 1, e = (b >> 10) & 31, m = b & 1023;
    return e === 0 ? s * m * 2 ** -24 : e === 31 ? (m ? NaN : s * Infinity) : s * (1 + m / 1024) * 2 ** (e - 15); };
  const nWG = Math.ceil(dOut / TM) * S;
  for (let wgl = 0; wgl < nWG; wgl++) {
    const tile = Math.floor(wgl / S), split = wgl % S, row0 = P.row0(tile), b0 = P.blk0(split);
    const C = rng(MT).map(() => rng(NT).map(() => new Float64Array(M * Nm)));
    for (let s = 0; s < nStages; s++) {
      const W = new Float64Array(TM * KSP).fill(NaN), X = new Float64Array(KS * N).fill(NaN);
      for (let t = 0; t < SG; t++) {
        for (let j = 0; j < WPT; j++) {
          const li = t + j * SG, r = P.itemRow(li), row = row0 + r, blk = P.blk(b0, s, P.itemBlk(li));
          const sw = sc[P.scIdx(row, blk)], sv = f16s(P.scHalf(blk) ? sw >>> 16 : sw);
          const wb = P.smW(r, KB > 1 ? P.itemBlk(li) * 32 : 0);
          if (!q8) {
            const w = [0, 1, 2, 3].map((i) => qs[P.wIdx(row, blk) * 4 + i]);
            for (let jj = 0; jj < 4; jj++) for (let i = 0; i < 4; i++) {
              W[wb + 4 * jj + i] = h(f32((((w[jj] >>> (8 * i)) & 15) - 8) * sv));
              W[wb + 16 + 4 * jj + i] = h(f32((((w[jj] >>> (8 * i + 4)) & 15) - 8) * sv));
            }
          } else {
            const w = [0, 1].flatMap((hh) => [0, 1, 2, 3].map((i) => qs[P.wIdx(row, blk, hh) * 4 + i]));
            for (let wd = 0; wd < 8; wd++) for (let i = 0; i < 4; i++) W[wb + 4 * wd + i] = h(f32(((w[wd] << (24 - 8 * i)) >> 24) * sv));
          }
        }
        for (let j = 0; j < XPT; j++) {
          const li = t + j * SG, v = P.xVec(b0 + s * KB, li);
          for (let c = 0; c < 4; c++) X[li * 4 + c] = h(xT[v * 4 + c]);
        }
      }
      if (W.some((v, i) => i % KSP < KS && Number.isNaN(v)) || X.some(Number.isNaN)) throw new Error("sgm twin: a workgroup tile cell was never staged");
      for (let kk = 0; kk < KT; kk++) for (let m = 0; m < MT; m++) for (let n = 0; n < NT; n++) {
        const lo = P.leftOff(m, kk), ro = P.rightOff(n, kk), c = C[m][n];
        for (let i = 0; i < M; i++) for (let jn = 0; jn < Nm; jn++) {
          let a = 0;
          for (let k = 0; k < Kc; k++) a += W[lo + i * KSP + k] * X[ro + k * N + jn];
          c[i * Nm + jn] += a;
        }
      }
    }
    for (let m = 0; m < MT; m++) for (let n = 0; n < NT; n++) {
      const off = P.storeOff(split, n, row0, m, dOut), c = C[m][n];
      for (let i = 0; i < M; i++) for (let jn = 0; jn < Nm; jn++) y[off + jn * dOut + i] = c[i * Nm + jn];   // column-major, stride dOut
    }
  }
}
