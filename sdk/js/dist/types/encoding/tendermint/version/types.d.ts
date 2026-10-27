import type { GeneratedType } from "@cosmjs/proto-signing";
import type { Consensus as Consensus_type } from "../../../types/tendermint/version";
import type { MessageFns } from "../../common";
export interface Consensus extends Consensus_type {
}
export declare const Consensus: MessageFns<Consensus, "tendermint.version.Consensus">;
export declare const registry: Array<[string, GeneratedType]>;
export declare const aminoConverters: {
    "/tendermint.version.Consensus": {
        aminoType: string;
        toAmino: (message: Consensus) => {
            block: number;
            app: number;
        };
        fromAmino: (object: Consensus) => {
            block: number;
            app: number;
        };
    };
};
