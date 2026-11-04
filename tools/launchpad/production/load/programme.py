"""Read the root programme with its owning KVX parser, preserving task IDs."""
import importlib.util
from pathlib import Path

ROOT = Path(__file__).resolve().parents[4]


def load_programme(path):
    path = Path(path)
    if path.is_symlink() or not path.is_file() or path.stat().st_size > 64 * 1024 * 1024:
        raise ValueError("bounded current programme required")
    raw = path.read_bytes()
    parser_spec = importlib.util.spec_from_file_location(
        "load_programme_kvx", ROOT / "spec/layerx-merge/programme/kvx.py"
    )
    parser = importlib.util.module_from_spec(parser_spec)
    parser_spec.loader.exec_module(parser)
    sections = parser.load(path)
    if path.read_bytes() != raw:
        raise ValueError("programme changed while reading admission")
    programme = {"meta": sections.get("meta", {}), "task": {}}
    for section, values in sections.items():
        if section.startswith("task."):
            node = programme["task"]
            for part in section.removeprefix("task.").split("."):
                node = node.setdefault(part, {})
            node.update(values)
    return programme, raw
