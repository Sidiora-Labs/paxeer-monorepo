// Vendored from https://github.com/Nehanth/pooled/blob/45b5ee275a8754a37fce4a6ce174f31e4a216ef4/harness/tools.js (MIT; see LICENSE-ENGINE).
// Tool calling for the Qwen chat template: how tools are described to the model, how its tool
// calls are recognised in the streamed answer, and how results go back. DOM-free and
// engine-free so the harness (and the unit tests) can use it anywhere.
//
// Two formats exist in the Qwen family, and a model is trained on exactly one of them:
//   "json" (Qwen2.5 / Qwen3, Hermes-style):
//       <tool_call>
//       {"name": "read_file", "arguments": {"path": "src/a.js"}}
//       </tool_call>
//   "xml" (Qwen3-Coder, Qwen3.5 and later):
//       <tool_call>
//       <function=read_file>
//       <parameter=path>
//       src/a.js
//       </parameter>
//       </function>
//       </tool_call>
// Results go back in a user turn as <tool_response> ... </tool_response> blocks, one per call,
// in call order. detectStyle() reads the GGUF's own chat template (tokenizer.chat_template) so the
// prompt matches what the model was trained on; the parser accepts both formats either way, plus
// the near misses room models write: a ```json fence inside the call, JSON arguments inside
// <function=NAME>, <function name="x"> / <parameter name="p"> / <invoke> spellings, and several
// calls in one <tool_call> block (splitCallBody).

// tools: [{ name, description, parameters: JSON schema object }]
import { schemaTypes, stringCapable } from "./jsonschema.js";

export function detectStyle(chatTemplate = "") {
  return /<function=|<parameter=/.test(chatTemplate) ? "xml" : "json";
}

const fnJSON = (t) => JSON.stringify({ type: "function", function: { name: t.name, description: t.description || "", parameters: t.parameters || { type: "object", properties: {} } } });

// JSON as the chat templates' `tojson` writes it (Python's json.dumps: ", " and ": " separators,
// keys in their order, non-ASCII as is). The API path renders tools and past calls with it, so the
// prompt is byte for byte what the model saw in training.
export function pyJSON(v) {
  if (v === null || v === undefined) return "null";
  if (typeof v === "string") return JSON.stringify(v);
  if (typeof v === "number") return Number.isFinite(v) ? JSON.stringify(v) : "null";
  if (typeof v === "boolean") return v ? "true" : "false";
  if (Array.isArray(v)) return "[" + v.map(pyJSON).join(", ") + "]";
  if (typeof v === "object") return "{" + Object.entries(v).filter(([, x]) => x !== undefined).map(([k, x]) => JSON.stringify(k) + ": " + pyJSON(x)).join(", ") + "}";
  return "null";
}
// a tool as the templates list it: {"type": "function", "function": {...}}
const fnPy = (t) => pyJSON({ type: "function", function: { name: t.name, ...(t.description ? { description: t.description } : {}), parameters: t.parameters || { type: "object", properties: {} } } });

// The tool block the GGUF chat templates write into the system turn, exactly (the API path):
//   json (Qwen3):   system + "\n\n" + "# Tools ... <tools>" one tool per line "</tools> ... JSON call format"
//   xml (Qwen3.5+): [prefix + "\n\n"] "# Tools ... <tools>...</tools>" + XML call format + <IMPORTANT>, then "\n\n" + system
// prefix: the Qwen3.8 template's reasoning-effort sentence.
export function toolsSystemPromptExact(tools, opts = {}) { return toolsSystemParts(tools, opts).map((p) => p.text).join(""); }
// The same as parts: { text, lit } where lit marks the template's own text (its tags, e.g. the
// <tool_call> in the format example, are the model's special tokens, as a tokenizer reading the
// whole rendered prompt would make them) and the rest is the client's (system text, tool JSON).
export function toolsSystemParts(tools, { style = "json", system = "", prefix = "" } = {}) {
  const lit = (text) => ({ text, lit: true }), own = (text) => ({ text, lit: false });
  if (!tools || !tools.length) return [lit(prefix ? prefix + (system ? "\n\n" : "") : ""), own(system)];
  const list = [lit("<tools>"), ...tools.flatMap((t) => [lit("\n"), own(fnPy(t))]), lit("\n</tools>")];
  if (style === "xml") {
    return [lit((prefix ? prefix + "\n\n" : "") + "# Tools\n\nYou have access to the following functions:\n\n"), ...list, lit(XML_FORMAT + (system ? "\n\n" : "")), own(system)];
  }
  return [own(system), lit((system ? "\n\n" : "") + "# Tools\n\nYou may call one or more functions to assist with the user query.\n\nYou are provided with function signatures within <tools></tools> XML tags:\n"),
    ...list, lit("\n\nFor each function call, return a json object with function name and arguments within <tool_call></tool_call> XML tags:\n<tool_call>\n{\"name\": <function-name>, \"arguments\": <args-json-object>}\n</tool_call>")];
}
const XML_FORMAT = "\n\nIf you choose to call a function ONLY reply in the following format with NO suffix:\n\n"
  + "<tool_call>\n<function=example_function_name>\n<parameter=example_parameter_1>\nvalue_1\n</parameter>\n"
  + "<parameter=example_parameter_2>\nThis is the value for the second parameter\nthat can span\nmultiple lines\n</parameter>\n</function>\n</tool_call>\n\n"
  + "<IMPORTANT>\nReminder:\n- Function calls MUST follow the specified format: an inner <function=...></function> block must be nested within <tool_call></tool_call> XML tags\n"
  + "- Required parameters MUST be specified\n- You may provide optional reasoning for your function call in natural language BEFORE the function call, but NOT after\n"
  + "- If there is no function call available, answer the question like normal with your current knowledge and do not tell the user about function calls\n</IMPORTANT>";

// The text between <tool_call> and </tool_call> for one past call, as the templates render it:
//   json: "\n{\"name\": \"f\", \"arguments\": {...}}\n"
//   xml:  "\n<function=f>\n<parameter=k>\nV\n</parameter>\n</function>\n"   (V: a string as is, else tojson)
export function callBodyText(call, style = "json") {
  const args = call.arguments ?? call.args ?? {};
  if (style === "xml") {
    let s = "\n<function=" + call.name + ">\n";
    if (args && typeof args === "object" && !Array.isArray(args)) for (const [k, v] of Object.entries(args)) s += "<parameter=" + k + ">\n" + (typeof v === "string" ? v : pyJSON(v)) + "\n</parameter>\n";
    return s + "</function>\n";
  }
  return "\n{\"name\": \"" + call.name + "\", \"arguments\": " + (typeof args === "string" ? args : pyJSON(args)) + "}\n";
}
// what goes before call i of an assistant turn whose content is `content`
export function callSeparator(i, content, style = "json") {
  if (i > 0) return "\n";
  if (style === "xml") return content.trim() ? "\n\n" : "";
  return content ? "\n" : "";
}

// The system prompt with the tool block appended, as the Qwen templates render it (Code mode's
// wording; the API path uses toolsSystemPromptExact).
export function toolsSystemPrompt(tools, { style = "json", system = "" } = {}) {
  if (!tools || !tools.length) return system;
  const list = "# Tools\n\nYou have access to the following functions:\n\n<tools>\n" + tools.map(fnJSON).join("\n") + "\n</tools>";
  if (style === "xml") {   // Qwen3.5+ templates: tool block first, the caller's system text appended
    return list + "\n\nIf you choose to call a function ONLY reply in the following format with NO suffix:\n\n"
      + "<tool_call>\n<function=example_function_name>\n<parameter=example_parameter_1>\nvalue_1\n</parameter>\n"
      + "<parameter=example_parameter_2>\nThis is the value for the second parameter\nthat can span\nmultiple lines\n</parameter>\n</function>\n</tool_call>\n\n"
      + "<IMPORTANT>\nReminder:\n- Function calls MUST follow the specified format: an inner <function=...></function> block must be nested within <tool_call></tool_call> XML tags\n"
      + "- Required parameters MUST be specified\n- You may provide optional reasoning for your function call in natural language BEFORE the function call, but NOT after\n"
      + "- If there is no function call available, answer the question like normal with your current knowledge and do not tell the user about function calls\n</IMPORTANT>"
      + (system ? "\n\n" + system : "");
  }
  return (system ? system + "\n\n" : "") + list + "\n\nFor each function call, return a json object with function name and arguments within <tool_call></tool_call> XML tags:\n"
    + "<tool_call>\n{\"name\": <function-name>, \"arguments\": <args-json-object>}\n</tool_call>";
}

// The text of a tool-results user turn: one <tool_response> block per result, in call order.
export function toolResponses(results) {
  return results.map((r) => "<tool_response>\n" + (typeof r === "string" ? r : JSON.stringify(r)) + "\n</tool_response>").join("\n");
}

// An assistant turn's tool calls rendered back in the model's format (for history rebuilt from
// structured calls; a live session should keep the sampled ids instead, see room/conversation.js).
export function renderCalls(calls, style = "json") {
  return calls.map((c) => style === "xml"
    ? "<tool_call>\n<function=" + c.name + ">\n" + Object.entries(c.arguments || {}).map(([k, v]) =>
      "<parameter=" + k + ">\n" + (typeof v === "string" ? v : JSON.stringify(v)) + "\n</parameter>\n").join("") + "</function>\n</tool_call>"
    : "<tool_call>\n" + JSON.stringify({ name: c.name, arguments: c.arguments || {} }) + "\n</tool_call>").join("\n");
}

// Parse one <tool_call> body (the text between the tags) in either format.
// Returns { name, arguments } or { error, raw } when the model produced something malformed.
export function parseCallBody(body, schemaFor = () => null) {
  const raw = body;
  const b = unfence(normalizeXmlCall(body).trim());
  const fm = /^<function=([^>\s]+)>([\s\S]*?)(?:<\/function>\s*)?$/.exec(b);
  if (fm) {
    const name = fm[1], args = {};
    const props = schemaFor(name)?.properties || {};
    // a missing </parameter> before the next parameter (on a line of its own) or </function> is tolerated
    // (seen in the wild); a <parameter= inside a line is the value's own text
    const re = /<parameter=([^>\s]+)>\n?([\s\S]*?)(?:\n?<\/parameter>|\n(?=<parameter=)|\n?$)/g;
    let m;
    while ((m = re.exec(fm[2]))) args[m[1]] = coerce(m[2], props[m[1]], schemaFor(name));
    // no <parameter=> at all but a JSON object: <function=serve>\n{"dir": "."}\n</function>
    if (!Object.keys(args).length && /^\s*\{/.test(fm[2])) {
      try { const o = parseLooseJSON(unfence(fm[2].trim())); if (o && typeof o === "object" && !Array.isArray(o)) return { name, arguments: o.arguments ?? o.parameters ?? o }; } catch { /* no arguments */ }
    }
    return { name, arguments: args };
  }
  try {
    let o = parseLooseJSON(b);
    if (Array.isArray(o) && o.length === 1) o = o[0];   // [{"name": ...}]
    if (o && typeof o.function === "object" && o.function) o = o.function;   // OpenAI's {"type": "function", "function": {...}}
    const name = o && [o.name, o.tool, o.function, o.tool_name].find((v) => typeof v === "string");
    if (!name) return { error: "tool call has no name", raw };
    const k = ["arguments", "parameters", "args", "input", "params"].find((x) => o[x] !== undefined);
    let a;
    if (k) a = o[k];
    else {
      // the arguments written next to the name: {"name": "serve", "dir": "."}
      a = {};
      for (const [x, v] of Object.entries(o)) if (!["name", "tool", "function", "tool_name", "type", "id"].includes(x)) a[x] = v;
    }
    if (typeof a === "string") { try { a = parseLooseJSON(a); } catch { /* leave as text */ } }
    return { name: name.trim(), arguments: a ?? {} };
  } catch (e) {
    const msg = String(e.message || e);
    return { error: "tool call is not valid JSON: " + (msg.length > 120 ? msg.slice(0, 120) + "…" : msg), raw };
  }
}

// The XML call spellings other models use, as the Qwen one: <function name="x"> / <invoke name="x">
// for <function=x>, <parameter name="p"> for <parameter=p>, and a quoted <function="x">. Only tags
// at the start of a line (or right after the opener) change, so a value that merely quotes one is
// left alone.
export function normalizeXmlCall(text) {
  if (!/<(?:function|invoke|parameter)[\s="']/.test(text)) return text;
  return text
    .replace(/(^|\n)([ \t]*)<(?:function|invoke)\s+name\s*=\s*["']?([^"'>\s]+)["']?\s*>/g, "$1$2<function=$3>")
    .replace(/(^|\n)([ \t]*)<\/invoke>/g, "$1$2</function>")
    .replace(/(^|\n)([ \t]*)<function=["']([^"'>\s]+)["']\s*>/g, "$1$2<function=$3>")
    .replace(/(^|\n)([ \t]*)<parameter\s+name\s*=\s*["']?([^"'>\s]+)["']?\s*>/g, "$1$2<parameter=$3>")
    .replace(/(^|\n)([ \t]*)<parameter=["']([^"'>\s]+)["']\s*>/g, "$1$2<parameter=$3>");
}

// a call body wrapped in a Markdown fence (```json ... ```), as small models write it
// (only a fence that opens the body: a value that merely ends with ``` keeps it)
const unfence = (s) => (/^```/.test(s) ? s.replace(/^```[\w-]*[ \t]*\n?/, "").replace(/\n?```\s*$/, "") : s);

// One <tool_call> body that holds several calls, as separate bodies: <function=a>..</function>
// <function=b>..</function> (a stray <tool_call> between them, the closer forgotten, is dropped),
// a JSON array of calls, or JSON objects one after another. A body with one call comes back as is.
export function splitCallBody(body) {
  const b = normalizeXmlCall(body);
  // (both tags on lines of their own: a value that merely contains them stays one call)
  const between = /(?<=(?:^|\n)[ \t]*<\/function>)[ \t]*\n\s*(?:<\/?tool_call>\s*)*(?=<function=[^>\s<]+>)/;
  if (between.test(b)) return b.split(new RegExp(between.source, "g")).filter((s) => s.trim());
  const t = unfence(b.trim());
  if (!t.startsWith("{") && !t.startsWith("[")) return [body];
  const named = (o) => o && typeof o === "object" && [o.name, o.tool, o.function?.name ?? o.function].some((v) => typeof v === "string");
  try {
    const o = JSON.parse(t);
    if (Array.isArray(o) && o.length > 1 && o.every(named)) return o.map((x) => JSON.stringify(x));
    return [body];
  } catch { /* maybe several objects */ }
  const parts = [];
  let at = 0;
  while (at < t.length) {
    const s = t.indexOf("{", at);
    if (s < 0 || t.slice(at, s).trim()) break;
    const e = objectEnd(t, s);
    if (e < 0) break;
    parts.push(t.slice(s, e));
    at = e;
  }
  if (parts.length > 1 && !t.slice(at).trim()) {
    try { if (parts.every((p) => named(JSON.parse(p)))) return parts; } catch { /* not clean JSON */ }
  }
  return [body];
}

// the index just past the JSON object that starts at `at` (strings respected), or -1 when it never closes
function objectEnd(text, at) {
  let depth = 0, inStr = false, esc = false;
  for (let i = at; i < text.length; i++) {
    const c = text[i];
    if (inStr) { if (esc) esc = false; else if (c === "\\") esc = true; else if (c === '"') inStr = false; continue; }
    if (c === '"') inStr = true;
    else if (c === "{" || c === "[") depth++;
    else if ((c === "}" || c === "]") && --depth === 0) return i + 1;
  }
  return -1;
}

// JSON.parse, then the near misses small models write for a call: extra or missing closing braces,
// and a stray "{" before "arguments" ({"name": "x", {"arguments": {...}}}). Strings are respected.
// Also: raw newlines / tabs inside strings (escaped), trailing commas, and with { open: true } a
// string the answer ended inside (closed there, for salvaging a cut write).
export function parseLooseJSON(s, { open = false } = {}) {
  try { return JSON.parse(s); } catch (first) {
    let t = s.trim().replace(/("name"\s*:\s*"[^"]*"\s*,\s*)\{\s*(?="(?:arguments|parameters)"\s*:)/, "$1");
    // balance braces outside strings: drop unmatched closers, close what is still open
    let out = "", depth = 0, inStr = false, esc = false;
    const stack = [];
    for (const c of t) {
      if (inStr) {
        if (esc) { esc = false; out += c; continue; }
        if (c === "\\") { esc = true; out += c; continue; }
        if (c === '"') inStr = false;
        out += c === "\n" ? "\\n" : c === "\r" ? "\\r" : c === "\t" ? "\\t" : c;
        continue;
      }
      if (c === '"') { inStr = true; out += c; continue; }
      if (c === "{" || c === "[") { depth++; stack.push(c === "{" ? "}" : "]"); }
      if (c === "}" || c === "]") {
        if (depth === 0) continue;
        depth--; stack.pop();
        out = out.replace(/,\s*$/, "");   // a trailing comma
      }
      out += c;
      if (depth === 0 && out.trim().startsWith("{") && (c === "}")) break;   // one object: ignore what follows it
    }
    if (inStr) {
      if (!open) throw first;
      if (esc) out = out.slice(0, -1);   // a cut escape
      out += '"';
    }
    out = out.replace(/,\s*$/, "").replace(/(?:,|:)\s*$/, (m) => (m.trim() === ":" ? ": null" : ""));
    while (stack.length) out += stack.pop();
    return JSON.parse(out);
  }
}

// XML parameters are text: turn them into what the schema allows, trying the types it lists in
// vLLM's order (null, integer, number, boolean, object, array, string); no schema: a JSON value
// when the text is one (not a string), else the text.
export function coerce(text, schema, root = schema) {
  const types = schemaTypes(schema, root || schema);
  if (!types) {
    try { const v = JSON.parse(text); if (typeof v !== "string") return v; } catch { /* text */ }
    return text;
  }
  const t = text.trim();
  for (const ty of COERCE_ORDER) {
    if (!types.has(ty)) continue;
    if (ty === "null" && t === "null") return null;
    if (ty === "integer" && /^-?\d+$/.test(t)) { const n = Number(t); if (Number.isSafeInteger(n)) return n; }
    if (ty === "number" && /^-?(?:\d+\.?\d*|\.\d+)(?:[eE][+-]?\d+)?$/.test(t)) { const n = Number(t); if (Number.isFinite(n)) return n; }
    if (ty === "boolean") { const b = t.toLowerCase(); if (b === "true") return true; if (b === "false") return false; }
    if (ty === "object" || ty === "array") {
      try { const v = JSON.parse(t); if (ty === "object" ? v && typeof v === "object" && !Array.isArray(v) : Array.isArray(v)) return v; } catch { /* not JSON */ }
    }
    if (ty === "string") return text;
  }
  return text;
}
const COERCE_ORDER = ["null", "integer", "number", "boolean", "object", "array", "string"];

// Streaming recogniser: feed() the decoded answer text as it grows (deltas); get back the text
// that is safe to show (never a half-typed "<tool_c") and any tool calls completed so far.
// end() flushes the rest; an unterminated <tool_call> at the end is reported as an error call.
export class ToolCallParser {
  constructor({ schemaFor = () => null } = {}) {
    this.buf = ""; this.inCall = false; this.calls = []; this.schemaFor = schemaFor;
  }
  feed(delta) {
    this.buf += delta;
    let text = "";
    const calls = [];
    for (;;) {
      if (!this.inCall) {
        if (this.eatNL && this.buf) { if (this.buf[0] === "\n") this.buf = this.buf.slice(1); this.eatNL = false; }   // the newline between calls
        const i = this.buf.indexOf("<tool_call>");
        if (i < 0) {
          // hold back a tail that could be the start of the tag
          const keep = partialTagTail(this.buf, "<tool_call>");
          text += this.buf.slice(0, this.buf.length - keep);
          this.buf = this.buf.slice(this.buf.length - keep);
          break;
        }
        text += this.buf.slice(0, i);
        this.buf = this.buf.slice(i + "<tool_call>".length);
        this.inCall = true;
      } else {
        const j = this.buf.indexOf("</tool_call>");
        if (j < 0) break;
        for (const body of splitCallBody(this.buf.slice(0, j))) {
          const c = parseCallBody(body, this.schemaFor);
          calls.push(c); this.calls.push(c);
        }
        this.buf = this.buf.slice(j + "</tool_call>".length);
        this.inCall = false; this.eatNL = true;
      }
    }
    return { text, calls };
  }
  end() {
    const r = { text: "", calls: [] };
    if (this.inCall) {
      // a model that stops right after </function> without closing the call still meant it; without
      // </function> the answer was cut mid-call (length cap) and its last value is a fragment
      // also accept a call that ends right after a closed parameter (seen from Qwen: no </function>)
      const buf = normalizeXmlCall(this.buf);
      let done = /<\/function>/.test(buf) || /^\s*<function=[^>\s]+>[\s\S]*<\/parameter>\s*$/.test(buf);
      // a JSON call the model ended without </tool_call>: complete if it parses (loosely)
      if (!done && /^\s*\{/.test(this.buf)) { try { const o = parseLooseJSON(this.buf.trim()); done = !!o && typeof o.name === "string" && (o.arguments !== undefined || o.parameters !== undefined); } catch { /* still open */ } }
      const cs = done ? splitCallBody(this.buf).map((b) => parseCallBody(b, this.schemaFor)) : [{ error: "unterminated <tool_call>", raw: this.buf, open: true }];
      r.calls.push(...cs); this.calls.push(...cs);
    } else r.text = this.buf;
    this.buf = ""; this.inCall = false;
    return r;
  }
}

function partialTagTail(s, tag) {
  for (let k = Math.min(tag.length - 1, s.length); k > 0; k--) if (tag.startsWith(s.slice(s.length - k))) return k;
  return 0;
}


// ---------------------------------------------------------------------------------------------
// CallStream: the API path's incremental parser (docs/design/serve.md "Decoding pipeline"). Fed
// the answer text (after the reasoning) as it is sampled, it returns events:
//   { t: "text", text }         content, never a half-written "<tool_call>" (held back until it
//                               cannot be one); whitespace-only content around calls is dropped
//   { t: "call", i, name }      call i started; its name is complete and declared (and allowed)
//   { t: "args", i, a }         the next fragment of call i's arguments JSON
//   { t: "end", i, args }       call i complete; args === the concatenated fragments (a JSON object)
// end() flushes; `open` is then the call the answer ended inside ({ i, name, args } or null).
// Invariant: the fragments of a call concatenate to its final `args`, which parses to what
// parseCallBody() makes of the whole body. XML values stream as they are written: a string-capable
// value as a JSON string (escaped as it goes, the trailing "\n" before </parameter> held), arrays
// and objects as their JSON text (when the grammar guarantees it), anything that needs coercion
// (numbers, booleans, null, unions) is held to </parameter>. JSON-style calls stream the arguments
// object as written. A body that is not in the canonical shape (possible only without the grammar)
// is buffered and parsed whole at </tool_call>; if it does not parse, its markup becomes content.
// A body that goes wrong after its name went out ends with the parsed arguments and mismatch: true.
// ---------------------------------------------------------------------------------------------
const CLOSE = "</tool_call>", FN_OPEN = "<function=", P_OPEN = "<parameter=", P_CLOSE = "</parameter>", FN_CLOSE = "</function>";
const canon = (v) => JSON.stringify(sortKeys(v));
function sortKeys(v) {
  if (Array.isArray(v)) return v.map(sortKeys);
  if (v && typeof v === "object") { const o = {}; for (const k of Object.keys(v).sort()) o[k] = sortKeys(v[k]); return o; }
  return v;
}
// the longest tail of s that is a prefix of one of the patterns
function holdFor(s, pats) {
  let best = 0;
  for (const p of pats) for (let k = Math.min(p.length - 1, s.length); k > best; k--) if (p.startsWith(s.slice(s.length - k))) { best = k; break; }
  return best;
}
const escJSON = (s) => JSON.stringify(s).slice(1, -1);

export class CallStream {
  // tools: [{ name, parameters }]; allowed: the names a call may use (default: all of them);
  // constrained: the answer was sampled under the grammar (arrays / objects may stream raw)
  constructor({ style = "xml", tools = [], allowed = null, constrained = true } = {}) {
    this.style = style === "json" ? "json" : "xml";
    this.schemas = new Map(tools.map((t) => [t.name, t.parameters || null]));
    this.allowed = new Set(allowed || tools.map((t) => t.name));
    this.constrained = constrained;
    this.buf = ""; this.mode = "text"; this.ls = true;
    this.ws = "";            // trailing whitespace of the content, held until more content comes
    this.calls = [];         // [{ name, args }] complete
    this.cur = null;         // the call being parsed { i, name, frag, raw, st, bare, ... }
    this.open = null;
    this.nText = 0;          // content characters emitted
  }
  get n() { return this.calls.length + (this.cur?.name ? 1 : 0); }
  push(text) { this.buf += text; const out = []; this._run(out, false); return out; }
  end() {
    const out = [];
    this._run(out, true);
    if (this.mode === "call" && this.cur) {
      const c = this.cur;
      // complete when the body closed (</function>, or the JSON object) and only </tool_call> is missing
      if (c.name && (c.st === "afterFn" || c.st === "jtail")) this._finish(out);
      else if (c.name) this.open = { i: c.i, name: c.name, args: c.frag };
      // a call cut before its name was complete: nothing of it was shown, and nothing is
      this.cur = null; this.buf = ""; this.mode = "text";
    }
    if (this.mode === "text" && this.buf) { this._text(out, this.buf); this.buf = ""; }
    if (this.ws && !this.calls.length && !this.open) { out.push({ t: "text", text: this.ws }); this.nText += this.ws.length; }
    this.ws = "";
    return out;
  }

  _text(out, s) {
    if (!s) return;
    const lead = this.ws + s;
    const m = /\s*$/.exec(lead);
    const body = lead.slice(0, lead.length - m[0].length);
    this.ws = m[0];
    if (body) {
      // content after calls: only its non-whitespace part counts
      const text = this.calls.length && !this.nText ? body.replace(/^\s+/, "") : body;
      if (text) { out.push({ t: "text", text }); this.nText += text.length; }
    }
  }
  _emit(out, a) { if (!a) return; this.cur.frag += a; out.push({ t: "args", i: this.cur.i, a }); }
  _start(out, name) {
    const c = this.cur;
    c.name = name;
    out.push({ t: "call", i: c.i, name });
  }
  _finish(out, parsedArgs = null, mismatch = false) {
    const c = this.cur;
    let args = c.frag, bad = mismatch;
    if (parsedArgs) { if (!args) { args = JSON.stringify(parsedArgs); this._emit(out, args); } else if (!sameJSON(args, parsedArgs)) { args = JSON.stringify(parsedArgs); bad = true; } }
    else {
      // check the streamed JSON against a parse of the whole body
      const p = this._parseRaw(c);
      if (p && !p.error && !sameJSON(args, p.arguments)) { args = JSON.stringify(p.arguments ?? {}); bad = true; }
      try { JSON.parse(args); } catch { args = JSON.stringify(p?.arguments ?? {}); bad = true; }
    }
    this.calls.push({ name: c.name, args });
    out.push({ t: "end", i: c.i, args, ...(bad ? { mismatch: true } : {}) });
    this.cur = null;
  }
  _parseRaw(c) {
    return parseCallBody(c.raw.replace(/\s*<\/tool_call>\s*$/, ""), (n) => this.schemas.get(n));
  }
  _newCall(bare = false) {
    this.cur = { i: this.calls.length, name: null, frag: "", raw: "", st: "pre", bare, np: 0, pv: null, ws: this.ws };
    this.ws = "";
    this.mode = "call";
  }

  _run(out, final) {
    for (let guard = 0; guard < 100000; guard++) {
      if (this.mode === "text" || this.mode === "between") { if (!this._runText(out, final)) return; }
      else if (!this._runCall(out, final)) return;
    }
  }
  // text: find the next call opener; hold back what could still become one
  _runText(out, final) {
    let b = this.buf;
    if (this.mode === "between") {
      const t = b.replace(/^\s+/, "");
      if (!t) { this.buf = ""; return false; }
      this.buf = b = t; this.mode = "text"; this.ls = true;
    }
    let at = b.indexOf(OPEN_TAG), bareAt = -1, bareLen = 0;
    if (this.style === "xml" && this.allowed.size) {
      const re = /(^|\n)<function=([^>\n<]*)>/g;
      let m;
      while ((m = re.exec(b))) {
        const p = m.index + m[1].length;
        if (p === 0 && !this.ls) continue;
        if (this.allowed.has(m[2]) && (at < 0 || p < at)) { bareAt = p; bareLen = FN_OPEN.length; break; }
      }
    }
    if (bareAt >= 0) {
      this._text(out, b.slice(0, bareAt));
      this.buf = b.slice(bareAt + bareLen);
      this._newCall(true);
      this.cur.st = "fname";
      return true;
    }
    if (at >= 0) {
      this._text(out, b.slice(0, at));
      this.buf = b.slice(at + OPEN_TAG.length);
      this._newCall(false);
      return true;
    }
    if (final) return false;
    // hold back a tail that could start "<tool_call>" or (XML) a line-initial "<function=NAME>"
    let hold = holdFor(b, [OPEN_TAG]);
    if (this.style === "xml" && this.allowed.size) {
      const nl = b.lastIndexOf("\n");
      const tail = nl >= 0 ? b.slice(nl + 1) : this.ls ? b : null;
      if (tail != null && tail.length > hold && (FN_OPEN.startsWith(tail) || (tail.startsWith(FN_OPEN) && [...this.allowed].some((n) => (n + ">").startsWith(tail.slice(FN_OPEN.length)))))) hold = tail.length;
    }
    const emit = b.slice(0, b.length - hold);
    if (emit) { this._text(out, emit); this.ls = emit.endsWith("\n") || (this.ls && !emit); }
    this.buf = b.slice(b.length - hold);
    return false;
  }

  // one step of the call parser; false when it needs more text
  _runCall(out, final) {
    const c = this.cur;
    let b = this.buf;
    const take = (n) => { c.raw += b.slice(0, n); this.buf = b = b.slice(n); };
    const toBuffered = () => { c.st = "buffered"; return true; };
    switch (c.st) {
      case "pre": {
        const t = b.replace(/^\s+/, "");
        take(b.length - t.length);
        if (!b) return false;
        if (this.style === "xml") {
          if (b.startsWith(FN_OPEN)) { take(FN_OPEN.length); c.st = "fname"; return true; }
          if (FN_OPEN.startsWith(b) && !final) return false;
        } else {
          if (b[0] === "{") { c.st = "jhead"; return true; }
        }
        return toBuffered();
      }
      case "fname": {
        const j = b.indexOf(">");
        if (j < 0) { if (/[\n<]/.test(b) || b.length > 200) return toBuffered(); return false; }
        const name = b.slice(0, j);
        if (!this.allowed.has(name) || /[\s<]/.test(name)) return toBuffered();
        if (c.bare) c.raw = FN_OPEN;
        take(j + 1);
        this._start(out, name);
        c.st = "fbody";
        return true;
      }
      case "fbody": {
        const t = b.replace(/^\s+/, "");
        take(b.length - t.length);
        if (!b) return false;
        if (b.startsWith(P_OPEN)) { take(P_OPEN.length); c.st = "pname"; return true; }
        if (b.startsWith(FN_CLOSE)) { take(FN_CLOSE.length); this._emit(out, c.np ? "}" : "{}"); c.st = "afterFn"; return true; }
        if ((P_OPEN.startsWith(b) || FN_CLOSE.startsWith(b)) && !final) return false;
        return toBuffered();
      }
      case "pname": {
        const j = b.indexOf(">");
        if (j < 0) { if (/[\n<]/.test(b) || b.length > 200) return toBuffered(); return false; }
        if (b.length === j + 1 && !final) return false;   // is a "\n" coming?
        const p = b.slice(0, j);
        if (!p || /[<\n\r]/.test(p)) return toBuffered();
        take(j + 1 + (b[j + 1] === "\n" ? 1 : 0));
        this._emit(out, (c.np ? ", " : "{") + JSON.stringify(p) + ": ");
        c.np++;
        const schema = this.schemas.get(c.name), props = schema?.properties;
        const ps = props && typeof props === "object" ? props[p] : undefined;
        const sc = ps === undefined ? "some" : stringCapable(ps, schema);
        const types = ps === undefined ? null : schemaTypes(ps, schema);
        c.pv = { name: p, schema: ps, root: schema, text: "",
          mode: sc === "only" ? "s" : this.constrained && types && [...types].every((x) => x === "object" || x === "array") ? "j" : "b" };
        if (c.pv.mode === "s") this._emit(out, '"');
        c.st = "pval";
        return true;
      }
      case "pval": {
        const pv = c.pv;
        // the value ends at </parameter> (a "\n" before it is not part of it); without the grammar
        // also at a line-initial <parameter= or </function> (a forgotten </parameter>)
        let at = b.indexOf(P_CLOSE), cut = at >= 0 ? P_CLOSE.length : 0;
        for (const x of ["\n" + P_OPEN, "\n" + FN_CLOSE]) { const k = b.indexOf(x); if (k >= 0 && (at < 0 || k < at)) { at = k; cut = 1; } }
        if (at < 0) {
          const hold = final ? 0 : holdFor(b, ["\n" + P_CLOSE, P_CLOSE, "\n" + P_OPEN, "\n" + FN_CLOSE]);
          const part = b.slice(0, b.length - hold);
          if (!part) return false;
          take(part.length);
          pv.text += part;
          if (pv.mode === "s") this._emit(out, escJSON(part));
          else if (pv.mode === "j") this._emit(out, part);
          return false;
        }
        let part = b.slice(0, at);
        if (part.endsWith("\n") && cut === P_CLOSE.length) part = part.slice(0, -1);
        const rawTake = at + cut;
        take(rawTake);
        pv.text += part;
        if (pv.mode === "s") this._emit(out, escJSON(part) + '"');
        else if (pv.mode === "j") this._emit(out, part);
        else this._emit(out, JSON.stringify(coerce(pv.text, pv.schema, pv.root)));
        c.pv = null;
        c.st = "fbody";
        return true;
      }
      case "afterFn": {
        const t = b.replace(/^\s+/, "");
        take(b.length - t.length);
        if (!b) return false;
        if (b.startsWith(CLOSE)) { take(CLOSE.length); this._finish(out); this.mode = "between"; return true; }
        if (b.startsWith(FN_OPEN)) {   // another call in the same block (no grammar)
          this._finish(out);
          this._newCall(false);
          this.cur.st = "pre";
          return true;
        }
        if (CLOSE.startsWith(b) && !final) return false;
        if (c.bare) { this._finish(out); this.mode = "text"; this.ls = false; return true; }
        // stray text after </function> (no grammar): the call is complete; the text is dropped up to
        // </tool_call>, or up to a new <tool_call> (a closer the model forgot)
        const k = b.indexOf(CLOSE), o = b.indexOf(OPEN_TAG);
        if (o >= 0 && (k < 0 || o < k)) { take(o); this._finish(out); this.mode = "between"; return true; }
        if (k < 0) { if (final) return false; take(Math.max(0, b.length - CLOSE.length)); return false; }
        take(k + CLOSE.length); this._finish(out); this.mode = "between"; return true;
      }
      // ---- JSON style
      case "jhead": {
        const m = /^\{\s*"name"\s*:\s*"((?:[^"\\]|\\.)*)"/.exec(b);
        if (!m) {
          if (!final && /^\{\s*(?:"(?:n(?:a(?:m(?:e(?:"\s*(?::\s*(?:"(?:[^"\\]|\\.)*)?)?)?)?)?)?)?)?$/.test(b)) return false;
          return toBuffered();
        }
        let name;
        try { name = JSON.parse('"' + m[1] + '"'); } catch { return toBuffered(); }
        if (!this.allowed.has(name)) return toBuffered();
        take(m[0].length);
        this._start(out, name);
        c.st = "jsep";
        return true;
      }
      case "jsep": {
        const m = /^\s*,\s*"arguments"\s*:\s*/.exec(b);
        if (m && m[0].length < b.length) {
          take(m[0].length);
          if (b[0] !== "{") return toBuffered();
          c.st = "jval"; c.depth = 0; c.inStr = false; c.esc = false;
          return true;
        }
        const e = /^\s*\}/.exec(b);
        if (e) { take(e[0].length); this._emit(out, "{}"); c.st = "jtail"; return true; }
        if (!final && /^\s*(?:,\s*(?:"(?:a(?:r(?:g(?:u(?:m(?:e(?:n(?:t(?:s(?:"\s*(?::\s*)?)?)?)?)?)?)?)?)?)?)?)?)?$/.test(b)) return false;
        return toBuffered();
      }
      case "jval": {
        let i = 0, done = -1;
        for (; i < b.length; i++) {
          const ch = b[i];
          if (c.inStr) { if (c.esc) c.esc = false; else if (ch === "\\") c.esc = true; else if (ch === '"') c.inStr = false; continue; }
          if (ch === '"') c.inStr = true;
          else if (ch === "{" || ch === "[") c.depth++;
          else if (ch === "}" || ch === "]") { if (--c.depth === 0) { done = i + 1; break; } }
        }
        const n = done >= 0 ? done : b.length;
        const part = b.slice(0, n);
        take(n);
        this._emit(out, part);
        if (done >= 0) { c.st = "jtail"; return true; }
        return false;
      }
      case "jtail": {
        const k = b.indexOf(CLOSE);
        if (k < 0) { if (!final) return false; take(b.length); return false; }
        take(k + CLOSE.length);
        this._finish(out);
        this.mode = "between";
        return true;
      }
      case "buffered": {
        const k = b.indexOf(CLOSE);
        let end = k;
        if (c.bare && k < 0) { const f = b.indexOf(FN_CLOSE); if (f >= 0) end = f + FN_CLOSE.length; }
        if (end < 0) return false;
        const body = c.raw + b.slice(0, end);
        this.buf = b.slice(end + (end === k ? CLOSE.length : 0));
        this.fellBack = false;
        this._parseBuffered(out, body);
        this.mode = this.fellBack ? "text" : "between";
        return true;
      }
    }
    return false;
  }
  // a body that was not in the canonical shape: parse it whole (it may hold several calls)
  _parseBuffered(out, body) {
    const c = this.cur;
    const parts = c.bare && !c.name ? [FN_OPEN + body] : splitCallBody(body);
    const parsed = parts.map((p) => parseCallBody(p, (n) => this.schemas.get(n)));
    const good = parsed.filter((p) => !p.error && this.allowed.has(p.name) && p.arguments && typeof p.arguments === "object" && !Array.isArray(p.arguments));
    if (c.name) {
      // the name already went out: finish it with what the whole body says
      const k = good.findIndex((p) => p.name === c.name);
      const mine = k >= 0 ? good.splice(k, 1)[0] : null;
      this._finish(out, mine ? mine.arguments : {}, !mine);
    } else if (!good.length) {
      // not a call after all (only possible without the grammar): its markup is content
      this.cur = null;
      this._text(out, c.ws + (c.bare ? FN_OPEN : OPEN_TAG) + body + (c.bare ? "" : CLOSE));
      this.fellBack = true;
      return;
    } else {
      const first = good.shift();
      this._start(out, first.name);
      this._finish(out, first.arguments);
    }
    for (const p of good) {
      this._newCall(false);
      this._start(out, p.name);
      this._finish(out, p.arguments);
    }
    this.cur = null;
  }
}
const OPEN_TAG = "<tool_call>";
function sameJSON(s, v) { try { return canon(JSON.parse(s)) === canon(v); } catch { return false; } }
