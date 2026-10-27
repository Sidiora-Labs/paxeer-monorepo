import type { PublicKey } from "../crypto/keys";
import type { ProofOps } from "../crypto/proof";
import type { ConsensusParams } from "../types/params";
export declare enum MisbehaviorType {
    UNKNOWN = 0,
    DUPLICATE_VOTE = 1,
    LIGHT_CLIENT_ATTACK = 2,
    UNRECOGNIZED = -1
}
export interface ResponseInfo {
    data: string;
    /** this is the software version of the application. TODO: remove? */
    version: string;
    app_version: number;
    last_block_height: number;
    last_block_app_hash: Uint8Array;
}
export interface ResponseQuery {
    code: number;
    /** bytes data = 2; // use "value" instead. */
    log: string;
    /** nondeterministic */
    info: string;
    index: number;
    key: Uint8Array;
    value: Uint8Array;
    proof_ops?: ProofOps;
    height: number;
    codespace: string;
}
export interface ResponseCheckTx {
    code: number;
    data: Uint8Array;
    gas_wanted: number;
    codespace: string;
    sender: string;
    priority: number;
}
export interface ResponseDeliverTx {
    code: number;
    data: Uint8Array;
    /** nondeterministic */
    log: string;
    /** nondeterministic */
    info: string;
    gas_wanted: number;
    gas_used: number;
    /** nondeterministic */
    events: Event[];
    codespace: string;
}
export interface ResponseCommit {
    /** reserve 1 */
    retain_height: number;
}
export interface ResponseFinalizeBlock {
    events: Event[];
    tx_results: ExecTxResult[];
    validator_updates: ValidatorUpdate[];
    consensus_param_updates?: ConsensusParams;
    app_hash: Uint8Array;
}
export interface Event {
    type: string;
    attributes: EventAttribute[];
}
export interface EventAttribute {
    key: string;
    value: string;
    /** nondeterministic */
    index: boolean;
}
export interface ExecTxResult {
    code: number;
    data: Uint8Array;
    /** nondeterministic */
    log: string;
    /** nondeterministic */
    info: string;
    gas_wanted: number;
    gas_used: number;
    /** nondeterministic */
    events: Event[];
    codespace: string;
}
export interface TxResult {
    height: number;
    index: number;
    tx: Uint8Array;
    result?: ExecTxResult;
}
export interface ValidatorUpdate {
    pub_key?: PublicKey;
    power: number;
}
