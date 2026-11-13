import { decodeFunctionData, encodeFunctionData, zeroAddress } from 'viem';
import { privateKeyToAccount } from 'viem/accounts';
import { expect } from 'vitest';

import manifest from '../../package.json';
import depositBody from '../lib/bridge/fixtures/deposit-ready.json';
import releaseBody from '../lib/bridge/fixtures/release-ready.json';
import { inboundDigest, outboundDigest, verifyThresholdSignatures } from '../lib/bridge/digests';
import { ledgerKey, openReleasePath } from './release-path';

async function withReleasePath(run: (path: Awaited<ReturnType<typeof openReleasePath>>) => Promise<void>) {
  const previous = window.localStorage.getItem(ledgerKey);
  const path = await openReleasePath();
  try {
    window.localStorage.removeItem(ledgerKey);
    await run(path);
  } finally {
    await path.close();
    if (previous === null) window.localStorage.removeItem(ledgerKey);
    else window.localStorage.setItem(ledgerKey, previous);
  }
}

export function manifestTypegenPrecedesTypecheck() {
  expect(manifest.scripts.test).toBe('vitest run');
  expect(manifest.scripts.typecheck.split(' && ')).toEqual([ 'next typegen', 'tsc --noEmit -p tsconfig.json' ]);
}

export async function attestationWireFixturesSurviveListenerReopening() {
  await withReleasePath(async(path) => {
    const depositRequest = { chainId: 1n, txHash: `0x${ 'aa'.repeat(32) }` as const, logIndex: 7n };
    const releaseRequest = { chainId: 1n, paxeerTxHash: path.input.hash, paxeerNonce: 3n };
    let first: unknown;
    for (let session = 0; session < 2; session++) {
      const deposit = await path.api.fetchDepositAttestation(depositRequest, path.base);
      const release = await path.api.fetchReleaseAttestation(releaseRequest, path.base);
      if (!deposit || !release) throw new Error('Both committed bundles must cross the exported HTTP client.');
      expect(deposit.digest).toBe(inboundDigest({
        ...depositRequest, vault: deposit.vault, recipient32: deposit.recipient32,
        asset: deposit.asset, amount: BigInt(deposit.amount),
      }));
      expect(release.digest).toBe(outboundDigest({
        ...releaseRequest, vault: release.vault, recipient: release.recipient,
        asset: release.asset, amount: BigInt(release.amount),
      }));
      expect(release).toEqual(path.attestation);
      for (const received of [ deposit, release ]) {
        expect(path.api.attestationPollInterval(received, 12000)).toBe(false);
        expect(await verifyThresholdSignatures({
          ...received, attestors: [ path.attestor ], threshold: 1,
        })).toEqual({ ok: true, signers: [ path.attestor ] });
      }
      if (session === 0) first = { deposit, release };
      else expect({ deposit, release }).toEqual(first);
      expect(await path.api.fetchDepositAttestation({ ...depositRequest, logIndex: 8n }, path.base)).toBeNull();
      expect(await path.api.fetchReleaseAttestation({ ...releaseRequest, paxeerNonce: 4n }, path.base)).toBeNull();
      expect(() => path.api.parseReleaseAttestation({ ...releaseRequest, chainId: 2n }, release)).toThrow('different "chainId"');
      await path.reopen();
    }
  });
}

export async function componentReleaseConsumesPersistedSubmissionAndRefusesSignedFieldChanges() {
  await withReleasePath(async(path) => {
    let published = 0;
    const submitted = path.card.recordBridgeSubmission({ direction: 'out', input: path.input }, () => {
      expect(path.ledger.listTransfers(path.input.account)).toHaveLength(1);
      published++;
    });
    const claim = path.api.parseDepositAttestation({ chainId: 1n, txHash: depositBody.txHash as `0x${ string }`, logIndex: 7n }, depositBody);
    const deposit = path.submitted.depositRecord({ ...path.input, hash: claim.txHash, vault: claim.vault, recipient: path.input.destination });
    const readyDeposit = { ...deposit, logIndex: claim.logIndex };
    for (const received of [ claim, path.attestation ]) {
      expect(await verifyThresholdSignatures({ ...received, attestors: [ path.attestor ], threshold: 1 }))
        .toEqual({ ok: true, signers: [ path.attestor ] });
      expect(await verifyThresholdSignatures({ ...received, attestors: [ path.input.account ], threshold: 1 }))
        .toEqual({ ok: false, reason: 'Signature 1 is not from a registered attestor.' });
    }
    for (let attempt = 0; attempt < 2; attempt++) {
      await expect(path.actions.buildVerifiedClaim(readyDeposit, { ...claim, status: 'attesting' })).rejects.toThrow('The deposit attestation is not ready.');
      await expect(path.actions.buildVerifiedClaim(readyDeposit, { ...claim, signatures: [] })).rejects.toThrow('The deposit attestation is not ready.');
      await expect(path.actions.buildVerifiedClaim(readyDeposit, { ...claim, amount: '1' })).rejects.toThrow('The backend attestation does not match this deposit.');
      await expect(path.actions.buildVerifiedClaim(submitted, claim)).rejects.toThrow('Only deposits can be claimed.');
    }
    expect(published).toBe(1);
    expect(submitted).not.toHaveProperty('vault');
    expect(submitted.bridgeOutNonce).toBeUndefined();
    await path.reopen();
    const recovered = await path.reload();
    expect(recovered.vault).toBe(path.attestation.vault);
    expect(recovered.bridgeOutNonce).toBe('3');
    expect(path.ledger.listTransfers(path.input.account)).toEqual([ recovered ]);
    const { request } = await path.actions.buildVerifiedRelease(recovered, path.attestation);
    expect(request.address).toBe(path.attestation.vault);
    expect(request.chainId).toBe(1);
    expect(decodeFunctionData({ abi: request.abi, data: encodeFunctionData(request) })).toEqual({
      functionName: 'release', args: [ path.attestation.asset, 100n, path.input.destination, path.input.hash, 3n, path.attestation.signatures ],
    });
    for (const change of [
      { amount: '101' }, { asset: zeroAddress }, { recipient: zeroAddress },
      { paxeerNonce: '4' }, { paxeerTxHash: `0x${ 'ee'.repeat(32) }` as const }, { chainId: '2' },
    ]) {
      await expect(path.actions.buildVerifiedRelease(recovered, { ...path.attestation, ...change }))
        .rejects.toThrow('does not match this withdrawal');
    }
    await expect(path.actions.buildVerifiedRelease(recovered, { ...path.attestation, vault: zeroAddress })).rejects.toThrow('different vault');
    await expect(path.actions.buildVerifiedRelease(recovered, { ...path.attestation, digest: `0x${ '00'.repeat(32) }` }))
      .rejects.toThrow('digest does not match the withdrawal');
    for (const change of [ { status: 'attesting' as const }, { signatures: [] } ]) {
      await expect(path.actions.buildVerifiedRelease(recovered, { ...path.attestation, ...change })).rejects.toThrow('not ready');
    }
    await expect(path.actions.buildVerifiedRelease({ ...recovered, direction: 'in' }, path.attestation)).rejects.toThrow('Only withdrawals');
    await expect(path.actions.buildVerifiedRelease({ ...recovered, foreignChainKey: 'solana' }, path.attestation)).rejects.toThrow('cannot be released from an EVM wallet');
    const calls = path.calls.filter((call) => call.method === 'eth_call');
    expect(calls.length).toBeGreaterThan(4);
    expect(path.ledger.listTransfers(path.input.account)).toEqual([ recovered ]);
  });
}

export async function ledgerWithdrawalReopensIntoAuthoritativeReleaseVerification() {
  await withReleasePath(async(path) => {
    const submitted = path.submitted.withdrawalRecord(path.input);
    path.ledger.saveTransfer(submitted);
    expect(submitted).not.toHaveProperty('vault');
    expect(submitted.bridgeOutNonce).toBeUndefined();
    const persisted = window.localStorage.getItem(ledgerKey);
    await path.reopen();
    expect(window.localStorage.getItem(ledgerKey)).toBe(persisted);
    const recovered = await path.reload();
    expect(recovered.vault).toBe(path.attestation.vault);
    expect(recovered.bridgeOutNonce).toBe('3');
    expect((await path.actions.buildVerifiedRelease(recovered, path.attestation)).request.functionName).toBe('release');
    // Exercise the consumer's legacy-row resolution too, without filling the missing vault in the test.
    const { getTransactionReceipt } = await import('wagmi/actions');
    const { enrichRecord } = await import('@/lib/bridge/enrich');
    const receipt = await getTransactionReceipt(path.wallet.wagmiConfig, { hash: submitted.sourceTxHash, chainId: 125 });
    const enriched = enrichRecord(submitted, receipt);
    expect(enriched).not.toHaveProperty('vault');
    expect((await path.actions.buildVerifiedRelease(enriched, path.attestation)).request.address).toBe(path.attestation.vault);
    path.state.enabled = false;
    await expect(path.actions.buildVerifiedRelease(recovered, path.attestation)).rejects.toThrow('not registered and enabled');
    path.state.enabled = true;
    path.state.registeredVault = zeroAddress;
    await expect(path.actions.buildVerifiedRelease(recovered, path.attestation)).rejects.toThrow('not registered and enabled');
    expect(path.ledger.listTransfers(path.input.account)).toEqual([ recovered ]);
  });
}

export async function libraryHttpBundleReachesReloadedWithdrawalAndStrictThresholdChecks() {
  await withReleasePath(async(path) => {
    path.ledger.saveTransfer(path.submitted.withdrawalRecord(path.input));
    await path.reopen();
    const recovered = await path.reload();
    const received = await path.client().fetchQuery(path.panel.releaseAttestationOptions(recovered, false, path.base));
    if (!received) throw new Error('The real HTTP client did not receive its committed bundle.');
    expect(received).toEqual(path.attestation);
    const depositRequest = { chainId: 1n, txHash: depositBody.txHash as `0x${ string }`, logIndex: 7n };
    const deposit = await path.api.fetchDepositAttestation(depositRequest, path.base);
    if (!deposit) throw new Error('The deposit HTTP fixture must be delivered too.');
    const depositTransfer = { ...path.submitted.depositRecord({
      ...path.input, hash: deposit.txHash, vault: deposit.vault, recipient: path.input.destination,
    }), logIndex: deposit.logIndex };
    expect(() => path.actions.assertClaimMatches(depositTransfer, deposit)).not.toThrow();
    expect(() => path.actions.assertReleaseMatches(recovered, received)).not.toThrow();
    await expect(path.actions.buildVerifiedClaim(depositTransfer, { ...deposit, status: 'finalizing' })).rejects.toThrow('not ready');
    expect(() => path.actions.assertClaimMatches({ ...depositTransfer, amount: '1' }, deposit)).toThrow('does not match this deposit');
    expect(() => path.actions.assertReleaseMatches({ ...recovered, amount: '1' }, received)).toThrow('does not match this withdrawal');
    expect(() => path.api.parseDepositAttestation(depositRequest, { ...depositBody, amount: '-1' })).toThrow(path.api.AttestationApiError);
    expect(() => path.api.parseReleaseAttestation({ chainId: 1n, paxeerTxHash: path.input.hash, paxeerNonce: 3n }, {
      ...releaseBody, amount: '-1',
    })).toThrow(path.api.AttestationApiError);
    const outsiderForDeposit = privateKeyToAccount(`0x${ '12'.repeat(32) }`);
    for (const bundle of [ deposit, received ]) {
      expect(path.api.attestationPollInterval(bundle, 12000)).toBe(false);
      expect(await verifyThresholdSignatures({ ...bundle, attestors: [ path.attestor ], threshold: 1 }))
        .toEqual({ ok: true, signers: [ path.attestor ] });
      expect(await verifyThresholdSignatures({ ...bundle, signatures: [ await outsiderForDeposit.sign({ hash: bundle.digest }) ],
        attestors: [ path.attestor ], threshold: 1 })).toEqual({ ok: false, reason: 'Signature 1 is not from a registered attestor.' });
      for (const threshold of [ NaN, -1, 0.5, Infinity ]) {
        expect(await verifyThresholdSignatures({ ...bundle, attestors: [ path.attestor ], threshold }))
          .toEqual({ ok: false, reason: 'vault threshold is not a non-negative safe integer' });
      }
    }

    expect((await path.actions.buildVerifiedRelease(recovered, received)).request.address).toBe(received.vault);
    expect(await verifyThresholdSignatures({ ...received, attestors: [ path.attestor ], threshold: 1 }))
      .toEqual({ ok: true, signers: [ path.attestor ] });
    for (const threshold of [ NaN, -1, 0.5, Infinity ]) {
      expect(await verifyThresholdSignatures({ ...received, attestors: [ path.attestor ], threshold }))
        .toEqual({ ok: false, reason: 'vault threshold is not a non-negative safe integer' });
    }
    const outsider = privateKeyToAccount(`0x${ '12'.repeat(32) }`);
    const untrusted = { ...received, signatures: [ await outsider.sign({ hash: received.digest }) ] };
    await expect(path.actions.buildVerifiedRelease(recovered, untrusted)).rejects.toThrow('not from a registered attestor');
    await expect(path.actions.buildVerifiedRelease(recovered, { ...received, status: 'finalizing' })).rejects.toThrow('not ready');
    await expect(path.actions.buildVerifiedRelease(recovered, { ...received, amount: '1' })).rejects.toThrow('does not match this withdrawal');
    // Discard QueryClient memory: every observation must issue both HTTP requests again.
    const requestsBeforeRepeat = path.httpRequests.length;
    for (let observation = 0; observation < 2; observation++) {
      const client = path.client();
      const repeatedDeposit = await client.fetchQuery(path.panel.depositAttestationOptions(depositTransfer, false, path.base));
      const repeatedRelease = await client.fetchQuery(path.panel.releaseAttestationOptions(recovered, false, path.base));
      expect(repeatedDeposit).toEqual(deposit);
      expect(repeatedRelease).toEqual(received);
      expect(path.httpRequests).toHaveLength(requestsBeforeRepeat + (observation + 1) * 2);
      expect(path.httpRequests.slice(-2)).toEqual([
        path.api.depositAttestationPath(depositRequest),
        path.api.releaseAttestationPath({ chainId: 1n, paxeerTxHash: path.input.hash, paxeerNonce: 3n }),
      ]);
    }
    const requestsBeforeRefusals = path.httpRequests.length;
    path.damageAmounts();
    await expect(path.client().fetchQuery(path.panel.depositAttestationOptions(depositTransfer, false, path.base)))
      .rejects.toThrow(path.api.AttestationApiError);
    await expect(path.client().fetchQuery(path.panel.releaseAttestationOptions(recovered, false, path.base)))
      .rejects.toThrow(path.api.AttestationApiError);
    const missing = { ...recovered, bridgeOutNonce: '4' };
    expect(await path.client().fetchQuery(path.panel.releaseAttestationOptions(missing, false, path.base))).toBeNull();
    const missingDeposit = { ...depositTransfer, logIndex: '8' };
    expect(await path.client().fetchQuery(path.panel.depositAttestationOptions(missingDeposit, false, path.base))).toBeNull();
    expect(path.httpRequests).toHaveLength(requestsBeforeRefusals + 4);
  });
}

export async function settlementSubmissionReopensThroughBroadcastRevertAndConfirmation() {
  await withReleasePath(async(path) => {
    const submitted = path.card.recordBridgeSubmission({ direction: 'out', input: path.input }, () => {});
    expect(submitted).not.toHaveProperty('vault');
    let recovered = await path.reload();
    await path.actions.buildVerifiedRelease(recovered, path.attestation);
    path.ledger.updateTransfer(recovered.id, path.settlement.settlementBroadcastPatch(recovered, path.state.settlementHash, Date.now()));
    async function observe() {
      await path.reopen();
      recovered = await path.reload();
      return path.client().fetchQuery(path.panel.transferLiveOptions(recovered));
    }
    let live = await observe();
    expect(live.settlementStatus).toBe('pending');
    expect(path.settlement.phaseOf(recovered, live, true)).toBe('settling');
    expect(path.ledger.listTransfers(path.input.account)[0].releasedAt).toBeUndefined();
    path.ledger.updateTransfer(recovered.id, path.settlement.settlementBroadcastPatch(recovered, path.state.settlementHash, Date.now() - 130000));
    live = await observe();
    expect(live.settlementStatus).toBe('dropped');
    expect(path.settlement.phaseOf(recovered, live, true)).toBe('ready');
    expect(path.actions.settledRefusal(recovered, live)).toBeUndefined();
    path.state.settlementStatus = 'reverted';
    live = await observe();
    expect(live.settlementStatus).toBe('reverted');
    expect(path.settlement.phaseOf(recovered, live, true)).toBe('ready');
    expect(path.ledger.listTransfers(path.input.account)[0].releasedAt).toBeUndefined();
    path.state.settlementStatus = 'success';
    live = await observe();
    expect(live.released).toBe(false);
    expect(live.settlementStatus).toBe('pending');
    expect(path.settlement.phaseOf(recovered, live, true)).toBe('settling');
    expect(path.ledger.listTransfers(path.input.account)[0].releasedAt).toBeUndefined();
    path.state.released = true;
    live = await observe();
    expect(live.released).toBe(true);
    expect(live.settlementStatus).toBe('confirmed');
    expect(path.settlement.phaseOf(recovered, live, true)).toBe('settled');
    expect(path.actions.settledRefusal(recovered, live)).toBe('This withdrawal is already released from the vault.');
    const settled = path.ledger.listTransfers(path.input.account)[0];
    expect(settled.releasedAt).toEqual(expect.any(Number));
    expect(settled.settlementStatus).toBe('confirmed');
    await path.reopen();
    expect(await path.reload()).toEqual(settled);
    expect((await path.client().fetchQuery(path.panel.transferLiveOptions(settled))).released).toBe(true);
  });
}

export async function solanaRecipientFixtureReopensThroughRpcAndHook() {
  const { act, renderHook, waitFor } = await import('@testing-library/react/pure');
  const { QueryClient, QueryClientProvider } = await import('@tanstack/react-query');
  const { createElement } = await import('react');
  const { createServer } = await import('node:http');
  const { mkdtemp, readFile, writeFile, rm } = await import('node:fs/promises');
  const { tmpdir } = await import('node:os');
  const { join } = await import('node:path');
  const { 'default': fixture } = await import('./fixtures/recipient-vector.json');
  const { base58Encode, solanaRecipientHandle, recipientRecordRequest } = await import('@/lib/bridge/solana');
  const { useRecipientRecord } = await import('@/lib/bridge/use-bridge-state');
  const { solanaWithdrawalBlocker } = await import('@/lib/bridge/solana-route');
  const { recordBridgeSubmission } = await import('@/components/bridge/bridge-card');
  const { bridgeChainByKey } = await import('@/lib/bridge/chains');
  const { listTransfers } = await import('@/lib/bridge/transfers');
  const previous = window.localStorage.getItem(ledgerKey);
  const directory = await mkdtemp(join(tmpdir(), 'bridge-recipient-'));
  const filename = join(directory, 'account.json');
  const programId = base58Encode(Uint8Array.from({ length: 32 }, (_, index) => index));
  const request = recipientRecordRequest(programId, fixture.destination);
  if (!request) throw new Error('The production PDA derivation refused the Rust identity vector.');
  expect(solanaRecipientHandle(fixture.destination)).toBe(fixture.handle);
  await writeFile(filename, JSON.stringify({ [request.params[0]]: fixture.data }));
  const otherDestination = base58Encode(Uint8Array.from({ length: 32 }, (_, index) => index + 64));
  const errors: unknown[] = [];
  const received: unknown[] = [];
  try {
    window.localStorage.removeItem(ledgerKey);
    for (let session = 0; session < 2; session++) {
      // Re-open serialized account bytes and a fresh RPC listener and query cache.
      const accounts: Record<string, string> = JSON.parse(await readFile(filename, 'utf8'));
      const server = createServer(async(incoming, outgoing) => {
        try {
          expect(incoming.method).toBe('POST');
          let body = '';
          for await (const chunk of incoming) body += chunk.toString();
          const message = JSON.parse(body);
          expect(message.method).toBe('getAccountInfo');
          expect(message.jsonrpc).toBe('2.0');
          expect(message.params[1]).toEqual({ encoding: 'base64' });
          received.push(message);
          const data = accounts[message.params[0]];
          outgoing.setHeader('Content-Type', 'application/json');
          outgoing.end(JSON.stringify({ jsonrpc: '2.0', id: message.id, result: {
            context: { slot: 1 }, value: data === undefined ? null : {
              data: [ data, 'base64' ], owner: programId, executable: false, lamports: 1, rentEpoch: 0,
            },
          } }));
        } catch (error) {
          errors.push(error);
          outgoing.statusCode = 500;
          outgoing.end(String(error));
        }
      });
      await new Promise<void>((resolve, reject) => {
        server.once('error', reject);
        server.listen(0, '127.0.0.1', resolve);
      });
      const address = server.address();
      if (!address || typeof address === 'string') throw new Error('Recipient fixture RPC did not start.');
      const settings = { programId, rpcUrl: `http://127.0.0.1:${ address.port }` };
      const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } });
      const wrapper = ({ children }: import('react').PropsWithChildren) => createElement(QueryClientProvider, { client }, children);
      const hook = renderHook(({ destination }) => useRecipientRecord(destination, settings), {
        wrapper, initialProps: { destination: fixture.destination },
      });
      try {
        await waitFor(() => expect(hook.result.current.isSuccess).toBe(true));
        const lookup = hook.result.current.data;
        expect(lookup?.registered).toBe(true);
        expect(solanaWithdrawalBlocker(lookup, fixture.destination)).toBeUndefined();
        expect(received.at(-1)).toEqual(request);
        const foreign = bridgeChainByKey('solana');
        if (!foreign) throw new Error('The production Solana route is required.');
        const produced = recordBridgeSubmission({ direction: 'out', input: {
          foreign, asset: foreign.assets[0], hash: `0x${ 'aa'.repeat(32) }`,
          amount: 1n, account: '0x2222222222222222222222222222222222222222',
          destination: fixture.destination, requiredConfirmations: 1,
        } }, () => {});
        expect(listTransfers(produced.account)[0].recipient).toBe(fixture.destination);
        await act(async() => hook.rerender({ destination: otherDestination }));
        await waitFor(() => {
          expect(hook.result.current.isSuccess).toBe(true);
          expect(hook.result.current.data).toEqual({ registered: false });
        });
        expect(solanaWithdrawalBlocker(hook.result.current.data, otherDestination))
          .toBe('Register this destination\'s recipient record on Solana before withdrawing.');
        expect(listTransfers(produced.account)).toEqual([ produced ]);
      } finally {
        hook.unmount();
        client.clear();
        await new Promise<void>((resolve, reject) => {
          server.close((error) => error ? reject(error) : resolve());
          server.closeAllConnections();
        });
      }
    }
    expect(received).toHaveLength(4);
    expect(errors).toEqual([]);
  } finally {
    await rm(directory, { recursive: true });
    if (previous === null) window.localStorage.removeItem(ledgerKey);
    else window.localStorage.setItem(ledgerKey, previous);
  }
}
