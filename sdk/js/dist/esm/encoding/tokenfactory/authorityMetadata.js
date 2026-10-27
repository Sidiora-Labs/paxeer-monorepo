import { BinaryReader, BinaryWriter } from "@bufbuild/protobuf/wire";
export const DenomAuthorityMetadata = {
    $type: "paxprotocol.paxchain.tokenfactory.DenomAuthorityMetadata",
    encode(message, writer = new BinaryWriter()) {
        if (message.admin !== "") {
            writer.uint32(10).string(message.admin);
        }
        if (message.issuerDid !== "") {
            writer.uint32(18).string(message.issuerDid);
        }
        return writer;
    },
    decode(input, length) {
        const reader = input instanceof BinaryReader ? input : new BinaryReader(input);
        const end = length === undefined ? reader.len : reader.pos + length;
        const message = createBaseDenomAuthorityMetadata();
        while (reader.pos < end) {
            const tag = reader.uint32();
            switch (tag >>> 3) {
                case 1:
                    if (tag !== 10) {
                        break;
                    }
                    message.admin = reader.string();
                    continue;
                case 2:
                    if (tag !== 18) {
                        break;
                    }
                    message.issuerDid = reader.string();
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
            admin: isSet(object.admin) ? globalThis.String(object.admin) : "",
            issuerDid: isSet(object.issuerDid) ? globalThis.String(object.issuerDid) : ""
        };
    },
    toJSON(message) {
        const obj = {};
        if (message.admin !== "") {
            obj.admin = message.admin;
        }
        if (message.issuerDid !== "") {
            obj.issuerDid = message.issuerDid;
        }
        return obj;
    },
    create(base) {
        return DenomAuthorityMetadata.fromPartial(base ?? {});
    },
    fromPartial(object) {
        const message = createBaseDenomAuthorityMetadata();
        message.admin = object.admin ?? "";
        message.issuerDid = object.issuerDid ?? "";
        return message;
    }
};
function createBaseDenomAuthorityMetadata() {
    return { admin: "", issuerDid: "" };
}
function isSet(value) {
    return value !== null && value !== undefined;
}
export const registry = [["/paxprotocol.paxchain.tokenfactory.DenomAuthorityMetadata", DenomAuthorityMetadata]];
export const aminoConverters = {
    "/paxprotocol.paxchain.tokenfactory.DenomAuthorityMetadata": {
        aminoType: "tokenfactory/DenomAuthorityMetadata",
        toAmino: (message) => ({ ...message }),
        fromAmino: (object) => ({ ...object })
    }
};
