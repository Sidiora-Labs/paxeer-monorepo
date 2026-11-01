import type { AllowList, Metadata } from "../cosmos/bank/v1beta1/bank";
import type { PageRequest, PageResponse } from "../cosmos/base/query/v1beta1/pagination";
import type { DenomAuthorityMetadata } from "./authorityMetadata";
import type { Params } from "./params";
export type QueryParamsRequest = {};
export interface QueryParamsResponse {
    /** params defines the parameters of the module. */
    params?: Params;
}
export interface QueryDenomAuthorityMetadataRequest {
    denom: string;
}
export interface QueryDenomAuthorityMetadataResponse {
    authority_metadata?: DenomAuthorityMetadata;
}
export interface QueryDenomsFromCreatorRequest {
    creator: string;
    pagination?: PageRequest;
}
export interface QueryDenomsFromCreatorResponse {
    denoms: string[];
    pagination?: PageResponse;
}
export interface QueryDenomMetadataRequest {
    /** denom is the coin denom to query the metadata for. */
    denom: string;
}
export interface QueryDenomMetadataResponse {
    /** metadata describes and provides all the client information for the requested token. */
    metadata?: Metadata;
}
export interface QueryDenomAllowListRequest {
    /** denom is the coin denom to query the allowlist for. */
    denom: string;
}
export interface QueryDenomAllowListResponse {
    /** allow_list provides addresses allowed for the requested token. */
    allow_list?: AllowList;
}
