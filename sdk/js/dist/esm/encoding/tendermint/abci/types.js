import { BinaryReader, BinaryWriter } from "@bufbuild/protobuf/wire";
import { PublicKey } from "../crypto/keys";
import { ProofOps } from "../crypto/proof";
import { ConsensusParams } from "../types/params";
import { CheckTxType, MisbehaviorType } from "../../../types/tendermint/abci";
export const ResponseInfo = {
    $type: "tendermint.abci.ResponseInfo",
    encode(message, writer = new BinaryWriter()) {
        if (message.data !== "") {
            writer.uint32(10).string(message.data);
        }
        if (message.version !== "") {
            writer.uint32(18).string(message.version);
        }
        if (message.app_version !== 0) {
            writer.uint32(24).uint64(message.app_version);
        }
        if (message.last_block_height !== 0) {
            writer.uint32(32).int64(message.last_block_height);
        }
        if (message.last_block_app_hash.length !== 0) {
            writer.uint32(42).bytes(message.last_block_app_hash);
        }
        return writer;
    },
    decode(input, length) {
        const reader = input instanceof BinaryReader ? input : new BinaryReader(input);
        const end = length === undefined ? reader.len : reader.pos + length;
        const message = createBaseResponseInfo();
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    if (tag !== 10) {
                        break;
                    }
                    message.data = reader.string();
                    continue;
                case 2:
                    if (tag !== 18) {
                        break;
                    }
                    message.version = reader.string();
                    continue;
                case 3:
                    if (tag !== 24) {
                        break;
                    }
                    message.app_version = longToNumber(reader.uint64());
                    continue;
                case 4:
                    if (tag !== 32) {
                        break;
                    }
                    message.last_block_height = longToNumber(reader.int64());
                    continue;
                case 5:
                    if (tag !== 42) {
                        break;
                    }
                    message.last_block_app_hash = reader.bytes();
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
            data: isSet(object.data) ? globalThis.String(object.data) : "",
            version: isSet(object.version) ? globalThis.String(object.version) : "",
            app_version: isSet(object.app_version) ? globalThis.Number(object.app_version) : 0,
            last_block_height: isSet(object.last_block_height) ? globalThis.Number(object.last_block_height) : 0,
            last_block_app_hash: isSet(object.last_block_app_hash) ? bytesFromBase64(object.last_block_app_hash) : new Uint8Array(0)
        };
    },
    toJSON(message) {
        const obj = {};
        if (message.data !== "") {
            obj.data = message.data;
        }
        if (message.version !== "") {
            obj.version = message.version;
        }
        if (message.app_version !== 0) {
            obj.app_version = Math.round(message.app_version);
        }
        if (message.last_block_height !== 0) {
            obj.last_block_height = Math.round(message.last_block_height);
        }
        if (message.last_block_app_hash.length !== 0) {
            obj.last_block_app_hash = base64FromBytes(message.last_block_app_hash);
        }
        return obj;
    },
    create(base) {
        return ResponseInfo.fromPartial(base ?? {});
    },
    fromPartial(object) {
        const message = createBaseResponseInfo();
        message.data = object.data ?? "";
        message.version = object.version ?? "";
        message.app_version = object.app_version ?? 0;
        message.last_block_height = object.last_block_height ?? 0;
        message.last_block_app_hash = object.last_block_app_hash ?? new Uint8Array(0);
        return message;
    }
};
export const ResponseQuery = {
    $type: "tendermint.abci.ResponseQuery",
    encode(message, writer = new BinaryWriter()) {
        if (message.code !== 0) {
            writer.uint32(8).uint32(message.code);
        }
        if (message.log !== "") {
            writer.uint32(26).string(message.log);
        }
        if (message.info !== "") {
            writer.uint32(34).string(message.info);
        }
        if (message.index !== 0) {
            writer.uint32(40).int64(message.index);
        }
        if (message.key.length !== 0) {
            writer.uint32(50).bytes(message.key);
        }
        if (message.value.length !== 0) {
            writer.uint32(58).bytes(message.value);
        }
        if (message.proof_ops !== undefined) {
            ProofOps.encode(message.proof_ops, writer.uint32(66).fork()).join();
        }
        if (message.height !== 0) {
            writer.uint32(72).int64(message.height);
        }
        if (message.codespace !== "") {
            writer.uint32(82).string(message.codespace);
        }
        return writer;
    },
    decode(input, length) {
        const reader = input instanceof BinaryReader ? input : new BinaryReader(input);
        const end = length === undefined ? reader.len : reader.pos + length;
        const message = createBaseResponseQuery();
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    if (tag !== 8) {
                        break;
                    }
                    message.code = reader.uint32();
                    continue;
                case 3:
                    if (tag !== 26) {
                        break;
                    }
                    message.log = reader.string();
                    continue;
                case 4:
                    if (tag !== 34) {
                        break;
                    }
                    message.info = reader.string();
                    continue;
                case 5:
                    if (tag !== 40) {
                        break;
                    }
                    message.index = longToNumber(reader.int64());
                    continue;
                case 6:
                    if (tag !== 50) {
                        break;
                    }
                    message.key = reader.bytes();
                    continue;
                case 7:
                    if (tag !== 58) {
                        break;
                    }
                    message.value = reader.bytes();
                    continue;
                case 8:
                    if (tag !== 66) {
                        break;
                    }
                    message.proof_ops = ProofOps.decode(reader, reader.uint32());
                    continue;
                case 9:
                    if (tag !== 72) {
                        break;
                    }
                    message.height = longToNumber(reader.int64());
                    continue;
                case 10:
                    if (tag !== 82) {
                        break;
                    }
                    message.codespace = reader.string();
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
            code: isSet(object.code) ? globalThis.Number(object.code) : 0,
            log: isSet(object.log) ? globalThis.String(object.log) : "",
            info: isSet(object.info) ? globalThis.String(object.info) : "",
            index: isSet(object.index) ? globalThis.Number(object.index) : 0,
            key: isSet(object.key) ? bytesFromBase64(object.key) : new Uint8Array(0),
            value: isSet(object.value) ? bytesFromBase64(object.value) : new Uint8Array(0),
            proof_ops: isSet(object.proof_ops) ? ProofOps.fromJSON(object.proof_ops) : undefined,
            height: isSet(object.height) ? globalThis.Number(object.height) : 0,
            codespace: isSet(object.codespace) ? globalThis.String(object.codespace) : ""
        };
    },
    toJSON(message) {
        const obj = {};
        if (message.code !== 0) {
            obj.code = Math.round(message.code);
        }
        if (message.log !== "") {
            obj.log = message.log;
        }
        if (message.info !== "") {
            obj.info = message.info;
        }
        if (message.index !== 0) {
            obj.index = Math.round(message.index);
        }
        if (message.key.length !== 0) {
            obj.key = base64FromBytes(message.key);
        }
        if (message.value.length !== 0) {
            obj.value = base64FromBytes(message.value);
        }
        if (message.proof_ops !== undefined) {
            obj.proof_ops = ProofOps.toJSON(message.proof_ops);
        }
        if (message.height !== 0) {
            obj.height = Math.round(message.height);
        }
        if (message.codespace !== "") {
            obj.codespace = message.codespace;
        }
        return obj;
    },
    create(base) {
        return ResponseQuery.fromPartial(base ?? {});
    },
    fromPartial(object) {
        const message = createBaseResponseQuery();
        message.code = object.code ?? 0;
        message.log = object.log ?? "";
        message.info = object.info ?? "";
        message.index = object.index ?? 0;
        message.key = object.key ?? new Uint8Array(0);
        message.value = object.value ?? new Uint8Array(0);
        message.proof_ops = object.proof_ops !== undefined && object.proof_ops !== null ? ProofOps.fromPartial(object.proof_ops) : undefined;
        message.height = object.height ?? 0;
        message.codespace = object.codespace ?? "";
        return message;
    }
};
export const ResponseCheckTx = {
    $type: "tendermint.abci.ResponseCheckTx",
    encode(message, writer = new BinaryWriter()) {
        if (message.code !== 0) {
            writer.uint32(8).uint32(message.code);
        }
        if (message.data.length !== 0) {
            writer.uint32(18).bytes(message.data);
        }
        if (message.gas_wanted !== 0) {
            writer.uint32(40).int64(message.gas_wanted);
        }
        if (message.codespace !== "") {
            writer.uint32(66).string(message.codespace);
        }
        if (message.sender !== "") {
            writer.uint32(74).string(message.sender);
        }
        if (message.priority !== 0) {
            writer.uint32(80).int64(message.priority);
        }
        return writer;
    },
    decode(input, length) {
        const reader = input instanceof BinaryReader ? input : new BinaryReader(input);
        const end = length === undefined ? reader.len : reader.pos + length;
        const message = createBaseResponseCheckTx();
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    if (tag !== 8) {
                        break;
                    }
                    message.code = reader.uint32();
                    continue;
                case 2:
                    if (tag !== 18) {
                        break;
                    }
                    message.data = reader.bytes();
                    continue;
                case 5:
                    if (tag !== 40) {
                        break;
                    }
                    message.gas_wanted = longToNumber(reader.int64());
                    continue;
                case 8:
                    if (tag !== 66) {
                        break;
                    }
                    message.codespace = reader.string();
                    continue;
                case 9:
                    if (tag !== 74) {
                        break;
                    }
                    message.sender = reader.string();
                    continue;
                case 10:
                    if (tag !== 80) {
                        break;
                    }
                    message.priority = longToNumber(reader.int64());
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
            code: isSet(object.code) ? globalThis.Number(object.code) : 0,
            data: isSet(object.data) ? bytesFromBase64(object.data) : new Uint8Array(0),
            gas_wanted: isSet(object.gas_wanted) ? globalThis.Number(object.gas_wanted) : 0,
            codespace: isSet(object.codespace) ? globalThis.String(object.codespace) : "",
            sender: isSet(object.sender) ? globalThis.String(object.sender) : "",
            priority: isSet(object.priority) ? globalThis.Number(object.priority) : 0
        };
    },
    toJSON(message) {
        const obj = {};
        if (message.code !== 0) {
            obj.code = Math.round(message.code);
        }
        if (message.data.length !== 0) {
            obj.data = base64FromBytes(message.data);
        }
        if (message.gas_wanted !== 0) {
            obj.gas_wanted = Math.round(message.gas_wanted);
        }
        if (message.codespace !== "") {
            obj.codespace = message.codespace;
        }
        if (message.sender !== "") {
            obj.sender = message.sender;
        }
        if (message.priority !== 0) {
            obj.priority = Math.round(message.priority);
        }
        return obj;
    },
    create(base) {
        return ResponseCheckTx.fromPartial(base ?? {});
    },
    fromPartial(object) {
        const message = createBaseResponseCheckTx();
        message.code = object.code ?? 0;
        message.data = object.data ?? new Uint8Array(0);
        message.gas_wanted = object.gas_wanted ?? 0;
        message.codespace = object.codespace ?? "";
        message.sender = object.sender ?? "";
        message.priority = object.priority ?? 0;
        return message;
    }
};
export const ResponseDeliverTx = {
    $type: "tendermint.abci.ResponseDeliverTx",
    encode(message, writer = new BinaryWriter()) {
        if (message.code !== 0) {
            writer.uint32(8).uint32(message.code);
        }
        if (message.data.length !== 0) {
            writer.uint32(18).bytes(message.data);
        }
        if (message.log !== "") {
            writer.uint32(26).string(message.log);
        }
        if (message.info !== "") {
            writer.uint32(34).string(message.info);
        }
        if (message.gas_wanted !== 0) {
            writer.uint32(40).int64(message.gas_wanted);
        }
        if (message.gas_used !== 0) {
            writer.uint32(48).int64(message.gas_used);
        }
        for (const v of message.events) {
            Event.encode(v, writer.uint32(58).fork()).join();
        }
        if (message.codespace !== "") {
            writer.uint32(66).string(message.codespace);
        }
        return writer;
    },
    decode(input, length) {
        const reader = input instanceof BinaryReader ? input : new BinaryReader(input);
        const end = length === undefined ? reader.len : reader.pos + length;
        const message = createBaseResponseDeliverTx();
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    if (tag !== 8) {
                        break;
                    }
                    message.code = reader.uint32();
                    continue;
                case 2:
                    if (tag !== 18) {
                        break;
                    }
                    message.data = reader.bytes();
                    continue;
                case 3:
                    if (tag !== 26) {
                        break;
                    }
                    message.log = reader.string();
                    continue;
                case 4:
                    if (tag !== 34) {
                        break;
                    }
                    message.info = reader.string();
                    continue;
                case 5:
                    if (tag !== 40) {
                        break;
                    }
                    message.gas_wanted = longToNumber(reader.int64());
                    continue;
                case 6:
                    if (tag !== 48) {
                        break;
                    }
                    message.gas_used = longToNumber(reader.int64());
                    continue;
                case 7:
                    if (tag !== 58) {
                        break;
                    }
                    message.events.push(Event.decode(reader, reader.uint32()));
                    continue;
                case 8:
                    if (tag !== 66) {
                        break;
                    }
                    message.codespace = reader.string();
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
            code: isSet(object.code) ? globalThis.Number(object.code) : 0,
            data: isSet(object.data) ? bytesFromBase64(object.data) : new Uint8Array(0),
            log: isSet(object.log) ? globalThis.String(object.log) : "",
            info: isSet(object.info) ? globalThis.String(object.info) : "",
            gas_wanted: isSet(object.gas_wanted) ? globalThis.Number(object.gas_wanted) : 0,
            gas_used: isSet(object.gas_used) ? globalThis.Number(object.gas_used) : 0,
            events: globalThis.Array.isArray(object?.events) ? object.events.map((e) => Event.fromJSON(e)) : [],
            codespace: isSet(object.codespace) ? globalThis.String(object.codespace) : ""
        };
    },
    toJSON(message) {
        const obj = {};
        if (message.code !== 0) {
            obj.code = Math.round(message.code);
        }
        if (message.data.length !== 0) {
            obj.data = base64FromBytes(message.data);
        }
        if (message.log !== "") {
            obj.log = message.log;
        }
        if (message.info !== "") {
            obj.info = message.info;
        }
        if (message.gas_wanted !== 0) {
            obj.gas_wanted = Math.round(message.gas_wanted);
        }
        if (message.gas_used !== 0) {
            obj.gas_used = Math.round(message.gas_used);
        }
        if (message.events?.length) {
            obj.events = message.events.map((e) => Event.toJSON(e));
        }
        if (message.codespace !== "") {
            obj.codespace = message.codespace;
        }
        return obj;
    },
    create(base) {
        return ResponseDeliverTx.fromPartial(base ?? {});
    },
    fromPartial(object) {
        const message = createBaseResponseDeliverTx();
        message.code = object.code ?? 0;
        message.data = object.data ?? new Uint8Array(0);
        message.log = object.log ?? "";
        message.info = object.info ?? "";
        message.gas_wanted = object.gas_wanted ?? 0;
        message.gas_used = object.gas_used ?? 0;
        message.events = object.events?.map((e) => Event.fromPartial(e)) || [];
        message.codespace = object.codespace ?? "";
        return message;
    }
};
export const ResponseCommit = {
    $type: "tendermint.abci.ResponseCommit",
    encode(message, writer = new BinaryWriter()) {
        if (message.retain_height !== 0) {
            writer.uint32(24).int64(message.retain_height);
        }
        return writer;
    },
    decode(input, length) {
        const reader = input instanceof BinaryReader ? input : new BinaryReader(input);
        const end = length === undefined ? reader.len : reader.pos + length;
        const message = createBaseResponseCommit();
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 3:
                    if (tag !== 24) {
                        break;
                    }
                    message.retain_height = longToNumber(reader.int64());
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
        return { retain_height: isSet(object.retain_height) ? globalThis.Number(object.retain_height) : 0 };
    },
    toJSON(message) {
        const obj = {};
        if (message.retain_height !== 0) {
            obj.retain_height = Math.round(message.retain_height);
        }
        return obj;
    },
    create(base) {
        return ResponseCommit.fromPartial(base ?? {});
    },
    fromPartial(object) {
        const message = createBaseResponseCommit();
        message.retain_height = object.retain_height ?? 0;
        return message;
    }
};
export const ResponseFinalizeBlock = {
    $type: "tendermint.abci.ResponseFinalizeBlock",
    encode(message, writer = new BinaryWriter()) {
        for (const v of message.events) {
            Event.encode(v, writer.uint32(10).fork()).join();
        }
        for (const v of message.tx_results) {
            ExecTxResult.encode(v, writer.uint32(18).fork()).join();
        }
        for (const v of message.validator_updates) {
            ValidatorUpdate.encode(v, writer.uint32(26).fork()).join();
        }
        if (message.consensus_param_updates !== undefined) {
            ConsensusParams.encode(message.consensus_param_updates, writer.uint32(34).fork()).join();
        }
        if (message.app_hash.length !== 0) {
            writer.uint32(42).bytes(message.app_hash);
        }
        return writer;
    },
    decode(input, length) {
        const reader = input instanceof BinaryReader ? input : new BinaryReader(input);
        const end = length === undefined ? reader.len : reader.pos + length;
        const message = createBaseResponseFinalizeBlock();
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    if (tag !== 10) {
                        break;
                    }
                    message.events.push(Event.decode(reader, reader.uint32()));
                    continue;
                case 2:
                    if (tag !== 18) {
                        break;
                    }
                    message.tx_results.push(ExecTxResult.decode(reader, reader.uint32()));
                    continue;
                case 3:
                    if (tag !== 26) {
                        break;
                    }
                    message.validator_updates.push(ValidatorUpdate.decode(reader, reader.uint32()));
                    continue;
                case 4:
                    if (tag !== 34) {
                        break;
                    }
                    message.consensus_param_updates = ConsensusParams.decode(reader, reader.uint32());
                    continue;
                case 5:
                    if (tag !== 42) {
                        break;
                    }
                    message.app_hash = reader.bytes();
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
            events: globalThis.Array.isArray(object?.events) ? object.events.map((e) => Event.fromJSON(e)) : [],
            tx_results: globalThis.Array.isArray(object?.tx_results) ? object.tx_results.map((e) => ExecTxResult.fromJSON(e)) : [],
            validator_updates: globalThis.Array.isArray(object?.validator_updates) ? object.validator_updates.map((e) => ValidatorUpdate.fromJSON(e)) : [],
            consensus_param_updates: isSet(object.consensus_param_updates) ? ConsensusParams.fromJSON(object.consensus_param_updates) : undefined,
            app_hash: isSet(object.app_hash) ? bytesFromBase64(object.app_hash) : new Uint8Array(0)
        };
    },
    toJSON(message) {
        const obj = {};
        if (message.events?.length) {
            obj.events = message.events.map((e) => Event.toJSON(e));
        }
        if (message.tx_results?.length) {
            obj.tx_results = message.tx_results.map((e) => ExecTxResult.toJSON(e));
        }
        if (message.validator_updates?.length) {
            obj.validator_updates = message.validator_updates.map((e) => ValidatorUpdate.toJSON(e));
        }
        if (message.consensus_param_updates !== undefined) {
            obj.consensus_param_updates = ConsensusParams.toJSON(message.consensus_param_updates);
        }
        if (message.app_hash.length !== 0) {
            obj.app_hash = base64FromBytes(message.app_hash);
        }
        return obj;
    },
    create(base) {
        return ResponseFinalizeBlock.fromPartial(base ?? {});
    },
    fromPartial(object) {
        const message = createBaseResponseFinalizeBlock();
        message.events = object.events?.map((e) => Event.fromPartial(e)) || [];
        message.tx_results = object.tx_results?.map((e) => ExecTxResult.fromPartial(e)) || [];
        message.validator_updates = object.validator_updates?.map((e) => ValidatorUpdate.fromPartial(e)) || [];
        message.consensus_param_updates =
            object.consensus_param_updates !== undefined && object.consensus_param_updates !== null
                ? ConsensusParams.fromPartial(object.consensus_param_updates)
                : undefined;
        message.app_hash = object.app_hash ?? new Uint8Array(0);
        return message;
    }
};
export const Event = {
    $type: "tendermint.abci.Event",
    encode(message, writer = new BinaryWriter()) {
        if (message.type !== "") {
            writer.uint32(10).string(message.type);
        }
        for (const v of message.attributes) {
            EventAttribute.encode(v, writer.uint32(18).fork()).join();
        }
        return writer;
    },
    decode(input, length) {
        const reader = input instanceof BinaryReader ? input : new BinaryReader(input);
        const end = length === undefined ? reader.len : reader.pos + length;
        const message = createBaseEvent();
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    if (tag !== 10) {
                        break;
                    }
                    message.type = reader.string();
                    continue;
                case 2:
                    if (tag !== 18) {
                        break;
                    }
                    message.attributes.push(EventAttribute.decode(reader, reader.uint32()));
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
            type: isSet(object.type) ? globalThis.String(object.type) : "",
            attributes: globalThis.Array.isArray(object?.attributes) ? object.attributes.map((e) => EventAttribute.fromJSON(e)) : []
        };
    },
    toJSON(message) {
        const obj = {};
        if (message.type !== "") {
            obj.type = message.type;
        }
        if (message.attributes?.length) {
            obj.attributes = message.attributes.map((e) => EventAttribute.toJSON(e));
        }
        return obj;
    },
    create(base) {
        return Event.fromPartial(base ?? {});
    },
    fromPartial(object) {
        const message = createBaseEvent();
        message.type = object.type ?? "";
        message.attributes = object.attributes?.map((e) => EventAttribute.fromPartial(e)) || [];
        return message;
    }
};
export const EventAttribute = {
    $type: "tendermint.abci.EventAttribute",
    encode(message, writer = new BinaryWriter()) {
        if (message.key !== "") {
            writer.uint32(10).string(message.key);
        }
        if (message.value !== "") {
            writer.uint32(18).string(message.value);
        }
        if (message.index !== false) {
            writer.uint32(24).bool(message.index);
        }
        return writer;
    },
    decode(input, length) {
        const reader = input instanceof BinaryReader ? input : new BinaryReader(input);
        const end = length === undefined ? reader.len : reader.pos + length;
        const message = createBaseEventAttribute();
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    if (tag !== 10) {
                        break;
                    }
                    message.key = reader.string();
                    continue;
                case 2:
                    if (tag !== 18) {
                        break;
                    }
                    message.value = reader.string();
                    continue;
                case 3:
                    if (tag !== 24) {
                        break;
                    }
                    message.index = reader.bool();
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
            key: isSet(object.key) ? globalThis.String(object.key) : "",
            value: isSet(object.value) ? globalThis.String(object.value) : "",
            index: isSet(object.index) ? globalThis.Boolean(object.index) : false
        };
    },
    toJSON(message) {
        const obj = {};
        if (message.key !== "") {
            obj.key = message.key;
        }
        if (message.value !== "") {
            obj.value = message.value;
        }
        if (message.index !== false) {
            obj.index = message.index;
        }
        return obj;
    },
    create(base) {
        return EventAttribute.fromPartial(base ?? {});
    },
    fromPartial(object) {
        const message = createBaseEventAttribute();
        message.key = object.key ?? "";
        message.value = object.value ?? "";
        message.index = object.index ?? false;
        return message;
    }
};
export const ExecTxResult = {
    $type: "tendermint.abci.ExecTxResult",
    encode(message, writer = new BinaryWriter()) {
        if (message.code !== 0) {
            writer.uint32(8).uint32(message.code);
        }
        if (message.data.length !== 0) {
            writer.uint32(18).bytes(message.data);
        }
        if (message.log !== "") {
            writer.uint32(26).string(message.log);
        }
        if (message.info !== "") {
            writer.uint32(34).string(message.info);
        }
        if (message.gas_wanted !== 0) {
            writer.uint32(40).int64(message.gas_wanted);
        }
        if (message.gas_used !== 0) {
            writer.uint32(48).int64(message.gas_used);
        }
        for (const v of message.events) {
            Event.encode(v, writer.uint32(58).fork()).join();
        }
        if (message.codespace !== "") {
            writer.uint32(66).string(message.codespace);
        }
        return writer;
    },
    decode(input, length) {
        const reader = input instanceof BinaryReader ? input : new BinaryReader(input);
        const end = length === undefined ? reader.len : reader.pos + length;
        const message = createBaseExecTxResult();
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    if (tag !== 8) {
                        break;
                    }
                    message.code = reader.uint32();
                    continue;
                case 2:
                    if (tag !== 18) {
                        break;
                    }
                    message.data = reader.bytes();
                    continue;
                case 3:
                    if (tag !== 26) {
                        break;
                    }
                    message.log = reader.string();
                    continue;
                case 4:
                    if (tag !== 34) {
                        break;
                    }
                    message.info = reader.string();
                    continue;
                case 5:
                    if (tag !== 40) {
                        break;
                    }
                    message.gas_wanted = longToNumber(reader.int64());
                    continue;
                case 6:
                    if (tag !== 48) {
                        break;
                    }
                    message.gas_used = longToNumber(reader.int64());
                    continue;
                case 7:
                    if (tag !== 58) {
                        break;
                    }
                    message.events.push(Event.decode(reader, reader.uint32()));
                    continue;
                case 8:
                    if (tag !== 66) {
                        break;
                    }
                    message.codespace = reader.string();
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
            code: isSet(object.code) ? globalThis.Number(object.code) : 0,
            data: isSet(object.data) ? bytesFromBase64(object.data) : new Uint8Array(0),
            log: isSet(object.log) ? globalThis.String(object.log) : "",
            info: isSet(object.info) ? globalThis.String(object.info) : "",
            gas_wanted: isSet(object.gas_wanted) ? globalThis.Number(object.gas_wanted) : 0,
            gas_used: isSet(object.gas_used) ? globalThis.Number(object.gas_used) : 0,
            events: globalThis.Array.isArray(object?.events) ? object.events.map((e) => Event.fromJSON(e)) : [],
            codespace: isSet(object.codespace) ? globalThis.String(object.codespace) : ""
        };
    },
    toJSON(message) {
        const obj = {};
        if (message.code !== 0) {
            obj.code = Math.round(message.code);
        }
        if (message.data.length !== 0) {
            obj.data = base64FromBytes(message.data);
        }
        if (message.log !== "") {
            obj.log = message.log;
        }
        if (message.info !== "") {
            obj.info = message.info;
        }
        if (message.gas_wanted !== 0) {
            obj.gas_wanted = Math.round(message.gas_wanted);
        }
        if (message.gas_used !== 0) {
            obj.gas_used = Math.round(message.gas_used);
        }
        if (message.events?.length) {
            obj.events = message.events.map((e) => Event.toJSON(e));
        }
        if (message.codespace !== "") {
            obj.codespace = message.codespace;
        }
        return obj;
    },
    create(base) {
        return ExecTxResult.fromPartial(base ?? {});
    },
    fromPartial(object) {
        const message = createBaseExecTxResult();
        message.code = object.code ?? 0;
        message.data = object.data ?? new Uint8Array(0);
        message.log = object.log ?? "";
        message.info = object.info ?? "";
        message.gas_wanted = object.gas_wanted ?? 0;
        message.gas_used = object.gas_used ?? 0;
        message.events = object.events?.map((e) => Event.fromPartial(e)) || [];
        message.codespace = object.codespace ?? "";
        return message;
    }
};
export const TxResult = {
    $type: "tendermint.abci.TxResult",
    encode(message, writer = new BinaryWriter()) {
        if (message.height !== 0) {
            writer.uint32(8).int64(message.height);
        }
        if (message.index !== 0) {
            writer.uint32(16).uint32(message.index);
        }
        if (message.tx.length !== 0) {
            writer.uint32(26).bytes(message.tx);
        }
        if (message.result !== undefined) {
            ExecTxResult.encode(message.result, writer.uint32(34).fork()).join();
        }
        return writer;
    },
    decode(input, length) {
        const reader = input instanceof BinaryReader ? input : new BinaryReader(input);
        const end = length === undefined ? reader.len : reader.pos + length;
        const message = createBaseTxResult();
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    if (tag !== 8) {
                        break;
                    }
                    message.height = longToNumber(reader.int64());
                    continue;
                case 2:
                    if (tag !== 16) {
                        break;
                    }
                    message.index = reader.uint32();
                    continue;
                case 3:
                    if (tag !== 26) {
                        break;
                    }
                    message.tx = reader.bytes();
                    continue;
                case 4:
                    if (tag !== 34) {
                        break;
                    }
                    message.result = ExecTxResult.decode(reader, reader.uint32());
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
            height: isSet(object.height) ? globalThis.Number(object.height) : 0,
            index: isSet(object.index) ? globalThis.Number(object.index) : 0,
            tx: isSet(object.tx) ? bytesFromBase64(object.tx) : new Uint8Array(0),
            result: isSet(object.result) ? ExecTxResult.fromJSON(object.result) : undefined
        };
    },
    toJSON(message) {
        const obj = {};
        if (message.height !== 0) {
            obj.height = Math.round(message.height);
        }
        if (message.index !== 0) {
            obj.index = Math.round(message.index);
        }
        if (message.tx.length !== 0) {
            obj.tx = base64FromBytes(message.tx);
        }
        if (message.result !== undefined) {
            obj.result = ExecTxResult.toJSON(message.result);
        }
        return obj;
    },
    create(base) {
        return TxResult.fromPartial(base ?? {});
    },
    fromPartial(object) {
        const message = createBaseTxResult();
        message.height = object.height ?? 0;
        message.index = object.index ?? 0;
        message.tx = object.tx ?? new Uint8Array(0);
        message.result = object.result !== undefined && object.result !== null ? ExecTxResult.fromPartial(object.result) : undefined;
        return message;
    }
};
export const ValidatorUpdate = {
    $type: "tendermint.abci.ValidatorUpdate",
    encode(message, writer = new BinaryWriter()) {
        if (message.pub_key !== undefined) {
            PublicKey.encode(message.pub_key, writer.uint32(10).fork()).join();
        }
        if (message.power !== 0) {
            writer.uint32(16).int64(message.power);
        }
        return writer;
    },
    decode(input, length) {
        const reader = input instanceof BinaryReader ? input : new BinaryReader(input);
        const end = length === undefined ? reader.len : reader.pos + length;
        const message = createBaseValidatorUpdate();
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    if (tag !== 10) {
                        break;
                    }
                    message.pub_key = PublicKey.decode(reader, reader.uint32());
                    continue;
                case 2:
                    if (tag !== 16) {
                        break;
                    }
                    message.power = longToNumber(reader.int64());
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
            pub_key: isSet(object.pub_key) ? PublicKey.fromJSON(object.pub_key) : undefined,
            power: isSet(object.power) ? globalThis.Number(object.power) : 0
        };
    },
    toJSON(message) {
        const obj = {};
        if (message.pub_key !== undefined) {
            obj.pub_key = PublicKey.toJSON(message.pub_key);
        }
        if (message.power !== 0) {
            obj.power = Math.round(message.power);
        }
        return obj;
    },
    create(base) {
        return ValidatorUpdate.fromPartial(base ?? {});
    },
    fromPartial(object) {
        const message = createBaseValidatorUpdate();
        message.pub_key = object.pub_key !== undefined && object.pub_key !== null ? PublicKey.fromPartial(object.pub_key) : undefined;
        message.power = object.power ?? 0;
        return message;
    }
};
export function checkTxTypeFromJSON(object) {
    switch (object) {
        case 0:
        case "NEW":
            return CheckTxType.NEW;
        case 1:
        case "RECHECK":
            return CheckTxType.RECHECK;
        case -1:
        case "UNRECOGNIZED":
        default:
            return CheckTxType.UNRECOGNIZED;
    }
}
export function checkTxTypeToJSON(object) {
    switch (object) {
        case CheckTxType.NEW:
            return "NEW";
        case CheckTxType.RECHECK:
            return "RECHECK";
        case CheckTxType.UNRECOGNIZED:
        default:
            return "UNRECOGNIZED";
    }
}
export function misbehaviorTypeFromJSON(object) {
    switch (object) {
        case 0:
        case "UNKNOWN":
            return MisbehaviorType.UNKNOWN;
        case 1:
        case "DUPLICATE_VOTE":
            return MisbehaviorType.DUPLICATE_VOTE;
        case 2:
        case "LIGHT_CLIENT_ATTACK":
            return MisbehaviorType.LIGHT_CLIENT_ATTACK;
        case -1:
        case "UNRECOGNIZED":
        default:
            return MisbehaviorType.UNRECOGNIZED;
    }
}
export function misbehaviorTypeToJSON(object) {
    switch (object) {
        case MisbehaviorType.UNKNOWN:
            return "UNKNOWN";
        case MisbehaviorType.DUPLICATE_VOTE:
            return "DUPLICATE_VOTE";
        case MisbehaviorType.LIGHT_CLIENT_ATTACK:
            return "LIGHT_CLIENT_ATTACK";
        case MisbehaviorType.UNRECOGNIZED:
        default:
            return "UNRECOGNIZED";
    }
}
function createBaseResponseInfo() {
    return { data: "", version: "", app_version: 0, last_block_height: 0, last_block_app_hash: new Uint8Array(0) };
}
function createBaseResponseQuery() {
    return {
        code: 0,
        log: "",
        info: "",
        index: 0,
        key: new Uint8Array(0),
        value: new Uint8Array(0),
        proof_ops: undefined,
        height: 0,
        codespace: ""
    };
}
function createBaseResponseCheckTx() {
    return { code: 0, data: new Uint8Array(0), gas_wanted: 0, codespace: "", sender: "", priority: 0 };
}
function createBaseResponseDeliverTx() {
    return { code: 0, data: new Uint8Array(0), log: "", info: "", gas_wanted: 0, gas_used: 0, events: [], codespace: "" };
}
function createBaseResponseCommit() {
    return { retain_height: 0 };
}
function createBaseResponseFinalizeBlock() {
    return {
        events: [],
        tx_results: [],
        validator_updates: [],
        consensus_param_updates: undefined,
        app_hash: new Uint8Array(0)
    };
}
function createBaseEvent() {
    return { type: "", attributes: [] };
}
function createBaseEventAttribute() {
    return { key: "", value: "", index: false };
}
function createBaseExecTxResult() {
    return { code: 0, data: new Uint8Array(0), log: "", info: "", gas_wanted: 0, gas_used: 0, events: [], codespace: "" };
}
function createBaseTxResult() {
    return { height: 0, index: 0, tx: new Uint8Array(0), result: undefined };
}
function createBaseValidatorUpdate() {
    return { pub_key: undefined, power: 0 };
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
export const registry = [
    ["/tendermint.abci.ResponseInfo", ResponseInfo],
    ["/tendermint.abci.ResponseQuery", ResponseQuery],
    ["/tendermint.abci.ResponseCheckTx", ResponseCheckTx],
    ["/tendermint.abci.ResponseDeliverTx", ResponseDeliverTx],
    ["/tendermint.abci.ResponseCommit", ResponseCommit],
    ["/tendermint.abci.ResponseFinalizeBlock", ResponseFinalizeBlock],
    ["/tendermint.abci.Event", Event],
    ["/tendermint.abci.EventAttribute", EventAttribute],
    ["/tendermint.abci.ExecTxResult", ExecTxResult],
    ["/tendermint.abci.TxResult", TxResult],
    ["/tendermint.abci.ValidatorUpdate", ValidatorUpdate]
];
export const aminoConverters = {
    "/tendermint.abci.ResponseInfo": {
        aminoType: "tendermint.abci.ResponseInfo",
        toAmino: (message) => ({ ...message }),
        fromAmino: (object) => ({ ...object })
    },
    "/tendermint.abci.ResponseQuery": {
        aminoType: "tendermint.abci.ResponseQuery",
        toAmino: (message) => ({ ...message }),
        fromAmino: (object) => ({ ...object })
    },
    "/tendermint.abci.ResponseCheckTx": {
        aminoType: "tendermint.abci.ResponseCheckTx",
        toAmino: (message) => ({ ...message }),
        fromAmino: (object) => ({ ...object })
    },
    "/tendermint.abci.ResponseDeliverTx": {
        aminoType: "tendermint.abci.ResponseDeliverTx",
        toAmino: (message) => ({ ...message }),
        fromAmino: (object) => ({ ...object })
    },
    "/tendermint.abci.ResponseCommit": {
        aminoType: "tendermint.abci.ResponseCommit",
        toAmino: (message) => ({ ...message }),
        fromAmino: (object) => ({ ...object })
    },
    "/tendermint.abci.ResponseFinalizeBlock": {
        aminoType: "tendermint.abci.ResponseFinalizeBlock",
        toAmino: (message) => ({ ...message }),
        fromAmino: (object) => ({ ...object })
    },
    "/tendermint.abci.Event": {
        aminoType: "tendermint.abci.Event",
        toAmino: (message) => ({ ...message }),
        fromAmino: (object) => ({ ...object })
    },
    "/tendermint.abci.EventAttribute": {
        aminoType: "tendermint.abci.EventAttribute",
        toAmino: (message) => ({ ...message }),
        fromAmino: (object) => ({ ...object })
    },
    "/tendermint.abci.ExecTxResult": {
        aminoType: "tendermint.abci.ExecTxResult",
        toAmino: (message) => ({ ...message }),
        fromAmino: (object) => ({ ...object })
    },
    "/tendermint.abci.TxResult": {
        aminoType: "tendermint.abci.TxResult",
        toAmino: (message) => ({ ...message }),
        fromAmino: (object) => ({ ...object })
    },
    "/tendermint.abci.ValidatorUpdate": {
        aminoType: "tendermint.abci.ValidatorUpdate",
        toAmino: (message) => ({ ...message }),
        fromAmino: (object) => ({ ...object })
    }
};
