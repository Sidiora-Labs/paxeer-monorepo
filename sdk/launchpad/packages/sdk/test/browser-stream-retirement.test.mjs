import { test } from 'node:test';
import assert from 'node:assert/strict';
import * as sdk from '../dist/index.js';

function browserStreamV1TransportRetired() {
  assert.equal(typeof sdk.browserStreamTransportV2, 'function');
  assert.equal(Object.hasOwn(sdk, 'browserStreamTransport'), false);
}

test(browserStreamV1TransportRetired);
