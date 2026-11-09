'use client';

import { useEffect, useMemo, useState, type ReactNode } from 'react';
import { formatUnits, parseAbi, decodeEventLog, decodeFunctionData, type Hex } from 'viem';
import { sendModuleTransaction, type ModuleTransaction } from '@paxeer/wallet';
import {
  fundAsk,
  inferenceId,
  type InferenceRoom,
  type InferencePeer,
} from '../../../../services/agent/sdk/typescript/src/inference_market';
import { TextField } from '@/components/ui/primitives';
import { FeeChoice } from './FeeChoice';
import {
  BuildError,
  SendButton,
  SentResult,
  SurfaceFrame,
  TransactionPreview,
  buildOrError,
} from './Surface';
import {
  inferenceRewardSource,
  parseSurfaceAmount,
  useFeeSelection,
  useInferenceAsks,
  useInferenceEarnings,
  useModuleSend,
  useSurfaceWallet,
  verifyInferenceSettlement,
  type InferenceCredit,
  type InferenceRead,
  type InferenceRewardSource,
  type SurfaceModule,
} from './useSurface';

const STREAM_EVENTS = parseAbi([
  'event AskStreamOpened(bytes32 indexed askRef, bytes32 indexed streamId, address settlementAccount)',
  'event StreamOpened(bytes32 indexed streamId, bytes32 payer, bytes32 streamAccount, bytes32 recipient, bytes32 assetId, uint8 mode, uint128 funding)',
]);
const FUNDING_ABI = parseAbi([
  'function streamOpenForAsk(bytes32 askRef, address settlementAccount, uint256 rate, uint8 rateUnit, uint256 totalCap, bytes32 hostAuthorityKey) returns (bytes32)',
]);

export function buildInferenceFunding(
  room: InferenceRoom,
  askRef: string,
  settlementAddress: string,
  pricePerThousand: string,
  totalCap: string
): ModuleTransaction {
  if (room.state !== 0) throw new Error('Choose an open room.');
  inferenceId(askRef);
  const ask = room.askStreams.find((entry) => entry.askId === askRef);
  if (!ask || !room.openAsks.includes(askRef)) throw new Error('Choose an open ask in this room.');
  if (ask.stream !== null) throw new Error('This ask already has a funded stream.');
  const price = parseSurfaceAmount(pricePerThousand, 18);
  if (price === 0n || price % 1000n !== 0n)
    throw new Error('Enter a positive price per thousand with at most 15 decimal places.');
  return fundAsk({
    askRef,
    settlementAccount: settlementAddress,
    rate: price / 1000n,
    rateUnit: 1,
    totalCap: parseSurfaceAmount(totalCap, 18),
    hostAuthorityKey: room.hostKey,
  });
}

export function InferenceAskForm({
  rooms,
  children,
}: {
  readonly rooms: readonly InferenceRoom[];
  readonly children: (
    transaction: ModuleTransaction | null,
    room: InferenceRoom | null
  ) => ReactNode;
}) {
  const [roomId, setRoomId] = useState('');
  const [askRef, setAskRef] = useState('');
  const [settlement, setSettlement] = useState('');
  const [price, setPrice] = useState('');
  const [totalCap, setTotalCap] = useState('');
  const room = rooms.find((entry) => entry.roomId === roomId) ?? null;
  const built = useMemo(
    () =>
      buildOrError(() =>
        room && askRef && settlement && price && totalCap
          ? buildInferenceFunding(room, askRef, settlement, price, totalCap)
          : null
      ),
    [room, askRef, settlement, price, totalCap]
  );
  return (
    <form
      aria-label="Fund an ask"
      onSubmit={(event) => event.preventDefault()}
      className="space-y-4"
    >
      <label className="block space-y-2 text-sm text-pax-light">
        Model and room
        <select
          name="model"
          value={roomId}
          onChange={(event) => {
            setRoomId(event.target.value);
            setAskRef('');
            setSettlement('');
          }}
          className="w-full rounded-xl bg-[var(--color-surface-control)] p-3"
        >
          <option value="">Choose a model and room</option>
          {rooms
            .filter((entry) => entry.state === 0)
            .map((entry) => (
              <option key={entry.roomId} value={entry.roomId}>
                {entry.modelDigest} · {entry.roomId}
              </option>
            ))}
        </select>
      </label>
      <label className="block space-y-2 text-sm text-pax-light">
        Open ask
        <select
          name="askRef"
          value={askRef}
          onChange={(event) => setAskRef(event.target.value)}
          className="w-full rounded-xl bg-[var(--color-surface-control)] p-3"
        >
          <option value="">Choose an unfunded ask</option>
          {room?.askStreams
            .filter((entry) => entry.stream === null)
            .map((entry) => (
              <option key={entry.askId} value={entry.askId}>
                {entry.askId}
              </option>
            ))}
        </select>
      </label>
      {room && (
        <p className="break-all text-xs text-pax-muted">
          Settlement account: {room.profile.settlementAccount}
        </p>
      )}
      <TextField
        label="Settlement wallet address"
        name="settlementAddress"
        value={settlement}
        onChange={(event) => setSettlement(event.target.value)}
      />
      <TextField
        label="Price cap per thousand tokens (PAX)"
        name="priceCap"
        inputMode="decimal"
        value={price}
        onChange={(event) => setPrice(event.target.value)}
      />
      <TextField
        label="Total cap (PAX)"
        name="totalCap"
        inputMode="decimal"
        value={totalCap}
        onChange={(event) => setTotalCap(event.target.value)}
      />
      <BuildError error={built.error} />
      <TransactionPreview transaction={built.value} />
      {children(built.value, room)}
    </form>
  );
}

export function InferenceAskList({ rooms }: { readonly rooms: readonly InferenceRoom[] }) {
  const asks = rooms.flatMap((room) => room.askStreams.map((ask) => ({ room, ask })));
  return (
    <section aria-label="Inference asks" className="space-y-3">
      <h2 className="font-semibold text-pax-light">Asks</h2>
      {asks.length === 0 && <p className="text-sm text-pax-muted">No open asks.</p>}
      <ul className="space-y-3">
        {asks.map(({ room, ask }) => (
          <li
            key={`${room.roomId}:${ask.askId}`}
            data-ask={ask.askId}
            className="space-y-2 rounded-2xl bg-[var(--color-surface-card)] p-4 text-xs text-pax-light"
          >
            <p className="break-all">Model: {room.modelDigest}</p>
            <p className="break-all">Ask: {ask.askId}</p>
            {ask.stream === null ? (
              <p>Stream not funded</p>
            ) : (
              <>
                <p className="break-all">Stream: {ask.streamId}</p>
                <dl className="space-y-1">
                  <div>
                    <dt>Stream reading (tokens)</dt>
                    <dd data-field="reading">{ask.stream.cumulativeMeter.toString()}</dd>
                  </div>
                  <div>
                    <dt>Remaining cap (PAX)</dt>
                    <dd data-field="remaining-cap">
                      {formatUnits(ask.stream.totalCap - ask.stream.accruedTotal, 18)}
                    </dd>
                  </div>
                  <div>
                    <dt>Settled (PAX)</dt>
                    <dd data-field="settled">{formatUnits(ask.stream.settledTotal, 18)}</dd>
                  </div>
                  <div>
                    <dt>Status</dt>
                    <dd>
                      {ask.stream.closed
                        ? 'Closed'
                        : ask.stream.paused
                          ? 'Paused'
                          : ask.stream.underfunded
                            ? 'Underfunded'
                            : 'Open'}
                    </dd>
                  </div>
                </dl>
              </>
            )}
          </li>
        ))}
      </ul>
    </section>
  );
}

export function InferenceEarnings({
  earnings,
}: {
  readonly earnings: InferenceRead<{
    readonly peer: InferencePeer;
    readonly credit: InferenceRead<InferenceCredit>;
  }>;
}) {
  return (
    <section
      aria-label="Peer earnings"
      className="space-y-2 rounded-2xl bg-[var(--color-surface-card)] p-4 text-xs text-pax-light"
    >
      <h2 className="text-base font-semibold">Peer earnings</h2>
      {earnings.state === 'loading' ? (
        <p role="status">Loading peer earnings…</p>
      ) : earnings.state === 'unavailable' ? (
        <p role="status">{earnings.reason}</p>
      ) : (
        <>
          <p className="break-all">Peer: {earnings.value.peer.peerKey}</p>
          <p>Enrolled: {earnings.value.peer.enrolled ? 'Yes' : 'No'}</p>
          <p>Faults: {earnings.value.peer.faultCount}</p>
          {earnings.value.credit.state === 'loading' ? (
            <p>Loading credits…</p>
          ) : earnings.value.credit.state === 'unavailable' ? (
            <p role="status">{earnings.value.credit.reason}</p>
          ) : (
            <dl className="space-y-1">
              <div>
                <dt>Reward asset</dt>
                <dd className="break-all">{earnings.value.credit.value.asset}</dd>
              </div>
              <div>
                <dt>Credited (base units)</dt>
                <dd data-field="credit">{earnings.value.credit.value.earned.toString()}</dd>
              </div>
              <div>
                <dt>Claimed (base units)</dt>
                <dd data-field="claimed">{earnings.value.credit.value.claimed.toString()}</dd>
              </div>
              <div>
                <dt>Claimable (base units)</dt>
                <dd data-field="claimable">
                  {(
                    earnings.value.credit.value.earned - earnings.value.credit.value.claimed
                  ).toString()}
                </dd>
              </div>
            </dl>
          )}
        </>
      )}
    </section>
  );
}

function InferenceFunding({
  transaction,
  room,
  sidRate,
  refresh,
}: {
  readonly transaction: ModuleTransaction | null;
  readonly room: InferenceRoom | null;
  readonly sidRate: string | null;
  readonly refresh: () => void;
}) {
  const { provider, address } = useSurfaceWallet();
  const fee = useFeeSelection(provider, address);
  const surfaceModule = useMemo<SurfaceModule | null>(
    () =>
      provider
        ? {
            address: '0x000000000000000000000000000000000000101b',
            send: (from, tx) => sendModuleTransaction(provider, from, tx),
            decodeEvent: (log) => {
              const decoded = decodeEventLog({
                abi: STREAM_EVENTS,
                data: log.data as Hex,
                topics: [...log.topics] as [Hex, ...Hex[]],
                strict: true,
              });
              return {
                event: decoded.eventName,
                fields: Object.fromEntries(
                  Object.entries(decoded.args).map(([name, value]) => [
                    name,
                    typeof value === 'number' ? BigInt(value) : value,
                  ])
                ),
              };
            },
          }
        : null,
    [provider]
  );
  const approval = useMemo(
    () => async (_owner: string, tx: ModuleTransaction) => {
      if (!provider || !room) throw new Error('Choose a room.');
      const {
        args: [, settlementAddress],
      } = decodeFunctionData({ abi: FUNDING_ABI, data: tx.data as Hex });
      await verifyInferenceSettlement(provider, settlementAddress, room.profile.settlementAccount);
      return undefined;
    },
    [provider, room]
  );
  const state = useModuleSend(surfaceModule, provider, address, fee, approval);
  useEffect(() => {
    if (state.sent) refresh();
  }, [state.sent, refresh]);
  return (
    <>
      <FeeChoice
        provider={provider}
        address={address}
        transaction={transaction}
        selection={fee}
        sidRate={sidRate}
      />
      <SendButton label="Fund ask" transaction={transaction} blocked={fee.blocked} state={state} />
      <SentResult state={state} />
    </>
  );
}

export function InferenceView({
  sidRate = null,
  rewardSource,
}: {
  readonly sidRate?: string | null;
  readonly rewardSource?: InferenceRewardSource | null;
}) {
  const { provider, address } = useSurfaceWallet();
  const { asks, refresh } = useInferenceAsks(provider, address);
  const source = useMemo(
    () => (rewardSource === undefined ? inferenceRewardSource() : rewardSource),
    [rewardSource]
  );
  const earnings = useInferenceEarnings(provider, address, source);
  return (
    <SurfaceFrame title="Inference" connected={provider !== null && address !== null}>
      {asks.state === 'loading' ? (
        <p role="status">Loading inference rooms…</p>
      ) : asks.state === 'unavailable' ? (
        <p role="alert">{asks.reason}</p>
      ) : (
        <>
          <InferenceAskForm rooms={asks.value}>
            {(transaction, room) => (
              <InferenceFunding
                transaction={transaction}
                room={room}
                sidRate={sidRate}
                refresh={refresh}
              />
            )}
          </InferenceAskForm>
          <InferenceAskList rooms={asks.value} />
        </>
      )}
      <InferenceEarnings earnings={earnings} />
    </SurfaceFrame>
  );
}
