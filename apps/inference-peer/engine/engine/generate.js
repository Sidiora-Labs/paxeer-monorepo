// Vendored from https://github.com/Nehanth/pooled/blob/45b5ee275a8754a37fce4a6ce174f31e4a216ef4/engine/generate.js (MIT; see LICENSE-ENGINE).
// A single generation attempt over caller-owned state. Readiness, request locking and
// recovery stay with the browser or room node; both use this decode loop.
import { badF32, packWire, wireDim, idsFor } from "../room/wire.js";
import { pickSampler } from "../room/sampling.js";
import { reusablePrefix } from "../room/conversation.js";
import { lookupDrafts, chainDenseSpec, denseLookupDrafts } from "../room/lookup.js";
import { MAX_NEW, MAX_SEQ } from "../room/models.js";
import { lapTimeout } from "../room/liveness.js";
import { pinSplit } from "../harness/prefix.js";
import { specWithOffload, dealOffloads } from "../room/plan.js";

const PIN_KEEP_MS = 600000;
const noop = () => {};

export function createGenerator({ state: ai, pipeline, options = {}, hooks = {} }) {
  const { fillDrafts: FILL_DRAFTS = true, lookup: LOOKUP = true, hostFuse: HOST_FUSE = true,
    denseSpec: DENSE_SPEC = true, draftK: DRAFT_K = 4, offloadSpec: OFFLOAD_SPEC = null,
    checkpointMax: CKPT_MAX = 2, maxNew: defaultMaxNew = MAX_NEW, maxSeq = MAX_SEQ } = options;
  // The existing callers have different Continue, cancellation and lookup lifetimes.
  const node = options.profile === "node", plainDraft = node ? 0 : false;
  const { aiPrefill, aiPipeToken, ckptResume, resetState, ckptClear, ckptSave, ckptEngine,
    lapWait, sendChain, noteLap } = pipeline;
  const { computePass = noop, mapPulse = noop, pushMap = noop, crumb = noop, noteSpeeds = noop,
    onSoloSpeed = noop, wakeChain = noop, chainRtt = () => 0, getPeerMeta = noop, log = noop,
    preparePrompt, finish } = hooks;

  function chainSpec(current) {
    return ai.chain.length ? {
      // pre: { hs, t0 } when the engine already ran the host's layers with the drafts (hostFuse)
      runTrunk: async (tokens, pos, pre = null) => {
        const tLap = pre?.t0 ?? performance.now();
        wakeChain(pos);
        const n = tokens.length, hdim = wireDim(current.engine), NC = current.engine.NC || 4;
        const hb = pre?.hs || new Float32Array(n * hdim);
        if (!pre) for (let c = 0; c < n; c += NC) {
          const m = Math.min(NC, n - c);
          hb.set(await current.engine.embedRunBatch(tokens.slice(c, c + m), pos + c, { base: c, total: n }), c * hdim);
        }
        if (badF32(hb)) throw new Error(`NaN after ${node ? "host" : "HOST"} layers (pos ${pos})`);
        const hostMs = performance.now() - tLap;
        const returned = lapWait("b" + pos, lapTimeout(ai.lapStat, 90000, chainRtt()), "verify");
        sendChain({ t: "ai-hidden-b", basePos: pos, n: tokens.length, spec: 1, ...packWire(hb, idsFor(current.engine, tokens)) });
        const h = await returned;
        if (badF32(h)) throw new Error(`NaN in hidden returned by peers (pos ${pos})`);
        noteLap(performance.now() - tLap, hostMs);
        return h;
      },
      // the rollback rides on the next frame (sendChain), strictly before it on every device
      onReject: async (k) => { ai.pendingCtl = { rb: k }; },
      preTrunk: true,
    } : {};
  }

  async function generate(ids, request = {}) {
    let { onToken = () => {}, stop, maxNew = defaultMaxNew, sample = pickSampler(ai.settings.sampling), signal, onStatus = () => {}, pin = 0, pinTag = "", spec: useSpec = true } = request;
    // The node's attempt keeps its engine reference; browser recovery owns replacement.
    const current = node ? { engine: ai.engine } : ai;
    const nodeCtxMax = current.engine?.maxSeq;
    const ctxMax = () => node ? nodeCtxMax : current.engine?.maxSeq || maxSeq;
    if (!current.engine) throw new Error("the model is not loaded");
    // a device in the chain is gone: its frames would go nowhere and wait out the lap timeouts
    if (ai.degraded) throw new Error("a device left: re-deal the layers first");
    const aborted = () => (!node && ai.abort) || !!signal?.aborted;
    const eos = (t) => stop.has(t);
    const tokens = [];
    let count = 0, capped = false, acc = null, copied = 0, first = null;
    let tPre = 0, tDecode = 0, reused = 0, prefilled = 0, preFrames = 0;
    // GPU sampling (?gpusample=1): the head's top-k / argmax runs on the GPU and "logits" below are
    // the sampler's candidates. Only for a sampler that says it reads them (.gpu); a wrapper that
    // masks logits has no .gpu and keeps the full-logits path. specStep checks the same itself.
    let prepared = null;
    const desc = current.engine.gpuDescFor?.(sample) || null;
    try {
      let logits = null;
      if (preparePrompt) {
        prepared = await preparePrompt(ids, { ...request, aborted, desc, maxNew, engine: current.engine });
        ({ logits, reused, prefilled, tPre, maxNew } = prepared);
      } else {
        reused = ckptResume(ids, reusablePrefix(ai.fed, ids));
        // Continue after a cap that landed on a written token: the caches hold the whole open answer,
        // so there is nothing to prefill, and the next token was already sampled when it stopped
        const pend = ai.pending; ai.pending = null;
        if (!reused && pend && pend.at === ai.pos && ai.fed?.length === ids.length && ai.fed.every((t, i) => t === ids[i])) {
          reused = ids.length; first = pend.next;
        }
        if (!reused) resetState();
        const rest = ids.slice(reused);
        // a follow-up's first token needs a draft-cache row too: the trunk hidden at the position
        // before it is still in the engine when the last answer ended on a speculative step
        if (reused && rest.length && FILL_DRAFTS && current.engine.mtp && ai.xAt === ai.pos) current.engine.mtpRun(null, rest[0], ai.pos, false);
        ai.xAt = null;
        prefilled = rest.length;
        maxNew = Math.min(maxNew, ctxMax() - ids.length);
        onStatus(reused ? `prefill: ${rest.length} new tokens (${reused} already in the room's caches)…` : `prefill: ${rest.length} tokens…`);
        const t0Pre = performance.now();
        ai.frames = 0;
        // the fixed start first, then its checkpoint, then the rest (the same tokens at the same
        // positions, so the answer is the same; the head's logits after the first part are unused)
        // Stopped during the first part: nothing more is sent, and the end-of-answer save below keeps
        // what the caches hold, as for any stop during a prefill.
        const pinned = ai.ckpt?.pinned?.()[0];
        const tag = pinTag || "code";
        if (pinned && ai.pinInfo && reused >= pinned.ids.length && pinned.ids.length <= ids.length && pinned.ids.every((t, i) => t === ids[i])) ai.pinInfo.hitAt = performance.now();
        if (pin && pinned && ai.pinInfo && ai.pinInfo.tag !== tag && performance.now() - ai.pinInfo.hitAt < PIN_KEEP_MS) pin = 0;
        const cut = CKPT_MAX && ckptEngine() ? pinSplit(reused, pin, ids.length) : 0;
        logits = null;
        if (!cut) logits = rest.length ? await aiPrefill(rest, { aborted, onStatus, desc }) : null;
        else {
          const part = await aiPrefill(ids.slice(reused, cut), { aborted, onStatus, desc });
          // the whole fixed start is in the caches (also when Stop came during its last lap): pin it,
          // or the next request, which reuses it from the plain end save, would never pin it
          if (ai.fed?.length === cut) { ckptSave(true); ai.pinInfo = { tag, hitAt: performance.now() }; }
          if (part && !aborted()) logits = await aiPrefill(ids.slice(cut), { aborted, onStatus, desc });
        }
        tPre = performance.now() - t0Pre;
        if (prefilled) computePass(prefilled);
        preFrames = ai.frames;
      }

      const t0 = performance.now();
      const emit = (tok, drafted) => {
        tokens.push(tok);
        count++;
        onToken(tok, drafted);
        if (!node) {
          mapPulse(); computePass(1);
          const tps = count / ((performance.now() - t0) / 1000);
          onStatus(`generating… ${count} tok · ${tps.toFixed(1)} tok/s`);
        }
      };
      // a deal with expert offload decodes plainly unless told otherwise (room/plan.js specWithOffload): a verify
      // pass makes the offloading device copy in several times the experts a plain token does. ai.offloadBy: the
      // deal's offload per device (both hosts set it); engine.experts: this device's own
      const offOk = specWithOffload(dealOffloads(ai.offloadBy) || !!current.engine?.experts, OFFLOAD_SPEC);
      if (!logits && first == null) { /* stopped during prefill */ }
      else if ((!node || useSpec) && offOk && current.engine.mtp && current.engine.specStep) {
        // speculative decoding: the model's own draft head proposes up to K tokens,
        // one batched trunk pass verifies them (byte-identical to plain decoding)
        const spec = chainSpec(current);
        if (ai.chain.length && ai.lastHidden && first == null) current.engine.setHidden(ai.lastHidden);
        current.engine.pos = ai.pos;
        // draft depth: pick by MEASURED tokens/sec per depth (K=3 warm-up, probe
        // 5 and 7 once, keep the best, re-probe now and then). Deep chains only
        // pay when the network round-trip dominates the lap; a lap-time
        // threshold can't tell GPU time from RTT and gets stuck deep.
        const kc = { cand: [3, 5, 7], ema: {}, n: {}, step: 0, used: node ? null : {} };
        const pickK = () => {
          if (!ai.chain.length) return 3;
          kc.step++;
          if (kc.step <= 3) return 3;
          const untried = kc.cand.find((k) => !kc.n[k]);
          if (untried) return untried;
          let best = 3;
          for (const k of kc.cand) if (kc.ema[k] > kc.ema[best]) best = k;
          if (kc.step % 16 === 0) { const alt = kc.cand.filter((k) => k !== best); return alt[(kc.step / 16) % alt.length | 0]; }
          return best;
        };
        const st0 = { ...current.engine.mtp.stats };
        // the first answer token is sampled here; specStep treats it as already chosen for this
        // position and returns only the tokens after it, so it has to be emitted (or end the
        // answer) before the loop, or the reply starts one word late
        let next = first ?? sample(logits), done = false, pendTok = null, lkFull = false;
        if (eos(next)) done = true; else emit(next, plainDraft);
        while (!done && count < maxNew && !aborted()) {
          // a speculative step touches positions pos .. pos+K (K drafts verified in one pass) and
          // drafts one more; shrink K near the end of the context and stop before it overflows
          let K = pickK();
          const roomLeft = ctxMax() - current.engine.pos - 2;
          if (roomLeft < 1) { capped = true; break; }
          // never draft past the answer cap: every token a step writes into the caches is then an
          // emitted one, so a capped answer is still a prefix of the next turn and nothing re-prefills
          K = Math.min(K, roomLeft, maxNew - count);
          const tStep = performance.now();
          // prompt lookup first: if the text is repeating something in the context, verify what
          // followed it last time (free to guess, up to 7 at once); otherwise the draft head
          // a lookup run that was accepted in full is probably a copy in progress (code being edited,
          // a file quoted back): let the next one run long, up to what one verify can take
          const lkMax = (node ? lkFull : ai.lkFull) ? (current.engine.maxDrafts || 7) : 7;
          const lk = LOOKUP && current.engine.specStepDrafts && ai.fed ? lookupDrafts([...ai.fed, next], Math.min(lkMax, roomLeft, maxNew - count)) : [];
          const viaLookup = lk.length >= 2;
          const toks = viaLookup ? await current.engine.specStepDrafts(next, sample, lk, spec) : await current.engine.specStep(next, sample, K, spec);
          if (viaLookup) copied += toks.length - 1;
          lkFull = viaLookup && toks.length === lk.length + 1;
          if (!node) ai.lkFull = lkFull;
          // specStep wrote `next` and the accepted drafts; its last token is the next `next`
          ai.fed.push(next, ...toks.slice(0, -1));
          const tps = toks.length / ((performance.now() - tStep) / 1000);
          if (!viaLookup) {
            kc.ema[K] = kc.n[K] ? 0.6 * kc.ema[K] + 0.4 * tps : tps;
            kc.n[K] = (kc.n[K] || 0) + 1;
            if (!node) kc.used[K] = (kc.used[K] || 0) + toks.length;
          }
          for (let j = 0; j < toks.length; j++) {
            const tk = toks[j];
            if (eos(tk)) { done = true; break; }
            if (count >= maxNew) { done = true; capped = true; if (j === toks.length - 1) pendTok = tk; break; }
            emit(tk, j < toks.length - 1 ? (viaLookup ? 2 : 1) : 0);   // all but the last were drafts the trunk accepted (2: from lookup)
          }
          next = toks[toks.length - 1];
          const d = current.engine.mtp.stats.drafts - st0.drafts;
          acc = d ? (current.engine.mtp.stats.accepted - st0.accepted) / d : null;
          if (!node && ai.chain.length) pushMap(count / ((performance.now() - t0) / 1000), acc, true);
        }
        if (!done && count >= maxNew) capped = true;
        ai.pos = current.engine.pos;
        ai.xAt = ai.pos;   // specStep left the trunk hidden at ai.pos - 1 in the engine
        if (!node && pendTok != null && !aborted()) ai.pending = { next: pendTok, at: ai.pos };
        const st = current.engine.mtp.stats;
        if (!node && st.drafts) crumb(`spec: ${st.accepted}/${st.drafts} drafts accepted${ai.lapStat ? ` · lap ${Math.round(ai.lapStat.lap)}ms` : ""}`
          + (ai.chain.length ? ` · K tok/s ${kc.cand.map((k) => `${k}:${kc.ema[k] ? kc.ema[k].toFixed(1) : "-"}`).join(" ")} · tokens by K ${JSON.stringify(kc.used)}` : ""));
      } else if ((node ? useSpec && !current.engine.mtp : DENSE_SPEC) && offOk && ai.chain.length && current.engine.specStepDrafts && !current.engine.specStep) {
        // a model without a draft head (the dense Qwen3s) in a split room: every plain token costs a
        // full lap round the chain, so when prompt lookup finds the text repeating something in the
        // context, the tokens that followed it go round as drafts in the same lap (specStepDrafts,
        // exact: same output as plain decoding). No drafts: a plain lap.
        const spec = chainSpec(current);
        const st0 = { ...(current.engine.specStats || { drafts: 0, accepted: 0 }) };
        const lapT = node ? null : { v: 0, nv: 0, p: 0, np: 0, d: 0 };   // browser diagnostics: verify / plain / drafting time
        let next = first ?? sample(logits), done = false, pendTok = null, lkFull = false;
        if (eos(next)) done = true; else emit(next, 0);
        while (!done && count < maxNew && !aborted()) {
          // a step writes positions pos .. pos+K; as in plain decoding, a token is piped only while
          // pos < ctxMax() - 1, and never draft past the answer cap
          const roomLeft = ctxMax() - ai.pos - 2;
          if (roomLeft < 0) { capped = true; break; }
          const ctxNow = [...(ai.fed || []), next], kMax = Math.min(current.engine.maxDrafts || 7, roomLeft, maxNew - count);
          // only while every device in the chain handles dense verify frames (its hello's dspec; one from
          // an older build does not): otherwise plain laps. Checked every step, so a device that joins or
          // comes back with another build switches it. A lookup guess that is wrong still costs its verify
          // columns: up to 3 drafts (one batched pass per device) until a run is accepted in full, then 7.
          const chainOK = chainDenseSpec(ai.chain.map((id) => getPeerMeta(id)));
          if (!node && !chainOK && !ai.dspecWarned) { ai.dspecWarned = true; log("room", "a device in the chain runs an older build: dense speculation is off (plain laps) until it updates"); }
          if (!node && chainOK) ai.dspecWarned = false;
          let lk = (node ? ai.fed : LOOKUP) ? denseLookupDrafts(ai.chain.map((id) => getPeerMeta(id)), ctxNow, kMax, { full: node ? lkFull : ai.lkFullD }) : [], via = 2;
          // nothing to copy: the draft model's guesses, when there is one (?draft=)
          // (only when the measured lap is long enough for the drafts to pay: DraftModel.pickK)
          const dk = !node && chainOK && !lk.length && ai.draft && kMax > 0 ? ai.draft.pickK(ai.lapStat?.lap, Math.min(DRAFT_K, kMax)) : 0;
          if (dk) { const td = performance.now(); lk = await ai.draft.propose(ctxNow, dk); via = 1; lapT.d += performance.now() - td; }
          let toks;
          const tStep = node ? 0 : performance.now();
          if (lk.length) {
            current.engine.pos = ai.pos;
            toks = await current.engine.specStepDrafts(next, sample, lk, spec);
            ai.fed?.push(next, ...toks.slice(0, -1));
            ai.pos = current.engine.pos;
            ai.lastHidden = current.engine.lastHidden;
            if (via === 2) copied += toks.length - 1;
            lkFull = via === 2 && toks.length === lk.length + 1;
            if (!node) ai.lkFullD = lkFull;
            if (via === 1) ai.draft.note(lk.length, toks.length - 1);
            if (!node) { lapT.v += performance.now() - tStep; lapT.nv++; }
          } else {
            lkFull = false;
            if (!node) ai.lkFullD = false;
            const lg = await aiPipeToken(next, true, undefined, desc);
            toks = [sample(lg)];
            if (!node) { lapT.p += performance.now() - tStep; lapT.np++; }
          }
          for (let j = 0; j < toks.length; j++) {
            const tk = toks[j];
            if (eos(tk)) { done = true; break; }
            if (count >= maxNew) { done = true; capped = true; if (j === toks.length - 1) pendTok = tk; break; }
            emit(tk, j < toks.length - 1 ? via : 0);   // all but the last were drafts the chain accepted (2: lookup, 1: draft model)
          }
          next = toks[toks.length - 1];
          const st = current.engine.specStats, d = st ? st.drafts - st0.drafts : 0;
          acc = d ? (st.accepted - st0.accepted) / d : null;
          if (!node) pushMap(count / ((performance.now() - t0) / 1000), acc, true);
        }
        if (!done && count >= maxNew) capped = true;
        if (!node && pendTok != null && !aborted()) ai.pending = { next: pendTok, at: ai.pos };
        if (node) ai.xAt = ai.pos;
        const st = current.engine.specStats;
        if (!node && st?.drafts) crumb(`dense spec: ${st.accepted - st0.accepted}/${st.drafts - st0.drafts} drafts accepted · ${lapT.nv} verify laps ${lapT.nv ? (lapT.v / lapT.nv).toFixed(1) : "-"} ms · ${lapT.np} plain laps ${lapT.np ? (lapT.p / lapT.np).toFixed(1) : "-"} ms · drafting ${Math.round(lapT.d)} ms`
          + (ai.draft ? ` · draft model: ${ai.draft.stats.drafted} drafted in ${ai.draft.stats.calls} calls, ${Math.round(ai.draft.stats.ms)} ms` : ""));
      } else {
        // plain decoding. An end token is not piped through the chain: the next turn's template
        // writes <|im_end|> itself, so both paths leave the caches holding exactly prompt + answer
        // ahead (greedy GPU sampling in a chain, HOST_FUSE): from the second lap on, the head of the
        // hidden the chain returned and the host's layers on its pick are one submit (engine
        // headAhead), the hidden goes out before the token is shown, and a pick that is not piped (a
        // stop token, the cap, an abort) has its layers undone (dropAhead)
        const ahead = HOST_FUSE && ai.chain.length > 0 && desc?.kind === "greedy" && !!current.engine.canHeadAhead?.();
        let deferred = false;   // ahead: the head of ai.lastHidden has not run yet
        try {
          for (let i = 0; i < maxNew && !aborted(); i++) {
            let next, pre = null;
            const tLap = performance.now();
            if (i === 0 && first != null) next = first;
            else if (!deferred) next = sample(logits);
            else {
              const r = await current.engine.headAhead(ai.lastHidden, ai.pos, desc);
              if (r.cands.bad) throw new Error(`NaN in logits (pos ${ai.pos})${node ? "" : " — head/lm_head kernel issue on host"}`);
              logits = r.cands; deferred = false;
              next = sample(r.cands);
              if (r.h && next === r.cands.ids[0]) pre = r.h;
            }
            if (eos(next)) break;
            if (ai.pos >= ctxMax() - 1) { emit(next, plainDraft); capped = true; break; }   // no position left for another token
            if (ahead) {
              if (pre) current.engine.keepAhead(); else current.engine.dropAhead();
              logits = null; deferred = true;
              await aiPipeToken(next, true, undefined, desc, { h: pre, t0: tLap, defer: true, onSent: () => emit(next, plainDraft) });
            } else {
              emit(next, plainDraft);
              logits = await aiPipeToken(next, true, undefined, desc);
            }
            if (!node && ai.chain.length) pushMap(count / ((performance.now() - t0) / 1000), null, true);
          }
        } finally { if (ahead) current.engine.dropAhead(); }   // a pick that was not piped: its layers undone
        if (count >= maxNew) {
          capped = true;
          // for Continue: the chosen id (sample reads logits or GPU candidates alike)
          if (!node && deferred && !aborted() && ai.pos < ctxMax() - 1) {
            logits = await current.engine.headFromHiddenIds(ai.lastHidden, desc);
            if (logits.bad) throw new Error(`NaN in logits (pos ${ai.pos})${node ? "" : " — head/lm_head kernel issue on host"}`);
          }
          if (!node && logits && !aborted() && ai.pos < ctxMax() - 1) ai.pending = { next: sample(logits), at: ai.pos };
        }
      }
      tDecode = performance.now() - t0;
    } catch (err) {
      ai.fed = null;            // the caches are in an unknown state: the next request starts clean
      ai.pendingCtl = {};
      ckptClear(true);
      throw err;
    }
    if (finish) return finish({ ...prepared, tokens, count, capped, acc, copied, tPre, tDecode, reused, prefilled, ctxMax: ctxMax() }, ids, aborted);
    const secs = tDecode / 1000, tps = count / Math.max(secs, 1e-3);
    const full = capped && ai.pos >= ctxMax() - 2;
    const stats = `${count} tok · ${tps.toFixed(1)} tok/s · ${ai.chain.length + 1} device${ai.chain.length ? "s" : ""}`
      + (acc != null ? ` · ${Math.round(acc * 100)}% drafts accepted` : "")
      + (copied ? ` · ${copied} tok by lookup` : "")
      + (aborted() ? " · stopped" : "")
      + (capped ? (full ? ` · stopped: context full (${ctxMax()} tokens)` : ` · stopped at ${count} tokens`) : "");
    if (ai.chain.length && count) { pushMap(tps, acc, false, true); noteSpeeds(); }
    else if (count > 8) onSoloSpeed(tps);
    ckptSave();   // this answer's end state, on every device, for a later regenerate or branch
    const reason = aborted() ? "abort" : capped ? (full ? "ctx" : "max") : "stop";
    return { tokens, reason, reused, prefilled, count, tps, acc, copied, tPre, tDecode, preFrames, stats, capped };
  }

  return generate;
}
