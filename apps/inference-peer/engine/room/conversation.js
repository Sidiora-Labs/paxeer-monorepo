// Vendored from https://github.com/Nehanth/pooled/blob/45b5ee275a8754a37fce4a6ce174f31e4a216ef4/room/conversation.js (MIT; see LICENSE-ENGINE).
// Conversation state for the room host: the chat template, the context budget, and how much of
// the conversation the room's caches already hold, so a new turn prefills only what is new.
// DOM-free so it can be unit tested.
//
import { toolsSystemParts, callBodyText, callSeparator } from "../harness/tools.js";

// Every device keeps its KV caches and recurrent (DeltaNet) state between questions. That state
// is a function of exactly the tokens written so far (`fed`), in order. A new turn can continue
// from it only when `fed` is a strict prefix of the new conversation's ids; anything else (the
// system prompt changed, older turns were dropped to fit the context, the answer stopped on a
// token the template does not end with) means a reset and a full re-prefill, which is always
// correct, just slower. Correctness never depends on guessing: it depends on `fed` being exact.

// Answer styles the host can pick. The key goes over the wire; the text is the system prompt.
export const PERSONAS = {
  default: { label: "plain", system: "" },
  concise: { label: "concise", system: "Answer in at most three short sentences." },
  eli5: { label: "explain like I'm five", system: "Explain everything as if to a curious five-year-old: short sentences, everyday words, one vivid comparison." },
  pirate: { label: "pirate", system: "You are a cheerful pirate. Answer every question correctly and helpfully, but talk like a pirate." },
  haiku: { label: "haiku", system: "Answer every question as a single haiku (three lines, 5-7-5 syllables). Nothing else." },
  swarm: { label: "the room speaks", system: "You are a mind split across the phones and laptops in this room; every word you say takes a lap through all of them. You find this delightful and say so now and then, but you still answer the question well." },
};

// Special-token ids the template needs; throws with a readable message when the tokenizer lacks them.
export function specials(tok) {
  const V = tok.vocab;
  const s = { imStart: V["<|im_start|>"], imEnd: V["<|im_end|>"], eot: V["<|endoftext|>"], think: V["<think>"], thinkEnd: V["</think>"] };
  if (!Number.isInteger(s.imStart) || !Number.isInteger(s.imEnd))
    throw new Error("this model's tokenizer has no chat tokens (<|im_start|>, <|im_end|>)");
  return s;
}

// ChatML ids for a conversation. turns: [{role: "user", text} | {role: "assistant", ids, open?}],
// ending with a user turn, or with an open assistant turn (Continue). Assistant turns carry the exact sampled ids (never re-tokenized text, which
// can split differently) so the history matches what the caches hold token for token.
// thinking=false pre-closes the think block on every assistant turn (Qwen3 family), so answers
// come straight; the first turn's ids are the same as the single-turn template this replaces.
export function buildIds(tok, { system = "", turns, thinking = false }) {
  const S = specials(tok);
  const nl = tok.encode("\n");
  const closeThink = !thinking && S.think !== undefined && S.thinkEnd !== undefined
    ? [S.think, ...tok.encode("\n\n"), S.thinkEnd, ...tok.encode("\n\n")] : [];
  const ids = [];
  // loops, not push(...): a long text is more ids than the engine takes as arguments (RangeError)
  const add = (a) => { for (const x of a) ids.push(x); };
  if (system) { ids.push(S.imStart); add(tok.encode("system\n" + system)); ids.push(S.imEnd, ...nl); }
  for (const t of turns) {
    if (t.role === "user") { ids.push(S.imStart); add(tok.encode("user\n" + t.text)); ids.push(S.imEnd, ...nl, S.imStart, ...tok.encode("assistant\n"), ...closeThink); }
    else if (t.open) { for (const x of t.ids) ids.push(x); }   // an answer being continued: left open, no end token
    else { for (const x of t.ids) ids.push(x); ids.push(S.imEnd, ...nl); }
  }
  if (ids.some((t) => !Number.isInteger(t)))
    throw new Error("tokenizer produced an invalid token id (special tokens missing) — " + JSON.stringify(ids.slice(0, 6)));
  return ids;
}

// Fit the conversation into the context: drop the oldest whole exchanges until the prompt leaves
// `reserve` tokens for the answer. Returns { ids, turns, dropped } or throws when even the last
// question alone does not fit.
export function fitContext(tok, { system, turns, thinking }, maxSeq, reserve) {
  let t = turns.slice(), dropped = 0;
  for (;;) {
    const ids = buildIds(tok, { system, turns: t, thinking });
    if (ids.length <= maxSeq - reserve) return { ids, turns: t, dropped };
    // drop the oldest user turn and the answer that followed it; never the question being asked
    const nextUser = t.findIndex((x, i) => i > 0 && x.role === "user");
    if (nextUser < 0) {
      throw new Error(`this question is ${ids.length} tokens; the room's context is ${maxSeq} tokens and an answer needs at least ${reserve}. Shorten it.`);
    }
    t = t.slice(nextUser);
    dropped++;
  }
}

// How many leading tokens of `ids` the caches already hold: fed.length when fed is a strict
// prefix of ids (at least one new token is left to prefill, since the last prompt token must run
// through the head), else 0, meaning reset and prefill everything.
export function reusablePrefix(fed, ids) {
  if (!fed || !fed.length || fed.length >= ids.length) return 0;
  for (let i = 0; i < fed.length; i++) if (fed[i] !== ids[i]) return 0;
  return fed.length;
}

// Split raw answer text into the thinking part and the answer part. A think block that has not
// closed yet (still streaming) is all thinking.
export function splitThink(raw) {
  const a = raw.indexOf("<think>");
  if (a < 0) return { think: null, answer: raw };
  const b = raw.indexOf("</think>", a);
  if (b < 0) return { think: raw.slice(a + 7).trim(), answer: raw.slice(0, a), open: true };
  return { think: raw.slice(a + 7, b).trim(), answer: raw.slice(0, a) + raw.slice(b + 8).replace(/^\s+/, "") };
}

// ---- API asks, v2 (docs/design/serve.md "Rendering"): tools, calls, reasoning, per template ----

// What a model's GGUF chat template does, read once per loaded model from its text (nothing is
// hard-coded per model name):
//   style         "json" (Hermes <tool_call>{...}</tool_call>, Qwen3) or "xml" (<function=...>, Qwen3.5+)
//   tools         the template has a tool-call format at all
//   effortLine    the system turn starts with a "Reasoning effort is set to ..." sentence (Qwen3.8)
//   thinkInPrompt the generation prompt opens the think block itself ("<think>\n")
//   thinkRule     which past assistant turns keep their think block:
//                   all                 every one (Qwen3.8: preserve_thinking undefined = keep)
//                   afterQuery          those after the last real user query, always (maybe empty)
//                   afterQueryNonEmpty  those after it whose reasoning is not empty (Qwen3)
//   trim          user / assistant / tool / system text is trimmed (render_content(...)|trim)
//   known         the thinking rule was recognized (else afterQuery, logged by the caller)
// No template in hand (a model whose tokenizer came from tokenizer.json): Qwen3's, when the
// vocabulary has <tool_call>.
export function templateProfile(chatTemplate, tok = null) {
  const t = String(chatTemplate || "");
  const hasCallTok = Number.isInteger(tok?.vocab?.["<tool_call>"]);
  if (!t) return { style: "json", tools: hasCallTok, effortLine: false, thinkInPrompt: false, thinkRule: "afterQueryNonEmpty", trim: false, known: true, fallback: true };
  const tools = /<tool_call>/.test(t);
  const style = /<function=|<parameter=/.test(t) ? "xml" : "json";
  const gen = t.slice(Math.max(0, t.lastIndexOf("add_generation_prompt")));
  const thinkInPrompt = /'<think>\\n'\s*-?\}\}/.test(gen);
  let thinkRule = "afterQuery", known = true;
  if (/preserve_thinking is undefined or preserve_thinking is true/.test(t)) thinkRule = "all";
  else if (/not loop\.last and reasoning_content/.test(t)) thinkRule = "afterQueryNonEmpty";
  else if (!/loop\.index0 > ns\.last_query_index/.test(t)) known = false;
  return { style, tools, effortLine: /Reasoning effort is set to/.test(t), thinkInPrompt, thinkRule, trim: /\|\s*trim/.test(t) && /render_content\(message\.content, true\)\|trim/.test(t), known };
}

export const EFFORT_TEXT = {
  xhigh: "Reasoning effort is set to xhigh. Please think carefully through the task, validate key assumptions, consider plausible alternatives, and prioritize correctness, consistency, and clarity in the final answer.",
  low: "Reasoning effort is set to low. Keep your thinking brief and focused, moving directly to the conclusion without unnecessary elaboration.",
};
// the Qwen3.8 effort sentence for a request: low -> low, medium -> none, anything else -> xhigh
export const effortPrefix = (profile, thinking, effort) => (!profile.effortLine || !thinking ? "" : effort === "low" ? EFFORT_TEXT.low : effort === "medium" ? "" : EFFORT_TEXT.xhigh);

// The ids of the generation header's tail after "assistant\n": what the answer is sampled after.
//   thinking, the template opens the block: <think> "\n"
//   thinking, the model opens it:           (nothing)
//   thinking off:                           <think> "\n\n" </think> "\n\n"   (as buildIds)
export function headerTail(tok, S, profile, thinking) {
  if (S.think === undefined || S.thinkEnd === undefined) return [];
  if (!thinking) return [S.think, ...tok.encode("\n\n"), S.thinkEnd, ...tok.encode("\n\n")];
  return profile.thinkInPrompt ? [S.think, ...tok.encode("\n")] : [];
}

// Render an API conversation to ids the way the model's chat template renders it, structurally:
// every tag the renderer writes (<|im_start|>, <|im_end|>, <think>, </think>, <tool_call>,
// </tool_call>, <tool_response>, </tool_response>) is its special id when the vocabulary has one,
// and every client string (system, user text, tool output, reasoning, content, arguments, the tool
// list) is encoded as plain text, so no client text can become a special token.
//   req: { system, tools, messages: [{role:"user",text,aside?} | {role:"assistant",text,calls?,reasoning?} | {role:"tool",text}],
//          params: { thinking, effort } }   (messages normalized: see cli/lib/common.js normalizeMessages)
//   o.encode(text) -> ids (a cache in front of tok.encode); o.turnIds(j, m) -> the exact ids of assistant
//   message j from the answer cache ({ ids, thinkEnd }) or null
// -> { ids, systemLen, exact }   systemLen: the ids of the system turn (the pin)
export function renderApi(tok, req, profile, { encode = (s) => tok.encode(s), turnIds = () => null, thinking = !!req.params?.thinking } = {}) {
  const S = specials(tok), V = tok.vocab || {};
  const sp = (tag) => (Number.isInteger(V[tag]) ? V[tag] : null);
  const T = { call: sp("<tool_call>"), callEnd: sp("</tool_call>"), resp: sp("<tool_response>"), respEnd: sp("</tool_response>") };
  const out = [];
  let run = "";
  // a loop, not push(...): a long request's text is hundreds of thousands of ids, past the engine's
  // argument limit (RangeError)
  const flush = () => { if (run) { for (const x of encode(run)) out.push(x); run = ""; } };
  const text = (s) => { run += s; };
  const ids = (a) => { flush(); for (const x of a) out.push(x); };
  const tag = (id, textForm) => { if (id == null) text(textForm); else ids([id]); };
  const tr = (s) => (profile.trim ? String(s).trim() : String(s));
  const style = profile.style;
  // tool_choice none: the tools are left out of the prompt (with them listed, models write call
  // markup anyway, in any spelling the grammar has not banned: seen from Qwen3.6 as "<tool.call>")
  const tools = req.tools && req.tools.length && req.params?.toolChoice !== "none" ? req.tools : null;

  // the template's own text: its tags are special ids (client text never is)
  const TAG_RE = new RegExp(Object.entries({ "<tool_call>": T.call, "</tool_call>": T.callEnd, "<tool_response>": T.resp, "</tool_response>": T.respEnd, "<think>": S.think, "</think>": S.thinkEnd })
    .filter(([, id]) => Number.isInteger(id)).map(([t]) => t.replace(/[/|]/g, "\\$&")).join("|") || "(?!)", "g");
  const literal = (s) => {
    let at = 0;
    for (const m of s.matchAll(TAG_RE)) { text(s.slice(at, m.index)); ids([V[m[0]]]); at = m.index + m[0].length; }
    text(s.slice(at));
  };

  // system turn
  const prefix = effortPrefix(profile, thinking, req.params?.effort);
  const system = tr(req.system || "");
  const parts = toolsSystemParts(tools, { style, system, prefix });
  if (parts.some((p) => p.text)) {
    ids([S.imStart]); text("system\n");
    for (const p of parts) if (p.lit) literal(p.text); else text(p.text);
    ids([S.imEnd]); text("\n");
  }
  flush();
  const systemLen = out.length;

  // the last real user query (not an aside, not tool results): reasoning before it is dropped
  const msgs = req.messages;
  let lastQuery = -1;
  for (let j = msgs.length - 1; j >= 0; j--) if (msgs[j].role === "user" && !msgs[j].aside) { lastQuery = j; break; }
  let exact = 0;
  const closedBlock = S.think !== undefined && S.thinkEnd !== undefined ? [S.think, ...tok.encode("\n\n"), S.thinkEnd, ...tok.encode("\n\n")] : [];
  for (let j = 0; j < msgs.length; j++) {
    const m = msgs[j];
    if (m.role === "user") { ids([S.imStart]); text("user\n" + tr(m.text)); ids([S.imEnd]); text("\n"); continue; }
    if (m.role === "tool") {
      ids([S.imStart]); text("user");
      for (; j < msgs.length && msgs[j].role === "tool"; j++) {
        text("\n"); tag(T.resp, "<tool_response>"); text("\n" + tr(msgs[j].text) + "\n"); tag(T.respEnd, "</tool_response>");
      }
      j--;
      ids([S.imEnd]); text("\n");
      continue;
    }
    // assistant
    ids([S.imStart]); flush(); out.push(...tok.encode("assistant\n"));
    const reasoning = String(m.reasoning || "");
    const hit = turnIds(j, m);
    const reasoned = !!reasoning.trim() || !!hit?.reasoned;
    // Thinking off: every turn without reasoning keeps the pre-closed empty block it was sampled
    // after (as buildIds, and v1, render it). The templates drop it before the last query (Qwen3
    // everywhere); keeping it is what the model saw and keeps each prompt an extension of the last,
    // so the room's caches reuse the whole conversation instead of re-prefilling it. With thinking
    // on, each template's rule; a Qwen3 answer sampled after the empty block keeps it after the query.
    const block = !thinking && !reasoned ? true
      : profile.thinkRule === "all" || (j > lastQuery && (profile.thinkRule === "afterQuery" || reasoned || (!!hit && hit.thinkEnd > 0)));
    if (hit) {
      // the exact ids it was sampled as (header tail included): with the block, or from the content on
      exact++;
      ids(block ? hit.ids : hit.ids.slice(hit.thinkEnd));
    } else {
      const content = profile.trim ? String(m.text || "").trim() : style === "json" && block ? String(m.text || "").replace(/^\n+/, "") : String(m.text || "");
      if (block) {
        const r = profile.trim ? reasoning.trim() : reasoning.replace(/^\n+|\n+$/g, "");
        if (!r && closedBlock.length) ids(closedBlock);
        else { tag(S.think ?? null, "<think>"); text("\n" + r + "\n"); tag(S.thinkEnd ?? null, "</think>"); text("\n\n"); }
      }
      text(content);
      (m.calls || []).forEach((c, i) => {
        text(callSeparator(i, content, style));
        tag(T.call, "<tool_call>"); text(callBodyText({ name: c.name, arguments: c.args }, style)); tag(T.callEnd, "</tool_call>");
      });
    }
    ids([S.imEnd]); text("\n");
  }
  // the generation header
  ids([S.imStart]); flush(); out.push(...tok.encode("assistant\n"), ...headerTail(tok, S, profile, thinking));
  if (out.some((t) => !Number.isInteger(t))) throw new Error("tokenizer produced an invalid token id (special tokens missing)");
  return { ids: out, systemLen, exact };
}
