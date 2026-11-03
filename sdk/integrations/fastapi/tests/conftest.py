import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[4]

sys.path[:0] = [
    str(ROOT / "sdk/integrations/fastapi"),
    str(ROOT / "services/agent/sdk/python"),
]
