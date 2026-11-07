import { expect, test } from 'vitest';
import { SignTypedDataBody } from '../src/routes/sign.js';
import { AgentSignTypedDataBody } from '../src/schemas/agent.js';

function gatewayTypedDataRecordsZod4() {
  const document = {
    domain: { name: 'Gateway', version: '1', chainId: 125 },
    types: {
      Mail: [
        { name: 'contents', type: 'string' },
        { name: 'amount', type: 'uint256' },
      ],
      Person: [{ name: 'name', type: 'string' }],
    },
    primaryType: 'Mail',
    message: {
      contents: 'Keep this message',
      amount: '42',
      enabled: true,
      recipient: { name: 'Alice' },
      attachments: ['first', 'second'],
      optional: null,
    },
  };

  const agent = AgentSignTypedDataBody.safeParse({ typedData: document });
  expect(agent.success).toBe(true);
  expect(agent.data).toEqual({ typedData: document });

  const { domain, ...withoutDomain } = document;
  const agentWithoutDomain = AgentSignTypedDataBody.safeParse({ typedData: withoutDomain });
  expect(agentWithoutDomain.success).toBe(true);
  expect(agentWithoutDomain.data).toEqual({ typedData: withoutDomain });

  const agentDocument = {
    ...document,
    domain: { ...domain, extension: { enabled: true }, optional: null },
    types: { ...document.types, extension: { nested: true }, optional: null, scalar: 7 },
    extraProperty: { retained: ['agent', 'extension'] },
  };
  const agentWithExtras = AgentSignTypedDataBody.safeParse({ typedData: agentDocument });
  expect(agentWithExtras.success).toBe(true);
  expect(agentWithExtras.data).toEqual({ typedData: agentDocument });
  expect(AgentSignTypedDataBody.safeParse({ typedData: document, unknown: true }).success).toBe(
    false
  );
  expect(
    AgentSignTypedDataBody.safeParse({ typedData: { ...document, primaryType: '' } }).success
  ).toBe(false);

  const invalidContainers = [null, [], [{}], 'record', 42, true];
  for (const key of ['domain', 'types', 'message'] as const) {
    for (const invalid of invalidContainers) {
      expect(
        AgentSignTypedDataBody.safeParse({ typedData: { ...document, [key]: invalid } }).success
      ).toBe(false);
    }
  }

  for (const alias of ['typedData', 'typed_data'] as const) {
    const wallet = SignTypedDataBody.safeParse({ [alias]: document });
    expect(wallet.success).toBe(true);
    expect(wallet.data).toEqual({ [alias]: document });
    expect(SignTypedDataBody.safeParse({ [alias]: document, unknown: true }).success).toBe(false);
    expect(SignTypedDataBody.safeParse({ [alias]: { ...document, unknown: true } }).success).toBe(
      false
    );
    expect(SignTypedDataBody.safeParse({ [alias]: withoutDomain }).success).toBe(false);

    for (const key of ['domain', 'types', 'message'] as const) {
      for (const invalid of invalidContainers) {
        expect(
          SignTypedDataBody.safeParse({ [alias]: { ...document, [key]: invalid } }).success
        ).toBe(false);
      }
    }

    for (const invalid of invalidContainers) {
      expect(
        SignTypedDataBody.safeParse({ [alias]: { ...document, types: { Mail: [invalid] } } })
          .success
      ).toBe(false);
    }

    for (const invalid of [null, 'record', 42, true]) {
      expect(
        SignTypedDataBody.safeParse({ [alias]: { ...document, types: { Mail: invalid } } }).success
      ).toBe(false);
    }

    const emptyFields = { ...document, types: { Mail: [] } };
    const emptyArray = SignTypedDataBody.safeParse({ [alias]: emptyFields });
    expect(emptyArray.success).toBe(true);
    expect(emptyArray.data).toEqual({ [alias]: emptyFields });

    const invalidFields = [
      null,
      [],
      'field',
      42,
      {},
      { name: 'contents' },
      { type: 'string' },
      { name: 42, type: 'string' },
      { name: 'contents', type: null },
      { name: '', type: 'string' },
      { name: 'contents', type: '' },
      { name: 'n'.repeat(257), type: 'string' },
      { name: 'contents', type: 't'.repeat(257) },
      { name: 'contents', type: 'string', unknown: true },
    ];
    for (const field of invalidFields) {
      expect(
        SignTypedDataBody.safeParse({ [alias]: { ...document, types: { Mail: [field] } } }).success
      ).toBe(false);
    }

    for (const length of [1, 256]) {
      const boundedDocument = {
        ...document,
        primaryType: 'p'.repeat(length),
        types: { Mail: [{ name: 'n'.repeat(length), type: 't'.repeat(length) }] },
      };
      const bounded = SignTypedDataBody.safeParse({ [alias]: boundedDocument });
      expect(bounded.success).toBe(true);
      expect(bounded.data).toEqual({ [alias]: boundedDocument });
    }
    for (const primaryType of ['', 'p'.repeat(257)]) {
      expect(SignTypedDataBody.safeParse({ [alias]: { ...document, primaryType } }).success).toBe(
        false
      );
    }

    const fields = Array.from({ length: 256 }, (_, index) => ({
      name: `field${index}`,
      type: 'string',
    }));
    const atLimit = { ...document, types: { Mail: fields } };
    const boundedArray = SignTypedDataBody.safeParse({ [alias]: atLimit });
    expect(boundedArray.success).toBe(true);
    expect(boundedArray.data).toEqual({ [alias]: atLimit });
    expect(
      SignTypedDataBody.safeParse({
        [alias]: {
          ...document,
          types: { Mail: [...fields, { name: 'field256', type: 'string' }] },
        },
      }).success
    ).toBe(false);
  }

  expect(SignTypedDataBody.safeParse({ typedData: document, typed_data: document }).success).toBe(
    false
  );
}

test(gatewayTypedDataRecordsZod4, gatewayTypedDataRecordsZod4);
