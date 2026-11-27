import React, { useCallback, useState } from "react";
import { Button, Flex, Input, Text } from "@paxeer-network/ui";
import {
  useAccount,
  useConnect,
} from "@paxeer-network/paxeer-backend/walletRuntime";
import {
  usePaxeer,
  PAXEER_CONNECTOR_ID,
  readAgentWalletBalances,
} from "@paxeer-network/paxeer-backend/core/wallet/embedded";
import { getPublicClient } from "@paxeer-network/paxeer-backend/core/viem/clients";
import { paxeerChain } from "@paxeer-network/paxeer-backend/core/chain";
import type { Address, Hex } from "viem";
import { AgentsAccess, AgentsWorkspace } from "./agents.ui";

export function AgentsPage() {
  const auth = usePaxeer();
  const account = useAccount();
  const { connectors, connectAsync, isPending } = useConnect();
  const [email, setEmail] = useState("");
  const [sent, setSent] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const readBalances = useCallback(
    (address: Address) => readAgentWalletBalances(getPublicClient(), address),
    [],
  );
  const waitForReceipt = useCallback(
    async (hash: Hex) =>
      (
        await getPublicClient().waitForTransactionReceipt({
          hash,
          confirmations: 1,
          timeout: 60_000,
        })
      ).status,
    [],
  );
  const configured = Boolean(
    process.env.NEXT_PUBLIC_PAXEER_WALLET_API?.trim() &&
    process.env.NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY,
  );
  if (!configured || !auth.wallet) return <AgentsAccess state="unconfigured" />;
  if (
    auth.isLoading ||
    account.status === "reconnecting" ||
    account.status === "connecting"
  )
    return <AgentsAccess state="loading" />;
  const embedded =
    account.connector?.id === PAXEER_CONNECTOR_ID &&
    account.address?.toLowerCase() ===
      auth.publicWallet?.address.toLowerCase() &&
    account.chainId === paxeerChain.id;
  if (auth.isAuthenticated && embedded && auth.user && auth.publicWallet) {
    return (
      <AgentsWorkspace
        key={`${auth.user.id}:${auth.publicWallet.address}`}
        client={auth.wallet.agents}
        ownerUserId={auth.user.id}
        ownerAddress={auth.publicWallet.address}
        readBalances={readBalances}
        waitForReceipt={waitForReceipt}
        explorerUrl={paxeerChain.blockExplorers.default.url}
      />
    );
  }
  const perform = async (action: () => Promise<void>) => {
    setError(null);
    try {
      await action();
    } catch (cause) {
      setError(
        cause instanceof Error
          ? cause.message
          : "Unable to connect the embedded wallet",
      );
    }
  };
  return (
    <AgentsAccess state={account.isConnected ? "external" : "signed_out"}>
      {(error || auth.authError) && (
        <p role="alert">{error ?? auth.authError}</p>
      )}
      {auth.isAuthenticated ? (
        <Button
          size="md"
          loading={isPending}
          onClick={() =>
            void perform(async () => {
              const connector = connectors.find(
                (item) => item.id === PAXEER_CONNECTOR_ID,
              );
              if (!connector)
                throw new Error("The embedded wallet connector is unavailable");
              await connectAsync({ connector, chainId: paxeerChain.id });
            })
          }
        >
          Connect embedded wallet
        </Button>
      ) : (
        <Flex direction="column" gap={3} itemAlign="stretch">
          <label>
            Email address
            <Input
              aria-label="Email address"
              type="email"
              value={email}
              onValueChange={setEmail}
              autoComplete="email"
            />
          </label>
          <Button
            size="md"
            loading={auth.authBusy}
            disabled={!email.includes("@")}
            onClick={() =>
              void perform(async () => {
                const result = await auth.signInWithEmail(email);
                if (!result.ok)
                  throw new Error(result.error ?? "Unable to sign in");
                setSent(true);
              })
            }
          >
            Sign in with email
          </Button>
          {sent && (
            <Text as="p" size="xs" role="status">
              Check your email for the sign-in link.
            </Text>
          )}
          <Flex gap={2}>
            <Button
              size="md"
              variant="outlined"
              loading={auth.authBusy}
              onClick={() => void perform(() => auth.signInWithOAuth("google"))}
            >
              Google
            </Button>
            <Button
              size="md"
              variant="outlined"
              loading={auth.authBusy}
              onClick={() => void perform(() => auth.signInWithOAuth("github"))}
            >
              GitHub
            </Button>
          </Flex>
        </Flex>
      )}
      {auth.authError && (
        <Button size="md" variant="outlined" onClick={auth.refresh}>
          Retry embedded wallet
        </Button>
      )}
    </AgentsAccess>
  );
}
