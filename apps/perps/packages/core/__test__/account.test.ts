import { DefaultEVMWalletAdapter } from "../../default-evm-adapter/src";
import { EthersProvider } from "../../web3-provider-ethers/src";
import { Account } from "../src/account";
import { DefaultConfigStore } from "../src/configStore/defaultConfigStore";
import { LocalStorageStore } from "../src/keyStore";
import { AccountStatusEnum } from "@paxeer-network/types";

test("accounts start disconnected without a wallet", async () => {
  const account = new Account(
    new DefaultConfigStore({}),
    new LocalStorageStore(),
    [new DefaultEVMWalletAdapter(new EthersProvider())],
  );
  expect(account.stateValue.status).toBe(AccountStatusEnum.NotConnected);
  expect(account.stateValue.validating).toBe(false);
  expect(account.stateValue.address).toBeUndefined();
  await account.disconnect();
  expect(account.stateValue.status).toBe(AccountStatusEnum.NotConnected);
  expect(account.stateValue.subAccounts).toEqual([]);
});
