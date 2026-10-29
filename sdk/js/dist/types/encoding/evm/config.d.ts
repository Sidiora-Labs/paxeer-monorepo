import type { GeneratedType } from "@cosmjs/proto-signing";
import type { ChainConfig as ChainConfig_type } from "../../types/evm";
import type { MessageFns } from "../common";
export interface ChainConfig extends Omit<ChainConfig_type, "paxSstoreSetGasEip2200"> {
    pax_sstore_set_gas_eip2200: number;
}
export declare const ChainConfig: MessageFns<ChainConfig, "paxprotocol.paxchain.evm.ChainConfig">;
export declare const registry: Array<[string, GeneratedType]>;
export declare const aminoConverters: {
    "/paxprotocol.paxchain.evm.ChainConfig": {
        aminoType: string;
        toAmino: (message: ChainConfig) => {
            cancun_time: number;
            prague_time: number;
            verkle_time: number;
            pax_sstore_set_gas_eip2200: number;
        };
        fromAmino: (object: ChainConfig) => {
            cancun_time: number;
            prague_time: number;
            verkle_time: number;
            pax_sstore_set_gas_eip2200: number;
        };
    };
};
