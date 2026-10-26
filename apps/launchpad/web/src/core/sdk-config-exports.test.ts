import { expect, test } from 'vitest';
import * as sdkConfig from '@/core/sdk-config';

function sdkConfigExportsNoMediaApiUrl() {
  expect('mediaApiUrl' in sdkConfig).toBe(false);

  const gatewayOrigin = process.env.NEXT_PUBLIC_GATEWAY_URL;
  expect(gatewayOrigin).toBeTruthy();
  expect(sdkConfig.gatewayUrl('/v2/x')).toBe(`${gatewayOrigin!.replace(/\/+$/, '')}/v2/x`);
}

test(sdkConfigExportsNoMediaApiUrl);
