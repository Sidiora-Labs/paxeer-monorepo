import {
  Operations,
  type SdkClient,
} from '../../../../../../sdk/launchpad/packages/sdk/src/index.ts';
import type { QuoteBody } from '../../../../../../sdk/launchpad/packages/sdk/src/types.gen.ts';
import { createPublicClient, defineChain, getAddress, http } from 'viem';
import type { PublicConfiguration } from '../service-client.ts';
import { onchainQuote } from '../onchain-quote.ts';

export function streams(sdk: SdkClient, signal: AbortSignal) {
  return new Operations(sdk).call(
    'IF_STREAM_LIST',
    { query: { status: 'live', limit: 50 } },
    { signal },
  );
}

export function quote(
  services: {
    config: SdkClient['config'] & Pick<PublicConfiguration, 'walletConnect'>;
  },
  body: QuoteBody,
  signal: AbortSignal,
) {
  const configuration = services.config.walletConnect;
  if (!configuration) throw new Error('Chain125 native RPC authority required');
  const chain = defineChain({
    id: 125,
    name: 'Paxeer',
    nativeCurrency: { name: 'PAX', symbol: 'PAX', decimals: 18 },
    rpcUrls: { default: { http: [configuration.rpcURL] } },
  });
  const client = createPublicClient({
    chain,
    transport: http(configuration.rpcURL, { retryCount: 0, timeout: 15000 }),
  });
  return onchainQuote(
    client,
    getAddress('0x793006f4F321b8863F26DBD75b11bAa01DA181dc'),
    {
      pool: getAddress(body.poolAddress),
      side: body.side,
      inputRaw: body.inputRaw,
      slippageBps: body.slippageBps,
    },
    signal,
  );
}

export const IF_TX_STATUS = 'IF_TX_STATUS';

export function receipt(sdk: SdkClient, hash: string, signal: AbortSignal) {
  if (!/^0x[0-9a-fA-F]{64}$/.test(hash))
    throw new Error('Invalid transaction hash');
  return new Operations(sdk).call(
    IF_TX_STATUS,
    { path: { transactionHash: hash } },
    { signal },
  );
}

export const activityRoutes = { streams, quote, receipt };
