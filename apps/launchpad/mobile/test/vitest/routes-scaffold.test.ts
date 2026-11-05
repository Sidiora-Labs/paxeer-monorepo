import { test } from 'vitest';
import { routeModulesPreserveHeadPaths } from './routes-scaffold-proof.ts';
import { routeQuoteUsesChain125Interface } from './routes-quote-proof.ts';

test(routeModulesPreserveHeadPaths, routeModulesPreserveHeadPaths);
test(routeQuoteUsesChain125Interface, routeQuoteUsesChain125Interface);
