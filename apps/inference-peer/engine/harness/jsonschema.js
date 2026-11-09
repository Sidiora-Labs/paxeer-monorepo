// Vendored from https://github.com/Nehanth/pooled/blob/45b5ee275a8754a37fce4a6ce174f31e4a216ef4/harness/jsonschema.js (MIT; see LICENSE-ENGINE).
// JSON Schema -> a small node table the constrained decoder walks (harness/constrain.js), for tool
// arguments and response_format (docs/design/serve.md, "Constraint modes"). DOM-free.
//
// The subset: type (one or a list), properties, required, additionalProperties (false or a schema),
// items, prefixItems, minItems / maxItems, enum, const, anyOf / oneOf (as anyOf), allOf (merged for
// objects, else the first), nullable, $ref into $defs / definitions (recursive deeper than 6 -> any
// JSON value). Accepted and not enforced: pattern, format, string length and number range bounds,
// multipleOf, uniqueItems, default, description, title, $schema, examples. So types, required keys,
// enums, array lengths and structure are guaranteed; string and number bounds are not.
//
// Rules the automaton applies to objects: declared keys in any order, each at most once, required
// ones before the object closes; free keys only when there are no declared properties and
// additionalProperties is not false.
//
// Node kinds (compileSchema(...).nodes[id]):
//   { k: "any" }                                   any JSON value
//   { k: "obj", props: [[key, id]], req: [index], free: id | null (null = none) }
//   { k: "arr", items: id, prefix: [id], min?, max? }   min / max: minItems / maxItems when given
//   { k: "str", en: [string] | null }              en: the allowed values (enum / const)
//   { k: "num" } | { k: "int" } | { k: "bool" } | { k: "null" }
//   { k: "lit", texts: [jsonText] }                non-string enum / const values, written compactly
//   { k: "u", str, num, bool, null, obj, arr, lits }  a union, one branch per first character class
// Everything is capped (a request's schema must not stall everyone's room): SCHEMA_CAPS. Besides the
// size of the table: work (subschemas visited, shared by every schema of one request: $ref branches
// that point back at the same definition would otherwise be expanded k^refDepth times) and stack
// (nesting of any kind: values, anyOf / allOf, $ref), so no schema can freeze the host tab or
// overflow its stack. Subschemas are also memoized by (object, $ref depth).

export const SCHEMA_CAPS = { nodes: 10000, enum: 1000, anyOf: 64, allOf: 16, depth: 32, refDepth: 6, work: 200000, stack: 200 };
// a work budget to share between several compileSchema calls (one request's tools and format)
export const schemaWork = (caps = SCHEMA_CAPS) => ({ left: caps.work ?? SCHEMA_CAPS.work });

export class SchemaError extends Error {
  constructor(message) { super(message); this.name = "SchemaError"; }
}

const TYPES = new Set(["object", "array", "string", "number", "integer", "boolean", "null"]);

// schema -> { nodes, root }. Throws SchemaError past a cap or on an unresolvable $ref.
// doc: the document $refs resolve in (a tool's whole parameters schema, when compiling one parameter)
export function compileSchema(schema, { caps = SCHEMA_CAPS, doc = null, work = null } = {}) {
  const nodes = [];
  const budget = work || schemaWork(caps);
  const maxStack = caps.stack ?? SCHEMA_CAPS.stack;
  let stack = 0;
  const seen = new WeakMap();   // schema object -> Map(refs -> id)
  const ANY = add({ k: "any" });
  const memo = new Map();   // canonical node JSON -> id (dedupe, so the table stays small)
  function add(n) {
    if (nodes.length >= caps.nodes) throw new SchemaError(`the schema is too large (more than ${caps.nodes} nodes)`);
    nodes.push(n);
    return nodes.length - 1;
  }
  function intern(n) {
    const key = JSON.stringify(n);
    const hit = memo.get(key);
    if (hit !== undefined) return hit;
    const id = add(n);
    memo.set(key, id);
    return id;
  }
  const top = schema && typeof schema === "object" ? schema : {};
  const root = doc && typeof doc === "object" ? doc : top;

  function resolve(ref) {
    if (typeof ref !== "string" || !ref.startsWith("#")) throw new SchemaError(`$ref ${JSON.stringify(ref)}: only local references (#/...) are supported`);
    let cur = root;
    for (const part of ref.slice(1).split("/").filter(Boolean)) {
      const k = decodeURIComponent(part.replace(/~1/g, "/").replace(/~0/g, "~"));
      if (!cur || typeof cur !== "object" || !(k in cur)) throw new SchemaError(`$ref ${ref} does not resolve`);
      cur = cur[k];
    }
    return cur;
  }

  // s -> node id. depth: nesting of values; refs: $ref expansions on this path. The result depends
  // on (s, refs) only (depth just bounds it), so it is memoized by those.
  function node(s, depth, refs) {
    if (depth > caps.depth) throw new SchemaError(`the schema nests deeper than ${caps.depth} levels`);
    if (s === true || s == null) return ANY;
    if (s === false || typeof s !== "object" || Array.isArray(s)) return ANY;
    const byRefs = seen.get(s);
    if (byRefs?.has(refs)) return byRefs.get(refs);
    if (--budget.left < 0) throw new SchemaError("the schema is too complex (too many subschemas to expand)");
    if (++stack > maxStack) throw new SchemaError(`the schema nests deeper than ${maxStack} levels (counting anyOf, allOf and $ref)`);
    let id;
    try { id = node1(s, depth, refs); } finally { stack--; }
    if (byRefs) byRefs.set(refs, id); else seen.set(s, new Map([[refs, id]]));
    return id;
  }
  function node1(s, depth, refs) {
    if (s.$ref !== undefined) {
      if (refs >= caps.refDepth) return ANY;   // recursive: any JSON value below this depth
      const target = resolve(s.$ref);
      const { $ref, ...rest } = s;
      const extra = Object.keys(rest).some((k) => STRUCTURAL.has(k));
      return extra ? node(mergeAll([target, rest]), depth, refs + 1) : node(target, depth, refs + 1);
    }
    if (Array.isArray(s.allOf) && s.allOf.length) {
      if (s.allOf.length > caps.allOf) throw new SchemaError(`allOf has more than ${caps.allOf} parts`);
      const { allOf, ...rest } = s;
      // a part that is a $ref counts as one expansion (a self-referencing allOf ends at refDepth)
      let expanded = false;
      const parts = allOf.map((p) => {
        if (p && typeof p === "object" && p.$ref !== undefined) {
          if (refs >= caps.refDepth) return true;
          expanded = true;
          return resolve(p.$ref);
        }
        return p;
      });
      return node(mergeAll([rest, ...parts]), depth, expanded ? refs + 1 : refs);
    }
    const alts = s.anyOf ?? s.oneOf;
    if (Array.isArray(alts) && alts.length) {
      if (alts.length > caps.anyOf) throw new SchemaError(`anyOf / oneOf has more than ${caps.anyOf} branches`);
      const { anyOf, oneOf, ...rest } = s;
      const base = Object.keys(rest).some((k) => STRUCTURAL.has(k)) ? rest : null;
      const ids = alts.map((a) => node(base && a && typeof a === "object" ? mergeAll([base, a]) : a, depth, refs));
      if (s.nullable === true) ids.push(intern({ k: "null" }));
      return union(ids);
    }
    if (s.const !== undefined) return fromValues([s.const]);
    if (Array.isArray(s.enum)) {
      if (s.enum.length > caps.enum) throw new SchemaError(`an enum has more than ${caps.enum} values`);
      if (!s.enum.length) return ANY;
      const vals = s.nullable === true && !s.enum.includes(null) ? [...s.enum, null] : s.enum;
      return fromValues(vals);
    }
    let types = Array.isArray(s.type) ? s.type.filter((t) => TYPES.has(t)) : TYPES.has(s.type) ? [s.type] : null;
    if (!types || !types.length) {
      // no type: guess from the structural keywords, else any value
      if (s.properties || s.additionalProperties !== undefined || s.required) types = ["object"];
      else if (s.items || s.prefixItems) types = ["array"];
      else return ANY;
    }
    if (s.nullable === true && !types.includes("null")) types = [...types, "null"];
    const ids = types.map((t) => typed(t, s, depth, refs));
    return ids.length === 1 ? ids[0] : union(ids);
  }

  function typed(t, s, depth, refs) {
    switch (t) {
      case "string": return intern({ k: "str", en: null });
      case "number": return intern({ k: "num" });
      case "integer": return intern({ k: "int" });
      case "boolean": return intern({ k: "bool" });
      case "null": return intern({ k: "null" });
      case "array": {
        const prefix = Array.isArray(s.prefixItems) ? s.prefixItems.map((x) => node(x, depth + 1, refs)) : Array.isArray(s.items) ? s.items.map((x) => node(x, depth + 1, refs)) : [];
        const items = s.items && !Array.isArray(s.items) ? node(s.items, depth + 1, refs) : ANY;
        const a = { k: "arr", items, prefix };
        // array lengths (minItems / maxItems); a minimum over maxItems or a bad value is left out
        const lo = Number.isInteger(s.minItems) && s.minItems > 0 ? s.minItems : 0;
        const hi = Number.isInteger(s.maxItems) && s.maxItems >= 0 ? s.maxItems : null;
        if (lo && (hi == null || lo <= hi)) a.min = lo;
        if (hi != null && hi >= lo) a.max = hi;
        return add(a);
      }
      case "object": {
        const props = [];
        if (s.properties && typeof s.properties === "object") {
          for (const [key, sub] of Object.entries(s.properties)) props.push([key, node(sub, depth + 1, refs)]);
        }
        const req = Array.isArray(s.required) ? [...new Set(s.required)].map((r) => props.findIndex(([k]) => k === r)).filter((i) => i >= 0) : [];
        let free = null;
        if (!props.length && s.additionalProperties !== false) free = s.additionalProperties && typeof s.additionalProperties === "object" ? node(s.additionalProperties, depth + 1, refs) : ANY;
        return add({ k: "obj", props, req, free });
      }
    }
    return ANY;
  }

  // enum / const values -> a node: strings become a string enum, the rest compact literal texts
  function fromValues(vals) {
    const strs = vals.filter((v) => typeof v === "string");
    const others = vals.filter((v) => typeof v !== "string").map((v) => JSON.stringify(v));
    const ids = [];
    if (strs.length) ids.push(intern({ k: "str", en: [...new Set(strs)] }));
    if (others.length) ids.push(intern({ k: "lit", texts: [...new Set(others)] }));
    return ids.length === 1 ? ids[0] : union(ids);
  }

  // Several alternatives -> one node. They are grouped by the first character their values start
  // with (a string, a number, true / false, null, an object, an array), and each group merged into
  // one node: the result accepts every value any branch accepts (and possibly a little more when
  // two object branches differ: required keys are then only those all of them require).
  function union(ids) {
    ids = [...new Set(ids)];
    if (ids.includes(ANY)) return ANY;
    const flat = [];
    for (const id of ids) { const n = nodes[id]; if (n.k === "u") flat.push(...branches(n)); else flat.push(id); }
    const g = { str: [], num: [], bool: [], null: [], obj: [], arr: [], lits: [] };
    for (const id of new Set(flat)) {
      const n = nodes[id];
      if (n.k === "str") g.str.push(id);
      else if (n.k === "num" || n.k === "int") g.num.push(id);
      else if (n.k === "bool") g.bool.push(id);
      else if (n.k === "null") g.null.push(id);
      else if (n.k === "obj") g.obj.push(id);
      else if (n.k === "arr") g.arr.push(id);
      else if (n.k === "lit") g.lits.push(...n.texts);
    }
    const u = { k: "u", str: null, num: null, bool: null, null: null, obj: null, arr: null, lits: [] };
    if (g.str.length) u.str = g.str.some((id) => !nodes[id].en) ? intern({ k: "str", en: null }) : intern({ k: "str", en: [...new Set(g.str.flatMap((id) => nodes[id].en))] });
    if (g.num.length) u.num = g.num.some((id) => nodes[id].k === "num") ? intern({ k: "num" }) : intern({ k: "int" });
    if (g.bool.length) u.bool = intern({ k: "bool" });
    if (g.null.length) u.null = intern({ k: "null" });
    if (g.obj.length) u.obj = g.obj.length === 1 ? g.obj[0] : mergeObjects(g.obj);
    if (g.arr.length) u.arr = g.arr.length === 1 ? g.arr[0] : add({ k: "arr", items: union(g.arr.map((id) => nodes[id].items)), prefix: [] });
    // literals whose class already has a typed branch are covered by it
    const cls = (t) => (t[0] === '"' ? "str" : t[0] === "t" || t[0] === "f" ? "bool" : t[0] === "n" ? "null" : t[0] === "{" ? "obj" : t[0] === "[" ? "arr" : "num");
    u.lits = [...new Set(g.lits)].filter((t) => !u[cls(t)]);
    if (u.lits.length > caps.enum) throw new SchemaError(`an enum has more than ${caps.enum} values`);
    const set = ["str", "num", "bool", "null", "obj", "arr"].filter((k) => u[k] != null);
    if (set.length === 1 && !u.lits.length) return u[set[0]];
    if (!set.length && u.lits.length) return intern({ k: "lit", texts: u.lits });
    return add(u);
  }
  function branches(u) {
    const out = ["str", "num", "bool", "null", "obj", "arr"].map((k) => u[k]).filter((x) => x != null);
    if (u.lits.length) out.push(intern({ k: "lit", texts: u.lits }));
    return out;
  }
  function mergeObjects(ids) {
    const props = new Map(), reqSets = [];
    let free = null, anyFree = false, closedEmpty = false;
    for (const id of ids) {
      const n = nodes[id];
      if (!n.props.length) { if (n.free != null) { anyFree = true; free = free == null ? n.free : union([free, n.free]); } else closedEmpty = true; }
      for (const [k, v] of n.props) props.set(k, props.has(k) ? union([props.get(k), v]) : v);
      reqSets.push(new Set(n.req.map((i) => n.props[i][0])));
    }
    // a branch with free keys makes the merged object free (its keys could be anything)
    if (anyFree) return add({ k: "obj", props: [], req: [], free: union([free, ...props.values()]) });
    void closedEmpty;
    const list = [...props.entries()];
    const req = list.map(([k], i) => (reqSets.every((s) => s.has(k)) ? i : -1)).filter((i) => i >= 0);
    return add({ k: "obj", props: list, req, free: null });
  }

  const id = node(top, 0, 0);
  return { nodes, root: id };
}

// keywords that change the shape of a value (anything else is annotation or a bound we do not enforce)
const STRUCTURAL = new Set(["type", "properties", "required", "additionalProperties", "items", "prefixItems", "enum", "const", "anyOf", "oneOf", "allOf", "nullable", "$ref"]);

// allOf: one schema with the parts' shapes combined (objects: properties and required keys united;
// otherwise the first part that says anything wins)
function mergeAll(parts) {
  const out = {};
  for (const p of parts) {
    if (!p || typeof p !== "object") continue;
    for (const [k, v] of Object.entries(p)) {
      if (k === "properties" && v && typeof v === "object") out.properties = { ...(out.properties || {}), ...v };
      else if (k === "required" && Array.isArray(v)) out.required = [...new Set([...(out.required || []), ...v])];
      else if (k === "additionalProperties") out.additionalProperties = out.additionalProperties === false || v === false ? false : v;
      else if (k === "type" && out.type !== undefined) {
        const a = [].concat(out.type), b = [].concat(v), both = a.filter((t) => b.includes(t) || (t === "number" && b.includes("integer")));
        out.type = both.length ? (both.length === 1 ? both[0] : both) : out.type;
      } else if (!(k in out)) out[k] = v;
    }
  }
  return out;
}

// Can a value of this schema be a string? ("string-capable": XML parameters of such a schema are
// written raw, without quotes.) -> "only" (nothing but strings), "some" (strings and other types),
// or "none".
// Past the work budget or the nesting cap the answer is the safe "some" (both are far beyond a real
// schema; see SCHEMA_CAPS).
const refTarget = (root, ref) => {
  let cur = root;
  for (const part of String(ref).slice(1).split("/").filter(Boolean)) cur = cur?.[decodeURIComponent(part.replace(/~1/g, "/").replace(/~0/g, "~"))];
  return cur;
};
export function stringCapable(schema, root = schema, refs = 0, budget = { left: SCHEMA_CAPS.work, depth: 0 }) {
  if (!schema || typeof schema !== "object") return "some";   // no schema: anything
  if (--budget.left < 0 || budget.depth > SCHEMA_CAPS.stack) return "some";
  budget.depth++;
  try { return stringCapable1(schema, root, refs, budget); } finally { budget.depth--; }
}
function stringCapable1(schema, root, refs, budget) {
  if (schema.$ref !== undefined) {
    if (refs >= SCHEMA_CAPS.refDepth) return "some";
    try { return stringCapable(refTarget(root, schema.$ref), root, refs + 1, budget); } catch { return "some"; }
  }
  const alts = schema.anyOf ?? schema.oneOf;
  if (Array.isArray(alts) && alts.length) return combine(alts.map((a) => stringCapable(a, root, refs, budget)).concat(schema.nullable === true ? ["none"] : []));
  if (Array.isArray(schema.allOf) && schema.allOf.length) {
    const r = schema.allOf.map((a) => stringCapable(a, root, refs, budget));
    return r.includes("none") ? "none" : r.includes("only") ? "only" : "some";
  }
  if (schema.const !== undefined) return typeof schema.const === "string" ? (schema.nullable === true ? "some" : "only") : "none";
  if (Array.isArray(schema.enum) && schema.enum.length) {
    const s = schema.enum.filter((v) => typeof v === "string").length;
    return !s ? "none" : s === schema.enum.length && schema.nullable !== true ? "only" : "some";
  }
  const t = schema.type;
  if (t === undefined) return schema.properties || schema.items ? "none" : "some";
  const list = [].concat(t).concat(schema.nullable === true ? ["null"] : []);
  if (!list.includes("string")) return "none";
  return list.every((x) => x === "string") ? "only" : "some";
}
function combine(rs) {
  if (rs.every((r) => r === "only")) return "only";
  if (rs.every((r) => r === "none")) return "none";
  return "some";
}

// The types a schema allows, for coercing an XML parameter's text (null means: unknown, anything)
// (past the work budget or the nesting cap: null, unknown)
export function schemaTypes(schema, root = schema, refs = 0, budget = { left: SCHEMA_CAPS.work, depth: 0 }) {
  if (!schema || typeof schema !== "object") return null;
  if (--budget.left < 0 || budget.depth > SCHEMA_CAPS.stack) return null;
  budget.depth++;
  try { return schemaTypes1(schema, root, refs, budget); } finally { budget.depth--; }
}
function schemaTypes1(schema, root, refs, budget) {
  if (schema.$ref !== undefined) {
    if (refs >= SCHEMA_CAPS.refDepth) return null;
    let cur;
    try { cur = refTarget(root, schema.$ref); } catch { return null; }
    return schemaTypes(cur, root, refs + 1, budget);
  }
  const out = new Set();
  const alts = schema.anyOf ?? schema.oneOf;
  if (Array.isArray(alts) && alts.length) {
    for (const a of alts) { const t = schemaTypes(a, root, refs, budget); if (!t) return null; t.forEach((x) => out.add(x)); }
  } else if (Array.isArray(schema.allOf) && schema.allOf.length) {
    return schemaTypes(schema.allOf[0], root, refs, budget);
  } else if (schema.const !== undefined || (Array.isArray(schema.enum) && schema.enum.length)) {
    for (const v of schema.const !== undefined ? [schema.const] : schema.enum) out.add(v === null ? "null" : Array.isArray(v) ? "array" : typeof v === "number" ? (Number.isInteger(v) ? "integer" : "number") : typeof v);
  } else if (schema.type !== undefined) {
    for (const t of [].concat(schema.type)) if (TYPES.has(t)) out.add(t);
    if (!out.size) return null;
  } else if (schema.properties) out.add("object");
  else if (schema.items) out.add("array");
  else return null;
  if (schema.nullable === true) out.add("null");
  return out;
}
