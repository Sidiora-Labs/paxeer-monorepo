"""Strict replay of the recorded SSH/DNS peer; never emulate bringup code.

Only listed requests are accepted. The edge snapshot is supplied by the real
edge.sh producer in this run, preserving the manifest/site transport boundary.
"""
import ast
import json
import os
import re
import shlex
import sys
from pathlib import Path


def shell_words(command):
    # Bash printf %q uses ANSI-C quotes for multiline remote commands.
    # Decode that quoting before POSIX tokenization; never execute the text.
    command = re.sub(
        r"\$'((?:[^'\\]|\\.)*)'",
        lambda match: shlex.quote(ast.literal_eval("'" + match[1] + "'")),
        command,
    )
    return shlex.split(command)


def main():
    state = json.loads(Path(os.environ["BRINGUP_TRANSCRIPT_STATE"]).read_text())
    transcript = json.loads(Path(__file__).with_name("session.json").read_text())
    tool, *arguments = sys.argv[1:]

    def expand(value):
        for key, replacement in state["bindings"].items():
            value = value.replace("{" + key + "}", replacement)
        return value

    matched = None
    response = ""
    for entry in transcript["exchanges"]:
        if entry["tool"] != tool:
            continue
        expected = [expand(value) for value in entry["argv"]]
        actual = arguments
        if tool == "ssh":
            if len(actual) != len(expected) or actual[:-1] != expected[:-1]:
                continue
            if shell_words(actual[-1]) != shell_words(expected[-1]):
                continue
        elif actual != expected:
            continue
        matched = entry["id"]
        response = expand(entry.get("stdout", ""))
        if "stdout_file" in entry:
            response = Path(expand(entry["stdout_file"])).read_text()
        response = state.get("responses", {}).get(matched, response)
        code = entry.get("exit", 0)
        break
    with Path(state["log"]).open("a") as log:
        log.write(json.dumps({"tool": tool, "argv": arguments, "matched": matched}) + "\n")
    if matched is None:
        sys.exit("unrecorded peer request: " + repr((tool, arguments)))
    sys.stdout.write(response)
    sys.exit(code)


if __name__ == "__main__":
    main()
