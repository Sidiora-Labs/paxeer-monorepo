"""Describe the named dependencies of a PostgreSQL SELECT or INSERT statement."""

import json
import sys

from pglast.parser import parse_sql_json


def nodes(value):
    if isinstance(value, dict):
        yield value
        for child in value.values():
            yield from nodes(child)
    elif isinstance(value, list):
        for child in value:
            yield from nodes(child)


def names(parts):
    return [part["String"]["sval"] for part in parts if "String" in part]


def describe(sql):
    statements = json.loads(parse_sql_json(sql))["stmts"]
    kinds = {"SelectStmt": "SELECT", "InsertStmt": "INSERT"}
    if len(statements) != 1 or len(statements[0]["stmt"]) != 1:
        raise ValueError("expected one SELECT or INSERT statement")
    tree = statements[0]["stmt"]
    kind = kinds.get(next(iter(tree)))
    if kind is None:
        raise ValueError("expected one SELECT or INSERT statement")
    ctes = {node["CommonTableExpr"]["ctename"] for node in nodes(tree)
            if "CommonTableExpr" in node}
    tables, functions, columns = set(), set(), set()
    if kind == "INSERT":
        insert = tree["InsertStmt"]
        relation = insert["relation"]
        tables.add(relation.get("schemaname", "chat") + "." + relation["relname"])
        columns.update(column["ResTarget"]["name"] for column in insert.get("cols", []))
        conflict = insert.get("onConflictClause", {})
        columns.update(target["ResTarget"]["name"] for target in conflict.get("targetList", []))
    for node in nodes(tree):
        if "RangeVar" in node:
            relation = node["RangeVar"]
            if relation.get("schemaname") or relation["relname"] not in ctes:
                tables.add(relation.get("schemaname", "chat") + "." + relation["relname"])
        if "FuncCall" in node:
            function = names(node["FuncCall"]["funcname"])
            if function[-1] in {"nextval", "currval", "setval"}:
                raise ValueError("sequence calls require a sequence dependency description")
            functions.add(".".join(function if len(function) > 1 else ["pg_catalog", *function]))
        if "ColumnRef" in node:
            fields = node["ColumnRef"]["fields"]
            if all("String" in field for field in fields):
                columns.add(".".join(names(fields)))
    return {"kind": kind, "tables": sorted(tables), "sequences": [],
            "functions": sorted(functions), "columns": sorted(columns)}


if __name__ == "__main__":
    json.dump(describe(sys.stdin.read()), sys.stdout)
