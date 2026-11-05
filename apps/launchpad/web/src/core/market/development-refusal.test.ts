import { expect, test } from 'vitest';
import { CapabilityUnavailableError, Operations, SdkClient } from '@kindle/sdk';
import { dataApiUrl, gatewayUrl, metadataApiUrl } from '@/core/sdk-config';
import { toUnsupported as mapUnsupported } from './unsupported';

const toUnsupported = mapUnsupported;

async function developmentOperationsRenderUnsupportedInProduction() {
  const origins = {
    NEXT_PUBLIC_DATA_API_URL: 'https://data.example.test',
    NEXT_PUBLIC_GATEWAY_URL: 'https://gateway.example.test',
    NEXT_PUBLIC_METADATA_API_URL: 'https://metadata.example.test',
    NEXT_PUBLIC_PNL_API_URL: 'https://pnl.example.test',
  };
  const previous = Object.keys(origins).map(key => [key, process.env[key]] as const);

  try {
    Object.assign(process.env, origins);
    const apiOrigin = new URL(dataApiUrl('/')).origin;
    const streamOrigin = apiOrigin.replace(/^https:/, 'wss:');
    const client = new SdkClient({
      apiOrigin,
      identityOrigin: new URL(gatewayUrl('/')).origin,
      mediaOrigin: new URL(metadataApiUrl('/')).origin,
      streamOrigin,
      approvedStreamOrigins: [streamOrigin],
      production: true,
    });
    const operations = new Operations(client);
    const id = '00000000-0000-4000-8000-000000000001';
    const poolAddress = '0x1111111111111111111111111111111111111111';
    const calls = [
      {
        operation: 'IF_ALERT_CREATE',
        call: () => operations.call('IF_ALERT_CREATE', {
          body: {
            poolAddress,
            direction: 'crosses_above',
            cooldownSeconds: 300,
            threshold: {
              baseAsset: { kind: 'erc20', chainId: '125', address: '0x2222222222222222222222222222222222222222', decimals: 18, currencyCode: null },
              quoteAsset: { kind: 'erc20', chainId: '125', address: '0x3333333333333333333333333333333333333333', decimals: 18, currencyCode: null },
              numerator: '1',
              denominator: '1',
              source: {
                kind: 'external_display',
                amountBasis: 'provider_display',
                observedAt: '2026-01-01T00:00:00.000Z',
                position: null,
                providerId: null,
                sourceEventId: null,
                sourceReference: null,
              },
            },
          },
        }, { headers: { 'Idempotency-Key': id } }),
      },
      { operation: 'IF_ALERT_DELETE', call: () => operations.call('IF_ALERT_DELETE', { path: { alertId: id } }) },
      { operation: 'IF_ORDER_LIST', call: () => operations.call('IF_ORDER_LIST', { query: { status: 'active', limit: 100 } }) },
      { operation: 'IF_ORDER_CANCEL', call: () => operations.call('IF_ORDER_CANCEL', { path: { orderId: id } }, { headers: { 'Idempotency-Key': id } }) },
      { operation: 'IF_CUP_CURRENT', call: () => operations.call('IF_CUP_CURRENT', {}) },
      { operation: 'IF_CUP_GET', call: () => operations.call('IF_CUP_GET', { path: { cupId: id } }) },
      { operation: 'IF_CUP_STANDINGS', call: () => operations.call('IF_CUP_STANDINGS', { path: { cupId: id }, query: { limit: 100 } }) },
      { operation: 'IF_CUP_RESULTS', call: () => operations.call('IF_CUP_RESULTS', { path: { cupId: id } }) },
    ];

    expect(client.config.production).toBe(true);
    expect(calls).toHaveLength(8);
    for (const { operation, call } of calls) {
      const error = await call().then(
        () => { throw new Error(`${operation} unexpectedly succeeded in production`); },
        (cause: unknown) => cause,
      );
      const message = `Operation ${operation} is development-only`;
      expect(error, operation).toBeInstanceOf(CapabilityUnavailableError);
      expect(error, operation).toMatchObject({ status: 501, message });
      expect(toUnsupported(operation, error), operation).toEqual({
        state: 'unsupported',
        operation,
        status: 501,
        message,
      });
    }
  } finally {
    for (const [key, value] of previous) {
      if (value === undefined) delete process.env[key];
      else process.env[key] = value;
    }
  }
}

test(developmentOperationsRenderUnsupportedInProduction);
