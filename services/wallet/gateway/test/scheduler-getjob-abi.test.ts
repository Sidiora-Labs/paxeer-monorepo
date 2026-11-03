import { readFileSync } from 'node:fs';
import { decodeFunctionResult, encodeFunctionResult, type Abi } from 'viem';
import { expect, test } from 'vitest';
import { SCHEDULER_ABI } from '../src/precompiles.js';

const schedulerGetJobAbiTests = {
  'decodes a populated node getJob result'() {
    const source = readFileSync(
      new URL('../../../../precompiles/layerxagent/scheduler/scheduler.go', import.meta.url),
      'utf8',
    );
    const abiMatch = /\babiJSON\s*=\s*`([^`]+)`/u.exec(source);
    if (!abiMatch) throw new Error('node scheduler abiJSON was not found');
    const nodeAbi = JSON.parse(abiMatch[1]!) as Abi;
    const job = [
      1n,
      '0x1111111111111111111111111111111111111111',
      '0x2222222222222222222222222222222222222222',
      '0x12345678',
      1000000n,
      100000n,
      1n,
      true,
    ] as const;
    const encoded = encodeFunctionResult({ abi: nodeAbi, functionName: 'getJob', result: job });

    expect(SCHEDULER_ABI.find((entry) => entry.name === 'getJob')?.outputs).toEqual(
      nodeAbi.find((entry) => entry.type === 'function' && entry.name === 'getJob')?.outputs,
    );
    expect(
      decodeFunctionResult({ abi: SCHEDULER_ABI, functionName: 'getJob', data: encoded }),
    ).toEqual(job);
  },
};

test(
  schedulerGetJobAbiTests['decodes a populated node getJob result'],
  schedulerGetJobAbiTests['decodes a populated node getJob result'],
);
