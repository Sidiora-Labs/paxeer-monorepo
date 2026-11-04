"""Regenerate route inventory; browser and manual AT evidence stays pending."""

import argparse
from copy import deepcopy
import json
from pathlib import Path
import sys


ROOT = Path(__file__).resolve().parents[4]
SOURCE = "apps/launchpad/web/src/app/**/page.tsx"
CORPUS = Path(__file__).with_name("corpus.json")
PENDING = "pending-browser-and-manual-AT"
CHECKS = (
    "semantic-landmarks-skip-link-names-headings",
    "keyboard-focus-order-visible-200-percent",
    "dialog-trap-escape-restore-trigger",
    "async-error-announcement",
    "financial-column-headers-units-sign-text",
    "coalesced-live-region-chart-table-alternative",
)
DEDICATED_ROUTES = {
    "portfolio": "/portfolio",
    "watchlists": "/watchlist",
    "notifications": "/notifications",
}


def generate(root, corpus):
    app = root / SOURCE.split("/**/", 1)[0]
    if not app.is_dir():
        raise ValueError(f"Missing route tree: {app}")
    routes = sorted(
        "/" + "/".join(page.parent.relative_to(app).parts)
        for page in app.rglob("page.tsx")
    )
    if not routes:
        raise ValueError("Route tree has no pages")
    if corpus["observed"] is not False:
        raise ValueError("Inventory regeneration cannot replace observed evidence")
    existing = {row["route"]: row for row in corpus["routes"]}
    if len(existing) != len(corpus["routes"]):
        raise ValueError("Duplicate inventory route")
    if any(row["status"] != PENDING for row in existing.values()):
        raise ValueError("Inventory regeneration cannot replace route evidence")
    for journey in corpus["journeys"].values():
        if not journey or not set(journey) <= set(routes):
            raise ValueError("Journey references missing page routes")
    result = deepcopy(corpus)
    result["source"] = SOURCE
    result["routes"] = [
        deepcopy(existing[route])
        if route in existing
        else {
            "route": route,
            "checks": list(CHECKS),
            "account": "synthetic-only",
            "status": PENDING,
        }
        for route in routes
    ]
    result["missingDedicatedRoutes"] = [
        name for name, route in DEDICATED_ROUTES.items() if route not in routes
    ]
    return result


def refresh(root=ROOT, target=CORPUS, *, check=False):
    original = target.read_text(encoding="utf-8")
    content = json.dumps(generate(root, json.loads(original)), indent=2) + "\n"
    if check:
        if content != original:
            raise ValueError("Accessibility route inventory is stale")
    else:
        target.write_text(content, encoding="utf-8")


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--check", action="store_true")
    args = parser.parse_args(argv)
    try:
        refresh(check=args.check)
    except (OSError, ValueError, KeyError) as error:
        print(f"accessibility inventory: {error}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
