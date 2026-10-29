#!/usr/bin/env python3
"""Generate Swift and C# operation surfaces from the operation manifest."""

from __future__ import annotations

import argparse
import json
import re
from dataclasses import dataclass
from pathlib import Path


@dataclass(frozen=True)
class Operation:
    plane: str
    name: str
    method: str
    path: str
    request: str
    response: str
    idempotent: bool
    bodyless: bool
    transport: str = "http"
    to: str | None = None
    selector: str | None = None
    per_leg: bool = False

    @property
    def http_envelope_method(self) -> str:
        if self.transport == "evm":
            return "POST"
        return self.method


def operations(repo: Path) -> list[Operation]:
    manifest = json.loads(
        (repo / "sdk/platform/conformance/operations.json").read_text(encoding="utf-8")
    )
    if manifest["schema"] != 1:
        raise ValueError("unsupported operation manifest schema")
    found: dict[tuple[str, str], Operation] = {}
    for entry in manifest["operations"]:
        operation = Operation(
            plane=entry["plane"],
            name=entry["name"],
            method=entry["method"],
            path=entry["path"],
            request=entry["request"],
            response=entry["response"],
            idempotent=entry["idempotency"],
            bodyless=entry["bodyless"],
            transport=entry.get("transport", "http"),
            to=entry.get("to"),
            selector=entry.get("selector"),
            per_leg=entry.get("per_leg", False),
        )
        if operation.transport == "evm":
            if operation.path or operation.method not in (
                "eth_sendRawTransaction", "eth_call", "eth_getTransactionByHash"
            ):
                raise ValueError(f"invalid EVM transport for {operation.name}")
        elif operation.transport != "http" or operation.method not in (
            "GET", "POST", "PUT", "PATCH", "DELETE"
        ):
            raise ValueError(f"invalid HTTP transport for {operation.name}")
        key = (operation.plane, operation.name)
        if key in found:
            raise ValueError(f"duplicate operation {operation.plane}:{operation.name}")
        found[key] = operation
    return sorted(found.values(), key=lambda item: (item.plane, item.name))


def words(operation: Operation) -> list[str]:
    return [operation.plane, *re.split(r"[.\-_]", operation.name)]


def swift_name(operation: Operation) -> str:
    pieces = words(operation)
    return pieces[0] + "".join(piece[:1].upper() + piece[1:] for piece in pieces[1:])


def csharp_name(operation: Operation) -> str:
    return "".join(piece[:1].upper() + piece[1:] for piece in words(operation))


def swift_catalog(items: list[Operation]) -> str:
    cases = "\n".join(
        f'    case {swift_name(item)} = "{item.plane}:{item.name}"' for item in items
    )
    descriptors = "\n".join(
        "        case .{case_name}: return OperationDescriptor(plane: .{plane}, name: \"{name}\", "
        "method: .{method}, path: \"{path}\", requestType: \"{request}\", "
        "responseType: \"{response}\", requiresIdempotency: {idempotent}, bodyless: {bodyless})".format(
            case_name=swift_name(item),
            plane=item.plane,
            name=item.name,
            method=item.http_envelope_method.lower(),
            path=item.path,
            request=item.request.replace('"', '\\"'),
            response=item.response.replace('"', '\\"'),
            idempotent=str(item.idempotent).lower(),
            bodyless=str(item.bodyless).lower(),
        )
        for item in items
    )
    transports = "\n".join(
        f'        case .{swift_name(item)}: return OperationTransport(kind: "{item.transport}", '
        f'method: "{item.method}", to: {json.dumps(item.to) if item.to is not None else "nil"}, '
        f'selector: {json.dumps(item.selector) if item.selector is not None else "nil"}, '
        f'perLeg: {str(item.per_leg).lower()})'
        for item in items
    )
    methods = []
    for item in items:
        name = swift_name(item)
        if item.idempotent and item.bodyless:
            signature = (
                f"    func {name}(idempotencyKey: IdempotencyKey, "
                "pathParameters: [String: String] = [:]) async throws -> JSONValue"
            )
            call = (
                f"try await mutate(.{name}, request: .emptyObject, idempotencyKey: idempotencyKey, "
                "pathParameters: pathParameters)"
            )
        elif item.idempotent:
            signature = (
                f"    func {name}(_ request: JSONValue, idempotencyKey: IdempotencyKey, "
                "pathParameters: [String: String] = [:]) async throws -> JSONValue"
            )
            call = (
                f"try await mutate(.{name}, request: request, idempotencyKey: idempotencyKey, "
                "pathParameters: pathParameters)"
            )
        elif item.bodyless:
            signature = (
                f"    func {name}(_ request: JSONValue = .object([:]), "
                "pathParameters: [String: String] = [:]) async throws -> JSONValue"
            )
            call = f"try await read(.{name}, request: request, pathParameters: pathParameters)"
        else:
            signature = (
                f"    func {name}(_ request: JSONValue, "
                "pathParameters: [String: String] = [:]) async throws -> JSONValue"
            )
            call = f"try await read(.{name}, request: request, pathParameters: pathParameters)"
        methods.append(f"{signature} {{\n        {call}\n    }}")
    return f'''// Generated from sdk/platform/conformance/operations.json. Do not hand-edit.

public struct OperationTransport: Sendable, Equatable {{
    public let kind: String
    public let method: String
    public let to: String?
    public let selector: String?
    public let perLeg: Bool
}}

public enum PlatformOperation: String, CaseIterable, Sendable {{
{cases}

    public var transport: OperationTransport {{
        switch self {{
{transports}
        }}
    }}

    public var descriptor: OperationDescriptor {{
        switch self {{
{descriptors}
        }}
    }}
}}

public extension PlatformClient {{
{chr(10).join(methods)}
}}

private let sdkMetadata = SDKMetadata(name: "LayerXSDK", version: "0.1.0", agentOperations: {sum(item.plane == "agent" for item in items)}, humanOperations: {sum(item.plane == "human" for item in items)})

public func platform_sdk_swift() -> SDKMetadata {{ sdkMetadata }}
'''


def csharp_catalog(items: list[Operation]) -> str:
    enum_cases = ",\n".join(f"    {csharp_name(item)}" for item in items)
    descriptors = "\n".join(
        "            PlatformOperation.{case_name} => new(PlatformPlane.{plane}, \"{name}\", "
        "SdkHttpMethod.{method}, \"{path}\", \"{request}\", \"{response}\", {idempotent}, {bodyless}),".format(
            case_name=csharp_name(item),
            plane=item.plane.capitalize(),
            name=item.name,
            method=item.http_envelope_method.capitalize(),
            path=item.path,
            request=item.request.replace('"', '\\"'),
            response=item.response.replace('"', '\\"'),
            idempotent=str(item.idempotent).lower(),
            bodyless=str(item.bodyless).lower(),
        )
        for item in items
    )
    transports = "\n".join(
        f'            PlatformOperation.{csharp_name(item)} => new("{item.transport}", '
        f'"{item.method}", {json.dumps(item.to)}, {json.dumps(item.selector)}, '
        f'{str(item.per_leg).lower()}),'
        for item in items
    )
    methods = []
    for item in items:
        name = csharp_name(item)
        if item.idempotent and item.bodyless:
            signature = (
                f"    public static Task<JsonValue> {name}Async(this PlatformClient client, "
                "IdempotencyKey idempotencyKey, IReadOnlyDictionary<string, string>? pathParameters = null, "
                "CancellationToken cancellationToken = default)"
            )
            call = (
                f"client.MutateAsync(PlatformOperation.{name}, JsonValue.EmptyObject, idempotencyKey, "
                "pathParameters, cancellationToken)"
            )
        elif item.idempotent:
            signature = (
                f"    public static Task<JsonValue> {name}Async(this PlatformClient client, JsonValue request, "
                "IdempotencyKey idempotencyKey, IReadOnlyDictionary<string, string>? pathParameters = null, "
                "CancellationToken cancellationToken = default)"
            )
            call = (
                f"client.MutateAsync(PlatformOperation.{name}, request, idempotencyKey, "
                "pathParameters, cancellationToken)"
            )
        elif item.bodyless:
            signature = (
                f"    public static Task<JsonValue> {name}Async(this PlatformClient client, JsonValue? request = null, "
                "IReadOnlyDictionary<string, string>? pathParameters = null, CancellationToken cancellationToken = default)"
            )
            call = (
                f"client.ReadAsync(PlatformOperation.{name}, request ?? JsonValue.EmptyObject, "
                "pathParameters, cancellationToken)"
            )
        else:
            signature = (
                f"    public static Task<JsonValue> {name}Async(this PlatformClient client, JsonValue request, "
                "IReadOnlyDictionary<string, string>? pathParameters = null, CancellationToken cancellationToken = default)"
            )
            call = (
                f"client.ReadAsync(PlatformOperation.{name}, request, "
                "pathParameters, cancellationToken)"
            )
        methods.append(f"{signature} =>\n        {call};")
    return f'''// Generated from sdk/platform/conformance/operations.json. Do not hand-edit.
#nullable enable

namespace LayerX.Sdk;

public sealed record OperationTransport(
    string Kind, string Method, string? To, string? Selector, bool PerLeg);

public enum PlatformOperation
{{
{enum_cases}
}}

public static class GeneratedOperationCatalog
{{
    public static OperationTransport Transport(this PlatformOperation operation) => operation switch
    {{
{transports}
        _ => throw new ArgumentOutOfRangeException(nameof(operation)),
    }};

    public static OperationDescriptor Descriptor(this PlatformOperation operation) => operation switch
    {{
{descriptors}
        _ => throw new ArgumentOutOfRangeException(nameof(operation)),
    }};

    public static SdkMetadata platform_sdk_dotnet()
    {{
        return new("LayerX.Sdk", "0.1.0", {sum(item.plane == "agent" for item in items)}, {sum(item.plane == "human" for item in items)});
    }}
}}

public static class GeneratedPlatformClientExtensions
{{
{chr(10).join(methods)}
}}
'''


def render(repo: Path) -> dict[Path, str]:
    items = operations(repo)
    return {
        repo / "sdk/platform/swift/Sources/LayerXSDK/Generated/OperationCatalog.swift": swift_catalog(items),
        repo / "sdk/platform/dotnet/Generated/OperationCatalog.cs": csharp_catalog(items),
    }


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--check", action="store_true")
    parser.add_argument("repo", nargs="?", default=".")
    args = parser.parse_args()
    repo = Path(args.repo).resolve()
    stale: list[str] = []
    for path, content in render(repo).items():
        if args.check:
            if not path.is_file() or path.read_text(encoding="utf-8") != content:
                stale.append(str(path.relative_to(repo)))
        else:
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text(content, encoding="utf-8")
    if stale:
        raise SystemExit("stale generated portable SDK output: " + ", ".join(stale))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
