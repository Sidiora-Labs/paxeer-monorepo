import { fileURLToPath } from 'node:url';

import config from '../../../../apps/explorer/next.config.js';

config.outputFileTracingRoot = fileURLToPath(new URL('../../../../apps/explorer/', import.meta.url));
