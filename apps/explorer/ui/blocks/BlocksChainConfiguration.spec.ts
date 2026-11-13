// @vitest-environment jsdom

import { test } from 'vitest';

import {
  mobileBlockChainLink,
  blockTableConfiguration,
  desktopBlockValueScale,
  desktopBlockChainLink,
  selectedRollupBlockConfiguration,
} from './blocksChainTest';

test(mobileBlockChainLink, mobileBlockChainLink);
test(blockTableConfiguration, blockTableConfiguration);
test(desktopBlockValueScale, desktopBlockValueScale);
test(desktopBlockChainLink, desktopBlockChainLink);
test(selectedRollupBlockConfiguration, selectedRollupBlockConfiguration);
