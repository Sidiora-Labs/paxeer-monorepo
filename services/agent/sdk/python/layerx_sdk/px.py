"""Unified-network read helpers over one network-gateway JSON-RPC endpoint."""

from __future__ import annotations

import json
import re
import urllib.parse
import urllib.request
from collections.abc import Mapping, Sequence
from dataclasses import dataclass
from types import MappingProxyType
from typing import Literal, cast

from .agent_http import LayerXKeyCredential

_MAX_RESPONSE_BYTES = 9 * 1024 * 1024
_MAX_REQUEST_BYTES = 1024 * 1024
_MAX_U64 = 18446744073709551615
_EVM_ADDRESS = re.compile(r"^0x[0-9a-f]{40}$")
_HEX32 = re.compile(r"^[0-9a-f]{64}$")
_DID = re.compile(r"^did:layerx:[0-9a-f]{64}$")
_DECIMAL = re.compile(r"^(?:0|[1-9][0-9]*)$")
_HEX_QUANTITY = re.compile(r"^0x[0-9a-f]{1,64}$")

_MAX_U256 = (1 << 256) - 1
_ROW_ID = re.compile(r"^(?:explorer:)?[1-9][0-9]*$")
_CURSOR = re.compile(r"^[0-9a-f]{32}$")
_KIND = re.compile(r"^[a-z0-9_]{1,64}$")

PX_MAXIMUM_JOINED_ASSETS = 1024
HISTORY_DEFAULT_LIMIT = 50
HISTORY_MAXIMUM_LIMIT = 100
HISTORY_MAXIMUM_ACCOUNTS = 16

PxMethod = Literal[
    "px_resolveAccount",
    "px_getAccount",
    "px_getBalances",
    "px_listAssets",
    "px_getNetwork",
    "px_getReceiptKeys",
    "px_getHistory",
    "px_getUnifiedHistory",
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
PX_METHODS: tuple[PxMethod, ...] = (
    "px_resolveAccount",
    "px_getAccount",
    "px_getBalances",
    "px_listAssets",
    "px_getNetwork",
    "px_getReceiptKeys",
    "px_getHistory",
    "px_getUnifiedHistory",
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
)


@dataclass(frozen=True)
class PxResolvedIdentities:
    evm_address: str | None
    pax_address: str | None
    layerx_did: str | None
    layerx_account: str | None
    bound: bool


@dataclass(frozen=True)
class PxPaxeerAccount:
    address: str
    balance: int
    nonce: int


@dataclass(frozen=True)
class PxAccountJoin:
    account: PxResolvedIdentities
    paxeer: PxPaxeerAccount | None
    layerx: Mapping[str, object] | None


@dataclass(frozen=True)
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


@dataclass(frozen=True)
class PxPaxeerAssetBalance:
    denom: str
    amount: int


@dataclass(frozen=True)
class PxAssetBalance:
    asset_id: str
    denom: str | None
    custody: PxCustodyAsset | None
    paxeer: PxPaxeerAssetBalance | None
    layerx: Mapping[str, object] | None


@dataclass(frozen=True)
class PxAccountBalances:
    account: PxResolvedIdentities
    balances: tuple[PxAssetBalance, ...]
    joined_limit: int


@dataclass(frozen=True)
class PxAssetEntry:
    asset_id: str
    layerx: Mapping[str, object] | None
    paxeer: PxCustodyAsset | None


@dataclass(frozen=True)
class PxAssetTable:
    assets: tuple[PxAssetEntry, ...]
    joined_limit: int


@dataclass(frozen=True)
class PxPaxeerHead:
    chain_id: int
    latest_block: int


@dataclass(frozen=True)
class PxNetworkHead:
    network_id: str
    paxeer: PxPaxeerHead
    layerx: Mapping[str, object] | None
    # Gateway block-commit observation; callers verify inclusion with CommitTrust.
    latest_committed_height: int


@dataclass(frozen=True)
class PxReceiptKeyEntry:
    key_id: str
    public_key: str
    first_height: int
    last_height: int


@dataclass(frozen=True)
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


@dataclass(frozen=True)
class HistoryRow:
    id: str
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


@dataclass(frozen=True)
class HistoryPage:
    account: str
    items: tuple[HistoryRow, ...]
    next_cursor: str | None


@dataclass(frozen=True)
class HistorySide:
    side: str
    account: str


@dataclass(frozen=True)
class UnifiedHistoryPage:
    account: Mapping[str, object]
    accounts: tuple[HistorySide, ...]
    items: tuple[HistoryRow, ...]
    next_cursor: str | None


class PxRpcError(Exception):
    __slots__ = ("code", "data", "message")

    def __init__(self, code: int, message: str, data: object = None) -> None:
        super().__init__(f"JSON-RPC error {code}")
        self.code = code
        self.message = message
        self.data = data


class _NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        raise ValueError("px-redirect")


@dataclass(frozen=True)
class InferenceRoomProfile:
    minimum_bond: int
    canary_ratio_bps: int
    lap_size_tokens: int
    challenge_window_blocks: int
    slash_fraction_bps: int
    host_share_bps: int
    protocol_share_bps: int
    settlement_account: str


@dataclass(frozen=True)
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


@dataclass(frozen=True)
class InferenceAskStream:
    ask_id: str
    stream_id: str
    stream: InferenceAskStreamRecord | None


@dataclass(frozen=True)
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


@dataclass(frozen=True)
class InferenceLayerSplit:
    bid_id: str
    first_layer: int
    end_layer: int


@dataclass(frozen=True)
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


@dataclass(frozen=True)
class InferencePeerCapabilities:
    accelerator_memory_mib: int
    system_memory_mib: int
    bandwidth_class: int
    region_code: str
    attestation_family: int
    runtime_kind: int


@dataclass(frozen=True)
class InferenceAttestationBinding:
    family: int
    model_digest: str
    runtime_digest: str
    block_height: int


@dataclass(frozen=True)
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


@dataclass(frozen=True)
class InferenceReceiptBatch:
    deal_id: str
    first_lap_index: int
    last_lap_index: int
    cumulative_tokens: int
    batch_digest: str
    countersign_quorum: int


class PxClient:
    def __init__(
        self,
        endpoint: str,
        *,
        credential: LayerXKeyCredential | None = None,
        timeout: float = 30.0,
        maximum_response_bytes: int = _MAX_RESPONSE_BYTES,
    ) -> None:
        url = urllib.parse.urlsplit(endpoint)
        if (
            url.username
            or url.password
            or url.fragment
            or url.query
            or not url.hostname
            or url.scheme != "https"
            and not (
                url.scheme == "http"
                and (
                    url.hostname in ("localhost", "::1")
                    or re.fullmatch(r"127(?:\.[0-9]{1,3}){3}", url.hostname) is not None
                )
            )
        ):
            raise ValueError("invalid-px-endpoint")
        if (
            not isinstance(timeout, (int, float))
            or isinstance(timeout, bool)
            or timeout <= 0
        ):
            raise ValueError("invalid-px-timeout")
        if (
            type(maximum_response_bytes) is not int
            or not 0 < maximum_response_bytes <= _MAX_RESPONSE_BYTES
        ):
            raise ValueError("invalid-px-response-bound")
        self.endpoint = endpoint
        self._credential = credential
        self._timeout = float(timeout)
        self._maximum_response_bytes = maximum_response_bytes
        self._id = 0
        self._opener = urllib.request.build_opener(_NoRedirect())

    def resolve_account(self, account: str) -> PxResolvedIdentities:
        return decode_px_resolved_identities(
            self.call("px_resolveAccount", [px_account_key(account)])
        )

    def get_account(self, account: str) -> PxAccountJoin:
        return decode_px_account_join(
            self.call("px_getAccount", [px_account_key(account)])
        )

    def get_balances(self, account: str) -> PxAccountBalances:
        return decode_px_account_balances(
            self.call("px_getBalances", [px_account_key(account)])
        )

    def list_assets(self) -> PxAssetTable:
        return decode_px_asset_table(self.call("px_listAssets", []))

    def get_network(self) -> PxNetworkHead:
        return decode_px_network_head(self.call("px_getNetwork", []))

    def get_receipt_keys(self) -> tuple[PxReceiptKeyEntry, ...]:
        return decode_px_receipt_keys(self.call("px_getReceiptKeys", []))

    def get_history(
        self,
        account: str,
        *,
        cursor: str | None = None,
        limit: int | None = None,
        kind: str | None = None,
    ) -> HistoryPage:
        params = history_params(
            history_account(account), cursor=cursor, limit=limit, kind=kind
        )
        return decode_paxeer_history_page(
            self.call("px_getHistory", params), limit or HISTORY_DEFAULT_LIMIT
        )

    def get_unified_history(
        self,
        account: str,
        *,
        cursor: str | None = None,
        limit: int | None = None,
        kind: str | None = None,
    ) -> UnifiedHistoryPage:
        params = history_params(
            history_account(account), cursor=cursor, limit=limit, kind=kind
        )
        return decode_unified_history_page(
            self.call("px_getUnifiedHistory", params), limit or HISTORY_DEFAULT_LIMIT
        )

    def get_inference_rooms(self) -> tuple[InferenceRoom, ...]:
        rooms = tuple(decode_inference_room(row) for row in _inference_rows(
            self._call_result("px_getInferenceRooms", []), 16
        ))
        if any(left.room_id >= right.room_id for left, right in zip(rooms, rooms[1:])):
            raise ValueError("invalid-inference-room-ordering")
        return rooms

    def get_inference_room(self, room_id: str) -> InferenceRoom:
        identifier = _inference_id(room_id)
        room = decode_inference_room(self.call("px_getInferenceRoom", [identifier]))
        if room.room_id != identifier:
            raise ValueError("invalid-inference-room-binding")
        return room

    def get_inference_deal(self, deal_id: str) -> InferenceDeal:
        identifier = _inference_id(deal_id)
        deal = decode_inference_deal(self.call("px_getInferenceDeal", [identifier]))
        if deal.deal_id != identifier:
            raise ValueError("invalid-inference-deal-binding")
        return deal

    def get_inference_peer(self, peer_key: str) -> InferencePeer:
        identifier = _inference_id(peer_key)
        peer = decode_inference_peer(self.call("px_getInferencePeer", [identifier]))
        if peer.peer_key != identifier:
            raise ValueError("invalid-inference-peer-binding")
        return peer

    def get_inference_receipts(self, deal_id: str) -> tuple[InferenceReceiptBatch, ...]:
        identifier = _inference_id(deal_id)
        batches = tuple(decode_inference_receipt(row) for row in _inference_rows(
            self._call_result("px_getInferenceReceipts", [identifier]), 1
        ))
        if any(batch.deal_id != identifier for batch in batches):
            raise ValueError("invalid-inference-receipt-binding")
        return batches

    def call(
        self, method: str, params: list[object]
    ) -> Mapping[str, object]:
        result = self._call_result(method, params)
        if not isinstance(result, dict) or any(
            not isinstance(key, str) for key in result
        ):
            raise ValueError("invalid-px-result")
        return result

    def _call_result(self, method: str, params: list[object]) -> object:
        if method not in PX_METHODS or not isinstance(params, list):
            raise ValueError("invalid-px-method")
        self._id += 1
        request_id = self._id
        body = json.dumps(
            {"jsonrpc": "2.0", "id": request_id, "method": method, "params": params},
            ensure_ascii=True,
            allow_nan=False,
            separators=(",", ":"),
        ).encode("utf-8")
        if len(body) > _MAX_REQUEST_BYTES:
            raise ValueError("px-request-too-large")
        headers = {
            "Accept": "application/json",
            "Content-Type": "application/json",
            "Content-Length": str(len(body)),
            "User-Agent": "layerx-python/0.1.0",
        }
        if self._credential is not None:
            headers["Authorization"] = self._credential.use()
        request = urllib.request.Request(
            self.endpoint, method="POST", headers=headers, data=body
        )
        with self._opener.open(request, timeout=self._timeout) as response:
            if (
                response.status != 200
                or response.headers.get_content_type() != "application/json"
            ):
                raise ValueError("invalid-px-http-answer")
            raw = response.read(self._maximum_response_bytes + 1)
        if len(raw) > self._maximum_response_bytes:
            raise ValueError("px-body-too-large")
        answer = json.loads(raw.decode("utf-8"))
        if (
            not isinstance(answer, dict)
            or answer.get("jsonrpc") != "2.0"
            or type(answer.get("id")) is not int
            or answer["id"] != request_id
            or ("error" in answer) == ("result" in answer)
        ):
            raise ValueError("invalid-px-envelope")
        if "error" in answer:
            error = answer["error"]
            if (
                not isinstance(error, dict)
                or type(error.get("code")) is not int
                or not isinstance(error.get("message"), str)
            ):
                raise ValueError("invalid-px-error")
            raise PxRpcError(error["code"], error["message"], error.get("data"))
        return answer["result"]


def px_account_key(account: str) -> str:
    if not isinstance(account, str):
        raise ValueError("invalid-px-account")
    lowered = account.strip().lower()
    if (
        _EVM_ADDRESS.fullmatch(lowered) is None
        and _DID.fullmatch(lowered) is None
        and _HEX32.fullmatch(lowered) is None
    ):
        raise ValueError("invalid-px-account")
    return lowered


def px_quantity(value: object) -> int:
    if type(value) is int:
        if value < 0 or value > _MAX_U256:
            raise ValueError("invalid-px-quantity")
        return value
    if not isinstance(value, str):
        raise ValueError("invalid-px-quantity")
    text = value.strip().lower()
    if _HEX_QUANTITY.fullmatch(text) is not None:
        parsed = int(text, 16)
    elif _DECIMAL.fullmatch(text) is not None:
        parsed = int(text, 10)
    else:
        raise ValueError("invalid-px-quantity")
    if parsed > _MAX_U256:
        raise ValueError("invalid-px-quantity")
    return parsed


def px_optional_quantity(value: object) -> int | None:
    return None if value is None else px_quantity(value)


def _px_counted(value: object) -> int:
    parsed = px_quantity(value)
    if parsed > _MAX_U64:
        raise ValueError("invalid-px-count")
    return parsed


def _px_optional_counted(value: object) -> int | None:
    return None if value is None else _px_counted(value)


def decode_px_resolved_identities(value: object) -> PxResolvedIdentities:
    document = _document(value)
    _present(
        document,
        ("evm_address", "pax_address", "layerx_did", "layerx_account", "bound"),
    )
    bound = document["bound"]
    if type(bound) is not bool:
        raise ValueError("invalid-px-identities")
    return PxResolvedIdentities(
        _optional_pattern(document["evm_address"], _EVM_ADDRESS),
        _optional_text(document["pax_address"], 128),
        _optional_pattern(document["layerx_did"], _DID),
        _optional_pattern(document["layerx_account"], _HEX32),
        bound,
    )


def decode_px_account_join(value: object) -> PxAccountJoin:
    document = _document(value)
    _present(document, ("account", "paxeer", "layerx"))
    paxeer = document["paxeer"]
    return PxAccountJoin(
        decode_px_resolved_identities(document["account"]),
        None if paxeer is None else _decode_paxeer_account(paxeer),
        _document_or_none(document["layerx"]),
    )


def decode_px_account_balances(value: object) -> PxAccountBalances:
    document = _document(value)
    _present(document, ("account", "balances", "joined_limit"))
    rows = document["balances"]
    if not isinstance(rows, list) or len(rows) > PX_MAXIMUM_JOINED_ASSETS:
        raise ValueError("invalid-px-balances")
    return PxAccountBalances(
        decode_px_resolved_identities(document["account"]),
        tuple(decode_px_asset_balance(row) for row in rows),
        _px_counted(document["joined_limit"]),
    )


def decode_px_asset_balance(value: object) -> PxAssetBalance:
    row = _document(value)
    _present(row, ("asset_id", "denom", "custody", "paxeer", "layerx"))
    paxeer = row["paxeer"]
    balance: PxPaxeerAssetBalance | None = None
    if paxeer is not None:
        half = _document(paxeer)
        _present(half, ("denom", "amount"))
        balance = PxPaxeerAssetBalance(
            _text(half["denom"], 128), px_quantity(half["amount"])
        )
    return PxAssetBalance(
        _pattern(row["asset_id"], _HEX32),
        _optional_text(row["denom"], 128),
        decode_px_custody_asset(row["custody"]),
        balance,
        _document_or_none(row["layerx"]),
    )


def decode_px_custody_asset(value: object) -> PxCustodyAsset | None:
    if value is None:
        return None
    custody = _document(value)
    _present(
        custody,
        (
            "asset_id",
            "denom",
            "pointer",
            "enabled",
            "paused",
            "minimum_deposit",
            "custody_cap",
            "custodied",
            "released",
            "pending",
        ),
    )
    enabled = custody["enabled"]
    paused = custody["paused"]
    if type(enabled) is not bool or type(paused) is not bool:
        raise ValueError("invalid-px-custody-asset")
    return PxCustodyAsset(
        _pattern(custody["asset_id"], _HEX32),
        _text(custody["denom"], 128),
        _pattern(custody["pointer"], _EVM_ADDRESS),
        enabled,
        paused,
        px_quantity(custody["minimum_deposit"]),
        px_quantity(custody["custody_cap"]),
        px_quantity(custody["custodied"]),
        px_quantity(custody["released"]),
        px_quantity(custody["pending"]),
    )


def decode_px_asset_table(value: object) -> PxAssetTable:
    document = _document(value)
    _present(document, ("assets", "joined_limit"))
    rows = document["assets"]
    if not isinstance(rows, list) or len(rows) > PX_MAXIMUM_JOINED_ASSETS:
        raise ValueError("invalid-px-asset-map")
    assets: list[PxAssetEntry] = []
    for row in rows:
        entry = _document(row)
        _present(entry, ("asset_id", "layerx", "paxeer"))
        assets.append(
            PxAssetEntry(
                _pattern(entry["asset_id"], _HEX32),
                _document_or_none(entry["layerx"]),
                decode_px_custody_asset(entry["paxeer"]),
            )
        )
    return PxAssetTable(tuple(assets), _px_counted(document["joined_limit"]))


def decode_px_network_head(value: object) -> PxNetworkHead:
    document = _document(value)
    _present(document, ("network_id", "paxeer", "consensus"))
    if "anchor" in document:
        raise ValueError("invalid-px-finality")
    if document["consensus"] is None:
        raise ValueError("px-consensus-unavailable")
    consensus = _document(document["consensus"])
    _present(consensus, ("latest_height",))
    paxeer = _document(document["paxeer"])
    _present(paxeer, ("chain_id", "latest_block"))
    latest_block = _px_counted(paxeer["latest_block"])
    latest_committed_height = _px_counted(consensus["latest_height"])
    if latest_committed_height > latest_block:
        raise ValueError("invalid-px-finality")
    return PxNetworkHead(
        _text(document["network_id"], 128),
        PxPaxeerHead(_px_counted(paxeer["chain_id"]), latest_block),
        _document_or_none(document.get("layerx")),
        latest_committed_height,
    )


def decode_px_receipt_keys(value: object) -> tuple[PxReceiptKeyEntry, ...]:
    if (
        not isinstance(value, dict)
        or set(value) != {"keys"}
        or not isinstance(value["keys"], list)
    ):
        raise ValueError("invalid-px-receipt-keys")
    keys: list[PxReceiptKeyEntry] = []
    seen: set[str] = set()
    for value in value["keys"]:
        entry = _document(value)
        _present(entry, ("keyId", "publicKey", "firstHeight", "lastHeight"))
        if len(entry) != 4:
            raise ValueError("invalid-px-receipt-key")
        key_id = _receipt_key_bytes(entry["keyId"])
        public_key = _receipt_key_bytes(entry["publicKey"])
        first_height = _px_counted(entry["firstHeight"])
        last_height = _px_counted(entry["lastHeight"])
        if key_id == "00" * 32 or public_key == "00" * 32 or key_id in seen:
            raise ValueError("invalid-px-receipt-key")
        if first_height > last_height:
            raise ValueError("invalid-px-receipt-key-range")
        seen.add(key_id)
        keys.append(PxReceiptKeyEntry(key_id, public_key, first_height, last_height))
    return tuple(keys)


def _receipt_key_bytes(value: object) -> str:
    if not isinstance(value, str) or re.fullmatch(r"0x[0-9a-fA-F]{64}", value) is None:
        raise ValueError("invalid-px-receipt-key-bytes")
    return value[2:].lower()


def history_cursor(block_height: int, tx_index: int, log_index: int) -> str:
    block_height = _history_position(block_height, _MAX_U64)
    tx_index = _history_position(tx_index, 0xFFFFFFFF)
    log_index = _history_position(log_index, 0xFFFFFFFF)
    return f"{block_height:016x}{tx_index:08x}{log_index:08x}"


def history_params(
    account: str,
    *,
    cursor: str | None = None,
    limit: int | None = None,
    kind: str | None = None,
) -> list[object]:
    if cursor is not None and (
        not isinstance(cursor, str) or _CURSOR.fullmatch(cursor) is None
    ):
        raise ValueError("invalid-history-cursor")
    if limit is not None and (
        type(limit) is not int or not 1 <= limit <= HISTORY_MAXIMUM_LIMIT
    ):
        raise ValueError("invalid-history-limit")
    if kind is not None and (
        not isinstance(kind, str) or _KIND.fullmatch(kind) is None
    ):
        raise ValueError("invalid-history-kind")
    return [account, cursor, limit, kind]


def history_account(account: str) -> str:
    """An EVM address or did:layerx DID, the forms px_getHistory and px_getUnifiedHistory take."""
    lowered = account.strip().lower() if isinstance(account, str) else ""
    if lowered == "did:layerx:" + "0" * 64 or (
        _EVM_ADDRESS.fullmatch(lowered) is None and _DID.fullmatch(lowered) is None
    ):
        raise ValueError("invalid-account-identifier")
    return lowered


def decode_paxeer_history_page(
    value: object, limit: int = HISTORY_MAXIMUM_LIMIT
) -> HistoryPage:
    return _decode_history_page(value, _EVM_ADDRESS, limit)


def decode_unified_history_page(
    value: object, limit: int = HISTORY_MAXIMUM_LIMIT
) -> UnifiedHistoryPage:
    document = _document(value)
    _present(document, ("account", "accounts", "items", "next_cursor"))
    sides = document["accounts"]
    if not isinstance(sides, list) or len(sides) > HISTORY_MAXIMUM_ACCOUNTS:
        raise ValueError("invalid-history-accounts")
    accounts: list[HistorySide] = []
    for entry in sides:
        side = _document(entry)
        _present(side, ("side", "account"))
        name = _history_chain(side["side"])
        account = side["account"]
        expected = _HEX32 if name == "layerx" else _EVM_ADDRESS
        if not isinstance(account, str) or expected.fullmatch(account) is None:
            raise ValueError("invalid-history-account")
        accounts.append(HistorySide(name, account))
    items = _history_rows(document["items"], True, limit)
    return UnifiedHistoryPage(
        MappingProxyType(dict(_document(document["account"]))),
        tuple(accounts),
        items,
        _history_cursor(document["next_cursor"], items),
    )


def decode_history_row(value: object, unified: bool = False) -> HistoryRow:
    row = _document(value)
    tx_hash = row.get("tx_hash")
    if (
        not isinstance(tx_hash, str)
        or re.fullmatch(r"0x[0-9a-fA-F]{64}", tx_hash) is None
    ):
        raise ValueError("invalid-history-transaction-hash")
    _present(
        row,
        (
            "id",
            "height_or_seq",
            "chain",
            "kind",
            "direction",
            "account",
            "counterparty",
            "asset",
            "amount",
            "tx_id",
            "tx_hash",
            "ordinal",
            "final",
            "decoded",
            "asset_metadata",
            "blockHeight",
            "txIndex",
            "logIndex",
        ),
    )
    direction = row["direction"]
    if direction not in ("in", "out"):
        raise ValueError("invalid-history-direction")
    final = row["final"]
    if type(final) is not bool:
        raise ValueError("invalid-history-finality")
    kind = row["kind"]
    if not isinstance(kind, str) or _KIND.fullmatch(kind) is None:
        raise ValueError("invalid-history-kind")
    row_id = row["id"]
    if (
        not isinstance(row_id, str)
        or _ROW_ID.fullmatch(row_id) is None
        or int(row_id.removeprefix("explorer:")) > _MAX_U64
    ):
        raise ValueError("invalid-history-row-id")
    if unified:
        _present(row, ("side",))
    asset = _text(row["asset"], 256)
    counterparty = row["counterparty"]
    return HistoryRow(
        row_id,
        _history_decimal(row["height_or_seq"], _MAX_U64),
        _history_chain(row["chain"]),
        kind,
        str(direction),
        _text(row["account"], 256),
        None if counterparty is None else _text(counterparty, 256),
        asset,
        _history_decimal(row["amount"], _MAX_U256),
        _text(row["tx_id"], 256),
        tx_hash,
        _history_decimal(row["ordinal"], _MAX_U64),
        final,
        row["decoded"],
        decode_history_asset_metadata(row["asset_metadata"], asset),
        _history_position(row["blockHeight"], (1 << 53) - 1),
        _history_position(row["txIndex"], 0xFFFFFFFF),
        _history_position(row["logIndex"], 0xFFFFFFFF),
        _history_chain(row["side"]) if unified else None,
    )


def decode_history_asset_metadata(
    value: object, asset: str | None = None
) -> HistoryAssetMetadata | None:
    if value is None:
        return None
    document = _document(value)
    _present(
        document,
        (
            "asset",
            "chain",
            "kind",
            "address",
            "denom",
            "symbol",
            "decimals",
            "native_id",
            "pointer",
            "metadata",
        ),
    )
    label = _text(document["asset"], 256)
    if asset is not None and label != asset:
        raise ValueError("mismatched-history-asset")
    decimals = document["decimals"]
    if decimals is not None and (type(decimals) is not int or not 0 <= decimals <= 255):
        raise ValueError("invalid-history-decimals")
    return HistoryAssetMetadata(
        label,
        _history_chain(document["chain"]),
        _text(document["kind"], 64),
        _exact_optional(document["address"], _EVM_ADDRESS),
        _optional_text(document["denom"], 128),
        _optional_text(document["symbol"], 64),
        decimals,
        _exact_optional(document["native_id"], _HEX32),
        _exact_optional(document["pointer"], _EVM_ADDRESS),
        document["metadata"],
    )


def _decode_history_page(
    value: object, expected: re.Pattern[str], limit: int
) -> HistoryPage:
    document = _document(value)
    _present(document, ("account", "items", "next_cursor"))
    account = document["account"]
    if not isinstance(account, str) or expected.fullmatch(account) is None:
        raise ValueError("invalid-history-account")
    items = _history_rows(document["items"], False, limit)
    return HistoryPage(account, items, _history_cursor(document["next_cursor"], items))


def _history_rows(value: object, unified: bool, limit: int) -> tuple[HistoryRow, ...]:
    if not isinstance(value, list):
        raise ValueError("invalid-history-page")
    items = tuple(decode_history_row(row, unified) for row in value)
    for before, after in zip(items, items[1:]):
        if (before.block_height, before.tx_index, before.log_index) > (
            after.block_height,
            after.tx_index,
            after.log_index,
        ):
            raise ValueError("unordered-history-page")
    if len(items) > limit:
        boundary = items[limit - 1]
        if any(
            (row.block_height, row.tx_index, row.log_index)
            != (boundary.block_height, boundary.tx_index, boundary.log_index)
            for row in items[limit:]
        ):
            raise ValueError("invalid-history-page")
    return items


def _history_position(value: object, maximum: int) -> int:
    if type(value) is not int or not 0 <= value <= maximum:
        raise ValueError("invalid-history-position")
    return value


def _history_cursor(value: object, items: tuple[HistoryRow, ...]) -> str | None:
    if value is None:
        return None
    if not isinstance(value, str) or _CURSOR.fullmatch(value) is None or not items:
        raise ValueError("invalid-history-cursor")
    return value


def _history_chain(value: object) -> str:
    if value not in ("layerx", "paxeer"):
        raise ValueError("invalid-history-chain")
    return str(value)


def _history_decimal(value: object, maximum: int) -> int:
    if not isinstance(value, str) or _DECIMAL.fullmatch(value) is None:
        raise ValueError("invalid-history-decimal")
    parsed = int(value, 10)
    if parsed > maximum:
        raise ValueError("invalid-history-decimal")
    return parsed


def _exact_optional(value: object, expected: re.Pattern[str]) -> str | None:
    if value is None:
        return None
    if not isinstance(value, str) or expected.fullmatch(value) is None:
        raise ValueError("invalid-history-field")
    return value


def _decode_paxeer_account(value: object) -> PxPaxeerAccount:
    document = _document(value)
    _present(document, ("address", "balance", "nonce"))
    return PxPaxeerAccount(
        _pattern(document["address"], _EVM_ADDRESS),
        px_quantity(document["balance"]),
        _px_counted(document["nonce"]),
    )


def _document(value: object) -> Mapping[str, object]:
    if (
        not isinstance(value, Mapping)
        or isinstance(value, (str, bytes))
        or any(not isinstance(key, str) for key in value)
    ):
        raise ValueError("invalid-px-document")
    return value


def _document_or_none(value: object) -> Mapping[str, object] | None:
    return None if value is None else MappingProxyType(dict(_document(value)))


def _present(value: Mapping[str, object], required: Sequence[str]) -> None:
    if any(key not in value for key in required):
        raise ValueError("incomplete-px-document")


def _pattern(value: object, expected: re.Pattern[str]) -> str:
    if not isinstance(value, str) or expected.fullmatch(value.lower()) is None:
        raise ValueError("invalid-px-field")
    return value.lower()


def _optional_pattern(value: object, expected: re.Pattern[str]) -> str | None:
    return None if value is None or value == "" else _pattern(value, expected)


def _text(value: object, maximum: int) -> str:
    if not isinstance(value, str) or not 0 < len(value) <= maximum:
        raise ValueError("invalid-px-text")
    return value


def _optional_text(value: object, maximum: int) -> str | None:
    return None if value is None else _text(value, maximum)


def _inference_id(value: object, *, zero: bool = False) -> str:
    if (
        not isinstance(value, str)
        or _HEX32.fullmatch(value) is None
        or not zero and value == "00" * 32
    ):
        raise ValueError("invalid-inference-identifier")
    return value


def _inference_integer(value: object, maximum: int, minimum: int = 0) -> int:
    if type(value) is not int or not minimum <= value <= maximum:
        raise ValueError("invalid-inference-integer")
    return value


def _inference_quantity(value: object, bits: int = 64, positive: bool = False) -> int:
    if (
        not isinstance(value, str)
        or len(value) > 39
        or _DECIMAL.fullmatch(value) is None
    ):
        raise ValueError("invalid-inference-quantity")
    parsed = int(value)
    if not int(positive) <= parsed < 1 << bits:
        raise ValueError("invalid-inference-quantity")
    return parsed


def _inference_rows(value: object, maximum: int) -> list[object]:
    if not isinstance(value, list) or len(value) > maximum:
        raise ValueError("invalid-inference-rows")
    return value


def _inference_ids(value: object, maximum: int) -> tuple[str, ...]:
    ids = tuple(_inference_id(row) for row in _inference_rows(value, maximum))
    if any(left >= right for left, right in zip(ids, ids[1:])):
        raise ValueError("invalid-inference-order-identifiers")
    return ids


def _inference_boolean(value: object) -> bool:
    if type(value) is not bool:
        raise ValueError("invalid-inference-boolean")
    return value


def _inference_document(value: object, required: Sequence[str]) -> Mapping[str, object]:
    document = _document(value)
    _present(document, required)
    return document


def _inference_profile(value: object) -> InferenceRoomProfile:
    p = _inference_document(value, tuple(InferenceRoomProfile.__dataclass_fields__))
    profile = InferenceRoomProfile(
        minimum_bond=_inference_quantity(p["minimum_bond"], 128, True),
        canary_ratio_bps=_inference_integer(p["canary_ratio_bps"], 10_000),
        lap_size_tokens=_inference_quantity(p["lap_size_tokens"], positive=True),
        challenge_window_blocks=_inference_quantity(p["challenge_window_blocks"], positive=True),
        slash_fraction_bps=_inference_integer(p["slash_fraction_bps"], 10_000),
        host_share_bps=_inference_integer(p["host_share_bps"], 10_000),
        protocol_share_bps=_inference_integer(p["protocol_share_bps"], 10_000),
        settlement_account=_inference_id(p["settlement_account"]),
    )
    if profile.host_share_bps + profile.protocol_share_bps > 10_000:
        raise ValueError("invalid-inference-payout-shares")
    return profile


def _inference_stream(value: object) -> InferenceAskStreamRecord:
    s = _inference_document(value, tuple(InferenceAskStreamRecord.__dataclass_fields__))
    return InferenceAskStreamRecord(
        payer=_inference_id(s["payer"]),
        stream_account=_inference_id(s["stream_account"]),
        recipient=_inference_id(s["recipient"]),
        asset_id=_inference_id(s["asset_id"]),
        mode=_inference_integer(s["mode"], 2, 1),
        rate=_inference_quantity(s["rate"], 128, True),
        rate_unit=_inference_quantity(s["rate_unit"], positive=True),
        start_ms=_inference_quantity(s["start_ms"]),
        last_accrual_ms=_inference_quantity(s["last_accrual_ms"]),
        end_ms=_inference_quantity(s["end_ms"]),
        total_cap=_inference_quantity(s["total_cap"], 128),
        accrued_total=_inference_quantity(s["accrued_total"], 128),
        settled_total=_inference_quantity(s["settled_total"], 128),
        remainder_carry=_inference_quantity(s["remainder_carry"], 128),
        cumulative_meter=_inference_quantity(s["cumulative_meter"]),
        meter_authorities=tuple(_inference_id(row) for row in _inference_rows(s["meter_authorities"], 8)),
        underfunded=_inference_boolean(s["underfunded"]),
        paused=_inference_boolean(s["paused"]),
        closed=_inference_boolean(s["closed"]),
    )


def decode_inference_room(value: object) -> InferenceRoom:
    v = _inference_document(value, tuple(InferenceRoom.__dataclass_fields__))
    state = _inference_integer(v["state"], 1)
    open_asks = _inference_ids(v["open_asks"], 64)
    open_bids = _inference_ids(v["open_bids"], 128)
    if state == 1 and (open_asks or open_bids):
        raise ValueError("invalid-inference-closed-room-orders")
    streams = _inference_rows(v["ask_streams"], 64)
    if len(streams) != len(open_asks):
        raise ValueError("invalid-inference-ask-stream-count")
    ask_streams = []
    for index, value in enumerate(streams):
        s = _inference_document(value, ("ask_id", "stream_id", "stream"))
        ask_id = _inference_id(s["ask_id"])
        stream_id = _inference_id(s["stream_id"], zero=True)
        if ask_id != open_asks[index] or (stream_id == "00" * 32) != (s["stream"] is None):
            raise ValueError("invalid-inference-ask-stream-binding")
        ask_streams.append(InferenceAskStream(
            ask_id, stream_id, None if s["stream"] is None else _inference_stream(s["stream"])
        ))
    return InferenceRoom(
        room_id=_inference_id(v["room_id"]),
        host_key=_inference_id(v["host_key"]),
        model_digest=_inference_id(v["model_digest"]),
        runtime_digest=_inference_id(v["runtime_digest"]),
        profile=_inference_profile(v["profile"]),
        state=cast(Literal[0, 1], state),
        opened_at_epoch=_inference_quantity(v["opened_at_epoch"]),
        open_asks=open_asks, open_bids=open_bids, ask_streams=tuple(ask_streams),
    )


def decode_inference_deal(value: object) -> InferenceDeal:
    v = _inference_document(value, tuple(InferenceDeal.__dataclass_fields__))
    rows = _inference_rows(v["layers"], 32)
    if not rows:
        raise ValueError("invalid-inference-layers")
    layers = []
    end = 0
    seen = set()
    for row in rows:
        l = _inference_document(row, ("bid_id", "first_layer", "end_layer"))
        layer = InferenceLayerSplit(
            _inference_id(l["bid_id"]), _inference_integer(l["first_layer"], 65535),
            _inference_integer(l["end_layer"], 65535, 1),
        )
        if layer.first_layer != end or layer.end_layer <= layer.first_layer or layer.bid_id in seen:
            raise ValueError("invalid-inference-layer-partition")
        seen.add(layer.bid_id)
        end = layer.end_layer
        layers.append(layer)
    return InferenceDeal(
        room_id=_inference_id(v["room_id"]), deal_id=_inference_id(v["deal_id"]),
        ask_id=_inference_id(v["ask_id"]),
        agreed_price_per_thousand_tokens=_inference_quantity(v["agreed_price_per_thousand_tokens"], 128, True),
        roster_digest=_inference_id(v["roster_digest"]), layers=tuple(layers),
        state=cast(Literal[0, 1], _inference_integer(v["state"], 1)),
        settled_tokens=_inference_quantity(v["settled_tokens"]),
        settled_amount_owed=_inference_quantity(v["settled_amount_owed"], 128),
    )


def decode_inference_peer(value: object) -> InferencePeer:
    v = _inference_document(value, tuple(InferencePeer.__dataclass_fields__))
    b = _inference_document(v["attestation_binding"], tuple(InferenceAttestationBinding.__dataclass_fields__))
    valid_from, expiry = _inference_quantity(v["valid_from"]), _inference_quantity(v["expiry"])
    if valid_from >= expiry:
        raise ValueError("invalid-inference-peer-validity")
    status = _inference_integer(v["capabilities_status"], 4, 1)
    peer_capabilities = None
    if status == 4:
        if v["capabilities"] is not None:
            raise ValueError("invalid-inference-unavailable-capabilities")
    elif status == 1:
        c = _inference_document(v["capabilities"], tuple(InferencePeerCapabilities.__dataclass_fields__))
        region = c["region_code"]
        if not isinstance(region, str) or re.fullmatch(r"[a-zA-Z]{2}", region) is None:
            raise ValueError("invalid-inference-region-code")
        peer_capabilities = InferencePeerCapabilities(
            _inference_integer(c["accelerator_memory_mib"], 0xffff_ffff),
            _inference_integer(c["system_memory_mib"], 0xffff_ffff),
            _inference_integer(c["bandwidth_class"], 4), region,
            _inference_integer(c["attestation_family"], 255),
            _inference_integer(c["runtime_kind"], 2),
        )
    else:
        raise ValueError("invalid-inference-capability-availability")
    _inference_integer(v["bond_status"], 2, 2)
    return InferencePeer(
        peer_key=_inference_id(v["peer_key"]), worker=_inference_id(v["worker"]),
        delegate=_inference_id(v["delegate"]), metadata=_inference_id(v["metadata"]),
        generation=_inference_quantity(v["generation"], positive=True),
        key_version=_inference_quantity(v["key_version"], positive=True),
        metadata_revision=_inference_quantity(v["metadata_revision"], positive=True),
        valid_from=valid_from, expiry=expiry,
        state=cast(Literal[1, 2, 3, 4, 5, 6], _inference_integer(v["state"], 6, 1)),
        enrolled=_inference_boolean(v["enrolled"]),
        capabilities_status=cast(Literal[1, 4], status), capabilities=peer_capabilities,
        bond_status=2, fault_count=_inference_integer(v["fault_count"], 0xffff_ffff),
        attestation_binding=InferenceAttestationBinding(
            _inference_integer(b["family"], 255), _inference_id(b["model_digest"], zero=True),
            _inference_id(b["runtime_digest"], zero=True), _inference_quantity(b["block_height"]),
        ),
    )


def decode_inference_receipt(value: object) -> InferenceReceiptBatch:
    v = _inference_document(value, tuple(InferenceReceiptBatch.__dataclass_fields__))
    batch = InferenceReceiptBatch(
        _inference_id(v["deal_id"]), _inference_quantity(v["first_lap_index"]),
        _inference_quantity(v["last_lap_index"]), _inference_quantity(v["cumulative_tokens"]),
        _inference_id(v["batch_digest"]), _inference_integer(v["countersign_quorum"], 0xffff_ffff),
    )
    if not 0 <= batch.last_lap_index - batch.first_lap_index < 64:
        raise ValueError("invalid-inference-lap-interval")
    return batch
