import { test } from 'vitest';
import {
  mobileCachePositiveProducerRestore,
  mobileCacheMalformedPrivateRefusal,
} from '../support/offline-cache-proof.ts';

async function mobileCachePositiveProof() {
  await mobileCachePositiveProducerRestore();
}

async function mobileCacheRefusalProof() {
  await mobileCacheMalformedPrivateRefusal();
}

test(mobileCachePositiveProof, mobileCachePositiveProof);
test(mobileCacheRefusalProof, mobileCacheRefusalProof);
