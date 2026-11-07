// Vendored from https://github.com/Nehanth/pooled/blob/45b5ee275a8754a37fce4a6ce174f31e4a216ef4/engine/tokenizer.js (MIT; see LICENSE-ENGINE).
// Byte-level BPE tokenizer built from a tokenizer.json (Qwen/Llama family).

// Pre-tokenizer splits (text -> pieces that BPE runs on), written as llama.cpp's llama-vocab.cpp has
// them: tokenizer.json's (?i:'s|...) is spelled out, since JS regexes only get (?i:) in ES2025.
// Qwen's split differs from GPT-2's in ways that change ids: digits go one at a time, a leading
// punctuation char joins the word after it ("(foo", ".bar"), and newlines stay with the run before them.
const CONTRACTIONS = "(?:'[sS]|'[tT]|'[rR][eE]|'[vV][eE]|'[mM]|'[lL][lL]|'[dD])";
export const PRE_SPLITS = {
  gpt2: String.raw`'s|'t|'re|'ve|'m|'ll|'d| ?\p{L}+| ?\p{N}+| ?[^\s\p{L}\p{N}]+|\s+(?!\S)|\s+`,
  qwen2: CONTRACTIONS + String.raw`|[^\r\n\p{L}\p{N}]?\p{L}+|\p{N}| ?[^\s\p{L}\p{N}]+[\r\n]*|\s*[\r\n]+|\s+(?!\S)|\s+`,
  // Qwen 3.5+: combining marks (\p{M}) count as letters, so "é" spelled e + U+0301 stays one word
  qwen35: CONTRACTIONS + String.raw`|[^\r\n\p{L}\p{N}]?[\p{L}\p{M}]+|\p{N}| ?[^\s\p{L}\p{M}\p{N}]+[\r\n]*|\s*[\r\n]+|\s+(?!\S)|\s+`,
  // SmolLM / StarCoder run this first (every digit is its own piece), then GPT-2 on the rest
  digit: String.raw`\p{N}`,
};
// GGUF tokenizer.ggml.pre names (llama-vocab.cpp) that use a split other than plain GPT-2; each is
// a list of regexes run one after the other, like llama.cpp's regex_exprs
const GGUF_PRE = {
  "qwen2": ["qwen2"], "deepseek-r1-qwen": ["qwen2"], "kormo": ["qwen2"], "f2llmv2": ["qwen2"],
  "stablelm2": ["qwen2"], "megrez": ["qwen2"],
  "qwen35": ["qwen35"],
  "smollm": ["digit", "gpt2"], "starcoder": ["digit", "gpt2"], "refact": ["digit", "gpt2"], "command-r": ["digit", "gpt2"],
};

// JS's \s is not Unicode's White_Space (which llama.cpp and HF tokenizers use): it has U+FEFF and
// lacks U+0085. Swap in the Unicode set so "x \u0085y" splits the way the model saw it.
const WS = String.raw`\t-\r \x85\xA0\u1680\u2000-\u200A\u2028\u2029\u202F\u205F\u3000`;
function unicodeSpace(src) {
  let out = "", inClass = false;
  for (let i = 0; i < src.length; i++) {
    const c = src[i];
    if (c === "\\") {
      const n = src[++i];
      if (n === "s") out += inClass ? WS : `[${WS}]`;
      else if (n === "S" && !inClass) out += `[^${WS}]`;
      else out += c + n;
    } else {
      if (c === "[") inClass = true;
      else if (c === "]") inClass = false;
      out += c;
    }
  }
  return out;
}

// One split step, HF's "Isolated" behaviour: every match is a piece, and so is the text between.
function isolate(src) {
  const re = new RegExp(unicodeSpace(src), "gu");
  return (text) => {
    const out = [];
    let at = 0;
    for (const m of text.matchAll(re)) {
      if (m.index > at) out.push(text.slice(at, m.index));
      if (m[0]) out.push(m[0]);
      at = m.index + m[0].length;
    }
    if (at < text.length) out.push(text.slice(at));
    return out;
  };
}

// Picks the split for a tokenizer.json (its pre_tokenizer) or a GGUF (tj.pre, from tokenizerFromGGUF).
// Unknown or missing config falls back to GPT-2, which is what this file always used.
export function preSplitter(tj) {
  let srcs = [];
  if (tj.pre !== undefined) {
    srcs = (GGUF_PRE[tj.pre] || []).map((k) => PRE_SPLITS[k]);
  } else if (tj.pre_tokenizer) {
    const p = tj.pre_tokenizer, steps = p.type === "Sequence" ? p.pretokenizers || [] : [p];
    for (const s of steps) {
      if (s.type === "Digits") srcs.push(s.individual_digits ? PRE_SPLITS.digit : String.raw`\p{N}+`);
      else if (s.type === "ByteLevel" && s.use_regex !== false) srcs.push(PRE_SPLITS.gpt2);
      else if (s.type === "Split" && !s.invert && (s.behavior || "Isolated") === "Isolated" && s.pattern) {
        if (s.pattern.Regex) srcs.push(s.pattern.Regex.replaceAll("(?i:'s|'t|'re|'ve|'m|'ll|'d)", CONTRACTIONS));
        else if (s.pattern.String) srcs.push(s.pattern.String.replace(/[.*+?^${}()|[\]\\/]/g, "\\$&"));
      }
    }
  }
  let steps;
  try { steps = srcs.map(isolate); } catch { steps = []; }
  if (!steps.length) steps = [isolate(PRE_SPLITS.gpt2)];
  // each step splits every piece the step before it made (SmolLM: Digits, then GPT-2)
  return (text) => steps.reduce((pieces, step) => pieces.flatMap(step), [text]).filter(Boolean);
}

export function makeTokenizer(tj) {
  // special tokens (<|im_start|>, <think>, ...) live in added_tokens for
  // Qwen-family tokenizer.json files, not in model.vocab
  const vocab = { ...tj.model.vocab };
  for (const t of tj.added_tokens || []) if (t && t.content !== undefined) vocab[t.content] = t.id;
  const idToTok = {};
  for (const [t, i] of Object.entries(vocab)) idToTok[i] = t;
  const ranks = new Map();
  tj.model.merges.forEach((m, i) => ranks.set(Array.isArray(m) ? m.join(" ") : m, i));
  const bs = [];
  for (let i = 33; i <= 126; i++) bs.push(i);
  for (let i = 161; i <= 172; i++) bs.push(i);
  for (let i = 174; i <= 255; i++) bs.push(i);
  const cs = bs.slice();
  let n = 0;
  for (let b = 0; b < 256; b++) if (!bs.includes(b)) { bs.push(b); cs.push(256 + n); n++; }
  const byteToChar = {}, charToByte = {};
  bs.forEach((b, i) => { byteToChar[b] = String.fromCharCode(cs[i]); charToByte[String.fromCharCode(cs[i])] = b; });
  const split = preSplitter(tj);
  const enc = new TextEncoder(), dec = new TextDecoder();
  // Merges the lowest-ranked adjacent pair, leftmost first, until none is left. Short words (almost
  // all of them) rescan the list per merge; that is quadratic in the word's length, so a long run with
  // no spaces (a 32k-letter "word" took 18 s) goes through bpeLong instead, which gives the same parts.
  function bpe(word) {
    if (word.length > 64) return bpeLong(word);
    let parts = [...word];
    while (parts.length > 1) {
      let best = null, bestRank = Infinity;
      for (let i = 0; i < parts.length - 1; i++) {
        const r = ranks.get(parts[i] + " " + parts[i + 1]);
        if (r !== undefined && r < bestRank) { bestRank = r; best = i; }
      }
      if (best === null) break;
      parts = [...parts.slice(0, best), parts[best] + parts[best + 1], ...parts.slice(best + 2)];
    }
    return parts;
  }
  // The same merges with a linked list and a min-heap of candidate pairs keyed by (rank, position):
  // O(n log n). A heap entry is stale once either side changed (merged away or grown).
  function bpeLong(word) {
    const sym = [...word], n = sym.length;
    const next = new Int32Array(n), prev = new Int32Array(n), ver = new Uint32Array(n);
    for (let i = 0; i < n; i++) { next[i] = i + 1; prev[i] = i - 1; }
    const heap = [];   // [rank, i, j, ver i, ver j]
    const less = (a, b) => a[0] < b[0] || (a[0] === b[0] && a[1] < b[1]);
    const push = (e) => {
      heap.push(e);
      for (let k = heap.length - 1; k > 0;) { const p = (k - 1) >> 1; if (!less(heap[k], heap[p])) break; [heap[k], heap[p]] = [heap[p], heap[k]]; k = p; }
    };
    const pop = () => {
      const top = heap[0], last = heap.pop();
      if (heap.length) {
        heap[0] = last;
        for (let k = 0; ;) {
          const l = 2 * k + 1, r = l + 1; let m = k;
          if (l < heap.length && less(heap[l], heap[m])) m = l;
          if (r < heap.length && less(heap[r], heap[m])) m = r;
          if (m === k) break;
          [heap[k], heap[m]] = [heap[m], heap[k]]; k = m;
        }
      }
      return top;
    };
    const cand = (i, j) => { if (i < 0 || j >= n) return; const r = ranks.get(sym[i] + " " + sym[j]); if (r !== undefined) push([r, i, j, ver[i], ver[j]]); };
    for (let i = 0; i + 1 < n; i++) cand(i, i + 1);
    while (heap.length) {
      const [, i, j, vi, vj] = pop();
      if (sym[i] === null || sym[j] === null || next[i] !== j || ver[i] !== vi || ver[j] !== vj) continue;
      sym[i] += sym[j]; sym[j] = null; ver[i]++; ver[j]++;
      next[i] = next[j]; if (next[j] < n) prev[next[j]] = i;
      cand(prev[i], i); cand(i, next[i]);
    }
    const parts = [];
    for (let i = 0; i < n; i = next[i]) parts.push(sym[i]);
    return parts;
  }
  return {
    vocab,
    encode(text) {
      const ids = [];
      for (const piece of split(text)) {
        let word = "";
        for (const b of enc.encode(piece)) word += byteToChar[b];
        for (const tok of bpe(word)) ids.push(vocab[tok]);
      }
      return ids;
    },
    decode(ids) {
      const bytes = [];
      for (const id of ids) {
        const tok = idToTok[id];
        if (tok === undefined) continue;
        for (const ch of tok) { const b = charToByte[ch]; if (b !== undefined) bytes.push(b); }
      }
      return dec.decode(new Uint8Array(bytes));
    },
  };
}
