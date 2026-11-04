import type { FastifyInstance } from 'fastify';
import { keccak256, type Hex } from 'viem';
import { TransactionRejectedError, type RpcPool } from '../rpc/pool.js';
import { ApprovalRecovery, type RecoveryEntry } from './approvalRecovery.js';

export class HumanSendRecovery {
  private readonly store: ApprovalRecovery;
  constructor(
    private readonly app: FastifyInstance,
    pool: ApprovalRecovery['pool'],
    private readonly rpc: RpcPool
  ) {
    this.store = new ApprovalRecovery(pool);
  }
  async retain(signatureId: string, raw: Hex): Promise<void> {
    const hash = keccak256(raw);
    await this.store.save({
      domain: 'human',
      approval_id: signatureId,
      binding: hash,
      phase: 'signed',
      evidence: { raw, tx_hash: hash, signature_id: signatureId },
    });
    const saved = await this.store.pool.query(
      'update wallet_signatures set tx_hash=$2 where id=$1',
      [signatureId, hash]
    );
    if (saved.rowCount !== 1) throw new Error('human send reservation vanished before submission');
  }
  async rejected(signatureId: string): Promise<void> {
    const entry = await this.store.read('human', signatureId);
    if (entry) await this.store.save({ ...entry, phase: 'rejected' });
  }
  async reconcile(): Promise<void> {
    const pending = await this.store.pool.query<RecoveryEntry>(
      "select * from wallet_approval_recovery where domain='human' and phase='signed'"
    );
    for (const entry of pending.rows) {
      await this.store
        .locked('human', entry.approval_id, async () => {
          const current = await this.store.read('human', entry.approval_id);
          if (!current || current.phase !== 'signed') return;
          const { raw, tx_hash: hash } = current.evidence;
          if (!raw || !hash || keccak256(raw) !== hash)
            throw new Error('human send recovery evidence mismatch');
          // A receipt settles the reservation; spend remains in its original daily window.
          const receipt = await this.rpc.sequencerReceipt(hash);
          if (receipt !== null) {
            if (receipt.rung !== 'submitted')
              await this.store.save({ ...current, phase: 'settled' });
            return;
          }
          // Recover only the retained signed bytes, never a freshly prepared payment.
          try {
            const sent = await this.rpc.sendRawTransactionVia(raw);
            if (sent.hash !== hash) throw new Error('human send recovery hash mismatch');
          } catch (error) {
            // A later refusal cannot disprove acceptance of an earlier uncertain write.
            if (!(error instanceof TransactionRejectedError)) throw error;
          }
        })
        .catch((error) =>
          this.app.log.warn(
            { err: error, signature: entry.approval_id },
            'human send reconciliation deferred'
          )
        );
    }
  }
  async start(): Promise<void> {
    await this.store.initialize();
    let running: Promise<void> | null = null;
    const resume = (): Promise<void> => {
      if (running) return running;
      running = this.reconcile()
        .catch((error) => this.app.log.warn({ err: error }, 'human send recovery unavailable'))
        .finally(() => {
          running = null;
        });
      return running;
    };
    this.app.addHook('onReady', async () => {
      await resume();
    });
    const timer = setInterval(() => {
      void resume();
    }, 5_000);
    timer.unref();
    this.app.addHook('onClose', async () => {
      clearInterval(timer);
      await running;
    });
  }
}
