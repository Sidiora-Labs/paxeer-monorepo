// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, test } from 'vitest';
import { createServer } from 'node:http';
import type { AddressInfo } from 'node:net';
import { useLayoutEffect, useMemo } from 'react';
import { ethers } from 'ethers';
import {
  SIDIORA_TOKEN,
  abiSelector,
  exchangeCancelOrderCall,
  gasQuoteDigest,
} from '@sidiora/layerx-sdk';
import {
  FEE_TOKEN_PRECOMPILE,
  SIDIORA_FEE_DENOM,
  constructionDigest,
  exchange,
  feeToken,
  type IntentLeg,
  type ModuleTransaction,
  type SponsoredAdapterRequest,
  type WireDigestConstruction,
} from '@paxeer/wallet';
import { WalletProvider, useWallet } from '@/wallet/WalletProvider';
import { IdentitySession } from '@/wallet/identity';
import { EMAIL, EMAIL_CODE, startGateway } from '@/wallet/test/gateway';
import { mount } from '@/account/test/render';
import { FeeChoice } from './FeeChoice';
import { SendButton, SentResult, SurfaceFrame } from './Surface';
import { useFeeSelection, useModuleSend, useSurfaceWallet } from './useSurface';
import { SurfaceChain, click, mountSurface, text, until, type Mounted } from './test/chain';

let chain: SurfaceChain;
let mounted: Mounted | null = null;

beforeEach(() => {
  chain = new SurfaceChain();
});

afterEach(async () => {
  await mounted?.unmount();
  mounted = null;
});

const TRANSACTION: ModuleTransaction = exchangeCancelOrderCall(`0x${'22'.repeat(32)}`);
const RATE = '3114000';

const LEGS: IntentLeg[] = [
  {
    index: 0,
    mechanism: 'paxeer_transfer',
    domain: 'paxeer',
    source: { kind: 'paxeer-wallet' },
    destination: { kind: 'human' },
    money: { amount: '1000000', currency: 'SID' },
    fee: { amount: '7', currency: 'PAX' },
  },
  {
    index: 1,
    mechanism: 'kernel_send',
    domain: 'layerx',
    source: { kind: 'human' },
    destination: { kind: 'agent', account: 'agent:did:layerx:main' },
    money: { amount: '1000000', currency: 'SID' },
    fee: { amount: '1200', currency: 'SID' },
  },
  {
    index: 2,
    mechanism: 'budget_grant',
    domain: 'layerx',
    source: { kind: 'agent' },
    destination: { kind: 'agent-budget' },
    money: { amount: '500', currency: 'SID' },
    fee: { amount: '300', currency: 'SID' },
  },
];

function Harness({ legs }: { legs: readonly IntentLeg[] }) {
  const { provider, address } = useSurfaceWallet();
  const selection = useFeeSelection(provider, address);
  return (
    <SurfaceFrame title="Fees" connected={provider !== null}>
      <FeeChoice
        provider={provider}
        address={address}
        transaction={TRANSACTION}
        selection={selection}
        sidRate={RATE}
        legs={legs}
      />
      <output data-role="preference-rung">{selection.preferenceRung ?? 'none'}</output>
    </SurfaceFrame>
  );
}

function choice(container: HTMLElement, id: string): HTMLElement {
  const node = container.querySelector<HTMLElement>(`[data-fee-choice="${id}"]`);
  if (!node) throw new Error(`no fee choice ${id}`);
  return node;
}

function part(container: HTMLElement, id: string, role: string): string {
  return choice(container, id).querySelector(`[data-role="${role}"]`)?.textContent ?? '';
}

function SponsoredSendHarness({ hashes }: { hashes: string[] }) {
  const connection = useWallet();
  const { provider, address } = useSurfaceWallet();
  const selection = useFeeSelection(provider, address);
  const module = useMemo(() => (provider ? exchange(provider) : null), [provider]);
  const send = useModuleSend(module, provider, address, selection);
  useLayoutEffect(() => {
    if (send.sent) hashes.push(send.sent.hash);
  }, [send.sent, hashes]);
  return (
    <>
      <button onClick={() => void connection.verifyEmailCode(EMAIL, EMAIL_CODE)}>Sign in</button>
      <SurfaceFrame title="Sponsored resend" connected={provider !== null}>
        <FeeChoice
          provider={provider}
          address={address}
          transaction={TRANSACTION}
          selection={selection}
          sidRate={RATE}
        />
        <button data-action="quote" onClick={() => void selection.prepare(TRANSACTION, 100n, 1n)}>
          Quote this call
        </button>
        <SendButton
          label="Send"
          transaction={TRANSACTION}
          blocked={selection.blocked}
          state={send}
        />
        <SentResult state={send} />
      </SurfaceFrame>
    </>
  );
}

describe('FeeChoice', () => {
  async function fee_choice_requotes_an_identical_sponsored_send_after_the_first_settles() {
    const identityGateway = await startGateway();
    const sponsor = new ethers.Wallet(ethers.id('fee-choice-resend-sponsor'));
    const paymaster = `0x${'33'.repeat(20)}`;
    const quotes: unknown[] = [];
    const submissions: SponsoredAdapterRequest[] = [];
    const failures: unknown[] = [];
    const hashes: string[] = [];
    const previous = {
      api: process.env.NEXT_PUBLIC_PAXEER_WALLET_API,
      sponsor: process.env.NEXT_PUBLIC_PAXEER_GAS_SPONSOR,
      paymaster: process.env.NEXT_PUBLIC_PAXEER_GAS_PAYMASTER,
    };
    const server = createServer((request, response) => {
      void (async () => {
        let raw = '';
        for await (const chunk of request) raw += String(chunk);
        const body = raw ? JSON.parse(raw) : null;
        response.setHeader('content-type', 'application/json');
        const send = (answer: unknown) =>
          response.end(
            JSON.stringify(answer, (_key, value) =>
              typeof value === 'bigint' ? value.toString() : value
            )
          );
        if (request.url?.startsWith('/auth/')) {
          const upstream = await fetch(`${identityGateway.url}${request.url}`, {
            method: request.method,
            headers: {
              apikey: identityGateway.config.identityKey,
              'content-type': 'application/json',
            },
            body: raw || undefined,
          });
          response.statusCode = upstream.status;
          response.end(await upstream.text());
          return;
        }
        if (request.url === '/v1/wallet/me') {
          send({ wallet: { address: chain.address, chain_id: 125 }, chain: { id: 125 } });
          return;
        }
        if (request.url === '/v1/wallet/sponsored/quote') {
          quotes.push(body);
          const quote = {
            sponsor: sponsor.address.toLowerCase(),
            token: SIDIORA_TOKEN,
            decimals: 6,
            maxTokenAmount: BigInt(body.maxTokenAmount),
            tokenAmount: 1n,
            gasCost: BigInt(body.gasCost),
            quoteNonce: BigInt(quotes.length),
            deadline: BigInt(Math.floor(Date.now() / 1000)) + 300n,
          };
          const digest = gasQuoteDigest(125n, body.account, quote);
          if (!digest.ok) throw new Error(JSON.stringify(digest.refusal));
          send({
            quote,
            relayerSignature: sponsor.signingKey.sign(digest.value).serialized,
            feeDenom: 'uhpx',
          });
          return;
        }
        if (request.url === '/v1/wallet/sign-digest') {
          const construction: WireDigestConstruction = body.construction;
          send({
            address: chain.address,
            signature: chain.wallet.signingKey.sign(constructionDigest(construction)).serialized,
          });
          return;
        }
        if (request.url === '/v1/wallet/sponsored/submit') {
          const submission: SponsoredAdapterRequest = body;
          submissions.push(submission);
          expect(
            ethers
              .recoverAddress(
                constructionDigest(submission.construction),
                submission.account_signature
              )
              .toLowerCase()
          ).toBe(chain.address);
          const call = submission.construction.calls[0];
          const receipt = await chain.request({
            method: 'paxeer_sendTransactionReceipted',
            params: [
              {
                from: submission.account,
                to: call.to,
                data: call.data,
                value: ethers.toQuantity(BigInt(call.value)),
              },
            ],
          });
          send({
            status: 'pending',
            tx_hash: (receipt as { txHash: string }).txHash,
            station: null,
          });
          return;
        }
        if (request.url === '/v1/wallet/sponsored/status') {
          const submission = submissions.find(
            (item) => item.construction.quote.quoteNonce === body.quoteNonce
          );
          if (!submission) throw new Error('no retained submission');
          const hash = hashes[submissions.indexOf(submission)];
          if (!hash) throw new Error('the sponsored send has not settled');
          send({
            status: 'confirmed',
            tx_hash: hash,
            station: {
              state: 'completed',
              deadline: submission.construction.quote.deadline,
              submission: { sponsorNonce: '0', transactionHash: hash },
              replacement: null,
              completion: {
                outcome: 'included',
                transactionHash: hash,
                blockNumber: '1',
                sidCollected: '1',
                paxSpent: '1',
              },
            },
          });
          return;
        }
        if (request.url !== '/rpc') throw new Error(`unexpected route ${request.url}`);
        let result: unknown;
        if (body.method === 'eth_getCode') result = `0xef0100${paymaster.slice(2)}`;
        else if (body.method === 'eth_getTransactionCount')
          result = ethers.toQuantity(chain.sent.length);
        else if (
          body.method === 'eth_call' &&
          body.params[0].data.startsWith(abiSelector('getLayerXDid(address)'))
        ) {
          result = ethers.AbiCoder.defaultAbiCoder().encode(
            ['bytes32', 'string'],
            [`0x${'61'.repeat(32)}`, `did:layerx:${'61'.repeat(32)}`]
          );
        } else if (body.method === 'eth_call' && body.params[0].data === abiSelector('nonce()')) {
          result = ethers.zeroPadValue(ethers.toBeHex(chain.sent.length), 32);
        } else result = await chain.request(body);
        send({ jsonrpc: '2.0', id: body.id, result });
      })().catch((error: unknown) => {
        failures.push(error);
        response.statusCode = 500;
        response.end(JSON.stringify({ error: String(error) }));
      });
    });
    try {
      await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
      const url = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
      process.env.NEXT_PUBLIC_PAXEER_WALLET_API = url;
      process.env.NEXT_PUBLIC_PAXEER_GAS_SPONSOR = sponsor.address;
      process.env.NEXT_PUBLIC_PAXEER_GAS_PAYMASTER = paymaster;
      window.localStorage.clear();
      const config = {
        ...identityGateway.config,
        gatewayUrl: url,
        rpcUrl: `${url}/rpc`,
        identityUrl: url,
      };
      const identity = IdentitySession.create(config, {
        persistSession: false,
        detectSessionInUrl: false,
      });
      mounted = await mount(
        <WalletProvider config={config} identity={identity} eventTarget={new EventTarget()}>
          <SponsoredSendHarness hashes={hashes} />
        </WalletProvider>
      );
      const { container } = mounted;
      await click(container.querySelector('button'), 'embedded sign in');
      await until(() => container.querySelector('[data-fee-choice]') !== null, 'embedded wallet');
      await click(choice(container, 'sid_sponsored'), 'SID sponsored');
      await until(
        () => text(container, '[data-role="sponsored-phase"]') === 'idle',
        'retained batch check'
      );
      const sendButton = () => container.querySelector<HTMLButtonElement>('[data-action="send"]');
      for (let attempt = 0; attempt < 2; attempt += 1) {
        expect(sendButton()?.disabled).toBe(true);
        await click(container.querySelector('[data-action="quote"]'), 'fresh quote');
        await until(
          () => text(container, '[data-role="sponsored-phase"]') === 'quoted',
          'SID quote'
        );
        expect(quotes).toHaveLength(attempt + 1);
        expect(sendButton()?.disabled).toBe(true);
        await click(
          container.querySelector('[data-action="sponsored-consent"]'),
          'exact quote consent'
        );
        expect(sendButton()?.disabled).toBe(false);
        await click(sendButton(), 'send identical call');
        await until(() => hashes.length === attempt + 1, 'confirmed send');
        expect(text(container, '[data-role="status"]')).toBe('confirmed');
        expect(text(container, '[data-role="sponsored-phase"]')).toBe('confirmed');
        expect(text(container, '[data-role="sponsored-hash"]')).toBe(hashes[attempt]);
        expect(submissions).toHaveLength(attempt + 1);
        await click(
          container.querySelector('[data-action="sponsored-recover"]'),
          'settled station status'
        );
        await until(
          () => text(container, '[data-role="sponsored-phase"]') === 'confirmed',
          'station completion'
        );
      }
      expect(failures).toEqual([]);
      expect(quotes).toHaveLength(2);
      expect(submissions).toHaveLength(2);
      expect(submissions[0].construction.calls).toEqual(submissions[1].construction.calls);
      expect(submissions[0].construction.quote.quoteNonce).not.toBe(
        submissions[1].construction.quote.quoteNonce
      );
      expect(hashes).toHaveLength(2);
      expect(hashes[1]).not.toBe(hashes[0]);
      expect(text(container, '[data-role="hash"]')).toBe(hashes[1]);
      expect(hashes.slice(1)).not.toContain(hashes[0]);
    } finally {
      await mounted?.unmount();
      mounted = null;
      for (const [key, value] of Object.entries({
        NEXT_PUBLIC_PAXEER_WALLET_API: previous.api,
        NEXT_PUBLIC_PAXEER_GAS_SPONSOR: previous.sponsor,
        NEXT_PUBLIC_PAXEER_GAS_PAYMASTER: previous.paymaster,
      })) {
        if (value === undefined) delete process.env[key];
        else process.env[key] = value;
      }
      await new Promise<void>((resolve, reject) => {
        server.close((error) => (error ? reject(error) : resolve()));
        server.closeAllConnections();
      });
      await identityGateway.close();
    }
  }

  test(fee_choice_requotes_an_identical_sponsored_send_after_the_first_settles);

  it('fee_choice_labels_every_fee_path_with_its_denomination_and_chain_gas', async () => {
    mounted = await mountSurface(chain, <Harness legs={LEGS} />);
    const { container } = mounted;
    await until(() => part(container, 'pax_gas', 'gas') === '0.000021 PAX', 'gas estimate');

    expect(part(container, 'pax_gas', 'label')).toBe('PAX gas');
    expect(part(container, 'pax_gas', 'denomination')).toBe(
      'PAX · Paxeer · 18 decimals · native coin'
    );
    expect(part(container, 'pax_gas', 'path')).toBe(
      'Signed with eth_sendTransaction; gas is paid in PAX'
    );
    expect(part(container, 'pax_gas', 'gas')).toBe('0.000021 PAX');

    expect(part(container, 'sid_sponsored', 'label')).toBe('SID sponsored');
    expect(part(container, 'sid_sponsored', 'denomination')).toBe(
      `SID · Sidiora · 6 decimals · token ${SIDIORA_TOKEN.toLowerCase()}`
    );
    expect(part(container, 'sid_sponsored', 'path')).toBe(
      'Signed with eth_sign as a sponsored_batch construction and submitted through the gateway; the sponsor is repaid in SID'
    );
    expect(part(container, 'sid_sponsored', 'gas')).toBe('0.000066 SID');

    expect(part(container, 'sid_native', 'label')).toBe('SID native');
    expect(part(container, 'sid_native', 'denomination')).toBe(
      `SID · Sidiora · 6 decimals · fee denom ${SIDIORA_FEE_DENOM}`
    );
    expect(part(container, 'sid_native', 'path')).toBe(
      `Signed with eth_sendTransaction; gas is charged in ${SIDIORA_FEE_DENOM} through the fee token precompile ${FEE_TOKEN_PRECOMPILE}`
    );
    expect(part(container, 'sid_native', 'gas')).toBe('0.000066 SID');

    const legs = Array.from(
      container.querySelectorAll('[aria-label="LayerX fee per leg"] [data-layerx-leg]')
    );
    expect(legs.map((leg) => leg.getAttribute('data-layerx-leg'))).toEqual(['1', '2']);
    expect(legs.map((leg) => leg.textContent)).toEqual([
      'Leg 1 · kernel_send1200 SID',
      'Leg 2 · budget_grant300 SID',
    ]);
    expect(text(container, '[aria-label="Fee path"]')).not.toContain('1200 SID');
    expect(chain.methods).toContain('eth_estimateGas');
    expect(chain.methods).toContain('eth_gasPrice');
  });

  async function fee_choice_applies_the_preference_once_the_change_is_receipted() {
    mounted = await mountSurface(chain, <Harness legs={[]} />);
    const { container } = mounted;
    const noPreference = 'Fee token: none, gas is paid in PAX';
    await until(
      () => text(container, '[data-role="fee-denom"]') === noPreference,
      'initial fee denom'
    );
    expect(text(container, '[data-role="preference-rung"]')).toBe('none');

    await click(choice(container, 'sid_native'), 'sid native');
    expect(text(container, '[data-role="fee-denom"]')).toBe(noPreference);
    expect(text(container, '[data-role="preference-rung"]')).toBe('none');
    expect(chain.sent).toEqual([]);

    await click(container.querySelector('[data-action="fee-preference"]'), 'use SID');
    await until(
      () => text(container, '[data-role="fee-denom"]') === `Fee token: ${SIDIORA_FEE_DENOM}`,
      'receipted fee preference'
    );
    expect(text(container, '[data-role="preference-rung"]')).toBe('receipted');
    expect(container.querySelector('[data-role="fee-blocked"]')).toBeNull();
    expect(chain.sent).toEqual([
      {
        from: chain.address,
        to: FEE_TOKEN_PRECOMPILE,
        data: feeToken(chain).setFeeDenom(SIDIORA_FEE_DENOM).data,
        value: '0x0',
      },
    ]);
    expect(
      chain.methods.filter((method) => method === 'paxeer_sendTransactionReceipted')
    ).toHaveLength(1);
    expect(chain.methods).not.toContain('eth_sendTransaction');
    expect(chain.methods).not.toContain('eth_getTransactionReceipt');
    const sendIndex = chain.methods.indexOf('paxeer_sendTransactionReceipted');
    expect(chain.methods.slice(sendIndex + 1)).toContain('eth_call');

    await click(choice(container, 'pax_gas'), 'pax gas');
    expect(text(container, '[data-role="fee-denom"]')).toBe(`Fee token: ${SIDIORA_FEE_DENOM}`);
    await click(container.querySelector('[data-action="fee-preference"]'), 'use PAX');
    await until(
      () => text(container, '[data-role="fee-denom"]') === noPreference,
      'receipted cleared preference'
    );
    expect(text(container, '[data-role="preference-rung"]')).toBe('receipted');
    expect(container.querySelector('[data-role="fee-blocked"]')).toBeNull();
    expect(chain.sent[1]).toEqual({
      from: chain.address,
      to: FEE_TOKEN_PRECOMPILE,
      data: feeToken(chain).clearFeeDenom().data,
      value: '0x0',
    });
    expect(
      chain.methods.filter((method) => method === 'paxeer_sendTransactionReceipted')
    ).toHaveLength(2);
    expect(chain.methods).not.toContain('eth_getTransactionReceipt');
  }

  test(fee_choice_applies_the_preference_once_the_change_is_receipted);

  it('fee_choice_switches_the_fee_token_preference_through_the_precompile', async () => {
    mounted = await mountSurface(chain, <Harness legs={[]} />);
    const { container } = mounted;
    await until(
      () => text(container, '[data-role="fee-denom"]') === 'Fee token: none, gas is paid in PAX',
      'fee denom read'
    );
    expect(container.querySelector('[data-role="fee-blocked"]')).toBeNull();
    expect(text(container, '[data-role="layerx-none"]')).toBe('This action has no LayerX leg');
    const getFeeDenom = feeToken(chain).getFeeDenomCallData(chain.address).toLowerCase();
    expect(chain.calls).toContainEqual({ to: FEE_TOKEN_PRECOMPILE, data: getFeeDenom });

    await click(choice(container, 'sid_native'), 'sid native');
    expect(text(container, '[data-role="fee-blocked"]')).toBe(
      `set ${SIDIORA_FEE_DENOM} as the fee token first`
    );
    const action = () => container.querySelector('[data-action="fee-preference"]');
    expect(action()?.textContent).toBe('Use SID for gas');
    await click(action(), 'use SID');
    await until(
      () => text(container, '[data-role="fee-denom"]') === `Fee token: ${SIDIORA_FEE_DENOM}`,
      'fee denom set'
    );
    expect(chain.sent).toEqual([
      {
        from: chain.address,
        to: FEE_TOKEN_PRECOMPILE,
        data: feeToken(chain).setFeeDenom(SIDIORA_FEE_DENOM).data,
        value: '0x0',
      },
    ]);
    expect(chain.sent[0]?.data.startsWith(abiSelector('setFeeDenom(string)'))).toBe(true);
    expect(container.querySelector('[data-role="fee-blocked"]')).toBeNull();

    await click(choice(container, 'pax_gas'), 'pax gas');
    expect(text(container, '[data-role="fee-blocked"]')).toBe(
      'clear the fee token preference first'
    );
    expect(action()?.textContent).toBe('Use PAX for gas');
    await click(action(), 'use PAX');
    await until(
      () => text(container, '[data-role="fee-denom"]') === 'Fee token: none, gas is paid in PAX',
      'fee denom cleared'
    );
    expect(chain.sent[1]).toEqual({
      from: chain.address,
      to: FEE_TOKEN_PRECOMPILE,
      data: feeToken(chain).clearFeeDenom().data,
      value: '0x0',
    });
    expect(chain.sent[1]?.data).toBe(abiSelector('clearFeeDenom()'));

    await click(choice(container, 'sid_sponsored'), 'sid sponsored');
    expect(text(container, '[data-role="fee-blocked"]')).toBe(
      'SID sponsored fees need the embedded wallet. Sign in with the embedded wallet or pay gas in PAX.'
    );
    expect(text(container, '[data-role="fee-blocked"]')).not.toContain('refused:');
    expect(container.querySelector('[data-role="fee-blocked"]')?.getAttribute('data-reason')).toBe(
      'refused:safe_first_delegation'
    );
    expect(action()).toBeNull();
  });
});
