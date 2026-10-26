// @vitest-environment jsdom
import { expect, test } from 'vitest';
import { ethers } from 'ethers';
import { SurfaceChain, type SentCall } from './chain';

async function surfaces_test_chain_answers_the_sequencer_receipt_of_a_sent_transaction() {
  const chain = new SurfaceChain();
  const token = `0x${'ab'.repeat(20)}`;
  const spender = `0x${'cd'.repeat(20)}`;
  const abi = new ethers.Interface([
    'function approve(address spender, uint256 amount)',
    'function allowance(address owner, address spender) view returns (uint256)',
    'event Approval(address indexed owner, address indexed spender, uint256 value)',
  ]);
  const approvalLog = (amount: bigint) => ({
    address: token,
    ...abi.encodeEventLog('Approval', [chain.address, spender, amount]),
  });
  chain.logsFor = (call) => {
    const [, amount] = abi.decodeFunctionData('approve', call.data);
    return [approvalLog(amount)];
  };

  const unknownHash = ethers.ZeroHash;
  for (const method of ['eth_getTransactionReceipt', 'pax_sequencerReceipt']) {
    expect(await chain.request({ method, params: [unknownHash] })).toBeNull();
  }

  for (const [index, method] of [
    'eth_sendTransaction',
    'paxeer_sendTransactionReceipted',
  ].entries()) {
    const amount = BigInt(index + 1) * 7n;
    const tx: SentCall = {
      from: chain.wallet.address,
      to: ethers.getAddress(token),
      data: abi.encodeFunctionData('approve', [spender, amount]).toUpperCase().replace('0X', '0x'),
      value: '0x0',
    };
    const call = { ...tx, from: chain.address, to: token, data: tx.data.toLowerCase() };
    const hash = ethers.id(`${index + 1}:${call.to}:${call.data}:${call.value}`);
    const receipt = { status: '0x1', logs: [approvalLog(amount)] };
    const sequencerReceipt = { rung: 'receipted', txHash: hash, ...receipt };

    const result = await chain.request({ method, params: [tx] });
    expect(result).toEqual(method === 'eth_sendTransaction' ? hash : sequencerReceipt);
    expect(chain.sent).toHaveLength(index + 1);
    expect(chain.sent[index]).toEqual(call);
    expect(await chain.request({ method: 'pax_sequencerReceipt', params: [hash] })).toEqual(
      sequencerReceipt
    );
    expect(await chain.request({ method: 'eth_getTransactionReceipt', params: [hash] })).toEqual({
      transactionHash: hash,
      ...receipt,
    });
    expect(
      await chain.request({
        method: 'eth_call',
        params: [
          { to: token, data: abi.encodeFunctionData('allowance', [chain.address, spender]) },
        ],
      })
    ).toBe(abi.encodeFunctionResult('allowance', [amount]));

    await expect(
      chain.request({
        method,
        params: [{ ...tx, from: spender }],
      })
    ).rejects.toMatchObject({ message: 'unknown account', code: 4100 });
    expect(chain.sent).toHaveLength(index + 1);
    expect(await chain.request({ method: 'pax_sequencerReceipt', params: [hash] })).toEqual(
      sequencerReceipt
    );
  }
}

test(surfaces_test_chain_answers_the_sequencer_receipt_of_a_sent_transaction);
