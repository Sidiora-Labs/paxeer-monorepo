from __future__ import annotations

import asyncio
import json
from base64 import b64decode, b64encode
from contextvars import ContextVar
from dataclasses import asdict
from fcntl import LOCK_EX, LOCK_NB, flock
from os import O_DIRECTORY, O_RDONLY, close, environ, fsync, open as os_open, replace
from pathlib import Path
from re import fullmatch
from typing import Awaitable, Callable, Mapping

from fastapi import FastAPI, HTTPException, Request
from fastapi.responses import JSONResponse

from layerx_fastapi import (
    JsonValue,
    LayerXMountOptions,
    LayerXResource,
    ProposedFulfillment,
    PxReceiptKeyEntry,
    SingleProcessWebhookDeliveryStore,
    StoredFulfillment,
    mount_layerx,
)
from layerx_fastapi.protocol import MiddlewareError, MiddlewareErrorCode


fulfillment_idempotency_key: ContextVar[str] = ContextVar("fulfillment_idempotency_key")


class FileFulfillmentRepository:
    def __init__(self, directory: Path) -> None:
        self.directory = directory

    async def fulfill(
        self,
        proposed: ProposedFulfillment,
        release: Callable[[], Awaitable[LayerXResource]],
    ) -> StoredFulfillment[LayerXResource]:
        if fullmatch(r"[0-9a-f]{64}", proposed.idempotency_key) is None:
            raise MiddlewareError(MiddlewareErrorCode.FULFILLMENT_CONFLICT)
        self.ensure_directory(self.directory)
        path = self.directory / f"{proposed.idempotency_key}.json"
        admission = path.with_suffix(".admitted")
        with path.with_suffix(".lock").open("ab") as lock:
            while True:
                try:
                    flock(lock.fileno(), LOCK_EX | LOCK_NB)
                    break
                except BlockingIOError:
                    await asyncio.sleep(0.01)
            if path.exists():
                self.sync_directory(self.directory)
                return self.read(path, proposed)
            if admission.exists():
                self.validate(json.loads(admission.read_text(encoding="utf-8")), proposed)
                self.sync_directory(self.directory)
            else:
                self.write_atomic(admission, self.metadata(proposed))
            token = fulfillment_idempotency_key.set(proposed.idempotency_key)
            try:
                resource = await release()
            finally:
                fulfillment_idempotency_key.reset(token)
            record = {
                **self.metadata(proposed),
                "contentType": resource.content_type,
                "body": b64encode(resource.body).decode("ascii"),
            }
            self.write_atomic(path, record)
            return self.read(path, proposed)

    @staticmethod
    def metadata(proposed: ProposedFulfillment) -> dict[str, JsonValue]:
        return {
            "requestDigest": proposed.request_digest,
            "rawTransaction": b64encode(proposed.raw_transaction).decode("ascii"),
            "signedReceipt": b64encode(proposed.signed_receipt).decode("ascii"),
            "receiptKey": asdict(proposed.receipt_key),
        }

    @classmethod
    def validate(cls, record: Mapping[str, JsonValue], proposed: ProposedFulfillment) -> None:
        expected = cls.metadata(proposed)
        if any(record[name] != expected[name] for name in expected if name != "requestDigest"):
            raise MiddlewareError(MiddlewareErrorCode.FULFILLMENT_CONFLICT)
        if record["requestDigest"] != proposed.request_digest:
            raise HTTPException(status_code=409, detail="settled_receipt_reuse")

    @staticmethod
    def sync_directory(directory: Path) -> None:
        descriptor = os_open(directory, O_RDONLY | O_DIRECTORY)
        try:
            fsync(descriptor)
        finally:
            close(descriptor)

    @classmethod
    def ensure_directory(cls, directory: Path) -> None:
        missing = []
        parent = directory
        while not parent.exists():
            missing.append(parent)
            parent = parent.parent
        for parent in reversed(missing):
            parent.mkdir(exist_ok=True, mode=0o700)
            cls.sync_directory(parent.parent)
        cls.sync_directory(directory.parent)

    @classmethod
    def write_atomic(cls, path: Path, record: Mapping[str, JsonValue]) -> None:
        temporary = path.with_suffix(path.suffix + ".tmp")
        with temporary.open("wb") as handle:
            handle.write(json.dumps(record).encode("utf-8"))
            handle.flush()
            fsync(handle.fileno())
        replace(temporary, path)
        cls.sync_directory(path.parent)

    def read(self, path: Path, proposed: ProposedFulfillment) -> StoredFulfillment[LayerXResource]:
        stored = json.loads(path.read_text(encoding="utf-8"))
        self.validate(stored, proposed)
        return StoredFulfillment(
            idempotency_key=proposed.idempotency_key,
            request_digest=stored["requestDigest"],
            raw_transaction=b64decode(stored["rawTransaction"], validate=True),
            signed_receipt=b64decode(stored["signedReceipt"], validate=True),
            receipt_key=PxReceiptKeyEntry(**stored["receiptKey"]),
            resource=LayerXResource(
                content_type=stored["contentType"],
                body=b64decode(stored["body"], validate=True),
            ),
        )


class StaticResourceHandler:
    def __init__(self, body: bytes) -> None:
        self.body = body

    async def release(self, request: Request) -> LayerXResource:
        return await self.release_once(fulfillment_idempotency_key.get())

    async def release_once(self, idempotency_key: str) -> LayerXResource:
        if fullmatch(r"[0-9a-f]{64}", idempotency_key) is None:
            raise MiddlewareError(MiddlewareErrorCode.FULFILLMENT_CONFLICT)
        return LayerXResource(content_type="application/json", body=self.body)


class RecordingEventHandler:
    def __init__(self) -> None:
        self.settlements: list[dict[str, JsonValue]] = []

    async def handle(self, event: Mapping[str, JsonValue], delivery_id: str) -> None:
        self.settlements.append({"deliveryId": delivery_id, "event": dict(event)})


resource_body = Path(environ.get("LAYERX_RESOURCE_FILE", "./resource.json")).read_bytes()
events = RecordingEventHandler()

app = FastAPI(title="LayerX paid API", openapi_url=None)
mount = mount_layerx(
    app,
    LayerXMountOptions(
        resources=StaticResourceHandler(resource_body),
        fulfillments=FileFulfillmentRepository(Path(environ.get("LAYERX_FULFILLMENT_DIR", "./fulfillments"))),
        deliveries=SingleProcessWebhookDeliveryStore(),
        events=events,
        environment={
            **environ,
            "LAYERX_RECEIPT_KEYS_JSON": environ["LAYERX_RECEIPT_KEYS_JSON"],
        },
    ),
)


@app.get("/layerx/settlements")
async def settlements() -> JSONResponse:
    return JSONResponse({"settlements": events.settlements})


@app.get("/layerx/mount")
async def mount_description() -> JSONResponse:
    return JSONResponse(
        {
            "resource": mount.config.protected_path,
            "webhook": mount.config.webhook.path,
            "integration": "receipt-gated-x402-fastapi",
        }
    )
