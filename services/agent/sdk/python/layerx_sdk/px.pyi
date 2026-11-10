from collections.abc import Mapping
from typing import Literal

from .agent_http import LayerXKeyCredential

PxMethod = Literal[
    "px_resolveAccount", "px_getAccount", "px_getBalances", "px_listAssets", "px_getNetwork",
    "px_getReceiptKeys", "px_getHistory", "px_getUnifiedHistory",
    "px_getInferenceRooms",
    "px_getInferenceRoom",
    "px_getInferenceDeal",
    "px_getInferencePeer",
    "px_getInferenceReceipts",
    "px_register",
    "px_getCommit",
    "px_getValidators",
    "px_getSequenceBatch",
    "px_requestFunds",
    "px_listHolderRecords",
    "px_getCapabilities",
    "px_getRouteCatalogue",
]

HISTORY_DEFAULT_LIMIT: int
HISTORY_MAXIMUM_LIMIT: int
HISTORY_MAXIMUM_ACCOUNTS: int

PX_MAXIMUM_JOINED_ASSETS: int
PX_METHODS: tuple[PxMethod, ...]

class PxResolvedIdentities:
    evm_address: str | None
    pax_address: str | None
    layerx_did: str | None
    layerx_account: str | None
    bound: bool
    def __init__(self, evm_address: str | None, pax_address: str | None, layerx_did: str | None, layerx_account: str | None, bound: bool) -> None: ...

class PxPaxeerAccount:
    address: str
    balance: int
    nonce: int
    def __init__(self, address: str, balance: int, nonce: int) -> None: ...

class PxAccountJoin:
    account: PxResolvedIdentities
    paxeer: PxPaxeerAccount | None
    layerx: Mapping[str, object] | None
    def __init__(self, account: PxResolvedIdentities, paxeer: PxPaxeerAccount | None, layerx: Mapping[str, object] | None) -> None: ...

class PxCustodyAsset:
    asset_id: str
    denom: str
    pointer: str
    enabled: bool
    paused: bool
    minimum_deposit: int
    custody_cap: int
    custodied: int
    released: int
    pending: int
    def __init__(self, asset_id: str, denom: str, pointer: str, enabled: bool, paused: bool, minimum_deposit: int, custody_cap: int, custodied: int, released: int, pending: int) -> None: ...

class PxPaxeerAssetBalance:
    denom: str
    amount: int
    def __init__(self, denom: str, amount: int) -> None: ...

class PxAssetBalance:
    asset_id: str
    denom: str | None
    custody: PxCustodyAsset | None
    paxeer: PxPaxeerAssetBalance | None
    layerx: Mapping[str, object] | None
    def __init__(self, asset_id: str, denom: str | None, custody: PxCustodyAsset | None, paxeer: PxPaxeerAssetBalance | None, layerx: Mapping[str, object] | None) -> None: ...

class PxAccountBalances:
    account: PxResolvedIdentities
    balances: tuple[PxAssetBalance, ...]
    joined_limit: int
    def __init__(self, account: PxResolvedIdentities, balances: tuple[PxAssetBalance, ...], joined_limit: int) -> None: ...

class PxAssetEntry:
    asset_id: str
    layerx: Mapping[str, object] | None
    paxeer: PxCustodyAsset | None
    def __init__(self, asset_id: str, layerx: Mapping[str, object] | None, paxeer: PxCustodyAsset | None) -> None: ...

class PxAssetTable:
    assets: tuple[PxAssetEntry, ...]
    joined_limit: int
    def __init__(self, assets: tuple[PxAssetEntry, ...], joined_limit: int) -> None: ...

class PxPaxeerHead:
    chain_id: int
    latest_block: int
    def __init__(self, chain_id: int, latest_block: int) -> None: ...

class PxNetworkHead:
    network_id: str
    paxeer: PxPaxeerHead
    layerx: Mapping[str, object] | None
    latest_committed_height: int
    def __init__(self, network_id: str, paxeer: PxPaxeerHead, layerx: Mapping[str, object] | None, latest_committed_height: int) -> None: ...

class HistoryAssetMetadata:
    asset: str
    chain: str
    kind: str
    address: str | None
    denom: str | None
    symbol: str | None
    decimals: int | None
    native_id: str | None
    pointer: str | None
    metadata: object
    def __init__(self, asset: str, chain: str, kind: str, address: str | None, denom: str | None, symbol: str | None, decimals: int | None, native_id: str | None, pointer: str | None, metadata: object) -> None: ...

class HistoryRow:
    id: int
    height_or_seq: int
    chain: str
    kind: str
    direction: str
    account: str
    counterparty: str | None
    asset: str
    amount: int
    tx_id: str
    tx_hash: str
    ordinal: int
    final: bool
    decoded: object
    asset_metadata: HistoryAssetMetadata | None
    block_height: int
    tx_index: int
    log_index: int
    side: str | None
    def __init__(self, id: int, height_or_seq: int, chain: str, kind: str, direction: str, account: str, counterparty: str | None, asset: str, amount: int, tx_id: str, tx_hash: str, ordinal: int, final: bool, decoded: object, asset_metadata: HistoryAssetMetadata | None, block_height: int, tx_index: int, log_index: int, side: str | None) -> None: ...

class HistoryPage:
    account: str
    items: tuple[HistoryRow, ...]
    next_cursor: str | None
    def __init__(self, account: str, items: tuple[HistoryRow, ...], next_cursor: str | None) -> None: ...

class HistorySide:
    side: str
    account: str
    def __init__(self, side: str, account: str) -> None: ...

class UnifiedHistoryPage:
    account: Mapping[str, object]
    accounts: tuple[HistorySide, ...]
    items: tuple[HistoryRow, ...]
    next_cursor: str | None
    def __init__(self, account: Mapping[str, object], accounts: tuple[HistorySide, ...], items: tuple[HistoryRow, ...], next_cursor: str | None) -> None: ...

class PxRpcError(Exception):
    code: int
    message: str
    data: object
    def __init__(self, code: int, message: str, data: object = ...) -> None: ...

class InferenceRoomProfile:
    minimum_bond: int
    canary_ratio_bps: int
    lap_size_tokens: int
    challenge_window_blocks: int
    slash_fraction_bps: int
    host_share_bps: int
    protocol_share_bps: int
    settlement_account: str
    def __init__(self, minimum_bond: int, canary_ratio_bps: int, lap_size_tokens: int, challenge_window_blocks: int, slash_fraction_bps: int, host_share_bps: int, protocol_share_bps: int, settlement_account: str) -> None: ...

class InferenceAskStreamRecord:
    payer: str
    stream_account: str
    recipient: str
    asset_id: str
    mode: int
    rate: int
    rate_unit: int
    start_ms: int
    last_accrual_ms: int
    end_ms: int
    total_cap: int
    accrued_total: int
    settled_total: int
    remainder_carry: int
    cumulative_meter: int
    meter_authorities: tuple[str, ...]
    underfunded: bool
    paused: bool
    closed: bool
    def __init__(self, payer: str, stream_account: str, recipient: str, asset_id: str, mode: int, rate: int, rate_unit: int, start_ms: int, last_accrual_ms: int, end_ms: int, total_cap: int, accrued_total: int, settled_total: int, remainder_carry: int, cumulative_meter: int, meter_authorities: tuple[str, ...], underfunded: bool, paused: bool, closed: bool) -> None: ...

class InferenceAskStream:
    ask_id: str
    stream_id: str
    stream: InferenceAskStreamRecord | None
    def __init__(self, ask_id: str, stream_id: str, stream: InferenceAskStreamRecord | None) -> None: ...

class InferenceRoom:
    room_id: str
    host_key: str
    model_digest: str
    runtime_digest: str
    profile: InferenceRoomProfile
    state: Literal[0, 1]
    opened_at_epoch: int
    open_asks: tuple[str, ...]
    open_bids: tuple[str, ...]
    ask_streams: tuple[InferenceAskStream, ...]
    def __init__(self, room_id: str, host_key: str, model_digest: str, runtime_digest: str, profile: InferenceRoomProfile, state: Literal[0, 1], opened_at_epoch: int, open_asks: tuple[str, ...], open_bids: tuple[str, ...], ask_streams: tuple[InferenceAskStream, ...]) -> None: ...

class InferenceLayerSplit:
    bid_id: str
    first_layer: int
    end_layer: int
    def __init__(self, bid_id: str, first_layer: int, end_layer: int) -> None: ...

class InferenceDeal:
    room_id: str
    deal_id: str
    ask_id: str
    agreed_price_per_thousand_tokens: int
    roster_digest: str
    layers: tuple[InferenceLayerSplit, ...]
    state: Literal[0, 1]
    settled_tokens: int
    settled_amount_owed: int
    def __init__(self, room_id: str, deal_id: str, ask_id: str, agreed_price_per_thousand_tokens: int, roster_digest: str, layers: tuple[InferenceLayerSplit, ...], state: Literal[0, 1], settled_tokens: int, settled_amount_owed: int) -> None: ...

class InferencePeerCapabilities:
    accelerator_memory_mib: int
    system_memory_mib: int
    bandwidth_class: int
    region_code: str
    attestation_family: int
    runtime_kind: int
    def __init__(self, accelerator_memory_mib: int, system_memory_mib: int, bandwidth_class: int, region_code: str, attestation_family: int, runtime_kind: int) -> None: ...

class InferenceAttestationBinding:
    family: int
    model_digest: str
    runtime_digest: str
    block_height: int
    def __init__(self, family: int, model_digest: str, runtime_digest: str, block_height: int) -> None: ...

class InferencePeer:
    peer_key: str
    worker: str
    delegate: str
    metadata: str
    generation: int
    key_version: int
    metadata_revision: int
    valid_from: int
    expiry: int
    state: Literal[1, 2, 3, 4, 5, 6]
    enrolled: bool
    capabilities_status: Literal[1, 4]
    capabilities: InferencePeerCapabilities | None
    bond_status: Literal[2]
    fault_count: int
    attestation_binding: InferenceAttestationBinding
    def __init__(self, peer_key: str, worker: str, delegate: str, metadata: str, generation: int, key_version: int, metadata_revision: int, valid_from: int, expiry: int, state: Literal[1, 2, 3, 4, 5, 6], enrolled: bool, capabilities_status: Literal[1, 4], capabilities: InferencePeerCapabilities | None, bond_status: Literal[2], fault_count: int, attestation_binding: InferenceAttestationBinding) -> None: ...

class InferenceReceiptBatch:
    deal_id: str
    first_lap_index: int
    last_lap_index: int
    cumulative_tokens: int
    batch_digest: str
    countersign_quorum: int
    def __init__(self, deal_id: str, first_lap_index: int, last_lap_index: int, cumulative_tokens: int, batch_digest: str, countersign_quorum: int) -> None: ...

class PxReceiptKeyEntry:
    key_id: str
    public_key: str
    first_height: int
    last_height: int
    def __init__(self, key_id: str, public_key: str, first_height: int, last_height: int) -> None: ...

class PxClient:
    endpoint: str
    def __init__(self, endpoint: str, *, credential: LayerXKeyCredential | None = ..., timeout: float = ..., maximum_response_bytes: int = ...) -> None: ...
    def resolve_account(self, account: str) -> PxResolvedIdentities: ...
    def get_account(self, account: str) -> PxAccountJoin: ...
    def get_balances(self, account: str) -> PxAccountBalances: ...
    def list_assets(self) -> PxAssetTable: ...
    def get_network(self) -> PxNetworkHead: ...
    def get_receipt_keys(self) -> tuple[PxReceiptKeyEntry, ...]: ...
    def get_inference_rooms(self) -> tuple[InferenceRoom, ...]: ...
    def get_inference_room(self, room_id: str) -> InferenceRoom: ...
    def get_inference_deal(self, deal_id: str) -> InferenceDeal: ...
    def get_inference_peer(self, peer_key: str) -> InferencePeer: ...
    def get_inference_receipts(self, deal_id: str) -> tuple[InferenceReceiptBatch, ...]: ...
    def get_history(self, account: str, *, cursor: str | None = ..., limit: int | None = ..., kind: str | None = ...) -> HistoryPage: ...
    def get_unified_history(self, account: str, *, cursor: str | None = ..., limit: int | None = ..., kind: str | None = ...) -> UnifiedHistoryPage: ...
    def call(self, method: str, params: list[object]) -> Mapping[str, object]: ...

def px_account_key(account: str) -> str: ...
def px_quantity(value: object) -> int: ...
def px_optional_quantity(value: object) -> int | None: ...
def decode_px_resolved_identities(value: object) -> PxResolvedIdentities: ...
def decode_px_account_join(value: object) -> PxAccountJoin: ...
def decode_px_account_balances(value: object) -> PxAccountBalances: ...
def decode_px_asset_balance(value: object) -> PxAssetBalance: ...
def decode_px_custody_asset(value: object) -> PxCustodyAsset | None: ...
def decode_px_asset_table(value: object) -> PxAssetTable: ...
def decode_px_network_head(value: object) -> PxNetworkHead: ...
def history_cursor(block_height: int, tx_index: int, log_index: int) -> str: ...
def history_params(account: str, *, cursor: str | None = ..., limit: int | None = ..., kind: str | None = ...) -> list[object]: ...
def history_account(account: str) -> str: ...
def decode_paxeer_history_page(value: object, limit: int = ...) -> HistoryPage: ...
def decode_unified_history_page(value: object, limit: int = ...) -> UnifiedHistoryPage: ...
def decode_history_row(value: object, unified: bool = ...) -> HistoryRow: ...
def decode_history_asset_metadata(value: object, asset: str | None = ...) -> HistoryAssetMetadata | None: ...

def decode_px_receipt_keys(value: object) -> tuple[PxReceiptKeyEntry, ...]: ...
def decode_inference_room(value: object) -> InferenceRoom: ...
def decode_inference_deal(value: object) -> InferenceDeal: ...
def decode_inference_peer(value: object) -> InferencePeer: ...
def decode_inference_receipt(value: object) -> InferenceReceiptBatch: ...
