#!/usr/bin/env python3
"""Render required deployment variables into a TOML template on stdout."""

import json
import os
from pathlib import Path
import re
import sys
import tomllib


VARIABLE = re.compile(r"\$\{([A-Z][A-Z0-9_]*)\}")
HOST = re.compile(r"[a-zA-Z0-9](?:[a-zA-Z0-9.-]*[a-zA-Z0-9])?")


def render(template, environment):
    """Substitute TOML string values, refusing absent or empty deploy inputs."""
    # Substitute only assignments, never the explanatory header comments.
    lines = []
    for line in template.splitlines(keepends=True):
        if line.lstrip().startswith("#"):
            lines.append(line)
            continue

        def substitute(match):
            name = match[1]
            value = environment.get(name)
            if not value or not value.strip():
                raise ValueError(f"required deployment variable is unset or empty: {name}")
            if name.endswith("_HOST") and not HOST.fullmatch(value):
                raise ValueError(f"deployment variable must be a DNS host: {name}")
            # Templates place variables inside quoted TOML strings.
            return json.dumps(value, ensure_ascii=False)[1:-1]

        lines.append(VARIABLE.sub(substitute, line))
    rendered = "".join(lines)
    tomllib.loads(rendered)
    return rendered


def main():
    if len(sys.argv) != 2:
        raise ValueError("usage: render_deploy_template.py <template.toml>")
    print(render(Path(sys.argv[1]).read_text(encoding="utf-8"), os.environ), end="")


if __name__ == "__main__":
    try:
        main()
    except (OSError, ValueError) as error:
        print(str(error), file=sys.stderr)
        sys.exit(1)
