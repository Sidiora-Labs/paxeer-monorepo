import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[5]
sys.path.insert(0, str(ROOT / "services/agent/sdk/python"))
