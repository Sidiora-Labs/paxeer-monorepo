import { registerHooks } from 'node:module';
import { pathToFileURL } from 'node:url';
import { readFileSync } from 'node:fs';

const source = pathToFileURL(`${process.argv[2]}/apps/bridge/src/lib/bridge/`);
registerHooks({
  resolve(specifier, context, nextResolve) {
    if (context.parentURL?.startsWith(source.href) && (specifier === 'viem' || specifier.startsWith('viem/') || specifier.startsWith('@noble/'))) {
      return nextResolve(specifier, { ...context, parentURL: import.meta.url });
    }
    if (context.parentURL?.startsWith(source.href) && specifier.startsWith('./') && !specifier.endsWith('.ts')) {
      return nextResolve(`${specifier}.ts`, context);
    }
    return nextResolve(specifier, context);
  },
});

const { readConfig } = await import(new URL('config.ts', source));
const { buildBridgeChains } = await import(new URL('chains.ts', source));
const environments = JSON.parse(readFileSync(0, 'utf8'));
const results = environments.map((environment) => {
  const config = readConfig(environment);
  const chain = buildBridgeChains(config).find((chain) => chain.key === 'solana');
  return { program: config.solanaProgramId ?? '', state: chain.state };
});
process.stdout.write(JSON.stringify(results));
