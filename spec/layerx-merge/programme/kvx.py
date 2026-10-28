from __future__ import annotations

import re
from os import PathLike
from pathlib import Path

KvxValue = str | int | bool | list[str]
_STRING = re.compile(r'"(?:\\"|[^"])*"(?=\s*(?:[,\]#]|$))')


class KvxError(ValueError):
    pass


def _without_comment(line: str) -> str:
    index = 0
    while index < len(line):
        character = line[index]
        if character == '"':
            match = _STRING.match(line, index)
            if match is None:
                return line.strip()
            index = match.end()
            continue
        if character == "#":
            return line[:index].strip()
        index += 1
    return line.strip()


def _value(text: str, location: str) -> KvxValue:
    if text == "true":
        return True
    if text == "false":
        return False
    if re.fullmatch(r"[+-]?[0-9]+", text):
        return int(text)
    if _STRING.fullmatch(text):
        return text[1:-1].replace(r'\"', '"')
    if text.startswith("[") and text.endswith("]"):
        inner = text[1:-1].strip()
        values: list[str] = []
        while inner:
            match = _STRING.match(inner)
            if match is None:
                break
            values.append(match.group()[1:-1].replace(r'\"', '"'))
            inner = inner[match.end() :].strip()
            if not inner:
                return values
            if not inner.startswith(","):
                break
            inner = inner[1:].strip()
            if not inner:
                break
        else:
            return values
    raise KvxError(
        f"{location}: expected a quoted string, integer, boolean or string list value"
    )


def load(path: str | PathLike[str]) -> dict[str, dict[str, KvxValue]]:
    sections: dict[str, dict[str, KvxValue]] = {}
    current = None
    with Path(path).open(encoding="utf-8") as source:
        for number, raw in enumerate(source, start=1):
            location = f"{path}:{number}"
            if "\t" in raw:
                raise KvxError(f"{location}: tab is not allowed")
            line = _without_comment(raw)
            if not line:
                continue
            header = re.fullmatch(r"\[([A-Za-z0-9_.-]+)\]", line)
            if header:
                name = header.group(1)
                if name in sections:
                    raise KvxError(f"{location}: duplicate section [{name}]")
                current = sections[name] = {}
                continue
            if current is None:
                raise KvxError(f"{location}: expected a section header")
            key, separator, text = line.partition("=")
            key = key.strip()
            if not separator or not re.fullmatch(r"[A-Za-z0-9_.-]+", key):
                raise KvxError(f"{location}: expected key = value")
            if key in current:
                raise KvxError(f"{location}: duplicate key {key}")
            current[key] = _value(text.strip(), location)
    return sections


def check_format(path: str | PathLike[str]) -> dict[str, dict[str, KvxValue]]:
    sections = load(path)
    version = sections.get("meta", {}).get("format_version")
    if type(version) is not int or version != 1:
        raise KvxError(f"{path}: [meta] format_version must be integer 1")
    return sections
