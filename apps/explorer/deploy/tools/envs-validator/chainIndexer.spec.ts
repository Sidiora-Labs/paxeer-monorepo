import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { beforeAll, expect, test } from 'vitest';

let schema: typeof import('./schema').default;

beforeAll(async() => {
  execFileSync('corepack', [ 'yarn', 'install', '--frozen-lockfile', '--ignore-scripts' ], {
    cwd: fileURLToPath(new URL('.', import.meta.url)),
    timeout: 120_000,
  });
  schema = (await import('./schema')).default;
}, 150_000);

const baseEnvs = {
  NEXT_PUBLIC_APP_HOST: 'localhost',
  NEXT_PUBLIC_API_HOST: 'localhost',
  NEXT_PUBLIC_NETWORK_NAME: 'Paxeer',
  NEXT_PUBLIC_NETWORK_ID: '125',
};
const indexerEnvs = {
  NEXT_PUBLIC_PAXEER_X_INDEXER_API_HOST: 'https://example.com',
  NEXT_PUBLIC_PAXEER_X_INDEXER_API_BASE_PATH: '/network/indexer/',
};
const validationOptions = { stripUnknown: false, abortEarly: false };

async function chainIndexerSchemaAcceptsEnabledHostAndBasePath() {
  for (const enabled of [ true, 'true' ]) {
    const validated = await schema.validate({
      ...baseEnvs,
      ...indexerEnvs,
      NEXT_PUBLIC_PAXEER_X_ENABLED: enabled,
    }, validationOptions);

    expect(validated).toMatchObject({ ...indexerEnvs, NEXT_PUBLIC_PAXEER_X_ENABLED: true });
  }
}

async function chainIndexerSchemaRejectsEachSettingWithoutTheFeature() {
  for (const [ name, value ] of Object.entries(indexerEnvs)) {
    for (const enabled of [ undefined, false, 'false' ]) {
      await expect(schema.validate({
        ...baseEnvs,
        [name]: value,
        NEXT_PUBLIC_PAXEER_X_ENABLED: enabled,
      }, validationOptions)).rejects.toMatchObject({
        name: 'ValidationError',
        errors: [ `${ name } can only be used with NEXT_PUBLIC_PAXEER_X_ENABLED` ],
      });
    }
  }
}

async function chainIndexerSchemaRejectsAnInvalidHostUrl() {
  await expect(schema.validate({
    ...baseEnvs,
    ...indexerEnvs,
    NEXT_PUBLIC_PAXEER_X_ENABLED: true,
    NEXT_PUBLIC_PAXEER_X_INDEXER_API_HOST: 'not-a-url',
  }, validationOptions)).rejects.toMatchObject({
    name: 'ValidationError',
    errors: [ 'NEXT_PUBLIC_PAXEER_X_INDEXER_API_HOST is not a valid URL' ],
  });
}

async function chainIndexerSchemaAcceptsTheFeatureWithoutAnIndexer() {
  const validated = await schema.validate({
    ...baseEnvs,
    NEXT_PUBLIC_PAXEER_X_ENABLED: true,
  }, validationOptions);

  expect(validated.NEXT_PUBLIC_PAXEER_X_ENABLED).toBe(true);
  expect(validated).not.toHaveProperty('NEXT_PUBLIC_PAXEER_X_INDEXER_API_HOST');
  expect(validated).not.toHaveProperty('NEXT_PUBLIC_PAXEER_X_INDEXER_API_BASE_PATH');
}

test(chainIndexerSchemaAcceptsEnabledHostAndBasePath.name, chainIndexerSchemaAcceptsEnabledHostAndBasePath);
test(chainIndexerSchemaRejectsEachSettingWithoutTheFeature.name, chainIndexerSchemaRejectsEachSettingWithoutTheFeature);
test(chainIndexerSchemaRejectsAnInvalidHostUrl.name, chainIndexerSchemaRejectsAnInvalidHostUrl);
test(chainIndexerSchemaAcceptsTheFeatureWithoutAnIndexer.name, chainIndexerSchemaAcceptsTheFeatureWithoutAnIndexer);
