import { expect, test } from 'vitest';
import { PaxeerProvider } from '../src/provider.js';
import { WalletInterface } from '../src/wallet.js';

function wallet_interface_has_no_sign_custody(): void {
  const provider = new PaxeerProvider({
    gatewayUrl: 'https://gateway.invalid',
    rpcUrl: 'https://rpc.invalid',
    token: () => null,
  });
  const wallet = new WalletInterface(provider);

  expect(Reflect.get(wallet, 'signCustody')).toBeUndefined();
  expect('signCustody' in wallet).toBe(false);
}

test(wallet_interface_has_no_sign_custody);
