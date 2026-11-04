import { privateKeyToAccount } from 'viem/accounts';

export const walletParentSigner = privateKeyToAccount(`0x${'01'.repeat(32)}`);
export const walletMessageTarget = 'sidiora:eip1193';

type WalletRequest = {
  id: string;
  request: { method: string; params?: unknown[] | Record<string, unknown> };
};

/** A committed wallet peer using DOM message delivery and the real EIP-191 signer. */
export class WalletParentFixture {
  readonly requests: WalletRequest[] = [];
  readonly ready: Promise<void>;
  holdSigning = false;
  private readyResolve!: () => void;
  private readonly listener: (event: MessageEvent) => void;

  constructor(
    readonly parent: Window,
    readonly child: Window,
    readonly handshake = true,
  ) {
    this.ready = new Promise((resolve) => {
      this.readyResolve = resolve;
    });
    this.listener = (event) => {
      const data = event.data;
      if (data?.target !== walletMessageTarget) return;
      if (data.type === 'ready') {
        if (this.handshake) this.send({ target: walletMessageTarget, type: 'ready' });
        this.readyResolve();
      }
      if (data.type === 'request') {
        const request = data as WalletRequest;
        this.requests.push(request);
        if (!this.holdSigning) void this.answer(request);
      }
    };
    parent.addEventListener('message', this.listener);
  }

  // jsdom implements outbound postMessage delivery but leaves MessageEvent.source
  // null. Replay browser-provided provenance with actual parent/sibling Window objects.
  send(data: unknown, source: Window = this.parent, origin = this.parent.location.origin) {
    this.child.dispatchEvent(new MessageEvent('message', { data, source, origin }));
  }

  async answer(request: WalletRequest) {
    if (request.request.method === 'personal_sign') {
      const params = request.request.params;
      if (!Array.isArray(params) || typeof params[0] !== 'string')
        throw new Error('Invalid signing request');
      const signature = await walletParentSigner.signMessage({ message: params[0] });
      this.send({
        target: walletMessageTarget,
        type: 'response',
        id: request.id,
        result: signature,
      });
    } else if (request.request.method === 'eth_accounts') {
      this.send({
        target: walletMessageTarget,
        type: 'response',
        id: request.id,
        result: [walletParentSigner.address],
      });
    } else {
      this.send({
        target: walletMessageTarget,
        type: 'response',
        id: request.id,
        error: { code: 4200, message: 'Unsupported method' },
      });
    }
  }

  accountsChanged(source: Window = this.parent, origin = this.parent.location.origin) {
    this.send(
      {
        target: walletMessageTarget,
        type: 'event',
        event: 'accountsChanged',
        params: [walletParentSigner.address],
      },
      source,
      origin,
    );
  }

  close() {
    this.parent.removeEventListener('message', this.listener);
  }
}
