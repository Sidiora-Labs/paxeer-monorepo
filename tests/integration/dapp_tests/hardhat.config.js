require("@nomiclabs/hardhat-waffle");
require("@nomiclabs/hardhat-ethers");

function remoteNetwork(url) {
  const mnemonic = process.env.DAPP_TESTS_MNEMONIC;
  const accounts = mnemonic
    ? { mnemonic, path: "m/44'/118'/0'/0/0", initialIndex: 0, count: 1 }
    : undefined;

  return url && accounts ? { url, accounts } : undefined;
}

/** @type import('hardhat/config').HardhatUserConfig */
module.exports = {
  solidity: {
    version: "0.8.28",
    settings: {
      evmVersion: "cancun",
      optimizer: {
        enabled: true,
        runs: 1000,
      },
    },
  },
  mocha: {
    timeout: 100000000,
  },
  networks: {
    paxlocal: {
      url: process.env.PAXEER_LOCAL_EVM_RPC_URL || "http://127.0.0.1:8545",
      accounts: {
        mnemonic: "test test test test test test test test test test test junk",
        path: "m/44'/118'/0'/0/0",
        initialIndex: 0,
        count: 1,
      },
    },
    ...(remoteNetwork(process.env.PAXEER_TESTNET_EVM_RPC_URL)
      ? { testnet: remoteNetwork(process.env.PAXEER_TESTNET_EVM_RPC_URL) }
      : {}),
    ...(remoteNetwork(process.env.PAXEER_DEVNET_EVM_RPC_URL)
      ? { devnet: remoteNetwork(process.env.PAXEER_DEVNET_EVM_RPC_URL) }
      : {}),
  },
};
