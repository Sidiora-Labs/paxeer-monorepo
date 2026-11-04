import type { Hash, TransactionReceipt } from 'viem';

export interface RevokeSubmissionState {
  hash?: Hash;
  status: 'idle' | 'submitting' | 'pending' | 'confirmed' | 'failed';
  error?: unknown;
  rewards: 'idle' | 'pending' | 'delivered' | 'skipped' | 'failed';
  rewardsError?: unknown;
  rewardsAttempts: number;
}

interface RevokeSubmissionActions {
  submit: () => Promise<Hash>;
  receipt: (hash: Hash) => Promise<TransactionReceipt>;
  verify: (receipt: TransactionReceipt) => Promise<void>;
  registerRewards: () => Promise<{ token: string } | undefined>;
  confirmRewards: (hash: Hash, token: string, signal: AbortSignal) => Promise<unknown>;
}

export class RevokeSubmission {
  state: RevokeSubmissionState = { status: 'idle', rewards: 'idle', rewardsAttempts: 0 };
  rewardsDelivery?: Promise<boolean>;
  private submission?: Promise<boolean>;
  private rewardsToken?: string;
  private rewardsInFlight = false;

  constructor(
    private readonly actions: RevokeSubmissionActions,
    private readonly onChange: (state: RevokeSubmissionState) => void,
  ) {}

  private update(patch: Partial<RevokeSubmissionState>) {
    this.state = { ...this.state, ...patch };
    this.onChange(this.state);
  }

  submit(): Promise<boolean> {
    if (this.state.status === 'failed') {
      this.submission = this.state.hash ? this.followReceipt(this.actions.receipt(this.state.hash)) : this.send();
    } else {
      this.submission ??= this.send();
    }
    return this.submission;
  }

  private async send(): Promise<boolean> {
    this.update({ status: 'submitting' });
    try {
      const hash = await this.actions.submit();
      this.update({ hash, status: 'pending' });
      // Receipt tracking starts immediately and never waits for rewards delivery.
      const receipt = this.actions.receipt(hash);
      this.rewardsDelivery = this.retryRewards();
      return this.followReceipt(receipt);
    } catch (error) {
      this.update({ status: 'failed', error });
      return false;
    }
  }

  private async followReceipt(receipt: Promise<TransactionReceipt>): Promise<boolean> {
    this.update({ status: 'pending', error: undefined });
    try {
      const confirmed = await receipt;
      if (confirmed.status === 'reverted') throw new Error('Failed to revoke approval.');
      await this.actions.verify(confirmed);
      this.update({ status: 'confirmed' });
      return true;
    } catch (error) {
      this.update({ status: 'failed', error });
      return false;
    }
  }

  async retryRewards(): Promise<boolean> {
    if (!this.state.hash || this.rewardsInFlight || this.state.rewardsAttempts >= 3 ||
      this.state.rewards === 'delivered' || this.state.rewards === 'skipped') return false;
    this.rewardsInFlight = true;
    this.update({ rewards: 'pending', rewardsError: undefined, rewardsAttempts: this.state.rewardsAttempts + 1 });
    const controller = new AbortController();
    let timer: ReturnType<typeof setTimeout> | undefined;
    try {
      const delivery = async() => {
        if (!this.rewardsToken) {
          const registration = await this.actions.registerRewards();
          if (controller.signal.aborted) return;
          this.rewardsToken = registration?.token;
        }
        if (!this.rewardsToken) {
          this.update({ rewards: 'skipped' });
          return;
        }
        await this.actions.confirmRewards(this.state.hash!, this.rewardsToken, controller.signal);
        if (!controller.signal.aborted) this.update({ rewards: 'delivered' });
      };
      await Promise.race([
        delivery(),
        new Promise<never>((_, reject) => {
          timer = setTimeout(() => {
            controller.abort();
            reject(new Error('Rewards delivery timed out. Retry rewards without resubmitting the revoke.'));
          }, 10000);
        }),
      ]);
      return true;
    } catch (rewardsError) {
      this.update({ rewards: 'failed', rewardsError });
      return false;
    } finally {
      if (timer !== undefined) clearTimeout(timer);
      this.rewardsInFlight = false;
    }
  }
}
