import { fileURLToPath } from 'node:url';

import config from '../../../../apps/explorer/next.config.js';

config.outputFileTracingRoot = fileURLToPath(new URL('../../../../apps/explorer/', import.meta.url));
config.output = undefined;
config.experimental.optimizePackageImports = [...(config.experimental.optimizePackageImports ?? []), '@chakra-ui/react', '@ark-ui/react'];
config.transpilePackages = [...config.transpilePackages, '@ark-ui/react'];
