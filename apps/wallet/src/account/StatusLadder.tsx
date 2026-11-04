'use client';

import { useEffect, useState } from 'react';
import {
  JsonRpcClient,
  LADDER_RUNGS,
  readExplorerStatus,
  readReceiptKeyPins,
  readSequencerReceipt,
  ReceiptTimeoutError,
  statusLadder,
  type SequencerReceiptAnswer,
  type ReceiptKeyPin,
  type LadderRung,
  type LadderSource,
  type LadderStep,
} from '@paxeer/wallet';
import { ErrorState, Skeleton } from '@/components/ui/primitives';
import { cn } from '@/lib/cn';
import { useAccountClients } from './AccountProvider';
import { errorMessage, useAsync } from './hooks';

const RUNG_LABELS: Readonly<Record<LadderRung, string>> = {
  submitted: 'Submitted',
  receipted: 'Receipted',
  committed: 'Committed',
};
const SOURCE_LABELS: Readonly<Record<LadderSource, string>> = {
  receipt: 'receipt',
  explorer: 'explorer',
  journey: 'journey',
};

function rank(rung: LadderRung | null): number {
  return rung === null ? -1 : LADDER_RUNGS.indexOf(rung);
}

export function StatusLadder({ steps }: { steps: readonly LadderStep[] }) {
  for (const step of steps) {
    if (
      (step.rung !== null && !LADDER_RUNGS.includes(step.rung)) ||
      !Object.hasOwn(SOURCE_LABELS, step.source) ||
      typeof step.state !== 'string'
    )
      throw new TypeError('the status ladder carries malformed evidence');
  }
  return (
    <ol aria-label="Status" className="flex gap-2">
      {LADDER_RUNGS.map((rung) => {
        const reached = steps.filter((step) => rank(step.rung) >= rank(rung));
        const evidence =
          (rung === 'committed'
            ? reached.find((step) => step.rung === 'committed' && step.source === 'explorer')
            : undefined) ??
          reached.find((step) => step.rung === rung) ??
          statusLadder.highest(reached);
        const source = evidence?.source ?? null;
        return (
          <li
            key={rung}
            data-rung={rung}
            data-reached={source !== null}
            data-source={source ?? undefined}
            data-evidence={evidence?.state}
            title={evidence?.state}
            className={cn(
              'flex-1 rounded-xl px-3 py-2 text-center text-xs',
              source !== null ? 'bg-pax-accent/20 text-pax-light' : 'bg-white/[0.04] text-pax-muted'
            )}
          >
            <span className="block font-semibold">{RUNG_LABELS[rung]}</span>
            {source !== null && (
              <span className="block text-[10px] text-pax-muted">{SOURCE_LABELS[source]}</span>
            )}
          </li>
        );
      })}
    </ol>
  );
}

type ReceiptObservation = {
  key: string;
  receipt: SequencerReceiptAnswer | null;
  steps: readonly LadderStep[];
  busy: boolean;
  observed: boolean;
  error: unknown;
  version: number;
};

export interface TransactionLadderProps {
  hash: string;
  observation?: { attempts: number; intervalMs: number };
}

export function TransactionLadder({
  hash,
  observation = { attempts: 30, intervalMs: 1000 },
}: TransactionLadderProps) {
  const { config, endpoint, fetch: fetchImpl } = useAccountClients();
  const key = `${config.endpointUrl}:${hash.toLowerCase()}`;
  const initial: ReceiptObservation = {
    key,
    receipt: null,
    steps: [],
    busy: true,
    observed: false,
    error: null,
    version: 0,
  };
  const [state, setState] = useState<ReceiptObservation>(initial);
  const [generation, setGeneration] = useState(0);
  const receipt = state.key === key ? state : initial;
  const { attempts, intervalMs } = observation;
  useEffect(() => {
    const controller = new AbortController();
    let timer: ReturnType<typeof setTimeout> | undefined;
    setState({
      key,
      receipt: null,
      steps: [],
      busy: true,
      observed: false,
      error: null,
      version: 0,
    });
    const rpc = new JsonRpcClient({
      url: endpoint.rpc.url,
      fetch: (input, init) =>
        fetchImpl(input, {
          ...init,
          signal: AbortSignal.any([controller.signal, AbortSignal.timeout(10_000)]),
        }),
    });
    async function observe() {
      if (
        !Number.isSafeInteger(attempts) ||
        attempts < 1 ||
        attempts > 30 ||
        !Number.isFinite(intervalMs) ||
        intervalMs < 0 ||
        intervalMs > 10_000
      )
        throw new RangeError('receipt observation requires 1–30 attempts and a bounded interval');
      let last: SequencerReceiptAnswer | null = null;
      for (let attempt = 0; attempt < attempts; attempt += 1) {
        if (controller.signal.aborted) return;
        let pins: readonly ReceiptKeyPin[] = [];
        const call = async (method: string, params: readonly unknown[]) => {
          const value = await rpc.call(method, params);
          if (method === 'px_getReceiptKeys') pins = await readReceiptKeyPins(async () => value);
          return value;
        };
        last = await readSequencerReceipt(call, hash);
        const steps = last ? statusLadder.fromSequencerReceipt(last, pins) : [];
        if (controller.signal.aborted) return;
        const committed = last?.rung === 'committed';
        setState({
          key,
          receipt: last,
          steps,
          busy: !committed,
          observed: true,
          error: null,
          version: attempt + 1,
        });
        if (last?.status === '0x0')
          throw new Error(`The chain refused the transaction: ${last.vmError}`);
        if (committed) return;
        if (attempt + 1 < attempts) {
          await new Promise<void>((resolve) => {
            const finish = () => {
              clearTimeout(timer);
              controller.signal.removeEventListener('abort', finish);
              resolve();
            };
            controller.signal.addEventListener('abort', finish, { once: true });
            timer = setTimeout(finish, intervalMs);
          });
        }
      }
      throw new ReceiptTimeoutError(last);
    }
    void observe().catch((error: unknown) => {
      if (!controller.signal.aborted)
        setState((current) => ({ ...current, busy: false, observed: true, error }));
    });
    return () => {
      controller.abort();
      clearTimeout(timer);
    };
  }, [key, hash, endpoint, fetchImpl, attempts, intervalMs, generation]);
  const explorer = useAsync(async () => {
    if (!receipt.observed) return null;
    return readExplorerStatus(config.explorerUrl, hash, fetchImpl);
  }, `explorer:${key}:${config.explorerUrl}:${receipt.observed}:${receipt.version}:${generation}`);
  if (!receipt.observed) return <Skeleton className="h-10 w-full" />;
  const steps = [...receipt.steps];
  if (explorer.status === 'ready' && explorer.value !== null) {
    const status = explorer.value;
    if (
      status.rung !== 'committed' ||
      (status.block_number !== null &&
        receipt.receipt?.blockNumber !== undefined &&
        BigInt(status.block_number) === BigInt(receipt.receipt.blockNumber))
    ) {
      steps.push(statusLadder.fromExplorer(status));
    }
  }
  return (
    <div
      aria-busy={receipt.busy || explorer.status === 'loading'}
      data-observation={receipt.error ? 'error' : receipt.busy ? 'watching' : 'committed'}
      data-explorer={explorer.status}
    >
      <StatusLadder steps={steps} />
      {receipt.busy && (
        <p role="status" className="text-xs text-pax-muted">
          Checking for block commitment…
        </p>
      )}
      {receipt.error !== null && (
        <ErrorState
          title="The transaction status could not be read"
          message={errorMessage(receipt.error)}
          onRetry={() => setGeneration((value) => value + 1)}
        />
      )}
      {explorer.status === 'error' && (
        <ErrorState
          title="The explorer status could not be read"
          message={errorMessage(explorer.error)}
          onRetry={explorer.reload}
        />
      )}
    </div>
  );
}
