import {
  JsonRpcClient,
  observeTransfer,
  submittedTransfer,
  type TransferIdentity,
  type TransferObservation,
} from '@paxeer/wallet';

export interface ExplorerProofTarget {
  rpcUrl: string;
  explorerUrl: string;
}

export function observeConfiguredTransfer(
  target: ExplorerProofTarget,
  previous: TransferObservation
): Promise<TransferObservation> {
  const rpc = new JsonRpcClient({ url: target.rpcUrl });
  return observeTransfer((method, params) => rpc.call(method, params), previous, {
    url: target.explorerUrl,
  });
}

export async function waitForIndexedCommit(
  target: ExplorerProofTarget,
  identity: TransferIdentity,
  timeoutMs = 60_000
): Promise<TransferObservation> {
  const deadline = Date.now() + timeoutMs;
  let observation = submittedTransfer(identity);
  do {
    observation = await observeConfiguredTransfer(target, observation);
    if (
      observation.truth === 'included' &&
      observation.steps.some((step) => step.source === 'explorer' && step.rung === 'committed')
    ) {
      return observation;
    }
    if (observation.truth === 'reverted' || observation.truth === 'replaced') {
      throw new Error(`The configured-chain transfer was ${observation.truth}`);
    }
    await new Promise((resolve) => setTimeout(resolve, 50));
  } while (Date.now() < deadline);
  throw new Error(
    `Explorer did not index the committed transfer: ${observation.warning ?? observation.truth}`
  );
}
