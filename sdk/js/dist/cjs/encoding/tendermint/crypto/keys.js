"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.aminoConverters = exports.registry = exports.PublicKey = void 0;
const wire_1 = require("@bufbuild/protobuf/wire");
exports.PublicKey = {
    $type: "tendermint.crypto.PublicKey",
    encode(message, writer = new wire_1.BinaryWriter()) {
        if (message.ed25519 !== undefined) {
            writer.uint32(10).bytes(message.ed25519);
        }
        return writer;
    },
    decode(input, length) {
        const reader = input instanceof wire_1.BinaryReader ? input : new wire_1.BinaryReader(input);
        const end = length === undefined ? reader.len : reader.pos + length;
        const message = createBasePublicKey();
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    if (tag !== 10) {
                        break;
                    }
                    message.ed25519 = reader.bytes();
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
            ed25519: isSet(object.ed25519) ? bytesFromBase64(object.ed25519) : undefined
        };
    },
    toJSON(message) {
        const obj = {};
        if (message.ed25519 !== undefined) {
            obj.ed25519 = base64FromBytes(message.ed25519);
        }
        return obj;
    },
    create(base) {
        return exports.PublicKey.fromPartial(base ?? {});
    },
    fromPartial(object) {
        const message = createBasePublicKey();
        message.ed25519 = object.ed25519 ?? undefined;
        return message;
    }
};
function createBasePublicKey() {
    return { ed25519: undefined };
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
function isSet(value) {
    return value !== null && value !== undefined;
}
exports.registry = [["/tendermint.crypto.PublicKey", exports.PublicKey]];
exports.aminoConverters = {
    "/tendermint.crypto.PublicKey": {
        aminoType: "tendermint.crypto.PublicKey",
        toAmino: (message) => ({ ed25519: message.ed25519 }),
        fromAmino: (object) => ({ ed25519: object.ed25519 })
    }
};
