import { mkdtempSync, readFileSync, writeFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { expect, test } from 'vitest';
import { keccak256 } from 'viem';
import { PaxeerProvider } from '../src/provider.js';
import { WalletInterface } from '../src/wallet.js';
import { recoverTransferObservation, statusLadder, submittedTransfer } from '../src/ladder.js';
import { readSequencerReceipt } from '../src/receipt.js';
import type { Hex } from '../src/types.js';
import {
  authenticatedReceipt,
  committedReceipt,
  committedVector,
  recording,
  receiptPins,
  receiptWireKeys,
} from './support/receipt-recordings.js';
import { replay, rpcExchange } from './support/replay.js';

async function proof_wallet_sdk_producer_transport_consumer() {
  const me = recording<any>('../fixtures/gateway/wallet-me.json');
  const send = recording<any>('../fixtures/gateway/wallet-send.json');
  const construction = recording<any>('../fixtures/gateway/constructions.json');
  const authorization = 'Bearer recorded-session';
  const tx = { ...construction.transaction, from: construction.address };
  const transport = await replay([
    { path: me.path, method: me.method, authorization, response: me.body },
    { path: send.path, method: send.method, authorization, request: { tx }, response: send.body },
    rpcExchange('pax_sequencerReceipt', [committedReceipt.txHash], committedReceipt),
    rpcExchange('px_getReceiptKeys', [], receiptWireKeys(committedReceipt)),
    rpcExchange(
      'eth_getRawTransactionByHash',
      [committedReceipt.txHash],
      committedVector.rawTransaction
    ),
  ]);
  const temporary = mkdtempSync(join(tmpdir(), 'wallet-sdk-recovery-'));
  const path = join(temporary, 'observation.json');
  const provider = () =>
    new PaxeerProvider({
      gatewayUrl: transport.url,
      rpcUrl: `${transport.url}/node`,
      token: () => 'recorded-session',
    });
  let active = provider();
  try {
    const wallet = new WalletInterface(active);
    expect(await wallet.accounts()).toEqual([construction.address]);
    const hash = await wallet.sendTransaction({
      to: tx.to,
      value: BigInt(tx.value),
      data: tx.data,
      gas: BigInt(tx.gas),
      maxFeePerGas: BigInt(tx.maxFeePerGas),
      maxPriorityFeePerGas: BigInt(tx.maxPriorityFeePerGas),
      nonce: tx.nonce,
      chainId: tx.chainId,
    });
    expect(hash).toBe(send.body.tx_hash);
    expect(submittedTransfer({ hash: hash as Hex, chainId: 125 }).truth).toBe('submitted');
    expect(transport.seen.filter((entry) => entry.path === send.path)).toEqual([
      { path: send.path, method: send.method, authorization, body: { tx } },
    ]);

    // The gateway send and node capture are different transactions. Authenticate
    // the recorded node identity; never attach its receipt to the gateway hash.
    expect(committedReceipt.txHash).toBe(keccak256(committedVector.rawTransaction));
    expect(committedReceipt.txHash).not.toBe(hash);
    const rpc = (method: string, params: readonly unknown[]) => active.request({ method, params });
    const receipt = await readSequencerReceipt(rpc, committedReceipt.txHash);
    expect(receipt).toEqual(authenticatedReceipt(committedReceipt));
    const identity = { hash: committedReceipt.txHash, chainId: 125 };
    const observation = recoverTransferObservation(
      identity,
      {
        ...submittedTransfer(identity),
        lastVerified: {
          blockNumber: Number(BigInt(committedReceipt.blockNumber!)),
          blockHash: committedReceipt.blockHash,
          steps: statusLadder.fromSequencerReceipt(committedReceipt, receiptPins(committedReceipt)),
          sequencer: receipt,
        },
      },
      receiptPins(committedReceipt)
    );
    expect(observation.lastVerified?.sequencer).toEqual(committedReceipt);
    writeFileSync(path, JSON.stringify(observation));
    active.disconnect();
    expect(await active.request({ method: 'eth_accounts' })).toEqual([]);
    await transport.restart();
    active = provider();
    const recovered = recoverTransferObservation(
      identity,
      JSON.parse(readFileSync(path, 'utf8')),
      receiptPins(committedReceipt)
    );
    expect(recovered).toEqual(observation);
    expect(await readSequencerReceipt(rpc, recovered.identity.hash)).toEqual(receipt);
    expect(statusLadder.highest(recovered.lastVerified!.steps)?.rung).toBe('committed');
    const tampered = JSON.parse(readFileSync(path, 'utf8'));
    tampered.lastVerified.sequencer.logs[0].data += '00';
    expect(
      recoverTransferObservation(identity, tampered, receiptPins(committedReceipt)).lastVerified
    ).toBeUndefined();
    expect(
      recoverTransferObservation(
        { hash: hash as Hex, chainId: 125 },
        observation,
        receiptPins(committedReceipt)
      ).lastVerified
    ).toBeUndefined();
    expect(transport.errors).toEqual([]);
  } finally {
    active.disconnect();
    await transport.close();
    rmSync(temporary, { recursive: true, force: true });
  }
}

test(proof_wallet_sdk_producer_transport_consumer);
