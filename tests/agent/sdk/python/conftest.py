import sys
from pathlib import Path


SDK_ROOT = Path(__file__).resolve().parents[4] / "services/agent/sdk/python"
sys.path.insert(0, str(SDK_ROOT))
