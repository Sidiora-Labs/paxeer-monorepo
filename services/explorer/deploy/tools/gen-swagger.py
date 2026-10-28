#!/usr/bin/env python3
"""Generate swagger.yaml (OpenAPI 3.0.3) for the explorer backend.

Sources, all read statically from the backend tree:
  * the Phoenix routers under apps/block_scout_web/lib/block_scout_web (every route, every
    mount, compile-time flags evaluated for the configured chain type),
  * the OpenApiSpex `operation` blocks of the API v2 controllers (summaries, descriptions,
    parameters, response codes),
  * the shared parameter helpers in schemas/api/v2/general.ex and schemas/api/v2/paxeer_x,
  * the RPC documentation module etherscan.ex (module/action API under /api and /api/v1),
  * the JSON-RPC method list of Explorer.EthRPC (/api/eth-rpc).

Optionally merges OpenAPI documents exported by `mix openapi.spec.yaml` (--spex): for every
path+method they contain, their operation and component schemas replace the static ones.

Usage: gen-swagger.py [--chain-type paxeer_x] [--include-web] [--include-admin]
                      [--spex public.yaml --spex private.yaml] [-o swagger.yaml]
"""
import argparse
import glob
import json
import os
import re
import sys

import yaml

HERE = os.path.dirname(os.path.abspath(__file__))
EXPLORER = os.path.abspath(os.path.join(HERE, "..", ".."))
BACKEND = os.path.join(EXPLORER, "backend")
WEB = os.path.join(BACKEND, "apps", "block_scout_web", "lib", "block_scout_web")

ROUTER_FILES = {
    "Router": "router.ex",
    "HealthRouter": "health_router.ex",
    "ApiRouter": "routers/api_router.ex",
    "AccountRouter": "routers/account_router.ex",
    "AdminRouter": "routers/admin_router.ex",
    "WebRouter": "routers/web_router.ex",
    "SmartContractsApiV2Router": "routers/smart_contracts_api_v2_router.ex",
    "TokensApiV2Router": "routers/tokens_api_v2_router.ex",
    "APIKeyV2Router": "routers/api_key_v2_router.ex",
    "UtilsApiV2Router": "routers/utils_api_v2_router.ex",
    "AddressBadgesApiV2Router": "routers/address_badges_v2_router.ex",
}

VERBS = ("get", "post", "put", "patch", "delete", "options", "head")

RESPONSE_CODES = {
    "ok": "200",
    "created": "201",
    "no_content": "204",
    "bad_request": "400",
    "unauthorized": "401",
    "forbidden": "403",
    "not_found": "404",
    "unprocessable_entity": "422",
    "too_many_requests": "429",
    "internal_server_error": "500",
    "service_unavailable": "503",
}

HASH40 = {"type": "string", "pattern": "^0x[0-9a-fA-F]{40}$", "example": "0x0000000000000000000000000000000000000000"}
HASH64 = {"type": "string", "pattern": "^0x[0-9a-fA-F]{64}$"}

SCHEMA_MODULES = {
    "AddressHash": HASH40,
    "AddressHashNullable": {"type": "string", "nullable": True, "pattern": "^0x[0-9a-fA-F]{40}$"},
    "FullHash": HASH64,
    "FullHashNullable": {"type": "string", "nullable": True, "pattern": "^0x[0-9a-fA-F]{64}$"},
    "HexString": {"type": "string", "pattern": "^0x([0-9a-fA-F][0-9a-fA-F])*$"},
    "IntegerString": {"type": "string", "pattern": "^[0-9]+$"},
    "IntegerStringNullable": {"type": "string", "nullable": True, "pattern": "^[0-9]+$"},
    "FloatString": {"type": "string", "pattern": "^[0-9]+(\\.[0-9]+)?$"},
    "FloatStringNullable": {"type": "string", "nullable": True},
    "EmptyString": {"type": "string", "enum": [""]},
    "NullString": {"type": "string", "enum": ["null"]},
    "Timestamp": {"type": "string", "format": "date-time"},
    "TimestampNullable": {"type": "string", "format": "date-time", "nullable": True},
    "BlockNumber": {"type": "integer", "minimum": 0},
}


# --------------------------------------------------------------------------- helpers


def read(path):
    with open(path, encoding="utf-8") as f:
        return f.read()


def strip_docs(src):
    src = re.sub(r'@(module)?doc\s+"""(.*?)"""', "", src, flags=re.S)
    src = re.sub(r'@(module)?doc\s+"[^"\n]*"', "", src)
    return src


def strip_comment(line):
    out, in_str, esc = [], False, False
    for ch in line:
        if in_str:
            out.append(ch)
            if esc:
                esc = False
            elif ch == "\\":
                esc = True
            elif ch == '"':
                in_str = False
            continue
        if ch == '"':
            in_str = True
            out.append(ch)
        elif ch == "#":
            break
        else:
            out.append(ch)
    return "".join(out)


def bracket_delta(line):
    depth, in_str, esc = 0, False, False
    for ch in line:
        if in_str:
            if esc:
                esc = False
            elif ch == "\\":
                esc = True
            elif ch == '"':
                in_str = False
            continue
        if ch == '"':
            in_str = True
        elif ch in "([{":
            depth += 1
        elif ch in ")]}":
            depth -= 1
    return depth


def logical_lines(src):
    lines, buf, depth = [], "", 0
    for raw in strip_docs(src).splitlines():
        line = strip_comment(raw).strip()
        if not line and not buf:
            continue
        buf = f"{buf} {line}" if buf else line
        depth += bracket_delta(line)
        if depth <= 0:
            lines.append(buf)
            buf, depth = "", 0
    if buf:
        lines.append(buf)
    return lines


def split_top(expr, seps=(",",)):
    """Split expr on separators that sit outside brackets and strings."""
    parts, buf, depth, in_str, esc = [], "", 0, False, False
    i = 0
    while i < len(expr):
        ch = expr[i]
        if in_str:
            buf += ch
            if esc:
                esc = False
            elif ch == "\\":
                esc = True
            elif ch == '"':
                in_str = False
            i += 1
            continue
        if ch == '"':
            in_str = True
            buf += ch
        elif ch in "([{":
            depth += 1
            buf += ch
        elif ch in ")]}":
            depth -= 1
            buf += ch
        elif depth == 0 and any(expr.startswith(s, i) for s in seps):
            sep = next(s for s in seps if expr.startswith(s, i))
            parts.append(buf.strip())
            buf = ""
            i += len(sep)
            continue
        else:
            buf += ch
        i += 1
    if buf.strip():
        parts.append(buf.strip())
    return parts


def kw(body, key):
    m = re.search(rf"(?<![a-z_]){key}:\s*", body)
    if not m:
        return None
    rest = body[m.end():]
    parts = split_top(rest)
    return parts[0] if parts else None


def atom_or_string(value):
    if value is None:
        return None
    value = value.strip()
    if value.startswith(":"):
        return value[1:].strip('"')
    if value.startswith('"'):
        return value.strip('"')
    return value


def elixir_string(value):
    if value is None:
        return None
    value = value.strip()
    m = re.match(r'^"""\n?(.*?)"""$', value, re.S)
    if m:
        text = m.group(1)
        lines = [l for l in text.splitlines()]
        indent = min((len(l) - len(l.lstrip()) for l in lines if l.strip()), default=0)
        return "\n".join(l[indent:] for l in lines).strip()
    segments = re.findall(r'"((?:[^"\\]|\\.)*)"', value)
    return "".join(s.replace('\\"', '"').replace("\\n", "\n") for s in segments) if segments else value


def snake(name):
    return re.sub(r"(?<!^)(?=[A-Z])", "_", name).lower()


def resource_name(controller):
    last = controller.split(".")[-1]
    if last.endswith("Controller"):
        last = last[: -len("Controller")]
    return snake(last)


# --------------------------------------------------------------------------- conditions


def eval_cond(expr, env):
    e = expr.strip()
    e = e.replace("Mix.env()", "__mix_env")
    e = re.sub(r"@([a-z_0-9]+)\??", lambda m: "__" + m.group(1), e)
    e = re.sub(r"\{:(\w+), :(\w+)\}", r'("\1", "\2")', e)
    e = re.sub(r":(\w+)", r'"\1"', e)
    e = e.replace("!==", "!=").replace("===", "==").replace("&&", " and ").replace("||", " or ")
    e = re.sub(r"!(?!=)", " not ", e)
    names = {"__" + k: v for k, v in env.items()}
    try:
        return bool(eval(e, {"__builtins__": {}}, names))  # noqa: S307 - trusted repo source
    except Exception as exc:  # pragma: no cover - surfaces unexpected router syntax
        raise SystemExit(f"cannot evaluate router condition {expr!r}: {exc}")


# --------------------------------------------------------------------------- router walk


class Frame:
    def __init__(self, kind, prefix="", cond=True, alias=None, note=None, inert=False):
        self.kind = kind
        self.prefix = prefix
        self.cond = cond
        self.orig = cond
        self.alias = alias
        self.note = note
        self.inert = inert


class Route:
    def __init__(self, method, path, controller, action, router, notes, kind="route"):
        self.method = method
        self.path = path
        self.controller = controller
        self.action = action
        self.router = router
        self.notes = notes
        self.kind = kind

    def key(self):
        return (self.method, self.path)


def join_path(*parts):
    path = "/".join(p.strip("/") for p in parts if p and p.strip("/"))
    return "/" + path


class RouterWalker:
    def __init__(self, env):
        self.env = env
        self.routes = []
        self.specials = []
        self.seen = set()
        self.only = []

    def walk(self, router, mount="", alias_base="BlockScoutWeb", only_prefix=None):
        key = (router, mount)
        if key in self.seen:
            return
        self.seen.add(key)
        src = read(os.path.join(WEB, ROUTER_FILES[router]))
        stack = []
        self.only.append(only_prefix)
        for line in logical_lines(src):
            if line == "end" or line.startswith("end "):
                if stack:
                    stack.pop()
                continue
            if line == "else":
                for fr in reversed(stack):
                    if fr.kind == "if":
                        fr.cond = not fr.orig
                        break
                continue
            active = all(f.cond for f in stack) and not any(f.inert for f in stack)
            prefix = join_path(mount, *[f.prefix for f in stack])
            alias = alias_base
            for f in stack:
                if f.alias:
                    alias = f.alias if f.alias.startswith("BlockScoutWeb") else f"{alias}.{f.alias}"
            notes = [f.note for f in stack if f.note]
            opens = re.search(r"\bdo\s*$", line) is not None
            if opens:
                head = re.sub(r"\s*do\s*$", "", line)
                stack.append(self.open_frame(head, active, prefix, alias, notes, router, mount, only_prefix))
                continue
            if active:
                self.statement(line, prefix, alias, notes, router, mount, only_prefix)
        self.only.pop()

    def open_frame(self, head, active, prefix, alias, notes, router, mount, only_prefix):
        if head.startswith("defmodule"):
            return Frame("module")
        if head.startswith("pipeline"):
            return Frame("pipeline", inert=True)
        m = re.match(r'scope\s*\(?\s*"([^"]*)"\s*(?:,\s*(.*))?\)?$', head)
        if m:
            path, rest = m.group(1), (m.group(2) or "").strip()
            al = None
            if rest and not rest.startswith(("as:", "alias:", "unquote", "[")):
                al = split_top(rest)[0].strip()
                if not re.match(r"^[A-Z][\w.]*$", al):
                    al = None
            return Frame("scope", prefix=path, alias=al)
        m = re.match(r"chain_scope\s*\(?\s*:(\w+)", head)
        if m:
            chain = m.group(1)
            return Frame("chain", cond=(chain == self.env["chain_type"]), note=f"chain_type={chain}")
        m = re.match(r"if\s+(.*)$", head)
        if m:
            return Frame("if", cond=eval_cond(m.group(1), self.env), note=f"if {m.group(1).strip()}")
        m = re.match(r"resources\s*\(?\s*(.*)$", head)
        if m:
            spec = self.resources(m.group(1).rstrip(")"), prefix, alias, notes, router, active)
            return Frame("resources", prefix=spec)
        return Frame("other")

    def resources(self, args, prefix, alias, notes, router, active):
        parts = split_top(args)
        path = parts[0].strip('"')
        controller = f"{alias}.{parts[1]}" if not parts[1].startswith("BlockScoutWeb") else parts[1]
        rest = ", ".join(parts[2:])
        only = re.findall(r":(\w+)", kw(rest, "only") or "")
        singleton = (kw(rest, "singleton") or "").strip() == "true"
        param = (kw(rest, "param") or '"id"').strip('"')
        actions = only or ["index", "edit", "new", "show", "create", "update", "delete"]
        table = {
            "index": [("get", "")],
            "new": [("get", "/new")],
            "create": [("post", "")],
            "show": [("get", "" if singleton else f"/:{param}")],
            "edit": [("get", "/edit" if singleton else f"/:{param}/edit")],
            "update": [("put", "" if singleton else f"/:{param}"), ("patch", "" if singleton else f"/:{param}")],
            "delete": [("delete", "" if singleton else f"/:{param}")],
        }
        if active:
            for action in actions:
                for method, suffix in table[action]:
                    self.add(method, join_path(prefix, path + suffix), controller, action, router, notes)
        nested_param = f"{resource_name(controller)}_{param}"
        return path if singleton else f"{path}/:{nested_param}"

    def statement(self, line, prefix, alias, notes, router, mount, only_prefix):
        m = re.match(r"(" + "|".join(VERBS) + r')\s*\(?\s*"([^"]*)"\s*,\s*([\w.]+)\s*,\s*:(\w+)', line)
        if m:
            verb, path, controller, action = m.groups()
            controller = controller if controller.startswith("BlockScoutWeb") else f"{alias}.{controller}"
            full = join_path(prefix, path)
            self.add(verb, full, controller, action, router, notes)
            return
        m = re.match(r'match\s*\(\s*:\*\s*,\s*"([^"]*)"\s*,\s*([\w.]+)', line)
        if m:
            path, target = m.groups()
            target = target.split(".")[-1]
            if target in ROUTER_FILES:
                restrict = join_path(prefix, re.sub(r"/\*\w+$", "", path))
                self.walk(target, mount=prefix, only_prefix=restrict)
            return
        m = re.match(r'forward\s*\(?\s*"([^"]*)"\s*,\s*([\w.]+)(.*)$', line)
        if m:
            path, target, rest = m.groups()
            full = join_path(prefix, path)
            short = target.split(".")[-1]
            if short in ROUTER_FILES:
                self.walk(short, mount=full)
            elif short in ("RPCTranslator", "RPCTranslatorForwarder"):
                modules = re.findall(r'"(\w+)"\s*=>\s*\{([\w.]+),\s*\[([^\]]*)\]\}', rest)
                self.specials.append(("rpc", full, modules, notes, router))
            elif target.startswith("Absinthe.Plug.GraphiQL"):
                self.specials.append(("graphiql", full, None, notes, router))
            elif target.startswith("Absinthe.Plug"):
                self.specials.append(("graphql", full, None, notes, router))
            elif target.startswith("OpenApiSpex"):
                self.add("get", full, target, "render", router, notes)
            return
        m = re.match(r"resources\s*\(?\s*(.*)\)?$", line)
        if m:
            self.resources(m.group(1).rstrip(")"), prefix, alias, notes, router, True)

    def add(self, method, path, controller, action, router, notes):
        path = re.sub(r"//+", "/", path)
        if len(path) > 1:
            path = path.rstrip("/")
        restrict = self.only[-1] if self.only else None
        if restrict and not (path == restrict or path.startswith(restrict + "/")):
            return
        self.routes.append(Route(method, path, controller, action, router, list(notes)))


# --------------------------------------------------------------------------- parameter helpers


def parse_parameter(block, local=None):
    """Turn the text of a %Parameter{...} struct into an OpenAPI parameter dict."""
    name = atom_or_string(kw(block, "name"))
    where = atom_or_string(kw(block, "in")) or "query"
    required = (kw(block, "required") or "false").strip() == "true"
    description = elixir_string(kw(block, "description")) or ""
    schema_src = kw(block, "schema") or ""
    schema = parse_schema(schema_src, local)
    param = {"name": name, "in": where, "required": bool(required) or where == "path", "schema": schema}
    if description:
        param["description"] = description
    example = kw(block, "example")
    if example and example.strip().startswith('"'):
        param["example"] = elixir_string(example)
    return param


def parse_schema(src, local=None):
    src = (src or "").strip()
    if not src:
        return {"type": "string"}
    m = re.match(r"^%Schema\{(.*)\}$", src, re.S)
    if m:
        body = m.group(1)
        schema = {}
        typ = atom_or_string(kw(body, "type"))
        if typ:
            schema["type"] = typ
        for key in ("minimum", "maximum", "minLength", "maxLength"):
            v = kw(body, key)
            if v and re.match(r"^-?\d+$", v.strip()):
                schema[key] = int(v)
        enum = kw(body, "enum")
        if enum:
            vals = re.findall(r'"([^"]*)"', enum) or re.findall(r":(\w+)", enum)
            if vals:
                schema["enum"] = vals
        pattern = kw(body, "pattern")
        if pattern:
            pat = re.search(r'~r"((?:[^"\\]|\\.)*)"', pattern) or re.search(r"~r/((?:[^/\\]|\\.)*)/", pattern)
            if pat:
                schema["pattern"] = pat.group(1)
        if (kw(body, "nullable") or "").strip() == "true":
            schema["nullable"] = True
        any_of = kw(body, "anyOf")
        if any_of:
            inner = split_top(any_of.strip()[1:-1])
            schema["anyOf"] = [parse_schema(i, local) for i in inner]
            schema.pop("type", None)
        items = kw(body, "items")
        if items:
            schema["items"] = parse_schema(items, local)
        example = kw(body, "example")
        if example and example.strip().startswith('"'):
            schema["example"] = elixir_string(example)
        desc = kw(body, "description")
        if desc:
            schema["description"] = elixir_string(desc)
        return schema or {"type": "string"}
    last = src.split(".")[-1]
    if last in SCHEMA_MODULES:
        return dict(SCHEMA_MODULES[last])
    if local and last in local:
        return dict(local[last])
    return {"type": "string", "description": f"see backend schema module {src}"}


def collect_struct(src, start):
    """Return the text of the %...{ ... } struct starting at index start (balanced braces)."""
    depth, i, in_str, esc = 0, src.index("{", start), False, False
    begin = i
    while i < len(src):
        ch = src[i]
        if in_str:
            if esc:
                esc = False
            elif ch == "\\":
                esc = True
            elif ch == '"':
                in_str = False
        elif ch == '"':
            in_str = True
        elif ch == "{":
            depth += 1
        elif ch == "}":
            depth -= 1
            if depth == 0:
                return src[begin + 1 : i]
        i += 1
    return src[begin + 1 :]


class Helpers:
    """Shared parameter helper functions and module attributes, resolved from the schema modules."""

    def __init__(self):
        self.funcs = {}
        self.maps = {}
        self.attrs = {}
        self.load(os.path.join(WEB, "schemas", "api", "v2", "general.ex"))
        for extra in glob.glob(os.path.join(WEB, "schemas", "api", "v2", "paxeer_x", "*.ex")):
            self.load(extra)

    def load(self, path):
        src = strip_docs(read(path))
        for m in re.finditer(r"\n  defp? ([a-z_0-9]+)(?:\(([^)]*)\))? do\n(.*?)\n  end\n", src, re.S):
            name, args, body = m.group(1), m.group(2), m.group(3)
            if "%Parameter{" in body or "%OpenApiSpex.Parameter{" in body:
                pos = body.index("Parameter{")
                self.funcs[name] = (args, collect_struct(body, pos))
        for m in re.finditer(r"\n  @([a-z_0-9]+) %\{", src):
            name = m.group(1)
            body = collect_struct(src, m.end() - 1)
            entries = {}
            for e in re.finditer(r'"([\w-]+)"\s*=>\s*%(?:OpenApiSpex\.)?Parameter\{', body):
                entries[e.group(1)] = collect_struct(body, e.end() - 1)
            if entries:
                self.maps[name] = entries
        for m in re.finditer(r"\n  @([a-z_0-9]+) %(?:OpenApiSpex\.)?Parameter\{", src):
            self.attrs[m.group(1)] = collect_struct(src, m.end() - 1)
        for m in re.finditer(r"\n  defp? ([a-z_0-9]+)(?:\(\))?, do: @([a-z_0-9]+)", src):
            if m.group(2) in self.attrs:
                self.funcs[m.group(1)] = (None, self.attrs[m.group(2)])

    def paging(self, map_name, fields):
        table = self.maps.get(map_name, {})
        out = []
        for field in fields:
            if field in table:
                out.append(parse_parameter(table[field]))
            else:
                out.append({"name": field, "in": "query", "required": False, "schema": {"type": "string"}, "description": f"{field} for paging"})
        return out


def local_params(src):
    """Parameter definitions private to one controller file: @attr %Parameter{...} and defp x_param do ... end."""
    out = {}
    for m in re.finditer(r"\n  @([a-z_0-9]+) %(?:OpenApiSpex\.)?Parameter\{", src):
        out["@" + m.group(1)] = collect_struct(src, m.end() - 1)
    for m in re.finditer(r"\n  defp? ([a-z_0-9]+)(?:\(\))? do\n(.*?)\n  end\n", src, re.S):
        body = m.group(2)
        if "Parameter{" in body:
            out[m.group(1)] = collect_struct(body, body.index("Parameter{"))
    for m in re.finditer(r"\n  defp? ([a-z_0-9]+)(?:\(\))?, do: @([a-z_0-9]+)", src):
        if "@" + m.group(2) in out:
            out[m.group(1)] = out["@" + m.group(2)]
    maps = {}
    for m in re.finditer(r"\n  @([a-z_0-9]+) %\{", src):
        body = collect_struct(src, m.end() - 1)
        entries = {}
        for e in re.finditer(r'"([\w-]+)"\s*=>\s*%(?:OpenApiSpex\.)?Parameter\{', body):
            entries[e.group(1)] = collect_struct(body, e.end() - 1)
        if entries:
            maps[m.group(1)] = entries
    return out, maps


def resolve_params(expr, helpers, local, local_maps, warnings, where):
    """Resolve the Elixir expression of an operation's parameters: list into OpenAPI parameters."""
    params = []
    expr = (expr or "").strip()
    if not expr:
        return params
    items = []
    for chunk in split_top(expr, seps=("++",)):
        chunk = chunk.strip()
        if chunk.startswith("[") and chunk.endswith("]"):
            chunk = chunk[1:-1]
        for item in split_top(chunk, seps=(",", "|")):
            if item.strip():
                items.append(item.strip())
    for item in items:
        item = item.strip()
        if not item:
            continue
        if item.startswith("%") and "Parameter{" in item:
            params.append(parse_parameter(collect_struct(item, item.index("{")), local))
            continue
        km = re.match(r"^([a-z_0-9]+):\s*\[(.*)\]$", item, re.S)
        if km:
            params.append(parse_parameter(f"name: :{km.group(1)}, {km.group(2)}", local))
            continue
        if item.startswith("@"):
            if item in local:
                params.append(parse_parameter(local[item], local))
            elif item[1:] in helpers.attrs:
                params.append(parse_parameter(helpers.attrs[item[1:]], local))
            else:
                warnings.append(f"{where}: unresolved attribute {item}")
            continue
        m = re.match(r"^(?:[\w.]+\.)?([a-z_0-9]+)\((.*)\)$", item, re.S)
        if not m:
            warnings.append(f"{where}: unresolved parameter expression {item[:60]}")
            continue
        fname, args = m.group(1), m.group(2).strip()
        if fname == "base_params":
            params.append(parse_parameter(helpers.funcs["api_key_param"][1]))
            params.append(parse_parameter(helpers.funcs["key_param"][1]))
            continue
        if fname in ("define_paging_params", "define_state_changes_paging_params", "define_search_paging_params"):
            fields = re.findall(r'"([\w-]+)"', args)
            map_name = {"define_paging_params": "paging_params", "define_state_changes_paging_params": "state_changes_paging_params", "define_search_paging_params": "search_paging_params"}[fname]
            table = local_maps.get(map_name) or helpers.maps.get(map_name) or {}
            if map_name == "paging_params" and item.startswith("Parameters."):
                table = local_maps.get(map_name) or table
            for field in fields:
                if field in table:
                    params.append(parse_parameter(table[field], local))
                else:
                    params.extend(helpers.paging(map_name, [field]))
            continue
        if fname in local:
            params.append(parse_parameter(local[fname], local))
            continue
        if fname in helpers.funcs:
            fargs, block = helpers.funcs[fname]
            param = parse_parameter(block, local)
            if fargs and args:
                vals = re.findall(r'"([^"]*)"', args)
                if vals:
                    param["schema"] = {"type": "string", "enum": vals}
            params.append(param)
            continue
        warnings.append(f"{where}: unresolved helper {fname}()")
    deduped, seen = [], set()
    for p in params:
        if p.get("name") and (p["name"], p["in"]) not in seen:
            seen.add((p["name"], p["in"]))
            deduped.append(p)
    return deduped


# --------------------------------------------------------------------------- operation specs


class OperationSpecs:
    """OpenApiSpex `operation` blocks keyed by (controller module, action)."""

    def __init__(self, helpers):
        self.helpers = helpers
        self.ops = {}
        self.tags = {}
        self.warnings = []
        files = glob.glob(os.path.join(WEB, "controllers", "**", "*.ex"), recursive=True)
        for path in sorted(files):
            src = read(path)
            if "operation :" not in src and "operation(" not in src:
                continue
            self.load(path, src)

    def load(self, path, src):
        modm = re.search(r"defmodule ([\w.]+)", src)
        if not modm:
            return
        module = modm.group(1)
        tagm = re.search(r"\n\s*tags\(?\[([^\]]*)\]", src)
        if tagm:
            self.tags[module] = re.findall(r'"([^"]*)"', tagm.group(1))
        local, local_maps = local_params(strip_docs(src))
        clean = strip_docs(src)
        pattern = re.compile(r"\n  operation\(?\s*:(\w+),\n(.*?)(?=\n  (?:@doc|@spec|def |defp |operation|@[a-z_]+ |plug|action_fallback|alias|import|require|use |end\s*$))", re.S)
        for m in pattern.finditer(clean):
            action, body = m.group(1), m.group(2)
            summary = elixir_string(kw(body, "summary"))
            description = elixir_string(kw(body, "description"))
            params_expr = kw(body, "parameters")
            responses_expr = kw(body, "responses")
            request_body = kw(body, "request_body")
            params = resolve_params(params_expr, self.helpers, local, local_maps, self.warnings, f"{module}.{action}")
            responses = {}
            if responses_expr:
                inner = responses_expr.strip()
                if inner.startswith("["):
                    inner = inner[1:-1]
                for item in split_top(inner):
                    km = re.match(r"^(\w+):\s*(.*)$", item.strip(), re.S)
                    if not km:
                        continue
                    key, value = km.group(1), km.group(2).strip()
                    code = RESPONSE_CODES.get(key)
                    if not code:
                        continue
                    desc = None
                    content_type = "application/json"
                    schema_ref = None
                    tm = re.match(r'^\{\s*"((?:[^"\\]|\\.)*)"\s*,\s*"([^"]+)"\s*,\s*(.*)\}$', value, re.S)
                    if tm:
                        desc, content_type, schema_ref = tm.group(1), tm.group(2), tm.group(3).strip()
                    responses[code] = {"description": desc or self.default_desc(key), "content_type": content_type, "schema_src": schema_ref}
            self.ops[(module, action)] = {
                "summary": summary,
                "description": description,
                "parameters": params,
                "responses": responses,
                "request_body": request_body,
            }

    @staticmethod
    def default_desc(key):
        return {
            "ok": "Successful response",
            "created": "Created",
            "no_content": "No content",
            "bad_request": "Bad request",
            "unauthorized": "Unauthorized",
            "forbidden": "Forbidden",
            "not_found": "Not found",
            "unprocessable_entity": "Invalid parameter",
            "too_many_requests": "Rate limit exceeded",
            "internal_server_error": "Internal server error",
            "service_unavailable": "Service unavailable",
        }.get(key, key)


def schema_from_src(src, components):
    """Best-effort schema for a response schema expression; $ref when the component exists."""
    if not src:
        return {"type": "object"}
    src = src.strip()
    m = re.match(r"^(?:BlockScoutWeb\.)?Schemas\.([\w.]+)$", src)
    if m:
        name = m.group(1).replace(".", "")
        if name in components:
            return {"$ref": f"#/components/schemas/{name}"}
        for cand in components:
            if cand.replace("_", "").lower() == name.lower():
                return {"$ref": f"#/components/schemas/{cand}"}
        return {"type": "object", "description": f"Backend schema module BlockScoutWeb.Schemas.{m.group(1)}"}
    if src.startswith("paginated_response"):
        items = re.search(r"items:\s*(.*?)(?:,\s*next_page_params_example|\)\s*$)", src, re.S)
        inner = schema_from_src(items.group(1).strip() if items else "", components)
        return {
            "type": "object",
            "required": ["items", "next_page_params"],
            "properties": {
                "items": {"type": "array", "items": inner},
                "next_page_params": {"type": "object", "nullable": True, "description": "Pass these keys as query parameters to fetch the next page"},
            },
        }
    if src.startswith("%Schema{"):
        body = collect_struct(src, src.index("{"))
        items = kw(body, "items")
        typ = atom_or_string(kw(body, "type"))
        any_of = kw(body, "anyOf")
        if any_of:
            inner = split_top(any_of.strip()[1:-1])
            return {"anyOf": [schema_from_src(i, components) for i in inner]}
        if typ == "array" and items:
            return {"type": "array", "items": schema_from_src(items, components)}
        return parse_schema(src)
    return {"type": "object", "description": f"see backend schema {src[:80]}"}


# --------------------------------------------------------------------------- RPC docs


def elixir_literal_to_json(text):
    t = text
    t = re.sub(r"%\{", "{", t)
    t = t.replace("=>", ":")
    t = re.sub(r"\bnil\b", "null", t)
    t = re.sub(r"(?<=[\[{,\s]):(\w+)(?=\s*[,\]}])", r'"\1"', t)
    t = re.sub(r",(\s*[}\]])", r"\1", t)
    try:
        return json.loads(t)
    except Exception:
        return None


def load_rpc_docs(env):
    path = os.path.join(WEB, "etherscan.ex")
    src = read(path)
    examples = {}
    for m in re.finditer(r"\n  @([a-z_0-9]+_example_value(?:_error)?) %\{", src):
        body = "{" + collect_struct(src, m.end() - 1) + "}"
        val = elixir_literal_to_json(body)
        if val is not None:
            examples[m.group(1)] = val
    actions = {}
    for m in re.finditer(r"\n\s*@([a-z_0-9]+)_action %\{", src):
        attr = m.group(1)
        body = collect_struct(src, m.end() - 1)
        name = elixir_string(kw(body, "name"))
        description = elixir_string(kw(body, "description")) or ""
        params = {}
        for kind in ("required_params", "optional_params"):
            lst = kw(body, kind) or "[]"
            items = []
            inner = lst.strip()[1:-1]
            for item in split_top(inner):
                item = item.strip()
                if not item.startswith("%{"):
                    continue
                pb = collect_struct(item, 1)
                items.append({
                    "key": elixir_string(kw(pb, "key")),
                    "placeholder": elixir_string(kw(pb, "placeholder")),
                    "type": elixir_string(kw(pb, "type")),
                    "description": elixir_string(kw(pb, "description")) or "",
                })
            params[kind] = items
        responses = []
        rl = kw(body, "responses") or "[]"
        for item in split_top(rl.strip()[1:-1]):
            item = item.strip()
            if not item.startswith("%{"):
                continue
            rb = collect_struct(item, 1)
            ex = kw(rb, "example_value") or ""
            exm = re.search(r"@([a-z_0-9]+)", ex)
            responses.append({
                "code": elixir_string(kw(rb, "code")),
                "description": elixir_string(kw(rb, "description")),
                "example": examples.get(exm.group(1)) if exm else None,
            })
        actions[attr] = {"name": name, "description": description.strip(), **params, "responses": responses}
    modules = []
    for m in re.finditer(r"\n  @([a-z_0-9]+)_module %\{", src):
        body = collect_struct(src, m.end() - 1)
        name = elixir_string(kw(body, "name"))
        acts = kw(body, "actions") or ""
        refs = re.findall(r"@([a-z_0-9]+)", acts)
        expanded = []
        for ref in refs:
            if ref.endswith("_action"):
                expanded.append(ref[: -len("_action")])
            else:
                lm = re.search(rf"\n  @{ref} (?:if .*?, do: )?\[(.*?)\]", src, re.S)
                if lm:
                    chosen = lm.group(1)
                    if "if @bridged_tokens_enabled" in (src[lm.start(): lm.end()]) and not env["bridged_tokens_enabled"]:
                        em = re.search(rf"\n  @{ref} if .*?else: @([a-z_0-9]+)", src, re.S)
                        if em:
                            inner = re.search(rf"\n  @{em.group(1)} \[(.*?)\]", src, re.S)
                            chosen = inner.group(1) if inner else ""
                    for r2 in re.findall(r"@([a-z_0-9]+)_action", chosen):
                        expanded.append(r2)
                    for r3 in re.findall(r"@([a-z_0-9]+)(?<!_action)\b", chosen):
                        inner = re.search(rf"\n  @{r3} \[(.*?)\]", src, re.S)
                        if inner:
                            expanded.extend(re.findall(r"@([a-z_0-9]+)_action", inner.group(1)))
        modules.append((name, expanded))
    return actions, modules


def rpc_controller_actions():
    out = {}
    for path in glob.glob(os.path.join(WEB, "controllers", "api", "rpc", "*_controller.ex")):
        mod = os.path.basename(path)[: -len("_controller.ex")]
        names = sorted(set(re.findall(r"\n\s*def ([a-z_0-9]+)\(conn", read(path))))
        out[mod] = names
    return out


def eth_rpc_methods():
    path = os.path.join(BACKEND, "apps", "explorer", "lib", "explorer", "eth_rpc.ex")
    if not os.path.exists(path):
        return []
    return sorted(set(re.findall(r'"(eth_[A-Za-z]+)"', read(path))))


# --------------------------------------------------------------------------- assembly


def tag_for(path):
    if path.startswith("/api/v2/"):
        seg = path.split("/")[3]
        if seg.startswith("{"):
            return "api-v2"
        return {"paxeer-x": "paxeer-x", "main-page": "main-page", "token-transfers": "token-transfers", "internal-transactions": "internal-transactions"}.get(seg, seg)
    if path.startswith("/api/account/"):
        return "account"
    if path.startswith("/api/health"):
        return "health"
    if path.startswith("/api/eth-rpc") or path.startswith("/api/v1/eth-rpc"):
        return "eth-rpc"
    if path.startswith("/api/v1/graphql") or path.startswith("/graphiql") or path == "/schema.graphql":
        return "graphql"
    if path.startswith("/api/v1"):
        return "api-v1"
    if path == "/api":
        return "rpc"
    if path.startswith("/auth") or path.startswith("/account"):
        return "account"
    if path.startswith("/admin"):
        return "admin"
    if path.startswith("/verify_smart_contract"):
        return "smart-contracts"
    if path in ("/robots.txt", "/sitemap.xml", "/api-docs", "/eth-rpc-api-docs"):
        return "site"
    return "web"


def path_params(path):
    return re.findall(r"\{(\w+)\}", path)


def to_openapi_path(path):
    path = re.sub(r":(\w+)", r"{\1}", path)
    path = re.sub(r"\*(\w+)", r"{\1}", path)
    return path


PATH_PARAM_SCHEMAS = {
    "address_hash_param": HASH40,
    "address_hash": HASH40,
    "transaction_hash_param": HASH64,
    "transaction_hash": HASH64,
    "block_hash_or_number_param": {"type": "string", "description": "Block number or 0x-prefixed block hash"},
    "block_number_param": {"type": "integer", "minimum": 0},
    "token_id_param": {"type": "string", "pattern": "^[0-9]+$"},
    "hash": HASH64,
    "id": {"type": "string"},
    "market_id": {"type": "string"},
    "operation_hash_param": HASH64,
    "blob_hash_param": HASH64,
    "platform_id": {"type": "string"},
    "provider": {"type": "string", "description": "Ueberauth provider, e.g. auth0"},
    "api_key": {"type": "string"},
    "path": {"type": "string", "description": "Any remaining path"},
}


def operation_for(route, specs, components, env):
    module = route.controller
    spec = specs.ops.get((module, route.action))
    if not spec:
        short = module.split(".")
        for (m, a), s in specs.ops.items():
            if a == route.action and m.split(".")[-1] == short[-1]:
                spec = s
                break
    op_path = to_openapi_path(route.path)
    params = []
    seen = set()
    if spec:
        for p in spec["parameters"]:
            seen.add((p["name"], p["in"]))
            params.append(p)
    for name in path_params(op_path):
        if (name, "path") in seen:
            continue
        for p in params:
            if p["name"] == name and p["in"] == "query":
                p["in"] = "path"
                p["required"] = True
                seen.add((name, "path"))
                break
        if (name, "path") in seen:
            continue
        params.append({"name": name, "in": "path", "required": True, "schema": dict(PATH_PARAM_SCHEMAS.get(name, {"type": "string"}))})
        seen.add((name, "path"))
    params = [p for p in params if p["in"] != "path" or p["name"] in path_params(op_path)]
    short_ctrl = module.replace("BlockScoutWeb.", "")
    operation = {
        "tags": [tag_for(op_path)],
        "summary": (spec or {}).get("summary") or default_summary(route),
    }
    if spec and spec.get("description"):
        operation["description"] = spec["description"]
    operation["operationId"] = f"{short_ctrl.replace('.', '_')}_{route.action}"
    if params:
        operation["parameters"] = params
    if route.method in ("post", "put", "patch", "delete") and op_path.startswith(("/api/", "/verify_smart_contract")):
        operation["requestBody"] = request_body_for(route)
    operation["responses"] = responses_for(route, spec, components, op_path)
    if op_path.startswith("/api/account/v2") or op_path.startswith("/account/v2"):
        operation["security"] = [{"sessionCookie": []}, {"apiKey": []}]
    operation["x-controller"] = f"{module}.{route.action}"
    if route.notes:
        operation["x-compile-time-conditions"] = route.notes
    return op_path, operation


def default_summary(route):
    words = route.action.replace("_", " ")
    ctrl = route.controller.split(".")[-1].replace("Controller", "")
    return f"{words[:1].upper()}{words[1:]} ({ctrl})"


def request_body_for(route):
    ctype = "application/json"
    if "verification" in route.path or "verify_smart_contract" in route.path or "audit-reports" in route.path or "import" in route.path:
        return {
            "required": True,
            "content": {
                "application/json": {"schema": {"type": "object", "additionalProperties": True}},
                "multipart/form-data": {"schema": {"type": "object", "additionalProperties": True}},
            },
            "description": "Request fields as documented for this endpoint by the backend controller",
        }
    return {"required": False, "content": {ctype: {"schema": {"type": "object", "additionalProperties": True}}}}


def responses_for(route, spec, components, op_path):
    out = {}
    if spec and spec["responses"]:
        for code, r in sorted(spec["responses"].items()):
            entry = {"description": r["description"]}
            if r["content_type"] and code not in ("204",):
                entry["content"] = {r["content_type"]: {"schema": schema_from_src(r["schema_src"], components)}}
            out[code] = entry
    if not out:
        if op_path.endswith("/csv"):
            out["200"] = {"description": "CSV export", "content": {"text/csv": {"schema": {"type": "string"}}}}
        elif tag_for(op_path) in ("web", "site", "admin") or op_path.startswith("/auth") or (route.controller.startswith("BlockScoutWeb.Account.") and "/v2/" not in op_path):
            out["200"] = {"description": "HTML page", "content": {"text/html": {"schema": {"type": "string"}}}}
            if op_path.startswith("/auth"):
                out["302"] = {"description": "Redirect to the identity provider or back to the explorer"}
        else:
            out["200"] = {"description": "Successful response", "content": {"application/json": {"schema": {"type": "object", "additionalProperties": True}}}}
    if op_path.startswith("/api/v2") or op_path.startswith("/api/account/v2"):
        out.setdefault("429", {"$ref": "#/components/responses/RateLimited"})
        if op_path.startswith("/api/account/v2"):
            out.setdefault("401", {"$ref": "#/components/responses/Unauthorized"})
    if "{" in op_path and op_path.startswith("/api/v2") and "404" not in out:
        out.setdefault("404", {"$ref": "#/components/responses/NotFound"})
    if "{" in op_path and op_path.startswith("/api/v2"):
        out.setdefault("422", {"$ref": "#/components/responses/InvalidParameter"})
    return dict(sorted(out.items()))


def rpc_operations(mount, module_map, rpc_docs, modules_order, controller_actions, write_actions, env, notes):
    documented = {}
    for attr, doc in rpc_docs.items():
        for mod, _ in modules_order:
            if attr.startswith(mod + "_") and doc.get("name"):
                documented[(mod, doc["name"].lower())] = doc
    rows = []
    param_union = {}
    for module_name, controller, writes in module_map:
        ctrl_key = snake(controller.split(".")[-1].replace("Controller", ""))
        actions = controller_actions.get(ctrl_key, [])
        for action in actions:
            doc = documented.get((module_name, action.lower()))
            row = {
                "module": module_name,
                "action": action,
                "write": action in writes,
                "description": (doc or {}).get("description") or "Implemented by the backend; not described in the API docs module.",
                "required_params": [p for p in (doc or {}).get("required_params", [])],
                "optional_params": [p for p in (doc or {}).get("optional_params", [])],
            }
            ex = next((r.get("example") for r in (doc or {}).get("responses", []) if r.get("example") and r.get("description") == "successful operation"), None)
            if ex is not None:
                row["example"] = ex
            rows.append(row)
            for p in row["required_params"] + row["optional_params"]:
                entry = param_union.setdefault(p["key"], {"type": p.get("type") or "string", "description": p.get("description") or "", "used_by": []})
                entry["used_by"].append(f"{module_name}.{action}")
    module_names = [m for m, _, _ in module_map]
    action_names = sorted({r["action"] for r in rows})
    parameters = [
        {"name": "module", "in": "query", "required": True, "schema": {"type": "string", "enum": module_names}, "description": "RPC module"},
        {"name": "action", "in": "query", "required": True, "schema": {"type": "string", "enum": action_names}, "description": "RPC action of the module; see x-rpc-actions for the parameters of each module/action pair"},
        {"name": "apikey", "in": "query", "required": False, "schema": {"type": "string"}, "description": "API key for rate limiting"},
    ]
    for key, entry in sorted(param_union.items()):
        typ = "integer" if entry["type"] in ("integer", "number") else "string"
        parameters.append({
            "name": key,
            "in": "query",
            "required": False,
            "schema": {"type": typ},
            "description": f"{entry['description']} Used by: {', '.join(sorted(set(entry['used_by'])))}.".strip(),
        })
    description = (
        "Etherscan-compatible RPC API. Every call is `?module=<module>&action=<action>` plus the action's own "
        "parameters; GET and POST (form or JSON body) are equivalent. The response envelope is "
        "`{\"status\": \"1\"|\"0\", \"message\": \"OK\"|<error>, \"result\": ...}`. The full table of module/action "
        "pairs with their required and optional parameters is in the `x-rpc-actions` extension of this operation."
    )
    responses = {
        "200": {
            "description": "RPC envelope; `status` is \"1\" on success and \"0\" on an error, with the error text in `message`",
            "content": {"application/json": {"schema": {"$ref": "#/components/schemas/RpcEnvelope"}}},
        },
        "429": {"$ref": "#/components/responses/RateLimited"},
    }
    ops = {}
    for method in ("get", "post"):
        op = {
            "tags": ["rpc"],
            "summary": f"Etherscan-style RPC call ({method.upper()}) at {mount}",
            "description": description,
            "operationId": f"rpc_{method}_{mount.strip('/').replace('/', '_') or 'root'}",
            "parameters": parameters,
            "responses": responses,
            "x-rpc-actions": rows,
        }
        if method == "post":
            op["requestBody"] = {
                "required": False,
                "content": {
                    "application/x-www-form-urlencoded": {"schema": {"type": "object", "additionalProperties": True, "description": "Same keys as the query parameters"}},
                    "application/json": {"schema": {"type": "object", "additionalProperties": True, "description": "Same keys as the query parameters"}},
                },
            }
        if notes:
            op["x-compile-time-conditions"] = notes
        ops[method] = op
    return ops


def eth_rpc_operation(path, methods):
    return {
        "tags": ["eth-rpc"],
        "summary": f"JSON-RPC 2.0 endpoint ({path})",
        "description": "Ethereum JSON-RPC methods served from the explorer database. A request may be a single object or a batch array.",
        "operationId": "eth_rpc_" + path.strip("/").replace("/", "_").replace("-", "_"),
        "requestBody": {
            "required": True,
            "content": {
                "application/json": {
                    "schema": {
                        "oneOf": [
                            {"$ref": "#/components/schemas/JsonRpcRequest"},
                            {"type": "array", "items": {"$ref": "#/components/schemas/JsonRpcRequest"}},
                        ]
                    }
                }
            },
        },
        "responses": {
            "200": {
                "description": "JSON-RPC response or batch of responses",
                "content": {
                    "application/json": {
                        "schema": {
                            "oneOf": [
                                {"$ref": "#/components/schemas/JsonRpcResponse"},
                                {"type": "array", "items": {"$ref": "#/components/schemas/JsonRpcResponse"}},
                            ]
                        }
                    }
                },
            },
            "429": {"$ref": "#/components/responses/RateLimited"},
        },
        "x-supported-methods": methods,
    }


def graphql_operations(path):
    body = {
        "required": True,
        "content": {"application/json": {"schema": {"$ref": "#/components/schemas/GraphQLRequest"}}, "application/graphql": {"schema": {"type": "string"}}},
    }
    responses = {
        "200": {"description": "GraphQL result", "content": {"application/json": {"schema": {"$ref": "#/components/schemas/GraphQLResponse"}}}},
        "429": {"$ref": "#/components/responses/RateLimited"},
    }
    return {
        "post": {"tags": ["graphql"], "summary": "GraphQL query", "description": "Absinthe GraphQL endpoint over the explorer schema (see GET /schema.graphql). Complexity and token limits apply.", "operationId": "graphql_post", "requestBody": body, "responses": responses},
        "get": {
            "tags": ["graphql"],
            "summary": "GraphQL query via query string",
            "operationId": "graphql_get",
            "parameters": [
                {"name": "query", "in": "query", "required": True, "schema": {"type": "string"}},
                {"name": "variables", "in": "query", "required": False, "schema": {"type": "string"}, "description": "JSON-encoded variables"},
                {"name": "operationName", "in": "query", "required": False, "schema": {"type": "string"}},
            ],
            "responses": responses,
        },
    }


def base_components():
    return {
        "securitySchemes": {
            "sessionCookie": {"type": "apiKey", "in": "cookie", "name": "_explorer_key", "description": "Phoenix session cookie set after /api/account/v2 authentication"},
            "apiKey": {"type": "apiKey", "in": "query", "name": "apikey", "description": "API key that lifts the per-IP rate limit"},
        },
        "responses": {
            "RateLimited": {"description": "Rate limit exceeded", "content": {"application/json": {"schema": {"$ref": "#/components/schemas/Message"}}}},
            "NotFound": {"description": "Not found", "content": {"application/json": {"schema": {"$ref": "#/components/schemas/Message"}}}},
            "InvalidParameter": {"description": "Invalid parameter", "content": {"application/json": {"schema": {"$ref": "#/components/schemas/Message"}}}},
            "Unauthorized": {"description": "Not authenticated", "content": {"application/json": {"schema": {"$ref": "#/components/schemas/Message"}}}},
        },
        "schemas": {
            "Message": {"type": "object", "properties": {"message": {"type": "string"}}, "required": ["message"]},
            "RpcEnvelope": {
                "type": "object",
                "required": ["status", "message", "result"],
                "properties": {
                    "status": {"type": "string", "enum": ["0", "1"]},
                    "message": {"type": "string"},
                    "result": {"nullable": True, "description": "Action-specific payload; see x-rpc-actions"},
                },
            },
            "JsonRpcRequest": {
                "type": "object",
                "required": ["jsonrpc", "method", "id"],
                "properties": {
                    "jsonrpc": {"type": "string", "enum": ["2.0"]},
                    "id": {"oneOf": [{"type": "integer"}, {"type": "string"}]},
                    "method": {"type": "string"},
                    "params": {"type": "array", "items": {}},
                },
            },
            "JsonRpcResponse": {
                "type": "object",
                "required": ["jsonrpc", "id"],
                "properties": {
                    "jsonrpc": {"type": "string", "enum": ["2.0"]},
                    "id": {"oneOf": [{"type": "integer"}, {"type": "string"}], "nullable": True},
                    "result": {},
                    "error": {"type": "object", "properties": {"code": {"type": "integer"}, "message": {"type": "string"}}},
                },
            },
            "GraphQLRequest": {
                "type": "object",
                "required": ["query"],
                "properties": {"query": {"type": "string"}, "variables": {"type": "object", "additionalProperties": True}, "operationName": {"type": "string"}},
            },
            "GraphQLResponse": {
                "type": "object",
                "properties": {"data": {"type": "object", "additionalProperties": True, "nullable": True}, "errors": {"type": "array", "items": {"type": "object", "additionalProperties": True}}},
            },
        },
    }


def merge_spex(paths, components, spex_files):
    """Replace static operations with the ones exported by OpenApiSpex, and merge their component schemas."""
    merged = 0
    for file in spex_files:
        doc = yaml.safe_load(read(file))
        prefix = ""
        for server in doc.get("servers", []):
            url = server.get("url", "")
            m = re.match(r"^https?://[^/]+(/.*)?$", url)
            if m and m.group(1):
                prefix = m.group(1).rstrip("/")
            elif url.startswith("/"):
                prefix = url.rstrip("/")
        for name, schema in (doc.get("components", {}) or {}).get("schemas", {}).items():
            components["schemas"].setdefault(name, schema)
        for name, resp in (doc.get("components", {}) or {}).get("responses", {}).items():
            components["responses"].setdefault(name, resp)
        for path, item in (doc.get("paths") or {}).items():
            full = prefix + path if not path.startswith(prefix) or not prefix else path
            if len(full) > 1:
                full = full.rstrip("/")
            if full not in paths:
                alt = next((p for p in paths if p.lower() == full.lower()), None)
                if alt is None:
                    continue
                full = alt
            for method, op in item.items():
                if method not in VERBS:
                    continue
                if method in paths[full]:
                    keep = paths[full][method]
                    new = dict(op)
                    new["tags"] = keep.get("tags")
                    new["x-controller"] = keep.get("x-controller")
                    if keep.get("x-compile-time-conditions"):
                        new["x-compile-time-conditions"] = keep["x-compile-time-conditions"]
                    new["x-source"] = "OpenApiSpex export"
                    paths[full][method] = new
                    merged += 1
    return merged


def build(args):
    env = {
        "chain_type": args.chain_type,
        "chain_identity": (args.chain_type, None),
        "admin_panel_enabled": args.include_admin,
        "graphql_enabled": True,
        "api_router_reading_enabled": True,
        "web_router_enabled": args.include_web,
        "reading_enabled": True,
        "writing_enabled": True,
        "mud_enabled_compile_time": False,
        "bridged_tokens_enabled": False,
        "mix_env": "prod",
    }
    walker = RouterWalker(env)
    walker.walk("Router")
    helpers = Helpers()
    specs = OperationSpecs(helpers)
    components = base_components()

    paths = {}
    seen_ops = {}
    for route in walker.routes:
        op_path, op = operation_for(route, specs, components["schemas"], env)
        if op_path.startswith("/api/v2/{path}"):
            op["summary"] = "Catch-all for unknown /api/v2 paths"
            op["responses"] = {"404": {"$ref": "#/components/responses/NotFound"}}
        base_id = op["operationId"]
        n = seen_ops.get(base_id, 0) + 1
        seen_ops[base_id] = n
        if n > 1:
            op["operationId"] = f"{base_id}_{n}"
        item = paths.setdefault(op_path, {})
        if route.method in item:
            continue
        item[route.method] = op

    rpc_docs, modules_order = load_rpc_docs(env)
    controller_actions = rpc_controller_actions()
    for kind, mount, modules, notes, router in walker.specials:
        if kind == "rpc":
            module_map = [(name, ctrl, re.findall(r":(\w+)", writes)) for name, ctrl, writes in modules]
            paths.setdefault(mount, {}).update(rpc_operations(mount, module_map, rpc_docs, modules_order, controller_actions, None, env, notes))
        elif kind == "graphql":
            paths.setdefault(mount, {}).update(graphql_operations(mount))
        elif kind == "graphiql":
            paths.setdefault(mount, {})["get"] = {
                "tags": ["graphql"],
                "summary": "GraphiQL interactive explorer",
                "operationId": "graphiql",
                "responses": {"200": {"description": "HTML page", "content": {"text/html": {"schema": {"type": "string"}}}}},
            }
    methods = eth_rpc_methods()
    for path in list(paths):
        if path.endswith("/eth-rpc"):
            paths[path]["post"] = eth_rpc_operation(path, methods)

    merged = 0
    if args.spex:
        merged = merge_spex(paths, components, args.spex)

    order = ["paxeer-x", "search", "config", "stats", "main-page", "blocks", "transactions", "token-transfers", "internal-transactions", "addresses", "tokens", "smart-contracts", "withdrawals", "advanced-filters", "proxy", "import", "key", "utils", "scam-badge-addresses", "openapi", "account", "health", "api-v1", "rpc", "eth-rpc", "graphql", "site", "web", "admin"]

    def sort_key(p):
        tag = tag_for(p)
        return (order.index(tag) if tag in order else len(order), p)

    sorted_paths = {p: paths[p] for p in sorted(paths, key=sort_key)}
    used_tags = []
    for item in sorted_paths.values():
        for op in item.values():
            for t in op.get("tags", []):
                if t not in used_tags:
                    used_tags.append(t)
    tag_desc = {
        "paxeer-x": "Paxeer X Network surfaces: capabilities, anchors, kernel receipts, price feed, settlement status and sequencer signatures",
        "account": "My-account API (watchlists, tags, API keys, custom ABIs) and authentication; session cookie required",
        "rpc": "Etherscan-compatible module/action API (/api and /api/v1)",
        "eth-rpc": "Ethereum JSON-RPC served from the explorer database",
        "graphql": "GraphQL endpoint and schema",
        "health": "Liveness, readiness and indexing health",
        "api-v1": "Legacy v1 JSON endpoints",
        "api-v2": "Catch-all for unknown API v2 paths",
        "site": "Site metadata and documentation pages",
        "web": "Server-rendered HTML pages of the legacy web app (DISABLE_WEBAPP=false)",
        "admin": "Admin panel (ADMIN_PANEL_ENABLED=true)",
    }
    tags = [{"name": t, **({"description": tag_desc[t]} if t in tag_desc else {})} for t in sorted(used_tags, key=lambda t: order.index(t) if t in order else len(order))]

    route_count = sum(len([m for m in item if m in VERBS]) for item in sorted_paths.values())
    doc = {
        "openapi": "3.0.3",
        "info": {
            "title": "Paxeer Explorer API",
            "version": args.version,
            "description": (
                "Every HTTP route served by the Paxeer explorer backend (Blockscout fork) as compiled for "
                f"CHAIN_TYPE={args.chain_type}. Generated from the Phoenix routers, the OpenApiSpex operation specs of "
                "the API v2 controllers and the RPC documentation module by deploy/tools/gen-swagger.py. "
                f"{route_count} operations across {len(sorted_paths)} paths."
                + ("" if args.include_web else " The legacy server-rendered web app (DISABLE_WEBAPP, default true) is excluded; pass --include-web to add it.")
                + ("" if args.include_admin else " The admin panel (ADMIN_PANEL_ENABLED, default false) is excluded; pass --include-admin to add it.")
                + " The backend also serves its own OpenApiSpex document at GET /api/v2/openapi."
            ),
            "license": {"name": "GPL-3.0", "url": "https://www.gnu.org/licenses/gpl-3.0.html"},
        },
        "servers": [{"url": "/", "description": "Same origin as the explorer"}],
        "tags": tags,
        "paths": sorted_paths,
        "components": components,
    }
    return doc, walker, specs, merged


class Dumper(yaml.SafeDumper):
    pass


def str_presenter(dumper, data):
    if "\n" in data:
        return dumper.represent_scalar("tag:yaml.org,2002:str", data, style="|")
    return dumper.represent_scalar("tag:yaml.org,2002:str", data)


Dumper.add_representer(str, str_presenter)


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--chain-type", default="paxeer_x")
    ap.add_argument("--include-web", action="store_true", help="include the legacy HTML web router (DISABLE_WEBAPP=false)")
    ap.add_argument("--include-admin", action="store_true", help="include the admin panel router (ADMIN_PANEL_ENABLED=true)")
    ap.add_argument("--spex", action="append", default=[], help="OpenAPI YAML exported with mix openapi.spec.yaml; may repeat")
    ap.add_argument("--version", default=None)
    ap.add_argument("-o", "--output", default=os.path.join(EXPLORER, "swagger.yaml"))
    ap.add_argument("--report", action="store_true", help="print the route inventory and resolution warnings to stderr")
    args = ap.parse_args()
    if not args.version:
        vm = re.search(r'version:\s*"([^"]+)"', read(os.path.join(BACKEND, "mix.exs")))
        args.version = vm.group(1) if vm else "0.0.0"
    doc, walker, specs, merged = build(args)
    with open(args.output, "w", encoding="utf-8") as f:
        yaml.dump(doc, f, Dumper=Dumper, sort_keys=False, allow_unicode=True, width=120)
    ops = sum(len([m for m in item if m in VERBS]) for item in doc["paths"].values())
    print(f"wrote {args.output}: {len(doc['paths'])} paths, {ops} operations, {len(doc['components']['schemas'])} schemas, {merged} operations taken from OpenApiSpex exports", file=sys.stderr)
    if args.report:
        for r in sorted(walker.routes, key=lambda r: (r.path, r.method)):
            print(f"{r.method.upper():7} {r.path:70} {r.controller}.{r.action} [{r.router}]" + (f" {r.notes}" if r.notes else ""), file=sys.stderr)
        for kind, mount, modules, notes, router in walker.specials:
            print(f"{kind:7} {mount}", file=sys.stderr)
        for w in sorted(set(specs.warnings)):
            print(f"warning: {w}", file=sys.stderr)


if __name__ == "__main__":
    main()
