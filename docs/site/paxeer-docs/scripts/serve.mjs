import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { serve } from '../../docs-assistant/server/index.mjs';
await serve({ root: resolve(dirname(fileURLToPath(import.meta.url)), '..'), kind: 'static' });
