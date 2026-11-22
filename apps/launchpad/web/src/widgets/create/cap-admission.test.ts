import { createHash } from 'node:crypto';
import { readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import { expect, test } from 'vitest';
import type { ConnectedCapabilitiesPreflight } from '@kindle/sdk-v2';
import {
  KnownCapabilityAdmissionError,
  type CreationAdmissionStage,
} from './cap-admission';
import {
  creationWireDraft,
  creationWireFixture,
  h,
} from './creation-wire.fixture';

type Fixture = Awaited<ReturnType<typeof creationWireFixture>>;
const stages = ['approval', 'creation'] as const;

async function withAdmission(
  stage: CreationAdmissionStage,
  action: (fixture: Fixture, send: () => Promise<string>) => Promise<void>,
) {
  const fixture = await creationWireFixture();
  try {
    const draft = creationWireDraft();
    if (stage === 'creation') {
      const approval = await fixture.adapter.approve('1000000', fixture.scope);
      await fixture.adapter.approvalProof(approval, '1000000', fixture.scope);
    }
    await action(fixture, () =>
      stage === 'approval'
        ? fixture.adapter.approve('1000000', fixture.scope, draft)
        : fixture.adapter.sendCreation(draft, fixture.scope),
    );
  } finally {
    await fixture.close();
  }
}

async function admitted(stage: CreationAdmissionStage) {
  await withAdmission(stage, async (fixture, send) => {
    const before = fixture.peer.transactions.size;
    const tx = await send();
    expect(fixture.peer.transactions.has(tx as `0x${string}`)).toBe(true);
    expect(fixture.peer.transactions.size).toBe(before + 1);
    expect(fixture.peer.preflightResponses).toHaveLength(1);
    expect(fixture.peer.preflightResponses[0].simulationMode).toBe(
      stage === 'approval' ? 'usdl_approval' : 'create_market',
    );
    expect(fixture.peer.preflightResponses[0]).not.toHaveProperty('creationStage');
    expect(fixture.peer.stages.map((entry) => entry.stage)).toEqual([stage]);
  });
}

async function refused(
  stage: CreationAdmissionStage,
  alter: NonNullable<Fixture['peer']['alterPreflight']>,
  message: string,
) {
  await withAdmission(stage, async (fixture, send) => {
    const before = fixture.peer.transactions.size;
    fixture.peer.alterPreflight = alter;
    const error = await send().then(
      () => undefined,
      (error: unknown) => error,
    );
    expect(error).toBeInstanceOf(KnownCapabilityAdmissionError);
    expect(error).toHaveProperty('message', message);
    expect(fixture.peer.preflightResponses).toHaveLength(1);
    expect(fixture.peer.transactions.size).toBe(before);
    const proof = fixture.peer.preflightResponses[0];
    expect(proof.supported).toBe(true);
    expect(proof.executionPermitted).toBe(true);
  });
}

async function createAdmissionAcceptsApproval() {
  await admitted('approval');
}

async function createAdmissionAcceptsCreation() {
  await admitted('creation');
}

async function createAdmissionRefusesCreationModeForApproval() {
  await refused(
    'approval',
    (proof) => ({ ...proof, simulationMode: 'create_market' }),
    'Creation is unavailable under the committed deployment capability.',
  );
}

async function createAdmissionRefusesApprovalModeForCreation() {
  await refused(
    'creation',
    (proof) => ({ ...proof, simulationMode: 'usdl_approval' }),
    'Creation is unavailable under the committed deployment capability.',
  );
}

async function createAdmissionRefusesAnotherRequestDigest() {
  for (const stage of stages)
    await refused(
      stage,
      (proof, requestBody) => {
        const request = JSON.parse(requestBody);
        expect(request.creationStage).toBe(stage);
        expect(proof.requestSha256).toBe(
          createHash('sha256').update(requestBody).digest('hex'),
        );
        request.creationStage = stage === 'approval' ? 'creation' : 'approval';
        return {
          ...proof,
          requestSha256: createHash('sha256')
            .update(JSON.stringify(request))
            .digest('hex'),
        };
      },
      'Creation preflight differs from the reviewed context.',
    );
}

async function createAdmissionRefusesAnotherCommittedBlock() {
  for (const stage of stages)
    for (const change of [{ blockHash: h(99) }, { blockNumber: 99 }])
      await refused(
        stage,
        (proof) => ({ ...proof, ...change }),
        'Creation preflight differs from the reviewed context.',
      );
}

async function createAdmissionAnswersUseDeclaredSchema() {
  const require = createRequire(import.meta.url);
  const schemas = JSON.parse(
    readFileSync(require.resolve('@kindle/sdk-v2/schemas'), 'utf8'),
  ).$defs;
  const schema = schemas.ConnectedCapabilitiesPreflight;
  function assertMembers(name: string, value: object) {
    const entry = schemas[name];
    expect(entry.additionalProperties).toBe(false);
    for (const key of Object.keys(value))
      expect(Object.keys(entry.properties)).toContain(key);
    for (const key of entry.required) expect(value).toHaveProperty(key);
  }
  expect(schema.required).toContain('capability');
  for (const stage of stages)
    await withAdmission(stage, async (fixture, send) => {
      await send();
      expect(fixture.peer.preflightResponses).toHaveLength(1);
      const proof: ConnectedCapabilitiesPreflight =
        fixture.peer.preflightResponses[0];
      assertMembers('ConnectedCapabilitiesPreflight', proof);
      assertMembers('ConnectedCapabilitiesFamilyCapability', proof.capability);
      expect(proof.creationCost).toBeDefined();
      assertMembers('ConnectedCapabilitiesCreationCost', proof.creationCost!);
      assertMembers('ConnectedDeploymentRevision', proof.revision);
    });
}

test(
  'create admission accepts the declared approval answer',
  createAdmissionAcceptsApproval,
);
test(
  'create admission accepts the declared creation answer',
  createAdmissionAcceptsCreation,
);
test(
  'create admission refuses creation mode for approval',
  createAdmissionRefusesCreationModeForApproval,
);
test(
  'create admission refuses approval mode for creation',
  createAdmissionRefusesApprovalModeForCreation,
);
test(
  'create admission refuses another request digest',
  createAdmissionRefusesAnotherRequestDigest,
);
test(
  'create admission refuses another committed block',
  createAdmissionRefusesAnotherCommittedBlock,
);
test(
  'create admission answers use only declared members and contain every required member',
  createAdmissionAnswersUseDeclaredSchema,
);
