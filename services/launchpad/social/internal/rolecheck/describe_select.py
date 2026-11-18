"""Describe the named dependencies of a PostgreSQL SELECT statement."""

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
    if len(statements) != 1 or set(statements[0]["stmt"]) != {"SelectStmt"}:
        raise ValueError("expected one SELECT statement")
    tree = statements[0]["stmt"]
    ctes = {node["CommonTableExpr"]["ctename"] for node in nodes(tree)
            if "CommonTableExpr" in node}
    tables, functions, columns = set(), set(), set()
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
    return {"kind": "SELECT", "tables": sorted(tables), "sequences": [],
            "functions": sorted(functions), "columns": sorted(columns)}


if __name__ == "__main__":
    json.dump(describe(sys.stdin.read()), sys.stdout)
