import { registerHooks } from 'node:module';
import { existsSync } from 'node:fs';
import { readFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import { loopbackAgent } from './tls-agent.ts';

// Native Node type stripping runs the production sources without a compiler.
registerHooks({
  resolve(specifier, context, nextResolve) {
    try {
      return nextResolve(specifier, context);
    } catch (error) {
      if (error.code !== 'ERR_MODULE_NOT_FOUND' || !specifier.startsWith('.')) throw error;
      return nextResolve(`${specifier}.ts`, context);
    }
  },
});
const { AtomicJsonStore } = await import('../../atomic-json-store.ts');
const [mode, filePath, parameter] = process.argv.slice(2);
const send = (message) => process.send?.(message);
const command = () => new Promise((resolve) => process.once('message', resolve));
function parse(input) {
  if (
    input?.version !== 1 ||
    !Array.isArray(input.records) ||
    !input.records.every((item) => typeof item === 'string')
  )
    throw new TypeError('Invalid records');
  return { version: 1, records: input.records };
}
const store = new AtomicJsonStore({
  filePath,
  empty: () => ({ version: 1, records: [] }),
  parse,
  migrate(input) {
    if (!Array.isArray(input) || !input.every((item) => typeof item === 'string'))
      throw new TypeError('Invalid legacy records');
    if (mode === 'migrate') {
      send('migration-entered');
      const deadline = Date.now() + 8_000;
      while (!existsSync(parameter)) {
        if (Date.now() >= deadline) throw new Error('Migration barrier timed out');
        Atomics.wait(new Int32Array(new SharedArrayBuffer(4)), 0, 0, 10);
      }
    }
    return { version: 1, records: input };
  },
  async onPublication(phase) {
    if (mode === 'publish' && phase === parameter) {
      send(phase);
      await command();
    }
  },
});
try {
  if (mode === 'migrate') await store.read();
  else if (mode === 'hold') {
    await store.update(async (current) => {
      send('held');
      await command();
      return { ...current, records: [...current.records, parameter] };
    });
  } else if (mode === 'insert' || mode === 'publish') {
    await store.update((current) => ({
      ...current,
      records: [...current.records, mode === 'publish' ? 'C' : parameter],
    }));
  } else if (mode === 'campaign') {
    const { createPushStore } = await import('../../push-store.ts');
    const { createPushService } = await import('../../push-service.ts');
    const options = JSON.parse(parameter);
    options.ca = await readFile(fileURLToPath(new URL('./tls-cert.pem', import.meta.url)), 'utf8');
    options.createAgent = (agentOptions) => loopbackAgent(options.loopbackPort, agentOptions);
    const service = createPushService(createPushStore(filePath), options);
    send('ready');
    await command();
    await service.processPendingCampaigns();
  } else throw new Error('Unknown worker mode');
  send('done');
  process.disconnect();
} catch (error) {
  send({ error: error.message });
  process.exitCode = 1;
  process.disconnect();
}
