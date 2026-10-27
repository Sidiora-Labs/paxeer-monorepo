import type { GeneratedType } from "@cosmjs/proto-signing";
import type { EventAttribute as EventAttribute_type, Event as Event_type, ExecTxResult as ExecTxResult_type, ResponseCheckTx as ResponseCheckTx_type, ResponseCommit as ResponseCommit_type, ResponseDeliverTx as ResponseDeliverTx_type, ResponseFinalizeBlock as ResponseFinalizeBlock_type, ResponseInfo as ResponseInfo_type, ResponseQuery as ResponseQuery_type, TxResult as TxResult_type, ValidatorUpdate as ValidatorUpdate_type } from "../../../types/tendermint/abci";
import { CheckTxType, MisbehaviorType } from "../../../types/tendermint/abci";
import type { MessageFns } from "../../common";
export interface ResponseInfo extends ResponseInfo_type {
}
export interface ResponseQuery extends ResponseQuery_type {
}
export interface ResponseCheckTx extends ResponseCheckTx_type {
}
export interface ResponseDeliverTx extends ResponseDeliverTx_type {
}
export interface ResponseCommit extends ResponseCommit_type {
}
export interface ResponseFinalizeBlock extends ResponseFinalizeBlock_type {
}
export interface Event extends Event_type {
}
export interface EventAttribute extends EventAttribute_type {
}
export interface ExecTxResult extends ExecTxResult_type {
}
export interface TxResult extends TxResult_type {
}
export interface ValidatorUpdate extends ValidatorUpdate_type {
}
export declare const ResponseInfo: MessageFns<ResponseInfo, "tendermint.abci.ResponseInfo">;
export declare const ResponseQuery: MessageFns<ResponseQuery, "tendermint.abci.ResponseQuery">;
export declare const ResponseCheckTx: MessageFns<ResponseCheckTx, "tendermint.abci.ResponseCheckTx">;
export declare const ResponseDeliverTx: MessageFns<ResponseDeliverTx, "tendermint.abci.ResponseDeliverTx">;
export declare const ResponseCommit: MessageFns<ResponseCommit, "tendermint.abci.ResponseCommit">;
export declare const ResponseFinalizeBlock: MessageFns<ResponseFinalizeBlock, "tendermint.abci.ResponseFinalizeBlock">;
export declare const Event: MessageFns<Event, "tendermint.abci.Event">;
export declare const EventAttribute: MessageFns<EventAttribute, "tendermint.abci.EventAttribute">;
export declare const ExecTxResult: MessageFns<ExecTxResult, "tendermint.abci.ExecTxResult">;
export declare const TxResult: MessageFns<TxResult, "tendermint.abci.TxResult">;
export declare const ValidatorUpdate: MessageFns<ValidatorUpdate, "tendermint.abci.ValidatorUpdate">;
export declare function checkTxTypeFromJSON(object: any): CheckTxType;
export declare function checkTxTypeToJSON(object: CheckTxType): string;
export declare function misbehaviorTypeFromJSON(object: any): MisbehaviorType;
export declare function misbehaviorTypeToJSON(object: MisbehaviorType): string;
export declare const registry: Array<[string, GeneratedType]>;
export declare const aminoConverters: {
    "/tendermint.abci.ResponseInfo": {
        aminoType: string;
        toAmino: (message: ResponseInfo) => {
            data: string;
            version: string;
            app_version: number;
            last_block_height: number;
            last_block_app_hash: Uint8Array;
        };
        fromAmino: (object: ResponseInfo) => {
            data: string;
            version: string;
            app_version: number;
            last_block_height: number;
            last_block_app_hash: Uint8Array;
        };
    };
    "/tendermint.abci.ResponseQuery": {
        aminoType: string;
        toAmino: (message: ResponseQuery) => {
            code: number;
            log: string;
            info: string;
            index: number;
            key: Uint8Array;
            value: Uint8Array;
            proof_ops?: import("../../../types/tendermint/crypto").ProofOps;
            height: number;
            codespace: string;
        };
        fromAmino: (object: ResponseQuery) => {
            code: number;
            log: string;
            info: string;
            index: number;
            key: Uint8Array;
            value: Uint8Array;
            proof_ops?: import("../../../types/tendermint/crypto").ProofOps;
            height: number;
            codespace: string;
        };
    };
    "/tendermint.abci.ResponseCheckTx": {
        aminoType: string;
        toAmino: (message: ResponseCheckTx) => {
            code: number;
            data: Uint8Array;
            gas_wanted: number;
            codespace: string;
            sender: string;
            priority: number;
        };
        fromAmino: (object: ResponseCheckTx) => {
            code: number;
            data: Uint8Array;
            gas_wanted: number;
            codespace: string;
            sender: string;
            priority: number;
        };
    };
    "/tendermint.abci.ResponseDeliverTx": {
        aminoType: string;
        toAmino: (message: ResponseDeliverTx) => {
            code: number;
            data: Uint8Array;
            log: string;
            info: string;
            gas_wanted: number;
            gas_used: number;
            events: Event_type[];
            codespace: string;
        };
        fromAmino: (object: ResponseDeliverTx) => {
            code: number;
            data: Uint8Array;
            log: string;
            info: string;
            gas_wanted: number;
            gas_used: number;
            events: Event_type[];
            codespace: string;
        };
    };
    "/tendermint.abci.ResponseCommit": {
        aminoType: string;
        toAmino: (message: ResponseCommit) => {
            retain_height: number;
        };
        fromAmino: (object: ResponseCommit) => {
            retain_height: number;
        };
    };
    "/tendermint.abci.ResponseFinalizeBlock": {
        aminoType: string;
        toAmino: (message: ResponseFinalizeBlock) => {
            events: Event_type[];
            tx_results: ExecTxResult_type[];
            validator_updates: ValidatorUpdate_type[];
            consensus_param_updates?: import("../../../types/tendermint/types").ConsensusParams;
            app_hash: Uint8Array;
        };
        fromAmino: (object: ResponseFinalizeBlock) => {
            events: Event_type[];
            tx_results: ExecTxResult_type[];
            validator_updates: ValidatorUpdate_type[];
            consensus_param_updates?: import("../../../types/tendermint/types").ConsensusParams;
            app_hash: Uint8Array;
        };
    };
    "/tendermint.abci.Event": {
        aminoType: string;
        toAmino: (message: Event) => {
            type: string;
            attributes: EventAttribute_type[];
        };
        fromAmino: (object: Event) => {
            type: string;
            attributes: EventAttribute_type[];
        };
    };
    "/tendermint.abci.EventAttribute": {
        aminoType: string;
        toAmino: (message: EventAttribute) => {
            key: string;
            value: string;
            index: boolean;
        };
        fromAmino: (object: EventAttribute) => {
            key: string;
            value: string;
            index: boolean;
        };
    };
    "/tendermint.abci.ExecTxResult": {
        aminoType: string;
        toAmino: (message: ExecTxResult) => {
            code: number;
            data: Uint8Array;
            log: string;
            info: string;
            gas_wanted: number;
            gas_used: number;
            events: Event_type[];
            codespace: string;
        };
        fromAmino: (object: ExecTxResult) => {
            code: number;
            data: Uint8Array;
            log: string;
            info: string;
            gas_wanted: number;
            gas_used: number;
            events: Event_type[];
            codespace: string;
        };
    };
    "/tendermint.abci.TxResult": {
        aminoType: string;
        toAmino: (message: TxResult) => {
            height: number;
            index: number;
            tx: Uint8Array;
            result?: ExecTxResult_type;
        };
        fromAmino: (object: TxResult) => {
            height: number;
            index: number;
            tx: Uint8Array;
            result?: ExecTxResult_type;
        };
    };
    "/tendermint.abci.ValidatorUpdate": {
        aminoType: string;
        toAmino: (message: ValidatorUpdate) => {
            pub_key?: import("../../../types/tendermint/crypto").PublicKey;
            power: number;
        };
        fromAmino: (object: ValidatorUpdate) => {
            pub_key?: import("../../../types/tendermint/crypto").PublicKey;
            power: number;
        };
    };
};
