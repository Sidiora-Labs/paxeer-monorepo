import { useMemo, useSyncExternalStore } from "react";
import { useAccount, useTokenInfo } from "@paxeer-network/hooks";
import { ChainNamespace, NetworkId } from "@paxeer-network/types";
import { paxeerChain } from "@paxeer-network/paxeer-backend/core/chain";
import {
  getNativeSnapshot,
  subscribeNativeSnapshot,
} from "@paxeer-network/paxeer-backend/runtime";
import { Decimal } from "@paxeer-network/utils";

export type UseDepositFeeReturn = ReturnType<typeof useDepositFee>;

export function useDepositFee(options: {
  chainId?: number;
  nativeSymbol?: string;
  depositFee?: bigint;
  getIndexPrice: (token: string) => number;
  networkId: NetworkId;
}) {
  const { nativeSymbol, depositFee = 0, getIndexPrice, networkId } = options;
  const { account } = useAccount();

  const snapshot = useSyncExternalStore(
    subscribeNativeSnapshot,
    getNativeSnapshot,
    getNativeSnapshot,
  );
  const isPaxeer = options.chainId === paxeerChain.id;
  const nativePrice = snapshot.markets.find(
    (row) => row.symbol.toUpperCase() === nativeSymbol?.toUpperCase(),
  )?.index_price;
  const indexPrice = isPaxeer ? nativePrice : getIndexPrice(nativeSymbol!);

  const tokenInfo = useTokenInfo(nativeSymbol!, networkId);

  const feeProps = useMemo(() => {
    // deposit fee is native token, so evm decimals is 18, solana is 9
    const decimals =
      account.walletAdapter?.chainNamespace === ChainNamespace.solana ? 9 : 18;

    const dstGasFee = new Decimal(depositFee.toString())
      .div(new Decimal(10).pow(decimals))
      .toString();

    const feeAmount = isPaxeer
      ? Number.isFinite(indexPrice) && indexPrice > 0
        ? new Decimal(dstGasFee).mul(indexPrice).toString()
        : undefined
      : new Decimal(dstGasFee).mul(indexPrice || 0).toString();

    return {
      dstGasFee,
      feeQty: dstGasFee,
      feeAmount,
      dp: tokenInfo?.decimals || 8,
      showFeeQty: isPaxeer || undefined,
    };
  }, [
    depositFee,
    tokenInfo,
    indexPrice,
    isPaxeer,
    account.walletAdapter?.chainNamespace,
  ]);

  return feeProps;
}
