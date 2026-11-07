#!/usr/bin/env python3
"""Separate Rust test items for the explorer test-ratio gate without compiling."""

import argparse
from pathlib import Path
import re
import sys
from typing import NamedTuple


class Token(NamedTuple):
    text: str
    start: int
    end: int
    comment: bool = False


RAW_STRING = re.compile(r'(?:br|cr|r)(#*)"')
CHARACTER = re.compile(r"(?:b)?'(?:\\(?:u\{[0-9a-fA-F_]+\}|x[0-9a-fA-F]{2}|.)|[^'\\\n])'")
WORD = re.compile(r"(?:r#)?[A-Za-z_][A-Za-z_0-9]*|[0-9][A-Za-z_0-9]*")


def rust_tokens(source):
    """Keep offsets while treating comments and literals as indivisible tokens."""
    tokens = []
    pos = 0
    while pos < len(source):
        start = pos
        comment = False
        if source[pos].isspace():
            pos += 1
            continue
        if source.startswith("//", pos):
            end = source.find("\n", pos)
            pos = len(source) if end < 0 else end
            comment = True
        elif source.startswith("/*", pos):
            pos += 2
            depth = 1
            while depth and pos < len(source):
                if source.startswith("/*", pos):
                    depth += 1
                    pos += 2
                elif source.startswith("*/", pos):
                    depth -= 1
                    pos += 2
                else:
                    pos += 1
            if depth:
                raise ValueError("unterminated Rust block comment")
            comment = True
        elif match := RAW_STRING.match(source, pos):
            terminator = '"' + match.group(1)
            end = source.find(terminator, match.end())
            if end < 0:
                raise ValueError("unterminated Rust raw string")
            pos = end + len(terminator)
        elif source[pos] == '"' or source.startswith(('b"', 'c"'), pos):
            pos += 1 if source[pos] == '"' else 2
            while pos < len(source) and source[pos] != '"':
                pos += 2 if source[pos] == "\\" else 1
            if pos >= len(source):
                raise ValueError("unterminated Rust string")
            pos += 1
        elif match := CHARACTER.match(source, pos):
            pos = match.end()
        elif match := WORD.match(source, pos):
            pos = match.end()
        else:
            pos += 1
        tokens.append(Token(source[start:pos], start, pos, comment))
    return tokens


def delimiter_pairs(tokens):
    stack = []
    pairs = {}
    closers = {"}": "{", "]": "[", ")": "("}
    for index, token in enumerate(tokens):
        if token.text in ("{", "[", "("):
            stack.append(index)
        elif token.text in closers:
            if not stack or tokens[stack[-1]].text != closers[token.text]:
                raise ValueError("unbalanced Rust delimiters")
            opening = stack.pop()
            pairs[opening] = index
    if stack:
        raise ValueError("unclosed Rust delimiter")
    return pairs


def cfg_without_test(words):
    """Possible cfg values in a production build; unknown predicates stay possible."""
    if words == ["test"]:
        return {False}
    if len(words) < 3 or words[1] != "(" or words[-1] != ")":
        return {False, True}
    operator = words[0]
    if operator not in ("all", "any", "not"):
        return {False, True}
    parts = []
    start = 2
    depth = 0
    for index in range(2, len(words) - 1):
        if words[index] == "(":
            depth += 1
        elif words[index] == ")":
            depth -= 1
        elif words[index] == "," and depth == 0:
            parts.append(words[start:index])
            start = index + 1
    if start < len(words) - 1:
        parts.append(words[start:-1])
    values = [cfg_without_test(part) for part in parts]
    if operator == "not":
        if len(values) != 1:
            raise ValueError("cfg(not(...)) requires one predicate")
        return {not value for value in values[0]}
    result = {operator == "all"}
    for choices in values:
        result = {
            (left and right) if operator == "all" else (left or right)
            for left in result for right in choices
        }
    return result


def test_attribute(words):
    if words[:2] == ["cfg", "("] and words[-1:] == [")"]:
        return "test" in words[2:-1] and cfg_without_test(words[2:-1]) == {False}
    # #[test] and qualified harness attributes such as #[tokio::test].
    name = words[:words.index("(")] if "(" in words else words
    return name == ["test"] or (len(name) >= 4 and name[-3:] == [":", ":", "test"])


def test_item_ranges(source):
    tokens = [token for token in rust_tokens(source) if not token.comment]
    pairs = delimiter_pairs(tokens)
    openings = {closing: opening for opening, closing in pairs.items()}
    ranges = []
    enclosing = []
    index = 0
    while index < len(tokens):
        while enclosing and enclosing[-1][1] < index:
            enclosing.pop()
        token = tokens[index]
        if token.text == "{":
            enclosing.append((index, pairs[index]))
        inner = index + 1 < len(tokens) and tokens[index + 1].text == "!"
        bracket = index + (2 if inner else 1)
        if token.text != "#" or bracket >= len(tokens) or tokens[bracket].text != "[":
            index += 1
            continue
        end = pairs[bracket]
        words = [word.text for word in tokens[bracket + 1:end]]
        if inner:
            if test_attribute(words):
                scope_start = 0
                scope_end = len(source)
                if enclosing:
                    opening, closing = enclosing[-1]
                    cursor = opening - 1
                    while cursor >= 0 and tokens[cursor].text not in (";", "{", "}"):
                        cursor = openings.get(cursor, cursor) - 1
                    scope_start = tokens[cursor + 1].start
                    scope_end = tokens[closing].end
                ranges.append((scope_start, scope_end))
            index = end + 1
            continue
        # All outer attributes in the group belong to the same Rust item.
        test_only = test_attribute(words)
        cursor = end + 1
        while cursor + 1 < len(tokens) and tokens[cursor].text == "#" and tokens[cursor + 1].text == "[":
            attribute_end = pairs[cursor + 1]
            test_only |= test_attribute([word.text for word in tokens[cursor + 2:attribute_end]])
            cursor = attribute_end + 1
        if not test_only:
            index = cursor
            continue
        while cursor < len(tokens):
            value = tokens[cursor].text
            if value in (";", ","):
                ranges.append((token.start, tokens[cursor].end))
                break
            if value == "{":
                ranges.append((token.start, tokens[pairs[cursor]].end))
                break
            if value in ("(", "["):
                cursor = pairs[cursor]
            cursor += 1
        else:
            raise ValueError("test attribute has no complete Rust item")
        index = end + 1
    return ranges


def split_rust(source):
    ranges = test_item_ranges(source)
    is_test = bytearray(len(source))
    for start, end in ranges:
        is_test[start:end] = b"\x01" * (end - start)
    production = "".join(
        char for index, char in enumerate(source)
        if not is_test[index] or char == "\n"
    )
    tests = "".join(
        char for index, char in enumerate(source)
        if is_test[index] or char == "\n"
    )
    return production, tests


def classify_changes(before, after):
    old_parts = split_rust(before)
    new_parts = split_rust(after)
    changed = []
    for role, old, new in zip(("source", "test"), old_parts, new_parts):
        # Compare item tokens, so moving an unchanged inline module does not
        # turn it into a changed test. Comments remain tokens, not attributes.
        old_tokens = [token.text for token in rust_tokens(old)]
        new_items = rust_tokens(new)
        new_tokens = [token.text for token in new_items]
        if any(not token.comment for token in new_items) and old_tokens != new_tokens:
            changed.append(role)
    kind = "mixed" if len(changed) == 2 else changed[0] if changed else "neither"
    return kind, new_parts[0], new_parts[1]


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("base", type=Path)
    parser.add_argument("head", type=Path)
    parser.add_argument("output", type=Path)
    args = parser.parse_args(argv)
    try:
        kind, production, tests = classify_changes(
            args.base.read_text(encoding="utf-8"), args.head.read_text(encoding="utf-8")
        )
        args.output.mkdir(parents=True, exist_ok=True)
        (args.output / "source").write_text(production, encoding="utf-8")
        (args.output / "test").write_text(tests, encoding="utf-8")
    except (OSError, ValueError) as error:
        parser.exit(2, f"rust-test-ranges: {error}\n")
    print(kind)
    return 0


if __name__ == "__main__":
    sys.exit(main())
