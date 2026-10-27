"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.aminoConverters = exports.registry = exports.AllowedFeeDenom = exports.ParamsPreV580 = exports.Params = void 0;
const wire_1 = require("@bufbuild/protobuf/wire");
exports.Params = {
    $type: "paxprotocol.paxchain.evm.Params",
    encode(message, writer = new wire_1.BinaryWriter()) {
        if (message.priority_normalizer !== "") {
            writer.uint32(18).string(message.priority_normalizer);
        }
        if (message.base_fee_per_gas !== "") {
            writer.uint32(26).string(message.base_fee_per_gas);
        }
        if (message.minimum_fee_per_gas !== "") {
            writer.uint32(34).string(message.minimum_fee_per_gas);
        }
        for (const v of message.whitelisted_cw_code_hashes_for_delegate_call) {
            writer.uint32(66).bytes(v);
        }
        if (message.deliver_tx_hook_wasm_gas_limit !== 0) {
            writer.uint32(72).uint64(message.deliver_tx_hook_wasm_gas_limit);
        }
        if (message.max_dynamic_base_fee_upward_adjustment !== "") {
            writer.uint32(82).string(message.max_dynamic_base_fee_upward_adjustment);
        }
        if (message.max_dynamic_base_fee_downward_adjustment !== "") {
            writer.uint32(90).string(message.max_dynamic_base_fee_downward_adjustment);
        }
        if (message.target_gas_used_per_block !== 0) {
            writer.uint32(96).uint64(message.target_gas_used_per_block);
        }
        if (message.maximum_fee_per_gas !== "") {
            writer.uint32(106).string(message.maximum_fee_per_gas);
        }
        if (message.register_pointer_disabled !== false) {
            writer.uint32(112).bool(message.register_pointer_disabled);
        }
        if (message.pax_sstore_set_gas_eip2200 !== 0) {
            writer.uint32(120).uint64(message.pax_sstore_set_gas_eip2200);
        }
        for (const v of message.allowed_fee_denoms) {
            exports.AllowedFeeDenom.encode(v, writer.uint32(130).fork()).join();
        }
        if (message.max_fee_token_spread !== "") {
            writer.uint32(138).string(message.max_fee_token_spread);
        }
        if (message.fee_token_enabled !== false) {
            writer.uint32(144).bool(message.fee_token_enabled);
        }
        if (message.fee_token_distribution !== false) {
            writer.uint32(152).bool(message.fee_token_distribution);
        }
        if (message.max_fee_token_rate_age !== 0) {
            writer.uint32(160).int64(message.max_fee_token_rate_age);
        }
        return writer;
    },
    decode(input, length) {
        const reader = input instanceof wire_1.BinaryReader ? input : new wire_1.BinaryReader(input);
        const end = length === undefined ? reader.len : reader.pos + length;
        const message = createBaseParams();
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 2:
                    if (tag !== 18) {
                        break;
                    }
                    message.priority_normalizer = reader.string();
                    continue;
                case 3:
                    if (tag !== 26) {
                        break;
                    }
                    message.base_fee_per_gas = reader.string();
                    continue;
                case 4:
                    if (tag !== 34) {
                        break;
                    }
                    message.minimum_fee_per_gas = reader.string();
                    continue;
                case 8:
                    if (tag !== 66) {
                        break;
                    }
                    message.whitelisted_cw_code_hashes_for_delegate_call.push(reader.bytes());
                    continue;
                case 9:
                    if (tag !== 72) {
                        break;
                    }
                    message.deliver_tx_hook_wasm_gas_limit = longToNumber(reader.uint64());
                    continue;
                case 10:
                    if (tag !== 82) {
                        break;
                    }
                    message.max_dynamic_base_fee_upward_adjustment = reader.string();
                    continue;
                case 11:
                    if (tag !== 90) {
                        break;
                    }
                    message.max_dynamic_base_fee_downward_adjustment = reader.string();
                    continue;
                case 12:
                    if (tag !== 96) {
                        break;
                    }
                    message.target_gas_used_per_block = longToNumber(reader.uint64());
                    continue;
                case 13:
                    if (tag !== 106) {
                        break;
                    }
                    message.maximum_fee_per_gas = reader.string();
                    continue;
                case 14:
                    if (tag !== 112) {
                        break;
                    }
                    message.register_pointer_disabled = reader.bool();
                    continue;
                case 15:
                    if (tag !== 120) {
                        break;
                    }
                    message.pax_sstore_set_gas_eip2200 = longToNumber(reader.uint64());
                    continue;
                case 16:
                    if (tag !== 130) {
                        break;
                    }
                    message.allowed_fee_denoms.push(exports.AllowedFeeDenom.decode(reader, reader.uint32()));
                    continue;
                case 17:
                    if (tag !== 138) {
                        break;
                    }
                    message.max_fee_token_spread = reader.string();
                    continue;
                case 18:
                    if (tag !== 144) {
                        break;
                    }
                    message.fee_token_enabled = reader.bool();
                    continue;
                case 19:
                    if (tag !== 152) {
                        break;
                    }
                    message.fee_token_distribution = reader.bool();
                    continue;
                case 20:
                    if (tag !== 160) {
                        break;
                    }
                    message.max_fee_token_rate_age = longToNumber(reader.int64());
                    continue;
            }
            if ((tag & 7) === 4 || tag === 0) {
                break;
            }
            reader.skip(tag & 7);
        }
        return message;
    },
    fromJSON(object) {
        return {
            priority_normalizer: isSet(object.priority_normalizer) ? globalThis.String(object.priority_normalizer) : "",
            base_fee_per_gas: isSet(object.base_fee_per_gas) ? globalThis.String(object.base_fee_per_gas) : "",
            minimum_fee_per_gas: isSet(object.minimum_fee_per_gas) ? globalThis.String(object.minimum_fee_per_gas) : "",
            whitelisted_cw_code_hashes_for_delegate_call: globalThis.Array.isArray(object?.whitelisted_cw_code_hashes_for_delegate_call)
                ? object.whitelisted_cw_code_hashes_for_delegate_call.map((e) => bytesFromBase64(e))
                : [],
            deliver_tx_hook_wasm_gas_limit: isSet(object.deliver_tx_hook_wasm_gas_limit) ? globalThis.Number(object.deliver_tx_hook_wasm_gas_limit) : 0,
            max_dynamic_base_fee_upward_adjustment: isSet(object.max_dynamic_base_fee_upward_adjustment)
                ? globalThis.String(object.max_dynamic_base_fee_upward_adjustment)
                : "",
            max_dynamic_base_fee_downward_adjustment: isSet(object.max_dynamic_base_fee_downward_adjustment)
                ? globalThis.String(object.max_dynamic_base_fee_downward_adjustment)
                : "",
            target_gas_used_per_block: isSet(object.target_gas_used_per_block) ? globalThis.Number(object.target_gas_used_per_block) : 0,
            maximum_fee_per_gas: isSet(object.maximum_fee_per_gas) ? globalThis.String(object.maximum_fee_per_gas) : "",
            register_pointer_disabled: isSet(object.register_pointer_disabled) ? globalThis.Boolean(object.register_pointer_disabled) : false,
            pax_sstore_set_gas_eip2200: isSet(object.pax_sstore_set_gas_eip2200) ? globalThis.Number(object.pax_sstore_set_gas_eip2200) : 0,
            allowed_fee_denoms: globalThis.Array.isArray(object?.allowed_fee_denoms)
                ? object.allowed_fee_denoms.map((e) => exports.AllowedFeeDenom.fromJSON(e))
                : [],
            max_fee_token_spread: isSet(object.max_fee_token_spread) ? globalThis.String(object.max_fee_token_spread) : "",
            fee_token_enabled: isSet(object.fee_token_enabled) ? globalThis.Boolean(object.fee_token_enabled) : false,
            fee_token_distribution: isSet(object.fee_token_distribution) ? globalThis.Boolean(object.fee_token_distribution) : false,
            max_fee_token_rate_age: isSet(object.max_fee_token_rate_age) ? globalThis.Number(object.max_fee_token_rate_age) : 0
        };
    },
    toJSON(message) {
        const obj = {};
        if (message.priority_normalizer !== "") {
            obj.priority_normalizer = message.priority_normalizer;
        }
        if (message.base_fee_per_gas !== "") {
            obj.base_fee_per_gas = message.base_fee_per_gas;
        }
        if (message.minimum_fee_per_gas !== "") {
            obj.minimum_fee_per_gas = message.minimum_fee_per_gas;
        }
        if (message.whitelisted_cw_code_hashes_for_delegate_call?.length) {
            obj.whitelisted_cw_code_hashes_for_delegate_call = message.whitelisted_cw_code_hashes_for_delegate_call.map((e) => base64FromBytes(e));
        }
        if (message.deliver_tx_hook_wasm_gas_limit !== 0) {
            obj.deliver_tx_hook_wasm_gas_limit = Math.round(message.deliver_tx_hook_wasm_gas_limit);
        }
        if (message.max_dynamic_base_fee_upward_adjustment !== "") {
            obj.max_dynamic_base_fee_upward_adjustment = message.max_dynamic_base_fee_upward_adjustment;
        }
        if (message.max_dynamic_base_fee_downward_adjustment !== "") {
            obj.max_dynamic_base_fee_downward_adjustment = message.max_dynamic_base_fee_downward_adjustment;
        }
        if (message.target_gas_used_per_block !== 0) {
            obj.target_gas_used_per_block = Math.round(message.target_gas_used_per_block);
        }
        if (message.maximum_fee_per_gas !== "") {
            obj.maximum_fee_per_gas = message.maximum_fee_per_gas;
        }
        if (message.register_pointer_disabled !== false) {
            obj.register_pointer_disabled = message.register_pointer_disabled;
        }
        if (message.pax_sstore_set_gas_eip2200 !== 0) {
            obj.pax_sstore_set_gas_eip2200 = Math.round(message.pax_sstore_set_gas_eip2200);
        }
        if (message.allowed_fee_denoms?.length) {
            obj.allowed_fee_denoms = message.allowed_fee_denoms.map((e) => exports.AllowedFeeDenom.toJSON(e));
        }
        if (message.max_fee_token_spread !== "") {
            obj.max_fee_token_spread = message.max_fee_token_spread;
        }
        if (message.fee_token_enabled !== false) {
            obj.fee_token_enabled = message.fee_token_enabled;
        }
        if (message.fee_token_distribution !== false) {
            obj.fee_token_distribution = message.fee_token_distribution;
        }
        if (message.max_fee_token_rate_age !== 0) {
            obj.max_fee_token_rate_age = Math.round(message.max_fee_token_rate_age);
        }
        return obj;
    },
    create(base) {
        return exports.Params.fromPartial(base ?? {});
    },
    fromPartial(object) {
        const message = createBaseParams();
        message.priority_normalizer = object.priority_normalizer ?? "";
        message.base_fee_per_gas = object.base_fee_per_gas ?? "";
        message.minimum_fee_per_gas = object.minimum_fee_per_gas ?? "";
        message.whitelisted_cw_code_hashes_for_delegate_call = object.whitelisted_cw_code_hashes_for_delegate_call?.map((e) => e) || [];
        message.deliver_tx_hook_wasm_gas_limit = object.deliver_tx_hook_wasm_gas_limit ?? 0;
        message.max_dynamic_base_fee_upward_adjustment = object.max_dynamic_base_fee_upward_adjustment ?? "";
        message.max_dynamic_base_fee_downward_adjustment = object.max_dynamic_base_fee_downward_adjustment ?? "";
        message.target_gas_used_per_block = object.target_gas_used_per_block ?? 0;
        message.maximum_fee_per_gas = object.maximum_fee_per_gas ?? "";
        message.register_pointer_disabled = object.register_pointer_disabled ?? false;
        message.pax_sstore_set_gas_eip2200 = object.pax_sstore_set_gas_eip2200 ?? 0;
        message.allowed_fee_denoms = object.allowed_fee_denoms?.map((e) => exports.AllowedFeeDenom.fromPartial(e)) || [];
        message.max_fee_token_spread = object.max_fee_token_spread ?? "";
        message.fee_token_enabled = object.fee_token_enabled ?? false;
        message.fee_token_distribution = object.fee_token_distribution ?? false;
        message.max_fee_token_rate_age = object.max_fee_token_rate_age ?? 0;
        return message;
    }
};
exports.ParamsPreV580 = {
    $type: "paxprotocol.paxchain.evm.ParamsPreV580",
    encode(message, writer = new wire_1.BinaryWriter()) {
        if (message.priority_normalizer !== "") {
            writer.uint32(18).string(message.priority_normalizer);
        }
        if (message.base_fee_per_gas !== "") {
            writer.uint32(26).string(message.base_fee_per_gas);
        }
        if (message.minimum_fee_per_gas !== "") {
            writer.uint32(34).string(message.minimum_fee_per_gas);
        }
        for (const v of message.whitelisted_cw_code_hashes_for_delegate_call) {
            writer.uint32(66).bytes(v);
        }
        return writer;
    },
    decode(input, length) {
        const reader = input instanceof wire_1.BinaryReader ? input : new wire_1.BinaryReader(input);
        const end = length === undefined ? reader.len : reader.pos + length;
        const message = createBaseParamsPreV580();
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 2:
                    if (tag !== 18) {
                        break;
                    }
                    message.priority_normalizer = reader.string();
                    continue;
                case 3:
                    if (tag !== 26) {
                        break;
                    }
                    message.base_fee_per_gas = reader.string();
                    continue;
                case 4:
                    if (tag !== 34) {
                        break;
                    }
                    message.minimum_fee_per_gas = reader.string();
                    continue;
                case 8:
                    if (tag !== 66) {
                        break;
                    }
                    message.whitelisted_cw_code_hashes_for_delegate_call.push(reader.bytes());
                    continue;
            }
            if ((tag & 7) === 4 || tag === 0) {
                break;
            }
            reader.skip(tag & 7);
        }
        return message;
    },
    fromJSON(object) {
        return {
            priority_normalizer: isSet(object.priority_normalizer) ? globalThis.String(object.priority_normalizer) : "",
            base_fee_per_gas: isSet(object.base_fee_per_gas) ? globalThis.String(object.base_fee_per_gas) : "",
            minimum_fee_per_gas: isSet(object.minimum_fee_per_gas) ? globalThis.String(object.minimum_fee_per_gas) : "",
            whitelisted_cw_code_hashes_for_delegate_call: globalThis.Array.isArray(object?.whitelisted_cw_code_hashes_for_delegate_call)
                ? object.whitelisted_cw_code_hashes_for_delegate_call.map((e) => bytesFromBase64(e))
                : []
        };
    },
    toJSON(message) {
        const obj = {};
        if (message.priority_normalizer !== "") {
            obj.priority_normalizer = message.priority_normalizer;
        }
        if (message.base_fee_per_gas !== "") {
            obj.base_fee_per_gas = message.base_fee_per_gas;
        }
        if (message.minimum_fee_per_gas !== "") {
            obj.minimum_fee_per_gas = message.minimum_fee_per_gas;
        }
        if (message.whitelisted_cw_code_hashes_for_delegate_call?.length) {
            obj.whitelisted_cw_code_hashes_for_delegate_call = message.whitelisted_cw_code_hashes_for_delegate_call.map((e) => base64FromBytes(e));
        }
        return obj;
    },
    create(base) {
        return exports.ParamsPreV580.fromPartial(base ?? {});
    },
    fromPartial(object) {
        const message = createBaseParamsPreV580();
        message.priority_normalizer = object.priority_normalizer ?? "";
        message.base_fee_per_gas = object.base_fee_per_gas ?? "";
        message.minimum_fee_per_gas = object.minimum_fee_per_gas ?? "";
        message.whitelisted_cw_code_hashes_for_delegate_call = object.whitelisted_cw_code_hashes_for_delegate_call?.map((e) => e) || [];
        return message;
    }
};
exports.AllowedFeeDenom = {
    $type: "paxprotocol.paxchain.evm.AllowedFeeDenom",
    encode(message, writer = new wire_1.BinaryWriter()) {
        if (message.denom !== "") {
            writer.uint32(10).string(message.denom);
        }
        return writer;
    },
    decode(input, length) {
        const reader = input instanceof wire_1.BinaryReader ? input : new wire_1.BinaryReader(input);
        const end = length === undefined ? reader.len : reader.pos + length;
        const message = createBaseAllowedFeeDenom();
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    if (tag !== 10) {
                        break;
                    }
                    message.denom = reader.string();
                    continue;
            }
            if ((tag & 7) === 4 || tag === 0) {
                break;
            }
            reader.skip(tag & 7);
        }
        return message;
    },
    fromJSON(object) {
        return {
            denom: isSet(object.denom) ? globalThis.String(object.denom) : ""
        };
    },
    toJSON(message) {
        const obj = {};
        if (message.denom !== "") {
            obj.denom = message.denom;
        }
        return obj;
    },
    create(base) {
        return exports.AllowedFeeDenom.fromPartial(base ?? {});
    },
    fromPartial(object) {
        const message = createBaseAllowedFeeDenom();
        message.denom = object.denom ?? "";
        return message;
    }
};
function createBaseAllowedFeeDenom() {
    return {
        denom: ""
    };
}
function createBaseParams() {
    return {
        priority_normalizer: "",
        base_fee_per_gas: "",
        minimum_fee_per_gas: "",
        whitelisted_cw_code_hashes_for_delegate_call: [],
        deliver_tx_hook_wasm_gas_limit: 0,
        max_dynamic_base_fee_upward_adjustment: "",
        max_dynamic_base_fee_downward_adjustment: "",
        target_gas_used_per_block: 0,
        maximum_fee_per_gas: "",
        register_pointer_disabled: false,
        pax_sstore_set_gas_eip2200: 0,
        allowed_fee_denoms: [],
        max_fee_token_spread: "",
        fee_token_enabled: false,
        fee_token_distribution: false,
        max_fee_token_rate_age: 0
    };
}
function createBaseParamsPreV580() {
    return {
        priority_normalizer: "",
        base_fee_per_gas: "",
        minimum_fee_per_gas: "",
        whitelisted_cw_code_hashes_for_delegate_call: []
    };
}
function bytesFromBase64(b64) {
    if (globalThis.Buffer) {
        return Uint8Array.from(globalThis.Buffer.from(b64, "base64"));
    }
    else {
        const bin = globalThis.atob(b64);
        const arr = new Uint8Array(bin.length);
        for (let i = 0; i < bin.length; ++i) {
            arr[i] = bin.charCodeAt(i);
        }
        return arr;
    }
}
function base64FromBytes(arr) {
    if (globalThis.Buffer) {
        return globalThis.Buffer.from(arr).toString("base64");
    }
    else {
        const bin = [];
        arr.forEach((byte) => {
            bin.push(globalThis.String.fromCharCode(byte));
        });
        return globalThis.btoa(bin.join(""));
    }
}
function longToNumber(int64) {
    const num = globalThis.Number(int64.toString());
    if (num > globalThis.Number.MAX_SAFE_INTEGER) {
        throw new globalThis.Error("Value is larger than Number.MAX_SAFE_INTEGER");
    }
    if (num < globalThis.Number.MIN_SAFE_INTEGER) {
        throw new globalThis.Error("Value is smaller than Number.MIN_SAFE_INTEGER");
    }
    return num;
}
function isSet(value) {
    return value !== null && value !== undefined;
}
exports.registry = [
    ["/paxprotocol.paxchain.evm.Params", exports.Params],
    ["/paxprotocol.paxchain.evm.ParamsPreV580", exports.ParamsPreV580],
    ["/paxprotocol.paxchain.evm.AllowedFeeDenom", exports.AllowedFeeDenom]
];
exports.aminoConverters = {
    "/paxprotocol.paxchain.evm.AllowedFeeDenom": {
        aminoType: "evm/AllowedFeeDenom",
        toAmino: (message) => ({ ...message }),
        fromAmino: (object) => ({ ...object })
    },
    "/paxprotocol.paxchain.evm.Params": {
        aminoType: "evm/Params",
        toAmino: (message) => ({ ...message }),
        fromAmino: (object) => ({ ...object })
    },
    "/paxprotocol.paxchain.evm.ParamsPreV580": {
        aminoType: "evm/ParamsPreV580",
        toAmino: (message) => ({ ...message }),
        fromAmino: (object) => ({ ...object })
    }
};
