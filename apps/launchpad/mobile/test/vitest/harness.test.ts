import { expect, test } from 'vitest';
import { Operations, SdkClient } from '@kindle/sdk';

function mobileHarnessLoadsSdkSource() {
  const config = {
    apiOrigin: 'http://127.0.0.1:3000',
    identityOrigin: 'http://127.0.0.1:3000',
    mediaOrigin: 'http://127.0.0.1:3000',
    streamOrigin: 'ws://127.0.0.1:3000',
    approvedStreamOrigins: ['ws://127.0.0.1:3000'],
    production: false,
  };

  const operations = new Operations(new SdkClient(config));

  expect(operations).toBeInstanceOf(Operations);
  expect(operations.client).toBeInstanceOf(SdkClient);
  expect(operations.client.config).toEqual(config);
}

test(mobileHarnessLoadsSdkSource, mobileHarnessLoadsSdkSource);
