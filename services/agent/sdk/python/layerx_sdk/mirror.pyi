from dataclasses import dataclass
from collections.abc import Callable
from pathlib import Path
from typing import Literal

@dataclass(frozen=True)
class MirrorCandidate:
    source: int
    commitment: bytes
@dataclass(frozen=True)
class MirrorPolicy:
    kind: Literal["exact", "ordered-preference", "agreement"]
    candidates: tuple[MirrorCandidate, ...]
    minimum: int = ...
@dataclass(frozen=True)
class MirrorVerification:
    level: str
    committed_height: int
    committed_block_hash: bytes
    evidence_digest: bytes | None
    source_id: str
    target: str
    canonical_position: str
    provenance: str
    latest_batch: int | None
    batch_lag: str
    failover_count: int
    agreeing_sources: int
class MirrorVerificationError(RuntimeError):
    code: object
class MirrorVerifier:
    def __init__(self, executable: Path, configuration: Path, timeout_seconds: float = ..., *, exchange: Callable[[bytes], tuple[int, bytes]] | None = ...) -> None: ...
    def receipt(self, batch_number: int, policy: MirrorPolicy, index: int) -> MirrorVerification: ...
    def committed(self, batch_number: int, policy: MirrorPolicy) -> MirrorVerification: ...
