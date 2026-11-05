import {
  NativePlatformPush,
  PushController,
  type DeviceProofIssuer,
  type NativePushProvider,
  type PlatformPush,
} from './push.ts';
import type { MobileServices } from './service-client.ts';
import type { SessionController } from './session.ts';
import type { WalletExecutionProvider } from './lifecycle.ts';

export function createApplicationPush(input: {
  session: SessionController;
  services: MobileServices;
  wallet:
    | Pick<
        WalletExecutionProvider,
        'supportedChain' | 'connectedAccount' | 'signMessage'
      >
    | undefined;
  native: NativePushProvider;
  unlocked: () => boolean;
  changed: () => void;
  platform?: PlatformPush;
  proofIssuer?: DeviceProofIssuer;
  authorizeDevice?: MobileServices['deviceAuthorization'];
}) {
  const wallet = input.wallet;
  const operationWallet = wallet && {
    get supportedChain() {
      return wallet.supportedChain;
    },
    get connectedAccount() {
      return wallet.connectedAccount;
    },
    signMessage: (message: string, signal: AbortSignal) =>
      wallet.signMessage(message, signal),
  };
  const authorize =
    input.authorizeDevice ??
    input.platform?.authorizeDevice ??
    (operationWallet
      ? input.services.createDeviceAuthorization(
          operationWallet,
          input.unlocked,
        )
      : undefined);
  input.services.deviceAuthorization = authorize;
  const platform =
    input.platform ??
    new NativePlatformPush(
      input.session,
      input.proofIssuer ??
        ((registration) => input.services.issueDeviceProof(registration)),
      authorize,
      input.native,
    );
  return new PushController(
    platform,
    input.session,
    input.services,
    input.changed,
  );
}
