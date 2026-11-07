#!/usr/bin/env python3
"""Generate the Java-first LayerX schema model and typed operation catalogue."""

from __future__ import annotations

import argparse
import json
import os
import re
from pathlib import Path


JAVA_KEYWORDS = {
    "abstract", "assert", "boolean", "break", "byte", "case", "catch", "char", "class",
    "const", "continue", "default", "do", "double", "else", "enum", "extends", "final",
    "finally", "float", "for", "goto", "if", "implements", "import", "instanceof", "int",
    "interface", "long", "native", "new", "package", "private", "protected", "public",
    "return", "short", "static", "strictfp", "super", "switch", "synchronized", "this",
    "throw", "throws", "transient", "try", "void", "volatile", "while", "record", "sealed",
    "permits", "yield", "var",
}

PROGRAM_CALLS = {
    "program.activity": ("eth_getLogs", ["ProgramCallOutcome"]),
    "program.call": ("eth_sendRawTransaction", ["call"]),
    "program.deploy": ("eth_sendRawTransaction", ["deploy"]),
    "program.discover": ("eth_call", ["getProgram"]),
    "program.interface": ("eth_call", ["getInterface"]),
    "program.receipt": ("eth_getLogs", ["ProgramCallOutcome"]),
    "program.simulate": ("eth_call", ["simulateCall"]),
    "program.upgrade": ("eth_sendRawTransaction", ["upgrade"]),
    "program.wind-down": (
        "eth_sendRawTransaction", ["setExitRoute", "deprecate", "tombstone", "exitValue"]
    ),
}
EIP1559_PREPARE_FIELDS = [
    "preparation_ref:utf8", "chain_id:decimal_string", "nonce:decimal_string",
    "max_priority_fee_per_gas:decimal_string", "max_fee_per_gas:decimal_string",
    "gas_limit:decimal_string", "to:lowercase_hex", "data:lowercase_hex",
    "signing_hash:hex32", "activity_type:decimal_string", "actor:utf8", "authority:utf8",
    "account_sequence:decimal_string", "not_before:decimal_string", "not_after:decimal_string",
    "fee_limit:decimal_string", "payload:lowercase_hex", "payload_hash:hex32",
    "idempotency_key:hex32",
]
EIP1559_PREPARE_REQUEST_FIELDS = [
    "activity_type:decimal_string", "actor:utf8", "authority:utf8",
    "account_sequence:decimal_string", "not_before:decimal_string", "not_after:decimal_string",
    "idempotency_key:hex32", "fee_limit:decimal_string", "payload:lowercase_hex",
    "payload_hash:hex32",
]
PROGRAM_CALL_FIELDS = ["to:utf8", "data:lowercase_hex", "chainId:u64"]


def agent_plane(repo: Path) -> Plane:
    plane = Plane("Agent", sections(repo / "services/agent/schema/agent-api"))
    plane.models["Eip1559PrepareResult"] = {"fields": EIP1559_PREPARE_FIELDS}
    plane.models["McpNativeWriteV1"]["preparation"] = "NativePrepareResultV1"
    plane.models["NativeProgramCallFields"] = {"required": PROGRAM_CALL_FIELDS}
    for model in ("NativeProgramCallRequest", "NativeProgramDeployRequest",
                  "NativeProgramUpgradeRequest", "NativeProgramWindDownRequest"):
        plane.models[model] = {
            "required": ["native_call:NativeProgramCallFields", "signed_transaction:string"]
        }
    plane.models["ProgramSource"] = {
        "required": ["status:string"],
        "optional": ["source_digest:string", "environment_digest:string", "pipeline:string",
                     "expected_code_hash:string", "reproduced_artifact_digest:string"],
    }
    plane.models["VerifiedProgramDiscovery"] = {
        "required": ["program_id", "lifecycle", "version", "code_hash", "abi_version:ProgramGuestAbi",
                     "receipt_digest", "state_root", "observed_sequence", "observed_at",
                     "valid_through", "verification", "deployment_receipt_digest"],
        "optional": ["discovery_public_key", "discovery_signature"],
    }
    plane.models["VerifiedProgramInterface"] = {
        "required": ["program_id", "version", "code_hash", "abi_version:ProgramGuestAbi",
                     "interface", "interface_digest", "receipt_digest", "state_root",
                     "observed_sequence", "observed_at", "valid_through", "source:ProgramSource",
                     "verification"],
    }
    return plane


def human_plane(repo: Path) -> Plane:
    plane = Plane("Human", sections(repo / "services/human/schema/human-api"))
    plane.models["NotificationClass"] = {
        **plane.models["NotificationClass"],
        "retired_variants": string_list(plane.models["NotificationClass"].get("retired_variants"))
            + ["security-wallet-rebinding"],
    }
    plane.models["IntentDomain"] = {"variants": ["paxeer"]}
    plane.models["IntentEndpoint"] = {
        "required": ["kind:IntentEndpointKind"],
        "optional": ["account:string", "layerx_public_key:string", "evm_address:string",
                     "resolution:IntentAddressResolution"],
    }
    plane.models["IntentAddressResolution"] = {
        "required": PROGRAM_CALL_FIELDS + ["result:string"]
    }
    plane.models["IntentPayment"] = {
        "required": ["from:string", "to:string", "assetId:string", "amount:Amount",
                     "sequence:u64", "paymentReference:string"]
    }
    plane.models["IntentLeg"] = {
        "required": string_list(plane.models["IntentLeg"]["required"])
            + ["payment:IntentPayment"] + PROGRAM_CALL_FIELDS
    }
    plane.models["IntentSubmission"] = {
        **plane.models["IntentSubmission"],
        "required": string_list(plane.models["IntentSubmission"]["required"])
            + ["legs:IntentLeg[]"],
    }
    return plane


def program_transports(repo: Path) -> dict[str, tuple[str, list[str]]]:
    abi = json.loads((repo / "precompiles/layerxprograms/abi.json").read_text(encoding="utf-8"))
    signatures = {}
    for entry in abi:
        if entry["type"] not in {"function", "event"}:
            continue
        parameters = ",".join(parameter["type"] for parameter in entry["inputs"])
        signatures[entry["name"]] = f'layerxprograms.{entry["name"]}({parameters})'
    return {
        operation: (rpc, [signatures[call] for call in calls])
        for operation, (rpc, calls) in PROGRAM_CALLS.items()
    }


def sections(root: Path) -> dict[str, dict[str, object]]:
    result: dict[str, dict[str, object]] = {}
    for path in sorted(root.glob("*.kvx")):
        if path.name == "baseline.kvx":
            continue
        current: str | None = None
        for raw in path.read_text(encoding="utf-8").splitlines():
            line = raw.strip()
            if not line or line.startswith("#"):
                continue
            if line.startswith("[") and line.endswith("]"):
                current = line[1:-1]
                result.setdefault(current, {})
                continue
            if current is None or "=" not in line:
                continue
            key, encoded = (part.strip() for part in line.split("=", 1))
            try:
                result[current][key] = json.loads(encoded)
            except json.JSONDecodeError:
                result[current][key] = encoded
    return result


def java_name(value: str) -> str:
    words = re.split(r"[^A-Za-z0-9]+", value)
    rendered = "".join(word[:1].upper() + word[1:] for word in words if word)
    if not rendered or rendered[0].isdigit():
        rendered = "Value" + rendered
    return rendered


def constant_name(value: str) -> str:
    rendered = re.sub(r"[^A-Za-z0-9]+", "_", value).strip("_").upper()
    return ("VALUE_" if rendered[:1].isdigit() else "") + rendered


def field_name(value: str) -> str:
    rendered = re.sub(r"[^A-Za-z0-9_]", "_", value)
    if rendered in JAVA_KEYWORDS or rendered[:1].isdigit():
        rendered += "_"
    return rendered


def string_list(value: object) -> list[str]:
    return [item for item in value if isinstance(item, str)] if isinstance(value, list) else []


class Plane:
    def __init__(self, name: str, values: dict[str, dict[str, object]]) -> None:
        self.name = name
        self.values = values
        self.models = {
            key.split(".", 1)[1]: entries
            for key, entries in values.items()
            if key.startswith(("type.", "record.")) and entries.get("retired") != "true"
        }
        self.scalars = {
            key.split(".", 1)[1]: entries
            for key, entries in values.items()
            if key.startswith("scalar.")
        }

    def fields(self, model: str | None, fallback: list[str]) -> list[tuple[str, str | None, bool]]:
        resolved_model = (model or "").split("<", 1)[0]
        entries = self.models.get(resolved_model, {})
        raw_required = string_list(entries.get("required")) or string_list(entries.get("fields"))
        raw_optional = string_list(entries.get("optional"))
        if not raw_required and not raw_optional:
            raw_required = fallback
        fields: list[tuple[str, str | None, bool]] = []
        for encoded, optional in [(item, False) for item in raw_required] + [
            (item, True) for item in raw_optional
        ]:
            name, separator, declared = encoded.partition(":")
            if entries.get(f"{name}.retired") == "true":
                continue
            field_type = declared if separator else self.declared_field_type(entries, name)
            if field_type and field_type.endswith("?"):
                field_type = field_type[:-1]
                optional = True
            if field_type and field_type.endswith("_or_null"):
                field_type = field_type.removesuffix("_or_null")
                optional = True
            fields.append((name, field_type, optional))
        return fields

    def declared_field_type(self, entries: dict[str, object], name: str) -> str | None:
        value = entries.get(name)
        if not isinstance(value, str):
            return None
        base = value.removesuffix("[]")
        if base in self.models or base in self.scalars or base in {
            "string", "boolean", "integer", "object", "u8", "u16", "u32", "u64", "i32",
            "Amount", "Sequence", "BudgetLimit", "TimestampSeconds",
        }:
            return value
        return None

    def java_type(self, declared: str | None, optional: bool = False) -> str:
        if declared is None or "<" in declared:
            return "JsonNode"
        if declared.endswith("[]"):
            return f"List<{self.java_type(declared[:-2], True)}>"
        if declared in {"Amount", "BudgetLimit"}:
            return "ProtocolAmount"
        if declared in {"Sequence", "TimestampSeconds", "u64"}:
            return "BigInteger"
        if declared in {"integer", "u8", "u16", "u32", "i32"}:
            return "Long" if optional else "long"
        if declared in {"boolean", "bool"}:
            return "Boolean" if optional else "boolean"
        if declared == "canonical-map":
            return "ObjectNode"
        if declared == "object":
            return "JsonNode"
        if declared in self.models:
            return f"{self.name}Models.{self.model_name(declared)}"
        return "String"

    def model_name(self, model: str) -> str:
        if self.name == "Human" and model == "VerificationLevel":
            return "HumanVerificationLevel"
        return java_name(model)

    def variants(self, entries: dict[str, object]) -> list[str]:
        retired = set(string_list(entries.get("retired_variants")))
        return [variant for variant in string_list(entries.get("variants")) if variant not in retired]


def operation_specs(plane: Plane) -> list[dict[str, object]]:
    mutations = {key.split(".", 1)[1] for key in plane.values if key.startswith("mutation.")}
    result = []
    for section, entries in sorted(plane.values.items()):
        if not section.startswith("operation.") or entries.get("retired") == "true":
            continue
        wire = section.split(".", 1)[1]
        request_model = entries.get("request") if isinstance(entries.get("request"), str) else None
        response_model = entries.get("response") if isinstance(entries.get("response"), str) else None
        request_required = string_list(entries.get("request_required"))
        operation_required = string_list(entries.get("required"))
        request_fallback = request_required or operation_required
        response_fallback = string_list(entries.get("response_required"))
        result.append({
            "wire": wire,
            "name": java_name(wire),
            "request_fields": plane.fields(request_model, request_fallback),
            "response_fields": plane.fields(response_model, response_fallback),
            "response_model": response_model,
            "idempotency": bool(entries.get("idempotency")) or wire in mutations
                or (
                    entries.get("method") != "GET"
                    and "idempotency_key" in request_required + operation_required
                ),
        })
    return result


def record_source(name: str, fields: list[tuple[str, str | None, bool]], plane: Plane,
                  marker: str, indent: str, delegating: bool = False) -> list[str]:
    parameters = []
    for wire, declared, optional in fields:
        annotation = f'@JsonProperty("{wire}") ' if field_name(wire) != wire else ""
        if plane.name == "Agent" and name in {"NativePrepareResultV1", "NativeActivityV1"}:
            annotation = f'@JsonProperty(value = "{wire}", required = true) '
        if plane.scalars.get(declared or "", {}).get("wire") == "decimal_string" and plane.java_type(declared, optional) == "BigInteger":
            annotation += "@JsonSerialize(using = ToStringSerializer.class) "
            annotation += "@JsonDeserialize(using = SchemaTypes.DecimalU64Deserializer.class) "
        if declared in {"integer", "u8", "u16", "u32", "i32"}:
            annotation += "@JsonDeserialize(using = SchemaTypes.IntegerNumberDeserializer.class) "
        parameters.append(f"{annotation}{plane.java_type(declared, optional)} {field_name(wire)}")
    joined = ", ".join(parameters)
    lines = [f"{indent}public record {name}({joined}) implements SchemaTypes.{marker} {{"]
    validations = []
    for wire, declared, optional in fields:
        field = field_name(wire)
        java_type = plane.java_type(declared, optional)
        bounds = {"u8": ("0", "255"), "u16": ("0", "65535"),
                  "u32": ("0", "4294967295L"), "i32": ("-2147483648L", "2147483647L")}
        if declared in bounds:
            minimum, maximum = bounds[declared]
            guard = f"SchemaTypes.protocolBoundedLong({field}, {minimum}, {maximum});"
            validations.append(f"if ({field} != null) {guard}" if optional else guard)
        if optional:
            if java_type.startswith("List<"):
                validations.append(f"if ({field} != null) {field} = List.copyOf({field});")
            if declared in {"Sequence", "TimestampSeconds", "u64"}:
                validations.append(f"if ({field} != null) SchemaTypes.protocolU64({field});")
            continue
        if java_type.startswith("List<"):
            validations.append(f"{field} = List.copyOf(Objects.requireNonNull({field}, \"{wire}\"));")
        elif java_type not in {"long", "boolean"}:
            validations.append(f"Objects.requireNonNull({field}, \"{wire}\");")
        if declared in {"Sequence", "TimestampSeconds", "u64"}:
            validations.append(f"SchemaTypes.protocolU64({field});")
    if plane.name == "Agent" and name == "NativeProgramCallFields":
        validations.append('if (!ProgramsClient.PROGRAMS_ADDRESS.equals(to)) throw new IllegalArgumentException("invalid program target");')
    if plane.name == "Agent" and name in {"NativePrepareResultV1", "NativeActivityV1"}:
        validations.append('if (!"1".equals(version)) throw new IllegalArgumentException("invalid native version");')
    if plane.name == "Agent" and name == "NativePrepareResultV1":
        validations.append('if (approval_required != (approval_id != null)) throw new IllegalArgumentException("invalid native approval reference");')
    if validations:
        if delegating:
            lines.append(f"{indent}    @JsonCreator(mode = JsonCreator.Mode.DELEGATING)")
        lines.append(f"{indent}    public {name} {{")
        lines.extend(f"{indent}        {validation}" for validation in validations)
        lines.append(f"{indent}    }}")
    if delegating:
        wire, declared, optional = fields[0]
        lines.append(f"{indent}    @JsonValue public {plane.java_type(declared, optional)} wireValue() {{ return {field_name(wire)}; }}")
    sensitive_fields = {"token", "token_id", "session_token", "credential", "password", "secret", "api_key", "private_key", "signing_key"}
    if any(wire in sensitive_fields or wire.endswith(("_password", "_secret", "_private_key"))
           for wire, _, _ in fields):
        lines.append(f'{indent}    @Override public String toString() {{ return "[REDACTED]"; }}')
    lines.append(f"{indent}}}")
    return lines


def render_verification_status(plane: Plane) -> list[str]:
    entries = plane.models.get("VerificationStatus")
    if entries is None:
        raise SystemExit("missing record.VerificationStatus.verification_status")
    supported = 'Either {state:"achieved",rung:R}, {state:"unverified",requested:R,achieved:R,reason:ReasonCode} with achieved strictly below requested, {state:"pending",requested:R,achieved:R} with achieved strictly below requested for a submitted-but-unreceipted lookup, or {state:"absent",requested:R} when no submission is known; R is one of submitted, receipted, committed. An unverified or pending status that violates the ordering is refused as VerificationFailure verification.inconsistent.'
    if entries != {"verification_status": supported}:
        raise SystemExit("unsupported VerificationStatus contract")
    return """        @com.fasterxml.jackson.annotation.JsonInclude(com.fasterxml.jackson.annotation.JsonInclude.Include.NON_NULL)
        public record VerificationStatus(String state, String rung, String requested, String achieved, String reason) implements SchemaTypes.GeneratedResponse {
            @JsonCreator
            public VerificationStatus {
                Objects.requireNonNull(state, "state");
                boolean valid = switch (state) {
                    case "achieved" -> rung != null && requested == null && achieved == null && reason == null;
                    case "unverified" -> rung == null && requested != null && achieved != null && reason != null && !reason.isEmpty();
                    case "pending" -> rung == null && requested != null && achieved != null && reason == null;
                    case "absent" -> rung == null && requested != null && achieved == null && reason == null;
                    default -> false;
                };
                if (!valid) throw new IllegalArgumentException("invalid verification status shape");
                if (rung != null) rank(rung);
                if (requested != null) rank(requested);
                if (achieved != null && rank(achieved) >= rank(requested)) {
                    throw new IllegalArgumentException("verification.inconsistent");
                }
            }
            private static int rank(String rung) {
                return switch (rung) {
                    case "submitted" -> 0;
                    case "receipted" -> 1;
                    case "committed" -> 2;
                    default -> throw new IllegalArgumentException("unknown verification rung");
                };
            }
        }
""".splitlines()


def render_models(plane: Plane) -> list[str]:
    lines = [f"    public static final class {plane.name}Models {{", f"        private {plane.name}Models() {{}}"]
    for model, entries in sorted(plane.models.items()):
        if plane.name == "Agent" and model == "VerificationStatus":
            continue
        name = plane.model_name(model)
        variants = plane.variants(entries)
        payload_variant = any(f"{variant}.required" in entries for variant in variants)
        if variants and not payload_variant:
            declared_wires = entries.get("wire_values", string_list(entries.get("variants")))
            wire_values = string_list(declared_wires)
            if (
                not isinstance(declared_wires, list)
                or len(wire_values) != len(declared_wires)
                or len(wire_values) != len(string_list(entries.get("variants")))
            ):
                raise SystemExit(f"{plane.name} {model}: wire_values must pair one string with each variant")
            wire_values = [wire for variant, wire in zip(string_list(entries.get("variants")), wire_values)
                           if variant in variants]
            if "wire_values" in entries and all(re.fullmatch(r"0|[1-9][0-9]{0,8}", wire) for wire in wire_values):
                lines.append(f"        public enum {name} {{")
                for index, (variant, wire) in enumerate(zip(variants, wire_values)):
                    suffix = ";" if index + 1 == len(variants) else ","
                    lines.append(f"            {constant_name(variant)}({wire}){suffix}")
                lines.extend([
                    "            private final int wire;",
                    f"            {name}(int wire) {{ this.wire = wire; }}",
                    "            @JsonValue public int wire() { return wire; }",
                    f"            @JsonCreator public static {name} fromWire(int wire) {{",
                    f"                for ({name} value : values()) if (value.wire == wire) return value;",
                    "                throw new IllegalArgumentException(\"unknown schema variant\");",
                    "            }",
                    "        }",
                ])
                continue
            lines.append(f"        public enum {name} {{")
            for index, (variant, wire) in enumerate(zip(variants, wire_values)):
                suffix = ";" if index + 1 == len(variants) else ","
                lines.append(f'            {constant_name(variant)}("{wire}"){suffix}')
            lines.extend([
                "            private final String wire;",
                f"            {name}(String wire) {{ this.wire = wire; }}",
                "            @JsonValue public String wire() { return wire; }",
                f"            @JsonCreator public static {name} fromWire(String wire) {{",
                f"                for ({name} value : values()) if (value.wire.equals(wire)) return value;",
                "                throw new IllegalArgumentException(\"unknown schema variant\");",
                "            }",
                "        }",
            ])
            continue
        fields = plane.fields(model, [])
        if variants:
            allowed = ", ".join(f'"{variant}"' for variant in variants)
            marker = "GeneratedEvent" if "Event" in name else "GeneratedResponse"
            lines.append(f"        public record {name}(String kind, JsonNode value) implements SchemaTypes.{marker} {{")
            lines.append(f"            private static final Set<String> KINDS = Set.of({allowed});")
            lines.append(f"            public {name} {{")
            lines.append("                Objects.requireNonNull(kind, \"kind\");")
            lines.append("                Objects.requireNonNull(value, \"value\");")
            lines.append("                if (!KINDS.contains(kind)) throw PlatformSdkException.invalidArgument();")
            lines.append("            }")
            lines.append("        }")
        elif fields:
            marker = "GeneratedEvent" if "Event" in name else "GeneratedResponse"
            lines.extend(record_source(name, fields, plane, marker, "        "))
    if plane.name == "Agent":
        lines.extend(render_verification_status(plane))
    lines.append("    }")
    return lines


def render_operations(plane: Plane, specs: list[dict[str, object]],
                      transports: dict[str, tuple[str, list[str]]] | None = None) -> list[str]:
    lines = [f"    public static final class {plane.name}Operations {{", f"        private {plane.name}Operations() {{}}"]
    for spec in specs:
        request = f"{spec['name']}Request"
        response = f"{spec['name']}Response"
        lines.extend(record_source(request, spec["request_fields"], plane, "GeneratedRequest", "        "))
        response_fields = spec["response_fields"]
        delegating = not response_fields
        if delegating:
            response_model = spec["response_model"]
            declared = response_model if isinstance(response_model, str) and response_model in plane.models else "object"
            response_fields = [("value", declared, False)]
        lines.extend(record_source(response, response_fields, plane, "GeneratedResponse", "        ", delegating))
        lines.append(
            f"        public static final SchemaTypes.TypedOperation<{request}, {response}> {constant_name(spec['wire'])} = "
            f"new SchemaTypes.TypedOperation<>(OperationCatalog.Plane.{plane.name.upper()}, \"{spec['wire']}\", "
            f"{str(spec['idempotency']).lower()}, {request}.class, {response}.class);"
        )
        if plane.name == "Agent" and str(spec["wire"]).startswith("program."):
            if transports is None or spec["wire"] not in transports:
                raise SystemExit(f"missing layerxprograms transport for {spec['wire']}")
            rpc, calls = transports[spec["wire"]]
            arguments = ", ".join(json.dumps(call) for call in calls)
            lines.append(
                f"        public static final ProgramTransport {constant_name(spec['wire'])}_TRANSPORT = "
                f'new ProgramTransport("{rpc}", "0x000000000000000000000000000000000000101d", '
                f"List.of({arguments}));"
            )
    if plane.name == "Agent":
        prepare = next(spec for spec in specs if spec["wire"] == "prepare")
        lines.extend(record_source(
            "NativePrepareRequest", plane.fields("NativePrepareRequestV1", []),
            plane, "GeneratedRequest", "        ",
        ))
        evm_request = {
            "required": EIP1559_PREPARE_REQUEST_FIELDS, "optional": ["capability_id:hex32"]
        }
        request_plane = Plane("Agent", {"type.Eip1559PrepareRequest": evm_request})
        lines.extend(record_source(
            "Eip1559PrepareRequest", request_plane.fields("Eip1559PrepareRequest", []),
            plane, "GeneratedRequest", "        ",
        ))
        for constant, request, response in [
            ("NATIVE_PREPARE", "NativePrepareRequest", "NativePrepareResultV1"),
            ("EIP1559_PREPARE", "Eip1559PrepareRequest", "Eip1559PrepareResult"),
        ]:
            lines.append(
                f"        public static final SchemaTypes.TypedOperation<{request}, AgentModels.{response}> {constant} = "
                f'new SchemaTypes.TypedOperation<>(OperationCatalog.Plane.AGENT, "prepare", '
                f"{str(prepare['idempotency']).lower()}, {request}.class, AgentModels.{response}.class);"
            )
    lines.append("    }")
    return lines


def generate_jvm(repo: Path) -> str:
    agent = agent_plane(repo)
    human = human_plane(repo)
    transports = program_transports(repo)
    agent_specs, human_specs = operation_specs(agent), operation_specs(human)
    programs = {str(spec["wire"]) for spec in agent_specs if str(spec["wire"]).startswith("program.")}
    if programs != transports.keys():
        raise SystemExit("program operations and layerxprograms transports differ")
    output = [
        "// Code generated by sdk/platform/generators/generate_jvm.py from the LayerX API schemas. DO NOT EDIT.",
        "", "package com.sidiora.layerx.sdk;", "",
        "import com.fasterxml.jackson.annotation.JsonCreator;",
        "import com.fasterxml.jackson.annotation.JsonProperty;",
        "import com.fasterxml.jackson.annotation.JsonValue;",
        "import com.fasterxml.jackson.databind.JsonNode;",
        "import com.fasterxml.jackson.databind.node.ObjectNode;",
        "import com.fasterxml.jackson.databind.annotation.JsonDeserialize;",
        "import com.fasterxml.jackson.databind.annotation.JsonSerialize;",
        "import com.fasterxml.jackson.databind.ser.std.ToStringSerializer;",
        "import java.math.BigInteger;", "import java.util.List;", "import java.util.Map;",
        "import java.util.Objects;", "import java.util.Set;", "", "public final class GeneratedSchema {",
        "    private GeneratedSchema() {}",
        "    public record ProgramTransport(String rpcMethod, String to, List<String> calls) {",
        "        public ProgramTransport {",
        "            Objects.requireNonNull(rpcMethod, \"rpcMethod\");",
        "            Objects.requireNonNull(to, \"to\");",
        "            calls = List.copyOf(Objects.requireNonNull(calls, \"calls\"));",
        "        }",
        "    }",
        "    public static ProgramTransport programTransport(SchemaTypes.Operation operation) {",
        "        Objects.requireNonNull(operation, \"operation\");",
        "        if (operation.plane() != OperationCatalog.Plane.AGENT) throw PlatformSdkException.invalidArgument();",
        "        return switch (operation.wireName()) {",
    ]
    for operation in transports:
        output.append(f'            case "{operation}" -> AgentOperations.{constant_name(operation)}_TRANSPORT;')
    output.extend([
        "            default -> throw PlatformSdkException.invalidArgument();",
        "        };",
        "    }",
    ])
    output.extend(render_models(agent))
    output.extend(render_models(human))
    output.extend(render_operations(agent, agent_specs, transports))
    output.extend(render_operations(human, human_specs))
    output.append("    public static final Map<String, SchemaTypes.TypedOperation<?, ?>> AGENT = Map.ofEntries(")
    for index, spec in enumerate(agent_specs):
        suffix = ");" if index + 1 == len(agent_specs) else ","
        output.append(f'        Map.entry("{spec["wire"]}", AgentOperations.{constant_name(spec["wire"])} ){suffix}'.replace(" }", "}"))
    output.append("    public static final Map<String, SchemaTypes.TypedOperation<?, ?>> HUMAN = Map.ofEntries(")
    for index, spec in enumerate(human_specs):
        suffix = ");" if index + 1 == len(human_specs) else ","
        output.append(f'        Map.entry("{spec["wire"]}", HumanOperations.{constant_name(spec["wire"])} ){suffix}'.replace(" }", "}"))
    output.extend(["}", ""])
    return "\n".join(output)


def generate_contract(repo: Path) -> str:
    agent = agent_plane(repo)
    human = human_plane(repo)
    agent_specs = operation_specs(agent)
    output = [
        "// Code generated by sdk/platform/generators/generate_jvm.py from the LayerX API schemas. DO NOT EDIT.",
        "", "package com.sidiora.layerx.sdk;", "",
        "import java.util.List;", "import java.util.Map;", "import java.util.Set;", "",
        "final class GeneratedContract {", "    private GeneratedContract() {}",
        "    static final Set<String> AGENT_OPERATIONS = Set.of(",
    ]
    for index, spec in enumerate(agent_specs):
        suffix = ");" if index + 1 == len(agent_specs) else ","
        output.append(f'        "{spec["wire"]}"{suffix}')
    sets = {
        "AGENT_IDEMPOTENT": [str(spec["wire"]) for spec in agent_specs if spec["idempotency"]],
        "AGENT_ERROR_CLASSES": string_list(agent.values["type.ErrorClass"]["variants"]),
        "AGENT_RETRIABILITY": string_list(agent.values["type.Retriability"]["variants"]),
        "HUMAN_ERROR_CODES": human.variants(human.values["type.ErrorCode"]),
        "HUMAN_RETRIABILITY": string_list(human.values["type.Retriability"]["variants"]),
    }
    for name, values in sets.items():
        output.append(f"    static final Set<String> {name} = Set.of(")
        for index, value in enumerate(values):
            suffix = ");" if index + 1 == len(values) else ","
            output.append(f"        {json.dumps(value)}{suffix}")
    output.append("    static final Map<String, OperationCatalog.Route> HUMAN_ROUTES = Map.ofEntries(")
    human_specs = operation_specs(human)
    for index, spec in enumerate(human_specs):
        wire = str(spec["wire"])
        entries = human.values[f"operation.{wire}"]
        path = str(entries["path"])
        parameters = ", ".join(json.dumps(value) for value in re.findall(r"\{([^}]+)\}", path))
        idempotency = str(bool(entries.get("idempotency"))).lower()
        bodyless = str(entries.get("request") == "Empty").lower()
        suffix = ");" if index + 1 == len(human_specs) else ","
        output.append(
            f'        Map.entry("{wire}", new OperationCatalog.Route({json.dumps(entries["method"])}, '
            f'{json.dumps(path)}, List.of({parameters}), {idempotency}, {bodyless})){suffix}'
        )
    output.extend(["}", ""])
    return "\n".join(output)


def retain_human_schema(expected: str, current: str) -> str:
    for start, end in [
        ("    public static final class HumanModels {", "    public static final class AgentOperations {"),
        ("    public static final class HumanOperations {", "    public static final Map<String, SchemaTypes.TypedOperation<?, ?>> AGENT ="),
    ]:
        expected = (expected[:expected.index(start)] + current[current.index(start):current.index(end)]
                    + expected[expected.index(end):])
    return expected


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("repo", nargs="?", default=".")
    parser.add_argument("--check", action="store_true")
    parser.add_argument("--stdout", action="store_true")
    parser.add_argument("--agent-only", action="store_true")
    parser.add_argument("--verification-status", action="store_true")
    arguments = parser.parse_args()
    repo = Path(os.path.expandvars(arguments.repo)).resolve()
    if arguments.verification_status:
        plane = Plane("Agent", sections(repo / "services/agent/schema/agent-api"))
        print("\n".join(render_verification_status(plane)))
        return 0
    destination = repo / "sdk/platform/jvm/src/main/java/com/sidiora/layerx/sdk/GeneratedSchema.java"
    expected = generate_jvm(repo)
    contract = destination.parent / "GeneratedContract.java"
    expected_contract = generate_contract(repo)
    if arguments.agent_only:
        expected = retain_human_schema(expected, destination.read_text(encoding="utf-8"))
        human_start = "    static final Set<String> HUMAN_ERROR_CODES ="
        current_contract = contract.read_text(encoding="utf-8")
        expected_contract = (expected_contract[:expected_contract.index(human_start)]
                             + current_contract[current_contract.index(human_start):])
    if arguments.stdout:
        print(expected, end="")
        return 0
    if arguments.check:
        if not destination.is_file() or destination.read_text(encoding="utf-8") != expected:
            raise SystemExit("generated JVM schema is stale; run make platform-sdk-generate")
        if not contract.is_file() or contract.read_text(encoding="utf-8") != expected_contract:
            raise SystemExit("generated JVM contract is stale; run generate_jvm.py")
        return 0
    destination.write_text(expected, encoding="utf-8")
    contract.write_text(expected_contract, encoding="utf-8")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
