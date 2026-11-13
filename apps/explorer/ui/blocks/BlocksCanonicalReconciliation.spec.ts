// @vitest-environment jsdom

import { test } from 'vitest';

import { canonicalHashReplacement, distinctForkHashes, mixedListCanonicalPromotion, nativeBlockRefresh, refreshAtOverloadLimit } from './blocksRefreshTest';

test(nativeBlockRefresh, nativeBlockRefresh, 120_000);
test(canonicalHashReplacement, canonicalHashReplacement, 120_000);
test(distinctForkHashes, distinctForkHashes, 120_000);
test(refreshAtOverloadLimit, refreshAtOverloadLimit, 120_000);
test(mixedListCanonicalPromotion, mixedListCanonicalPromotion, 120_000);
