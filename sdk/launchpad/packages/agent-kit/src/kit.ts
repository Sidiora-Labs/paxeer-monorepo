import { Operations, SdkClient } from '@kindle/sdk';
import type { OperationInput, OperationResponse, TransportConfig } from '@kindle/sdk';
import {
  decodeFunctionResult,
  encodeFunctionData,
  erc20Abi,
  getAddress,
  zeroAddress,
  type Address,
  type Hex,
} from 'viem';
import { randomUUID } from 'node:crypto';
import {
  AgentGatewayClient,
  type GatewayConfig,
  type GatewayIdentity,
  type GatewayTransaction,
} from './gateway.js';
import { AgentKitError, requireThat } from './errors.js';
import {
  DEPLOYMENT,
  EXECUTION_CALLS,
  EVM_POOL_ABI,
  checkedDeployment,
  type LaunchpadDeployment,
  type EvmDeployment,
  HOME_CHAIN_ABI,
  QUOTE_TTL_MS,
  SIMULATION_TTL_MS,
  TRADE_DEADLINE_SECONDS,
  VIRTUAL_CHAIN_ID_BASE,
  protocolConfigAbi,
  quoterAbi,
  routerAbi,
} from './contracts.js';
import { launchpadPreset, type PresetCaps, rawAmount } from './preset.js';

export interface AgentKitConfig extends GatewayConfig {
  dataUrl: string;
  production?: boolean;
  evmDeployment?: EvmDeployment;
}

export interface QuoteInput {
  pool: string;
  side: 'buy' | 'sell';
  amountInRaw: string;
}

export interface TradeInput {
  pool: string;
  quoteId: string;
  amountInRaw: string;
  maxSlippageBps: number;
  idempotencyKey: string;
  acceptHighSlippage?: boolean;
  acceptHighImpact?: boolean;
}

export interface LaunchInput {
  name: string;
  symbol: string;
  feeStrategy: number;
  idempotencyKey: string;
}

export interface Quote extends QuoteInput {
  quoteId: string;
  wallet: Address;
  token: Address;
  outputRaw: string;
  feeRaw: string;
  priceImpactBps: number;
  quotedAt: number;
  expiresAt: number;
  chainId: number;
}

type CoinStats = OperationResponse<'IF_POOL_DETAIL'>['data']['stats'];
export type CoinResponse = OperationResponse<'IF_POOL_DETAIL'> & {
  price: NonNullable<CoinStats['spotPriceUsdlRaw']> | null;
  marketValueUsdlRaw: CoinStats['circulatingMarketCapUsdlRaw'];
  totalSupplyValuationUsdlRaw: CoinStats['totalSupplyValuationUsdlRaw'];
  progress: null;
  progressStatus: 'unavailable';
};

export function dataTransport(dataUrl: string, production = true): TransportConfig {
  const url = new URL(dataUrl);
  const streamOrigin = new URL(url.origin);
  streamOrigin.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:';
  return {
    apiOrigin: dataUrl,
    identityOrigin: dataUrl,
    mediaOrigin: dataUrl,
    streamOrigin: streamOrigin.origin,
    approvedStreamOrigins: [streamOrigin.origin],
    production,
  };
}

function idempotencyKey(key: string): void {
  requireThat(
    typeof key === 'string' && /^[A-Za-z0-9_.:-]{8,128}$/.test(key),
    'invalid_idempotency_key',
  );
}

export class LaunchpadAgentKit {
  readonly gateway: AgentGatewayClient;
  readonly deployment: Readonly<LaunchpadDeployment>;
  readonly #poolRegistry: Address | undefined;
  readonly operations: Operations;
  readonly #now: () => number;
  readonly #quotes = new Map<string, Readonly<Quote>>();
  readonly #submissions = new Map<string, string>();

  constructor(config: AgentKitConfig) {
    this.deployment = checkedDeployment(config.evmDeployment ?? DEPLOYMENT);
    this.#poolRegistry = config.evmDeployment
      ? getAddress(config.evmDeployment.poolRegistry)
      : undefined;
    if (this.#poolRegistry) requireThat(BigInt(this.#poolRegistry) !== 0n, 'invalid_pool_registry');
    this.gateway = new AgentGatewayClient({ ...config, chainId: this.deployment.chainId });
    this.operations = new Operations(
      new SdkClient(dataTransport(config.dataUrl, config.production), config.fetch),
    );
    this.#now = config.now ?? Date.now;
  }

  preset(caps: PresetCaps) {
    return launchpadPreset(caps, this.deployment);
  }

  listCoins(
    query: OperationInput<'IF_POOL_LIST'>['query'],
  ): Promise<OperationResponse<'IF_POOL_LIST'>> {
    return this.operations.call('IF_POOL_LIST', { query });
  }

  searchCoins(query: { q: string; limit?: number }): Promise<OperationResponse<'IF_SEARCH'>> {
    return this.operations.call('IF_SEARCH', { query: { ...query, type: 'pool' } });
  }

  async coin(pool: string): Promise<CoinResponse> {
    const response = await this.#pool(pool);
    return {
      ...response,
      price: response.data.stats.spotPriceUsdlRaw ?? null,
      marketValueUsdlRaw: response.data.stats.circulatingMarketCapUsdlRaw,
      totalSupplyValuationUsdlRaw: response.data.stats.totalSupplyValuationUsdlRaw,
      progress: null,
      progressStatus: 'unavailable' as const,
    };
  }

  trades(
    pool: string,
    query: OperationInput<'IF_TRADES'>['query'] = {},
  ): Promise<OperationResponse<'IF_TRADES'>> {
    return this.operations.call('IF_TRADES', {
      path: { poolAddress: getAddress(pool).toLowerCase() },
      query,
    });
  }

  candles(
    pool: string,
    query: OperationInput<'IF_CANDLES'>['query'],
  ): Promise<OperationResponse<'IF_CANDLES'>> {
    return this.operations.call('IF_CANDLES', {
      path: { poolAddress: getAddress(pool).toLowerCase() },
      query,
    });
  }

  holders(
    pool: string,
    query: OperationInput<'IF_HOLDERS'>['query'] = {},
  ): Promise<OperationResponse<'IF_HOLDERS'>> {
    return this.operations.call('IF_HOLDERS', {
      path: { poolAddress: getAddress(pool).toLowerCase() },
      query,
    });
  }

  balances(tokens: readonly string[] = []): Promise<unknown> {
    return this.gateway.balances(tokens);
  }

  async positions(
    query: OperationInput<'IF_CONNECTED_PUBLIC_PORTFOLIO_V3'>['query'] = {},
  ): Promise<OperationResponse<'IF_CONNECTED_PUBLIC_PORTFOLIO_V3'>> {
    const identity = await this.#identity();
    requireThat(identity.wallet, 'agent_wallet_required');
    return this.operations.call('IF_CONNECTED_PUBLIC_PORTFOLIO_V3', {
      path: { wallet: identity.wallet.address.toLowerCase() },
      query,
    });
  }

  async #pool(pool: string): Promise<OperationResponse<'IF_POOL_DETAIL'>> {
    const address = getAddress(pool).toLowerCase();
    const response = await this.operations.call('IF_POOL_DETAIL', {
      path: { poolAddress: address },
    });
    requireThat(
      response.data.pool.poolAddress.toLowerCase() === address &&
        response.data.stats.poolAddress.toLowerCase() === address &&
        response.data.token.address.toLowerCase() === response.data.pool.tokenAddress.toLowerCase(),
      'pool_identity_mismatch',
    );
    return response;
  }

  async #identity(): Promise<GatewayIdentity> {
    const identity = await this.gateway.identity();
    requireThat(identity.chain.id === this.deployment.chainId, 'unsupported_gateway_chain');
    requireThat(identity.wallet, 'agent_wallet_required');
    return identity;
  }

  #tx(to: Address, data: Hex): GatewayTransaction {
    return { to, data, value: '0', chainId: this.deployment.chainId };
  }

  async #read(to: Address, data: Hex): Promise<Hex> {
    const result = await this.gateway.simulate(this.#tx(to, data));
    requireThat(
      result.return_data !== null && /^0x(?:[0-9a-fA-F]{2})+$/.test(result.return_data),
      'missing_simulation_return_data',
    );
    return result.return_data;
  }

  async #tradeContext(pool: string): Promise<{ identity: GatewayIdentity; token: Address }> {
    const response = await this.#pool(pool);
    requireThat(response.quality === 'complete', 'pool_data_not_fresh');
    const value = response.data.pool;
    requireThat(
      /^(0|[1-9][0-9]{0,9})$/.test(value.homeChain) && BigInt(value.homeChain) <= 0xffffffffn,
      'invalid_pool_home_chain',
    );
    const home =
      value.homeChain === '0'
        ? this.deployment.chainId
        : VIRTUAL_CHAIN_ID_BASE + Number(value.homeChain);
    if (home !== this.deployment.chainId)
      throw new AgentKitError(
        'foreign_home_chain',
        `Pool ${value.poolAddress} belongs to home chain ${home}; the gateway uses chain ${this.deployment.chainId}.`,
      );
    requireThat(
      value.quoteAssetAddress.toLowerCase() === this.deployment.usdl.toLowerCase(),
      'unsupported_quote_asset',
    );
    const identity = await this.#identity();
    if (this.#poolRegistry) {
      const registered = decodeFunctionResult({
        abi: EVM_POOL_ABI,
        functionName: 'isRegisteredPool',
        data: await this.#read(
          this.#poolRegistry,
          encodeFunctionData({
            abi: EVM_POOL_ABI,
            functionName: 'isRegisteredPool',
            args: [getAddress(pool)],
          }),
        ),
      });
      requireThat(registered, 'unregistered_pool');
      const onChainToken = decodeFunctionResult({
        abi: EVM_POOL_ABI,
        functionName: 'tokenAddress',
        data: await this.#read(
          getAddress(pool),
          encodeFunctionData({
            abi: EVM_POOL_ABI,
            functionName: 'tokenAddress',
          }),
        ),
      });
      requireThat(
        getAddress(onChainToken) === getAddress(value.tokenAddress),
        'pool_identity_mismatch',
      );
    } else {
      const onChainHome = decodeFunctionResult({
        abi: HOME_CHAIN_ABI,
        functionName: 'homeChainOf',
        data: await this.#read(
          this.deployment.spender,
          encodeFunctionData({
            abi: HOME_CHAIN_ABI,
            functionName: 'homeChainOf',
            args: [getAddress(pool)],
          }),
        ),
      });
      if (onChainHome !== 0n && onChainHome !== BigInt(identity.chain.id))
        throw new AgentKitError(
          'foreign_home_chain',
          `Pool ${value.poolAddress} belongs to home chain ${onChainHome}; the gateway uses chain ${identity.chain.id}.`,
        );
    }
    return { identity, token: getAddress(value.tokenAddress) };
  }

  async quote(input: QuoteInput): Promise<Readonly<Quote>> {
    rawAmount(input.amountInRaw);
    requireThat(input.side === 'buy' || input.side === 'sell', 'invalid_side');
    const pool = getAddress(input.pool);
    const { identity, token } = await this.#tradeContext(pool);
    const quotedAt = this.#now();
    const data = await this.#read(
      this.deployment.quoter,
      encodeFunctionData({
        abi: quoterAbi,
        functionName: 'quoteExactInput',
        args: [pool, rawAmount(input.amountInRaw), input.side === 'buy'],
      }),
    );
    const output = decodeFunctionResult({ abi: quoterAbi, functionName: 'quoteExactInput', data });
    requireThat(
      output.amountOut > 0n && output.priceImpactBps <= BigInt(Number.MAX_SAFE_INTEGER),
      'invalid_quote_output',
    );
    requireThat(this.#now() < quotedAt + QUOTE_TTL_MS, 'quote_expired');
    const quote = Object.freeze({
      ...input,
      pool,
      quoteId: randomUUID(),
      wallet: getAddress(identity.wallet!.address),
      token,
      outputRaw: output.amountOut.toString(),
      feeRaw: output.feeAmount.toString(),
      priceImpactBps: Number(output.priceImpactBps),
      quotedAt,
      expiresAt: quotedAt + QUOTE_TTL_MS,
      chainId: identity.chain.id,
    });
    for (const [key, value] of this.#quotes)
      if (value.expiresAt <= this.#now()) this.#quotes.delete(key);
    requireThat(this.#quotes.size < 1000, 'quote_capacity_reached');
    this.#quotes.set(quote.quoteId, quote);
    return quote;
  }

  #fresh(quote: Readonly<Quote>): void {
    requireThat(this.#now() >= quote.quotedAt && this.#now() < quote.expiresAt, 'quote_expired');
  }

  buy(input: TradeInput): Promise<unknown> {
    return this.#trade('buy', input);
  }
  sell(input: TradeInput): Promise<unknown> {
    return this.#trade('sell', input);
  }

  async #trade(side: 'buy' | 'sell', input: TradeInput): Promise<unknown> {
    idempotencyKey(input.idempotencyKey);
    requireThat(
      Number.isInteger(input.maxSlippageBps) &&
        input.maxSlippageBps >= 1 &&
        input.maxSlippageBps <= 5000,
      'invalid_slippage',
    );
    const quote = this.#quotes.get(input.quoteId);
    requireThat(quote, 'fresh_quote_required');
    this.#fresh(quote);
    requireThat(
      quote.side === side &&
        quote.pool.toLowerCase() === getAddress(input.pool).toLowerCase() &&
        quote.amountInRaw === input.amountInRaw,
      'quote_context_mismatch',
    );
    requireThat(
      input.maxSlippageBps <= 500 || input.acceptHighSlippage === true,
      'slippage_consent_required',
    );
    requireThat(
      quote.priceImpactBps <= 1000 || input.acceptHighImpact === true,
      'impact_consent_required',
    );
    const { identity, token } = await this.#tradeContext(input.pool);
    requireThat(
      getAddress(identity.wallet!.address) === quote.wallet &&
        token === quote.token &&
        identity.chain.id === quote.chainId,
      'quote_context_mismatch',
    );
    const minimum = (rawAmount(quote.outputRaw) * BigInt(10000 - input.maxSlippageBps)) / 10000n;
    requireThat(minimum > 0n, 'zero_minimum');
    const deadline = BigInt(Math.floor(this.#now() / 1000) + TRADE_DEADLINE_SECONDS);
    const args = [getAddress(input.pool), rawAmount(input.amountInRaw), minimum, deadline] as const;
    const tx = this.#tx(
      this.deployment.router,
      encodeFunctionData({ abi: routerAbi, functionName: side, args }),
    );
    this.#fresh(quote);
    return this.#execute({
      token: side === 'buy' ? this.deployment.usdl : token,
      amount: input.amountInRaw,
      wallet: quote.wallet,
      tx,
      method: EXECUTION_CALLS[side].method,
      args: args.map(String),
      idempotencyKey: input.idempotencyKey,
      beforeSend: () => {
        this.#fresh(quote);
        requireThat(this.#quotes.has(quote.quoteId), 'quote_already_submitted');
        this.#quotes.delete(quote.quoteId);
      },
    });
  }

  async launch(input: LaunchInput): Promise<unknown> {
    idempotencyKey(input.idempotencyKey);
    requireThat(
      typeof input.name === 'string' &&
        input.name.length >= 1 &&
        input.name.length <= 64 &&
        typeof input.symbol === 'string' &&
        input.symbol.length >= 1 &&
        input.symbol.length <= 16 &&
        !/[\p{Cc}\p{Bidi_Control}\u206a-\u206f]/u.test(input.name + input.symbol),
      'invalid_coin_text',
    );
    requireThat(
      Number.isInteger(input.feeStrategy) && input.feeStrategy >= 0 && input.feeStrategy <= 3,
      'invalid_fee_strategy',
    );
    const identity = await this.#identity();
    const fee = decodeFunctionResult({
      abi: protocolConfigAbi,
      functionName: 'creationFee',
      data: await this.#read(
        this.deployment.protocolConfig,
        encodeFunctionData({ abi: protocolConfigAbi, functionName: 'creationFee' }),
      ),
    });
    requireThat(fee.toString() === this.deployment.creationFeeRaw, 'creation_fee_changed');
    const args = [input.name, input.symbol, input.feeStrategy, zeroAddress] as const;
    const tx = this.#tx(
      this.deployment.router,
      encodeFunctionData({ abi: routerAbi, functionName: 'createMarket', args }),
    );
    return this.#execute({
      token: this.deployment.usdl,
      amount: fee.toString(),
      wallet: getAddress(identity.wallet!.address),
      exactAllowance: true,
      tx,
      method: EXECUTION_CALLS.launch.method,
      args: args.map(String),
      idempotencyKey: input.idempotencyKey,
      beforeSend: () => {},
    });
  }

  async #execute(input: {
    token: Address;
    amount: string;
    wallet: Address;
    exactAllowance?: boolean;
    tx: GatewayTransaction;
    method: string;
    args: string[];
    idempotencyKey: string;
    beforeSend: () => void;
  }): Promise<unknown> {
    requireThat(!this.#submissions.has(input.idempotencyKey), 'submission_already_started');
    const allowance = await this.gateway.allowance(input.token, this.deployment.spender);
    const needsApproval = input.exactAllowance
      ? allowance !== rawAmount(input.amount)
      : allowance < rawAmount(input.amount);
    const simulationStarted = this.#now();
    const tx = needsApproval
      ? this.#tx(
          input.token,
          encodeFunctionData({
            abi: erc20Abi,
            functionName: 'approve',
            args: [this.deployment.spender, rawAmount(input.amount)],
          }),
        )
      : input.tx;
    await this.gateway.simulate(tx);
    requireThat(
      this.#now() >= simulationStarted && this.#now() < simulationStarted + SIMULATION_TTL_MS,
      'simulation_expired',
    );
    const identity = await this.#identity();
    requireThat(
      identity.wallet && getAddress(identity.wallet.address) === input.wallet,
      'agent_wallet_changed',
    );
    requireThat(this.#now() < simulationStarted + SIMULATION_TTL_MS, 'simulation_expired');
    requireThat(!this.#submissions.has(input.idempotencyKey), 'submission_already_started');
    input.beforeSend();
    requireThat(this.#submissions.size < 10000, 'submission_capacity_reached');
    this.#submissions.set(input.idempotencyKey, input.tx.data);
    if (needsApproval) {
      return {
        submission: 'action',
        simulationPhase: 'approval',
        result: await this.gateway.allowanceAndCall({
          token: input.token,
          amount: input.amount,
          spender: this.deployment.spender,
          contract: input.tx.to,
          method: input.method,
          args: input.args,
          idempotency_key: input.idempotencyKey,
        }),
      };
    }
    return {
      submission: 'transaction',
      simulationPhase: 'call',
      result: await this.gateway.send(input.tx),
    };
  }
}
