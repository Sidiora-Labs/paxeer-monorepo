import {
  createPublicClient,
  http,
  type Address,
  type PublicClient,
} from "viem";
import { z } from "zod";
import { PerpsPrecompileABI } from "@paxeer-network/paxeer-backend/core/network/abi/PerpsPrecompile";
import { PRECOMPILES } from "@paxeer-network/paxeer-backend/core/network/precompiles";
import * as calls from "@paxeer-network/paxeer-backend/core/perps/calls";
import {
  accountViewFromState,
  accountRiskFromView,
} from "@paxeer-network/paxeer-backend/core/perps/accountView";
import { COLLATERAL_TOKENS } from "@paxeer-network/paxeer-backend/core/perps/config";
import { decodeMarketConfig } from "@paxeer-network/paxeer-backend/core/perps/marketConfig";
import { positionFromTuple } from "@paxeer-network/paxeer-backend/core/perps/positionWrites";
import {
  MARKETS_QUERY,
  TRADES_QUERY,
  CLOSED_POSITIONS_QUERY,
  decodeGatewayMarketsResult,
} from "@paxeer-network/paxeer-backend/hooks/perps/useSubgraph";
import { AgentKitError, jsonText } from "./result.js";
import { httpUrl } from "./gateway.js";

export interface PerpsDataOptions {
  nodeUrl: string;
  dataUrl: string;
  publicClient?: PublicClient;
  fetch?: typeof fetch;
}

export class PerpsDataClient {
  readonly rpc: PublicClient;
  readonly #graphql: string;
  readonly #fetch: typeof fetch;

  constructor(options: PerpsDataOptions) {
    this.rpc =
      options.publicClient ??
      createPublicClient({
        transport: http(httpUrl(options.nodeUrl), {
          retryCount: 0,
          timeout: 30_000,
        }),
      });
    this.#graphql = `${httpUrl(options.dataUrl)}/graphql`;
    this.#fetch = options.fetch ?? globalThis.fetch;
  }

  async #query(
    query: string,
    variables?: Record<string, unknown>,
  ): Promise<unknown> {
    const response = await this.#fetch(this.#graphql, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: jsonText({ query, variables }),
      signal: AbortSignal.timeout(30_000),
    });
    if (!response.ok)
      throw new AgentKitError(
        "DATA_UNAVAILABLE",
        "Trading data request failed",
        { status: response.status },
      );
    const result = z
      .object({
        data: z.record(z.string(), z.unknown()).optional(),
        errors: z.array(z.object({ message: z.string() })).optional(),
      })
      .parse(await response.json());
    if (result.errors?.length)
      throw new AgentKitError("DATA_QUERY_FAILED", result.errors[0].message);
    if (!result.data)
      throw new AgentKitError(
        "DATA_RESPONSE_INVALID",
        "Trading data response has no data",
      );
    return result.data;
  }

  async market(marketId: bigint) {
    const [config, tiers] = await Promise.all([
      this.rpc.readContract(calls.buildGetMarketConfig(marketId)),
      this.rpc.readContract(calls.buildGetMarginTiers(marketId)),
    ]);
    return { config: decodeMarketConfig(config), tiers };
  }

  async mark(marketId: bigint) {
    const [markPrice, indexPrice, midPrice, fresh] =
      await this.rpc.readContract(calls.buildGetMarkPriceDetails(marketId));
    return { markPrice, indexPrice, midPrice, fresh };
  }

  async markets() {
    const { markets } = decodeGatewayMarketsResult(
      await this.#query(MARKETS_QUERY),
    );
    return Promise.all(
      markets.map(async (row) => ({
        ...row,
        ...(await this.market(BigInt(row.marketId))),
        mark: await this.mark(BigInt(row.marketId)),
      })),
    );
  }

  async account(address: Address) {
    const state = await this.rpc.readContract(calls.buildGetAccount(address));
    const view = accountViewFromState(state);
    return {
      address,
      token: COLLATERAL_TOKENS.usdl,
      ...view,
      risk: accountRiskFromView(view),
      state,
    };
  }

  async position(positionId: bigint) {
    return positionFromTuple(
      await this.rpc.readContract(calls.buildGetPosition(positionId)),
    );
  }

  async positions(address: Address) {
    const ids = await this.rpc.readContract(
      calls.buildGetUserPositionIds(address),
    );
    const positions = await Promise.all(
      ids.map(async (positionId) => ({
        positionId,
        ...(await this.position(positionId)),
        details: await this.rpc.readContract(
          calls.buildGetPositionDetails(positionId),
        ),
        margin: await this.rpc.readContract(
          calls.buildGetPositionMargin(positionId),
        ),
      })),
    );
    return positions.filter((position) => position.active);
  }

  async order(orderId: bigint) {
    const tuple = await this.rpc.readContract(calls.buildGetOrder(orderId));
    const [
      user,
      marketId,
      isLong,
      orderType,
      triggerPrice,
      limitPrice,
      sizeUsd,
      leverage,
      collateralToken,
      collateralAmount,
      active,
    ] = tuple;
    return {
      user,
      marketId,
      isLong,
      orderType,
      triggerPrice,
      limitPrice,
      sizeUsd,
      leverage,
      collateralToken,
      collateralAmount,
      active,
    };
  }

  async openOrders(address: Address) {
    const ids = await this.rpc.readContract(
      calls.buildGetUserOrderIds(address),
    );
    const orders = await Promise.all(
      ids.map(async (orderId) => ({ orderId, ...(await this.order(orderId)) })),
    );
    return orders.filter((order) => order.active);
  }

  async history(address: Address, limit: number, offset: number) {
    const variables = { userAddress: address.toLowerCase(), limit, offset };
    const [trades, closed] = await Promise.all([
      this.#query(TRADES_QUERY, variables),
      this.#query(CLOSED_POSITIONS_QUERY, variables),
    ]);
    return {
      trades: z
        .object({ trades: z.array(z.record(z.string(), z.unknown())) })
        .parse(trades).trades,
      closedPositions: z
        .object({ positions: z.array(z.record(z.string(), z.unknown())) })
        .parse(closed).positions,
      limit,
      offset,
    };
  }

  async quoteOpen(
    marketId: bigint,
    collateralAmount: bigint,
    leverage: bigint,
    isLong: boolean,
  ) {
    return this.rpc.readContract({
      address: PRECOMPILES.perps,
      abi: PerpsPrecompileABI,
      functionName: "quoteOpenPosition",
      args: [
        marketId,
        COLLATERAL_TOKENS.usdl.address,
        collateralAmount,
        leverage,
        isLong,
      ],
    });
  }

  async quoteClose(positionId: bigint) {
    return this.rpc.readContract({
      address: PRECOMPILES.perps,
      abi: PerpsPrecompileABI,
      functionName: "quoteClosePosition",
      args: [positionId],
    });
  }

  async quotePartialClose(positionId: bigint, sizeUsd: bigint) {
    return this.rpc.readContract({
      address: PRECOMPILES.perps,
      abi: PerpsPrecompileABI,
      functionName: "quotePartialClose",
      args: [positionId, sizeUsd],
    });
  }
}
