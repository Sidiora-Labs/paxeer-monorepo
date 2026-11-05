import { useCallback, useEffect, useRef, useState } from "react";
import type { Address } from "viem";
import { getAccount, getWalletBalance } from "@/hooks/perps/useAccount";
import type { AccountView } from "@/core/perps/accountView";
import { classifyError } from "@/core/errors/classify";
import { paxeerChain } from "@/core/chain";

type BalanceRead<T> =
  | { status: "loading" | "idle" }
  | { status: "available"; value: T }
  | { status: "failed"; message: string };
interface Balances {
  key: string;
  wallet: BalanceRead<bigint>;
  account: BalanceRead<AccountView>;
}
const idle = { status: "idle" } as const;

export function useCollateralBalances(
  address: Address | null,
  chainId: number | null,
  connected: boolean,
  open: boolean,
) {
  const key =
    connected && address && chainId === paxeerChain.id ? `${chainId}:${address.toLowerCase()}` : "";
  const current = useRef({ key, generation: 0 });
  if (current.current.key !== key) {
    current.current = { key, generation: current.current.generation + 1 };
  }
  const request = useRef({ wallet: 0, account: 0 });
  const [state, setState] = useState<Balances>({ key: "", wallet: idle, account: idle });
  const visible = state.key === key && key ? state : { key, wallet: idle, account: idle };

  const refresh = useCallback(
    async (kind: "wallet" | "account" | "both" = "both") => {
      if (!key || !address) return;
      const generation = current.current.generation;
      const valid = () => current.current.key === key && current.current.generation === generation;
      const read = async <T>(name: "wallet" | "account", producer: () => Promise<T>) => {
        const sequence = ++request.current[name];
        const update = (value: BalanceRead<T>) => {
          if (!valid() || sequence !== request.current[name]) return;
          setState((previous) => ({
            ...(previous.key === key ? previous : { key, wallet: idle, account: idle }),
            [name]: value,
          }));
        };
        update({ status: "loading" });
        try {
          update({ status: "available", value: await producer() });
        } catch (error) {
          const classified = classifyError(error, `account/${name}-balance`);
          update({
            status: "failed",
            message: `${classified.message} ${classified.suggestedFix ?? ""}`,
          });
        }
      };
      await Promise.all([
        ...(kind !== "account" ? [read("wallet", () => getWalletBalance(address))] : []),
        ...(kind !== "wallet" ? [read("account", () => getAccount(address))] : []),
      ]);
    },
    [key, address],
  );

  useEffect(() => {
    if (open && key) void refresh();
    return () => {
      current.current.generation += 1;
    };
  }, [open, key, refresh]);

  function captureRefresh() {
    const { key: capturedKey, generation } = current.current;
    return async () => {
      if (current.current.key === capturedKey && current.current.generation === generation)
        await refresh();
    };
  }
  return { wallet: visible.wallet, account: visible.account, refresh, captureRefresh };
}
