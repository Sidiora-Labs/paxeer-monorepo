import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';

import config from '../../../../apps/explorer/next.config.js';

import { cacheTraceReadlinks } from './trace-readlinks.mjs';

const require = createRequire(new URL('../../../../apps/explorer/package.json', import.meta.url));
const tracerPath = require.resolve('next/dist/compiled/@vercel/nft');
const tracer = require(tracerPath);
require.cache[tracerPath].exports = cacheTraceReadlinks(tracer);

config.outputFileTracingRoot = fileURLToPath(new URL('../../../../apps/explorer/', import.meta.url));
config.output = undefined;
config.bundlePagesRouterDependencies = true;
config.serverExternalPackages = [...config.serverExternalPackages, 'next-themes'];
const walletPackages = ['@dynamic-labs/sdk-react-core', '@dynamic-labs/ethereum', '@dynamic-labs/wagmi-connector'];
config.experimental.optimizePackageImports = [
  ...(config.experimental.optimizePackageImports ?? []), '@chakra-ui/react', '@ark-ui/react', ...walletPackages,
];
config.transpilePackages = [...config.transpilePackages, '@ark-ui/react'];
