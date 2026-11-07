// Vendored from https://github.com/Nehanth/pooled/blob/45b5ee275a8754a37fce4a6ce174f31e4a216ef4/engine/wgsl/prune.js (MIT; see LICENSE-ENGINE).
// Per-entry-point shader modules: the WGSL a pipeline needs, not the whole engine.
//
// The engine builds one big module (Qwen35Engine: ~650 KB of WGSL, 130+ functions) and makes each
// pipeline from it with its own entryPoint. Dawn's shader compiler (Tint) then processes the whole
// module once per pipeline before it drops what that entry point does not use: on Windows (D3D12 +
// FXC) that alone is ~1.3 s per pipeline, ~150 s of a MoE shard's load, against ~0.05 s for the same
// kernel from a module holding just its own code. pruneWGSL keeps an entry point and what it reaches
// (functions, structs, module-scope vars / consts / overrides / aliases it names, transitively), in
// source order, plus every directive (enable / requires / diagnostic) and const_assert. Tint removes
// the rest anyway, so the compiled kernel is the same; only the work to get there shrinks.
//
// A reference is any identifier in a declaration's text that names another module-scope declaration;
// a local that shadows a module-scope name only keeps something extra, never drops something needed.

// replaces comments with spaces (same length), so offsets into the result are offsets into src
function stripComments(src) {
  let out = "", i = 0;
  const n = src.length;
  while (i < n) {
    if (src[i] === "/" && src[i + 1] === "/") { const j = src.indexOf("\n", i); const e = j < 0 ? n : j; out += " ".repeat(e - i); i = e; continue; }
    if (src[i] === "/" && src[i + 1] === "*") {   // block comments nest in WGSL
      let d = 0, j = i;
      while (j < n) {
        if (src[j] === "/" && src[j + 1] === "*") { d++; j += 2; continue; }
        if (src[j] === "*" && src[j + 1] === "/") { d--; j += 2; if (!d) break; continue; }
        j++;
      }
      out += src.slice(i, j).replace(/[^\n]/g, " "); i = j; continue;
    }
    out += src[i++];
  }
  return out;
}

const IDENT = /[A-Za-z_][A-Za-z0-9_]*/g;

// -> [{ kind, name, text, refs: Set }] in source order; kind "keep" for directives and const_assert
export function wgslDecls(src) {
  const s = stripComments(src), n = s.length, decls = [];
  let i = 0;
  while (i < n) {
    while (i < n && /\s|;/.test(s[i])) i++;   // a stray ';' at module scope is legal (an empty declaration)
    if (i >= n) break;
    const start = i;
    // attributes: @name or @name(...)
    for (;;) {
      while (i < n && /\s/.test(s[i])) i++;
      if (s[i] !== "@") break;
      i++;
      IDENT.lastIndex = i; const m = IDENT.exec(s); i = m && m.index === i ? i + m[0].length : i;
      while (i < n && /\s/.test(s[i])) i++;
      if (s[i] === "(") { let d = 0; do { if (s[i] === "(") d++; else if (s[i] === ")") d--; i++; } while (i < n && d); }
    }
    IDENT.lastIndex = i;
    const kw = IDENT.exec(s);
    if (!kw || kw.index !== i) throw new Error(`pruneWGSL: cannot parse module scope at offset ${i}: ${JSON.stringify(s.slice(i, i + 40))}`);
    const word = kw[0];
    // the declared name: the first identifier after the keyword (and after var's <address space>)
    let j = i + word.length;
    if (word === "var") { while (j < n && /\s/.test(s[j])) j++; if (s[j] === "<") j = s.indexOf(">", j) + 1; }
    IDENT.lastIndex = j;
    const nm = IDENT.exec(s);
    // the end: a ';' or, for fn / struct, the '}' that closes the body, at bracket depth 0
    const braced = word === "fn" || word === "struct";
    let d = 0, k = j;
    for (; k < n; k++) {
      const c = s[k];
      if (c === "(" || c === "[" || c === "{") d++;
      else if (c === ")" || c === "]" || c === "}") { d--; if (!d && c === "}" && braced) { k++; break; } }
      else if (c === ";" && !d && !braced) { k++; break; }
    }
    const text = s.slice(start, k);
    const keep = word === "enable" || word === "requires" || word === "diagnostic" || word === "const_assert";
    const name = keep ? null : nm?.[0];
    const refs = new Set(text.match(IDENT) || []);
    if (name) refs.delete(name);
    decls.push({ kind: keep ? "keep" : word, name, text, refs });
    i = k;
  }
  return decls;
}

// -> the WGSL for these entry points (a string or an array): what they reach, in source order
export function pruneWGSL(src, entries, decls = wgslDecls(src)) {
  const byName = new Map();
  for (const d of decls) if (d.name) byName.set(d.name, d);
  const want = new Set(), stack = [];
  for (const e of [].concat(entries)) {
    if (!byName.has(e)) throw new Error(`pruneWGSL: no entry point "${e}" in the module`);
    stack.push(e);
  }
  while (stack.length) {
    const nm = stack.pop();
    if (want.has(nm)) continue;
    want.add(nm);
    for (const r of byName.get(nm).refs) if (byName.has(r) && !want.has(r)) stack.push(r);
  }
  return decls.filter((d) => d.kind === "keep" || want.has(d.name)).map((d) => d.text.trim()).join("\n");
}

// One shader module per entry point (the same text shares one module), from a big source:
//   const modOf = moduleSet(device, code);  ... compute: { module: modOf(name), entryPoint: name }
// prune: false (or globalThis.POOLED_WGSL_PRUNE === false) hands every entry point the whole module,
// as before (A/B).
export function moduleSet(device, code, { prune = globalThis.POOLED_WGSL_PRUNE !== false, label } = {}) {
  let whole = null, decls = null;
  const byText = new Map();
  return (entry) => {
    if (!prune) return (whole ||= device.createShaderModule({ code, ...(label ? { label } : {}) }));
    decls ||= wgslDecls(code);
    const text = pruneWGSL(code, entry, decls);
    let m = byText.get(text);
    if (!m) byText.set(text, (m = device.createShaderModule({ code: text, label: label ? `${label}:${entry}` : entry })));
    return m;
  };
}
