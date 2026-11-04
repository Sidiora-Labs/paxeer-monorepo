import { fileURLToPath } from 'node:url';
import ts from 'typescript';
import { expect, test } from 'vitest';

import schema from './schema';

const baseEnvs = {
  NEXT_PUBLIC_APP_HOST: 'localhost',
  NEXT_PUBLIC_API_HOST: 'localhost',
  NEXT_PUBLIC_NETWORK_NAME: 'Paxeer',
  NEXT_PUBLIC_NETWORK_ID: '125',
};
const validationOptions = { stripUnknown: false, abortEarly: false };

function envSchemaDuplicateKeyCompilesValidatorTarget() {
  const configPath = fileURLToPath(new URL('./tsconfig.json', import.meta.url));
  const config = ts.getParsedCommandLineOfConfigFile(configPath, {}, {
    ...ts.sys,
    onUnRecoverableConfigFileDiagnostic(diagnostic) {
      throw new Error(ts.flattenDiagnosticMessageText(diagnostic.messageText, '\n'));
    },
  });
  expect(config).toBeDefined();
  if (!config) {
    throw new Error('Validator TypeScript configuration could not be loaded');
  }

  const program = ts.createProgram(config.fileNames, config.options);
  const diagnostics = [ ...config.errors, ...ts.getPreEmitDiagnostics(program) ];
  const formattedDiagnostics = ts.formatDiagnosticsWithColorAndContext(diagnostics, {
    getCanonicalFileName: name => name,
    getCurrentDirectory: ts.sys.getCurrentDirectory,
    getNewLine: () => '\n',
  });
  expect(diagnostics, formattedDiagnostics).toEqual([]);
}

async function envSchemaDuplicateKeyPreservesAcceptedBooleans() {
  const cases = [
    { input: true, expected: true },
    { input: false, expected: false },
    { input: 'true', expected: true },
    { input: 'false', expected: false },
    { input: 'TRUE', expected: true },
    { input: 'FALSE', expected: false },
    { input: '1', expected: true },
    { input: '0', expected: false },
    { input: 1, expected: true },
    { input: 0, expected: false },
  ];

  for (const { input, expected } of cases) {
    const validated = await schema.validate({ ...baseEnvs, NEXT_PUBLIC_PAXEER_X_ENABLED: input }, validationOptions);
    expect(validated.NEXT_PUBLIC_PAXEER_X_ENABLED).toBe(expected);
    expect(validated.NEXT_PUBLIC_NETWORK_ID).toBe(125);
  }

  const validated = await schema.validate(baseEnvs, validationOptions);
  expect(validated).not.toHaveProperty('NEXT_PUBLIC_PAXEER_X_ENABLED');
}

async function envSchemaDuplicateKeyRejectsInvalidValues() {
  for (const input of [ '', 'yes', 'no', 'enabled', '2', 2, -1, null, {}, [] ]) {
    await expect(schema.validate({ ...baseEnvs, NEXT_PUBLIC_PAXEER_X_ENABLED: input }, validationOptions))
      .rejects.toMatchObject({ name: 'ValidationError', errors: [ expect.stringContaining('NEXT_PUBLIC_PAXEER_X_ENABLED') ] });
  }

  await expect(schema.validate({ ...baseEnvs, NEXT_PUBLIC_PAXEER_X_ENABLE: 'true' }, validationOptions))
    .rejects.toMatchObject({ name: 'ValidationError', errors: [ expect.stringContaining('Unknown ENV variables') ] });
}

test(envSchemaDuplicateKeyCompilesValidatorTarget, 30_000);
test(envSchemaDuplicateKeyPreservesAcceptedBooleans);
test(envSchemaDuplicateKeyRejectsInvalidValues);
