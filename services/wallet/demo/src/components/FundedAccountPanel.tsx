'use client';

import { useEffect, useMemo, useState } from 'react';
import {
  EndpointClient,
  HumanClient,
  KernelAvailability,
  PaxeerProvider,
  WalletInterface,
  type Eip6963ProviderDetail,
  type Hex,
  type PaxeerWallet,
  type PlanIntentRequest,
  type TypedDataPayload,
} from '@paxeer/wallet';
import { FormField } from './FormField';
import { WalletForm } from './WalletForm';

export function unifiedOrigin(): string {
  const value = process.env.NEXT_PUBLIC_PAXEER_WALLET_API;
  if (!value)
    throw new Error('Configure NEXT_PUBLIC_PAXEER_WALLET_API as the canonical unified API origin.');
  const url = new URL(value);
  if (
    !['https:', 'http:'].includes(url.protocol) ||
    url.username ||
    url.password ||
    url.pathname !== '/' ||
    url.search ||
    url.hash ||
    (url.protocol === 'http:' && !['localhost', '127.0.0.1', '[::1]'].includes(url.hostname))
  ) {
    throw new Error(
      'The unified API requires an HTTPS origin, or a loopback HTTP origin, without a path or credentials.'
    );
  }
  return url.origin;
}

export function SupportedWalletPanel({
  paxeer,
  origin,
  injected,
}: {
  paxeer: PaxeerWallet;
  origin: string;
  injected?: Eip6963ProviderDetail;
}) {
  const [account, setAccount] = useState<Hex | null>(null);
  const [busy, setBusy] = useState(false);
  const [result, setResult] = useState('Connect explicitly to begin.');
  const [failed, setFailed] = useState(false);
  const [to, setTo] = useState('');
  const [value, setValue] = useState('');
  const [data, setData] = useState('0x');
  const [typed, setTyped] = useState('');
  const [plan, setPlan] = useState('');

  const token = useMemo(
    () => async () => (await paxeer.getSession())?.access_token ?? null,
    [paxeer]
  );
  const provider = useMemo(
    () =>
      injected?.provider ??
      new PaxeerProvider({
        gatewayUrl: origin,
        rpcUrl: `${origin}/`,
        token,
        confirm: (request) =>
          window.confirm(`Approve ${request.method}?\n${JSON.stringify(request.params)}`),
      }),
    [injected, origin, token]
  );
  const wallet = useMemo(() => new WalletInterface(provider, injected?.info), [provider, injected]);
  const endpoint = useMemo(
    () =>
      new EndpointClient({
        url: `${origin}/`,
        fetch: async (input, init) => {
          const authorization = await token();
          const headers = new Headers(init?.headers);
          if (authorization) headers.set('Authorization', `Bearer ${authorization}`);
          return fetch(input, { ...init, headers });
        },
      }),
    [origin, token]
  );
  const kernel = useMemo(() => new KernelAvailability(endpoint), [endpoint]);
  const human = useMemo(
    () => new HumanClient({ url: origin, kernel, authorization: token }),
    [origin, kernel, token]
  );

  useEffect(() => {
    const reset = () => {
      setAccount(null);
      setResult('Wallet changed. Connect again.');
    };
    provider.on('accountsChanged', reset);
    provider.on('chainChanged', reset);
    return () => {
      provider.removeListener('accountsChanged', reset);
      provider.removeListener('chainChanged', reset);
      if (provider instanceof PaxeerProvider) provider.disconnect();
    };
  }, [provider]);

  useEffect(() => {
    try {
      for (let index = localStorage.length - 1; index >= 0; index -= 1) {
        const key = localStorage.key(index);
        if (key?.startsWith('paxeer-demo:custody:125:')) localStorage.removeItem(key);
      }
    } catch (cause) {
      setFailed(true);
      setResult(cause instanceof Error ? cause.message : 'Retained custody cleanup unavailable');
    }
  }, []);

  async function action(run: () => Promise<unknown>) {
    if (busy) return;
    setBusy(true);
    setFailed(false);
    try {
      const answer = await run();
      setResult(
        typeof answer === 'string'
          ? answer
          : JSON.stringify(
              answer,
              (_key, item: unknown) => (typeof item === 'bigint' ? item.toString() : item),
              2
            )
      );
    } catch (cause) {
      setFailed(true);
      setResult(cause instanceof Error ? cause.message : 'Action unavailable');
    } finally {
      setBusy(false);
    }
  }

  async function connected() {
    const chain = await wallet.chainId();
    if (chain !== 125)
      throw new Error(`Wrong network: ${chain}. Select HyperPaxeer chain 125 in your wallet.`);
    const accounts = await wallet.accounts();
    if (!accounts[0]) throw new Error('The wallet exposed no account.');
    if (account && accounts[0].toLowerCase() !== account.toLowerCase())
      throw new Error('Account changed. Connect again.');
    return accounts[0];
  }

  const disabled = busy || !account;
  return (
    <section data-supported-wallet={wallet.mode} className="stack" aria-busy={busy}>
      <p className="small-copy muted">
        {wallet.mode === 'embedded'
          ? 'Embedded threshold custody. The gateway coordinates authenticated threshold signing.'
          : 'Injected wallet. Your wallet controls signing and custody.'}
      </p>
      <button
        type="button"
        className="button primary"
        data-wallet-accounts
        disabled={busy}
        onClick={() =>
          action(async () => {
            const address = await connected();
            setAccount(address);
            return address;
          })
        }
      >
        Connect {wallet.mode} wallet
      </button>
      {account && <code className="account-address">{account}</code>}
      <WalletForm
        className="form"
        aria-label="Send transaction"
        onSubmit={(event) => {
          event.preventDefault();
          if (disabled) return;
          void action(async () => {
            await connected();
            if (
              !/^0x[0-9a-fA-F]{40}$/.test(to) ||
              !/^(0|[1-9][0-9]*)$/.test(value) ||
              !/^0x(?:[0-9a-fA-F]{2})*$/.test(data)
            )
              throw new Error(
                'A valid recipient, integer wei amount and byte calldata are required.'
              );
            return wallet.sendTransaction({
              to: to as Hex,
              value: BigInt(value),
              data: data as Hex,
              chainId: 125,
            });
          });
        }}
      >
        <FormField
          label="Recipient"
          accessibleLabel="Transaction recipient"
          name="recipient"
          value={to}
          onValueChange={setTo}
          dataInput
          pattern="0x[0-9a-fA-F]{40}"
          disabled={busy}
          errorMessage="Enter a 0x address with 40 hexadecimal characters."
        />
        <FormField
          label="Amount in wei"
          name="value"
          value={value}
          onValueChange={setValue}
          dataInput
          numeric
          inputMode="numeric"
          pattern="(0|[1-9][0-9]*)"
          disabled={busy}
          errorMessage="Enter a whole wei amount without leading zeroes."
        />
        <FormField
          label="Canonical calldata"
          name="data"
          value={data}
          onValueChange={setData}
          dataInput
          multiline
          disabled={busy}
          validate={isCalldata}
          errorMessage="Enter 0x followed by complete hexadecimal byte pairs."
        />
        <button type="submit" className="button" data-wallet-send disabled={disabled}>
          Review and send transaction
        </button>
      </WalletForm>
      <WalletForm
        className="form"
        aria-label="Sign typed data"
        onSubmit={(event) => {
          event.preventDefault();
          if (disabled) return;
          void action(async () => {
            await connected();
            return wallet.signTypedData(JSON.parse(typed) as TypedDataPayload);
          });
        }}
      >
        <FormField
          label="Typed data JSON"
          name="typed-data"
          value={typed}
          onValueChange={setTyped}
          dataInput
          multiline
          disabled={busy}
          validate={isJson}
          errorMessage="Enter valid typed data JSON."
        />
        <button type="submit" className="button" data-wallet-typed disabled={disabled}>
          Review and sign typed data
        </button>
      </WalletForm>
      <WalletForm
        className="form"
        aria-label="Plan kernel intent"
        onSubmit={(event) => {
          event.preventDefault();
          if (disabled) return;
          void action(async () => {
            await connected();
            kernel.invalidate();
            const availability = await kernel.current();
            if (!availability.available) return availability;
            return human.planIntent(JSON.parse(plan) as PlanIntentRequest);
          });
        }}
      >
        <FormField
          label="Kernel intent JSON"
          name="kernel-intent"
          value={plan}
          onValueChange={setPlan}
          dataInput
          multiline
          disabled={busy}
          validate={isJson}
          errorMessage="Enter valid kernel intent JSON."
        />
        <button type="submit" className="button" data-kernel-plan disabled={disabled}>
          Read availability and plan intent
        </button>
      </WalletForm>
      <pre
        role={failed ? 'alert' : 'status'}
        aria-label="Wallet action result"
        tabIndex={0}
        data-wallet-result
        className={`result${failed ? ' error' : ''}`}
      >
        <code>{result}</code>
      </pre>
    </section>
  );
}

function isCalldata(value: string): boolean {
  return /^0x(?:[0-9a-fA-F]{2})*$/.test(value);
}

function isJson(value: string): boolean {
  try {
    JSON.parse(value);
    return true;
  } catch {
    return false;
  }
}
