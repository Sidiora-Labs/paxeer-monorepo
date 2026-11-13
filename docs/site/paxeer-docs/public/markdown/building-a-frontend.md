<!-- Source: https://docs.paxeer.app/building-a-frontend/ -->

# Build a Frontend

Connect a browser wallet to Paxeer with React and viem, show PAX balances, and read contracts at a consistent block.

Start with one clear interaction: connect a wallet and display its PAX balance. viem separates public reads from wallet requests, so your page can load chain state before asking for a signature.

## Use the application workspace

The frontend in apps/launchpad/web uses Next.js, React, viem and Wagmi. From the repository root, install the launchpad workspace dependencies and run its web application.

```
cd apps/launchpad
pnpm install
pnpm dev
```

Configure `apps/launchpad/web/.env.local` with `NEXT_PUBLIC_PAXEER_RPC_URL` set to a browser-accessible node root. Next.js exposes `NEXT_PUBLIC_` values to visitors: use a public browser endpoint and keep provider credentials on a server. The launchpad RPC selector gives NEXT_PUBLIC_RPC_URL and NEXT_PUBLIC_RPC_URL_FALLBACK precedence over this value.

## Define Paxeer once

The chain definition in `apps/launchpad/web/src/core/wallet-sdk/client.ts` makes the chain ID and native currency explicit. For `hyperpax_125-1` the endpoint must answer `eth_chainId` with `0x7d` (125). The node mapping in `modules/evm/config/config.go` also defaults unmapped chain names to 125. Check the endpoint and wallet `id` before requesting a signature.

```
import { createPublicClient, defineChain, http } from "viem";
import { getPaxeerRpcUrl } from "@/core/rpc";

const rpcUrl = getPaxeerRpcUrl();
export const paxeer = defineChain({
  id: 125,
  name: "Paxeer X Network",
  nativeCurrency: { name: "PAX", symbol: "PAX", decimals: 18 },
  rpcUrls: { default: { http: [rpcUrl] } },
});
export const publicClient = createPublicClient({
  chain: paxeer,
  transport: http(rpcUrl),
});
```

## Connect and display a balance

Use this example as a client component alongside `apps/launchpad/web/src/core/wallet-sdk/client.ts`. It requests accounts only after a click, adds Paxeer if the wallet does not know the chain, and clears old results whenever the account or network changes.

```
"use client";

import { useEffect, useRef, useState } from "react";
import { createWalletClient, custom, formatEther, type EIP1193Provider } from "viem";
import { paxeerWalletChain as paxeer, getPublicClient } from "@/core/wallet-sdk/client";

const publicClient = getPublicClient();

function injectedProvider() {
  return (window as Window & { ethereum?: EIP1193Provider }).ethereum;
}

export default function App() {
  const [result, setResult] = useState<string>();
  const [error, setError] = useState<string>();
  const [busy, setBusy] = useState(false);
  const requestId = useRef(0);
  const connecting = useRef(false);
  const mounted = useRef(true);
  const selection = useRef<{ account?: string; chainId?: number }>({});

  useEffect(() => {
    mounted.current = true;
    const provider = injectedProvider();
    const reset = () => {
      requestId.current += 1;
      setResult(undefined);
      setError(undefined);
      selection.current = {};
    };
    const accountsChanged = (accounts: string[]) => {
      if (accounts[0]?.toLowerCase() !== selection.current.account) reset();
    };
    const chainChanged = (chainId: string) => {
      if (Number(chainId) !== selection.current.chainId) reset();
    };
    provider?.on("accountsChanged", accountsChanged);
    provider?.on("chainChanged", chainChanged);
    return () => {
      mounted.current = false;
      requestId.current += 1;
      provider?.removeListener("accountsChanged", accountsChanged);
      provider?.removeListener("chainChanged", chainChanged);
    };
  }, []);

  async function connect() {
    if (connecting.current) return;
    connecting.current = true;
    requestId.current += 1;
    setBusy(true);
    setError(undefined);
    setResult(undefined);
    let readId: number | undefined;
    try {
      const provider = injectedProvider();
      if (!provider) throw new Error("Open this page in a wallet browser or install an EVM wallet.");
      if (await publicClient.getChainId() !== 125) throw new Error("RPC endpoint is not Paxeer chain 125.");
      const wallet = createWalletClient({ chain: paxeer, transport: custom(provider) });
      const [requestedAccount] = await wallet.requestAddresses();
      selection.current.account = requestedAccount?.toLowerCase();
      if (await wallet.getChainId() !== 125) {
        try {
          await wallet.switchChain({ id: 125 });
        } catch (cause) {
          const failure = cause as { code?: number; cause?: { code?: number } };
          if (failure.code !== 4902 && failure.cause?.code !== 4902) throw cause;
          await wallet.addChain({ chain: paxeer });
          await wallet.switchChain({ id: 125 });
        }
      }
      if (!mounted.current) return;
      selection.current = { account: requestedAccount?.toLowerCase(), chainId: 125 };
      // Begin the read after the wallet has completed account and chain selection.
      const id = ++requestId.current;
      readId = id;
      const [account] = await wallet.getAddresses();
      if (!account) throw new Error("No wallet account selected.");
      if (await wallet.getChainId() !== 125) throw new Error("Select Paxeer in your wallet.");
      if (id !== requestId.current || !mounted.current) return;
      selection.current = { account: account.toLowerCase(), chainId: 125 };
      const balance = await publicClient.getBalance({ address: account });
      if (id === requestId.current && mounted.current) {
        setResult(account + ": " + formatEther(balance) + " PAX");
      }
    } catch (cause) {
      if (mounted.current && (readId === undefined || readId === requestId.current)) {
        setError(cause instanceof Error ? cause.message : "Wallet request failed.");
      }
    } finally {
      connecting.current = false;
      if (mounted.current) setBusy(false);
    }
  }

  return (
    <main>
      <h1>Paxeer wallet</h1>
      <button onClick={connect} disabled={busy}>{busy ? "Connecting…" : "Connect and read balance"}</button>
      <p role="status">{result}</p>
      {error && <p role="alert">{error}</p>}
    </main>
  );
}
```

Run `pnpm dev` from apps/launchpad, open the web application, and connect an EVM wallet. The initial connection waits for account and network selection before reading. Later selection changes clear the old result and invalidate a read in flight; click again to refresh. This example reads balances without sending a transaction.

## Read a deployed wrapped-token balance

Use the WPAX contract address produced by [Foundry](https://docs.paxeer.app/foundry) or [Hardhat](https://docs.paxeer.app/hardhat). The interface comes from `contracts/src/WPAX.sol`; pass the deployment address and account to this example helper.

```
import { getAddress, parseAbi } from "viem";
import { getPublicClient } from "@/core/wallet-sdk/client";

export async function readWrappedBalance(contractAddress: string, owner: string) {
  const publicClient = getPublicClient();
  if (await publicClient.getChainId() !== 125) throw new Error("Unexpected chain");
  const address = getAddress(contractAddress);
  const blockNumber = await publicClient.getBlockNumber();
  const bytecode = await publicClient.getCode({ address, blockNumber });
  if (!bytecode || bytecode === "0x") throw new Error("No contract at this address");
  const balance = await publicClient.readContract({
    address,
    abi: parseAbi(["function balanceOf(address) view returns (uint256)"]),
    functionName: "balanceOf",
    args: [getAddress(owner)],
    blockNumber,
  });
  return { blockNumber, balance };
}
```

## Build a complete transaction experience

- For writes, simulate the exact call with the selected account, request the wallet signature, then wait for the receipt.

- Show separate states for signature requested, submitted, confirmed, and failed. A returned transaction hash means submission, not successful execution.

- Refresh affected queries after a successful receipt and keep the hash available for inspection.

- For ERC-20 amounts, read the token’s decimals and use `parseUnits`/`formatUnits`. Native PAX uses 18 decimals; arbitrary tokens may differ.

- On mobile, support an in-wallet browser or add a wallet connector that supports your application’s target devices.

For a larger React application, Wagmi adds connector lifecycle and query hooks on top of viem. Retain the same custom Paxeer chain definition and separate public reads from wallet requests.

References: [viem custom chains](https://viem.sh/docs/chains/introduction), [clients and transports](https://viem.sh/docs/clients/intro), and [contract reads](https://viem.sh/docs/contract/readContract).

**Paxeer source:** `apps/launchpad/web/src/core/wallet-sdk/client.ts`, apps/launchpad/web/src/core/rpc.ts, modules/evm/config/config.go (ChainIDMapping), and rpc/state.go (GetBalance and GetCode).
