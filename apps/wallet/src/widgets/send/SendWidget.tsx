'use client';

/**
 * Send widget — orchestrator for the transfer flow.
 *
 * Composes:
 * - {@link SendForm}             — token / recipient / amount inputs
 * - {@link TokenSelectorSheet}   — bottom-sheet token picker
 * - {@link ContactPickerSheet}   — bottom-sheet contact picker
 * - {@link SendConfirmDialog}    — confirmation drawer
 * - {@link QrScanner}            — recipient scanner
 * - {@link TransferSuccess}      — success view rendered when txHash is set
 *
 * Owns:
 * - Selected-token state (driven by `preSelectTokenAddress` prop on mount)
 * - Sheet open/close flags
 * - Form state machine (delegated to {@link useSendForm})
 *
 * Replaces the legacy `SendPage` component. Same prop contract.
 */

import { useEffect, useState, useMemo, useRef } from 'react';
import { ethers } from 'ethers';
import { useWallet, useWalletState } from '@/providers/WalletProvider';
import { SvgIcon } from '@/components/ui/SvgIcon';
import { QrScanner } from '@/components/QrScanner';
import { TransferSuccess } from '@/components/TransferSuccess';
import { useActiveNetwork } from '@/components/NetworkSwitcher';
import { useContacts } from '@/hooks/useContacts';
import { loadRecentRecipients } from '@/lib/recentRecipients';
import { SendForm } from './SendForm';
import { SendConfirmation } from './SendConfirmation';
import { TokenSelectorSheet } from './TokenSelectorSheet';
import { ContactPickerSheet } from './ContactPickerSheet';
import { useSendableTokens, type SendableToken } from './useSendableTokens';
import { useSendForm } from './useSendForm';
import { parseAtomicAmount, parseEip681 } from '@/lib/eip681';
import { getActiveRpcUrl } from '@/lib/constants';
import { estimateFeeWei, resolveFeeOverrides } from '@/lib/fees';
import { validateEvmAddress, validateTokenAmount } from '@/lib/txValidation';
import { readTokenPrecision } from './useSendableTokens';
import { preferencesRepository } from '@/platform/storage/repositories';

export interface FeePreview {
  amountPax: string;
  mode: string;
  nonce: number | null;
  source: string;
}

export async function readFeePreview(
  provider: ethers.Provider,
  address: string,
  tokenAddress: string | null | undefined
): Promise<FeePreview> {
  const fees = await resolveFeeOverrides(provider);
  const nonce =
    preferencesRepository.read().customNonce ??
    (await provider.getTransactionCount(address, 'latest'));
  const feeWei = estimateFeeWei(tokenAddress ? 500_000n : 210_000n, fees);
  return {
    amountPax: Number(ethers.formatEther(feeWei)).toLocaleString(undefined, {
      minimumFractionDigits: 2,
      maximumFractionDigits: 6,
    }),
    mode: fees.mode,
    nonce,
    source: fees.source,
  };
}

export interface SendWidgetProps {
  onBack: () => void;
  preSelectTokenAddress?: string;
  onPaxscan?: (path?: string) => void;
}

const matchPreselect = (
  list: SendableToken[],
  preSelect: string | undefined
): SendableToken | null => {
  if (!list.length) return null;
  if (!preSelect) return list[0];
  if (preSelect === 'pax') return list[0];
  return list.find((t) => t.address?.toLowerCase() === preSelect.toLowerCase()) ?? list[0];
};

function assetKey(token: SendableToken): string {
  return token.address?.toLowerCase() ?? 'native';
}

export function useSendWidgetState(preSelectTokenAddress?: string) {
  const activeNetwork = useActiveNetwork();
  const { activeAccount, accounts } = useWalletState();
  const wallet = useWallet();
  const { tokens, loading: tokensLoading } = useSendableTokens(activeAccount?.address);
  const form = useSendForm();
  const context = `${activeAccount?.address.toLowerCase() ?? ''}:${wallet.chainId}:${activeNetwork.chainId}:${wallet.writeEpoch}`;
  const currentContext = useRef(context);
  currentContext.current = context;
  const [selection, setSelection] = useState<{ context: string; key: string } | null>(null);
  const selectedToken =
    !tokensLoading && selection?.context === context
      ? (tokens.find((token) => assetKey(token) === selection.key) ?? null)
      : null;
  const setSelectedToken = (token: SendableToken) => {
    setSelection({ context, key: assetKey(token) });
  };
  const [scannerOpen, setScannerOpen] = useState(false);
  const [scanError, setScanError] = useState('');
  const previousContext = useRef(context);

  useEffect(() => {
    if (previousContext.current !== context) {
      previousContext.current = context;
      setScanError('');
      setScannerOpen(false);
    }
    if (!activeAccount || tokensLoading || selectedToken) return;
    const initial = matchPreselect(tokens, preSelectTokenAddress);
    if (initial) {
      if (selection?.context === context) form.setAmount('');
      setSelection({ context, key: assetKey(initial) });
    }
  }, [
    context,
    activeAccount,
    tokensLoading,
    tokens,
    selectedToken,
    preSelectTokenAddress,
    selection,
    form.setAmount,
  ]);

  const contextIsCurrent = () =>
    currentContext.current === context &&
    wallet.chainId !== null &&
    wallet.chainId === activeNetwork.chainId &&
    !!activeAccount &&
    wallet.isWriteCurrent(wallet.writeEpoch, activeAccount.address, wallet.chainId);

  const handleScan = (data: string) => {
    try {
      const parsed = parseEip681(data);
      if (!parsed)
        throw new Error('This QR code is not a valid wallet address or EIP-681 payment request.');
      if (!contextIsCurrent())
        throw new Error(
          'Switch the wallet and selected network to the same chain before scanning.'
        );
      if (parsed.chainId !== undefined && parsed.chainId !== wallet.chainId) {
        throw new Error(
          `This payment request is for chain ${parsed.chainId}. Switch networks explicitly before scanning it again.`
        );
      }
      if (tokensLoading) throw new Error('Wait for the wallet assets to load before scanning.');
      const recipient = validateEvmAddress(parsed.address);
      const isPayment = /^ethereum:/i.test(data.trim());
      const token = parsed.tokenAddress
        ? tokens.find(
            (candidate) => candidate.address?.toLowerCase() === parsed.tokenAddress?.toLowerCase()
          )
        : isPayment
          ? tokens.find((candidate) => !candidate.address)
          : selectedToken;
      if (!token)
        throw new Error('The requested asset or its precision is not available in this wallet.');
      const decimals = readTokenPrecision(token.decimals);
      if (decimals === null) throw new Error('The requested asset precision is unavailable.');
      const query = new URLSearchParams(
        data.includes('?') ? data.slice(data.indexOf('?') + 1) : ''
      );
      const allowed = parsed.tokenAddress
        ? ['address', 'uint256', 'gas', 'gasPrice']
        : ['value', 'amount', 'gas', 'gasPrice'];
      for (const key of query.keys()) {
        if (!allowed.includes(key) || query.getAll(key).length !== 1)
          throw new Error('The payment request contains an unsupported or repeated field.');
      }
      if (parsed.value !== undefined && parsed.amount !== undefined)
        throw new Error('The payment request contains conflicting amounts.');
      for (const field of [parsed.gas, parsed.gasPrice]) {
        if (field !== undefined && BigInt(field) > ethers.MaxUint256)
          throw new Error('The payment request gas field is invalid.');
      }
      const raw = parsed.tokenAddress ? parsed.uint256 : parsed.value;
      let amount = '';
      if (raw !== undefined) {
        const atomic = parseAtomicAmount(raw);
        if (atomic === null || atomic <= 0n || atomic > ethers.MaxUint256)
          throw new Error('The QR payment amount is invalid.');
        amount = ethers.formatUnits(atomic, decimals);
      } else if (parsed.amount !== undefined) {
        const atomic = validateTokenAmount(parsed.amount, decimals);
        if (atomic > ethers.MaxUint256) throw new Error('The QR payment amount is invalid.');
        amount = parsed.amount;
      }
      // Commit all request fields only after the entire request is valid.
      setSelectedToken(token);
      form.setTo(recipient);
      form.setAmount(amount);
      setScanError('');
      setScannerOpen(false);
    } catch (error) {
      setScanError(error instanceof Error ? error.message : 'The payment request is invalid.');
    }
  };

  return {
    activeNetwork,
    activeAccount,
    accounts,
    tokens,
    tokensLoading,
    form,
    selectedToken,
    setSelectedToken,
    scannerOpen,
    setScannerOpen,
    scanError,
    handleScan,
    context,
    validateBeforeConfirm: () => contextIsCurrent() && form.validateBeforeConfirm(selectedToken),
    submit: async () => {
      if (!contextIsCurrent()) {
        setScanError('The wallet or network changed. Review the request again before sending.');
        return;
      }
      await form.submit(selectedToken);
    },
  };
}

export function SendWidget({ onBack, preSelectTokenAddress, onPaxscan }: SendWidgetProps) {
  const {
    activeNetwork,
    activeAccount,
    accounts,
    tokens,
    tokensLoading,
    form,
    selectedToken,
    setSelectedToken,
    scannerOpen,
    setScannerOpen,
    scanError,
    handleScan,
    context,
    validateBeforeConfirm,
    submit,
  } = useSendWidgetState(preSelectTokenAddress);
  const { contacts, findByAddress } = useContacts();
  const [selectorOpen, setSelectorOpen] = useState(false);
  const [contactPickerOpen, setContactPickerOpen] = useState(false);
  const [feePreview, setFeePreview] = useState<FeePreview | 'unavailable' | null>(null);

  // Other wallet accounts (excluding active) for the picker
  const ownAccounts = useMemo(
    () =>
      accounts
        .filter((a) => a.address !== activeAccount?.address)
        .map((a) => ({ address: a.address, name: a.name || 'Account' })),
    [accounts, activeAccount]
  );

  // Recent recipients loaded fresh each time the picker opens
  const [recentRecipients, setRecentRecipients] = useState(() => loadRecentRecipients());
  const handleOpenPicker = () => {
    setRecentRecipients(loadRecentRecipients());
    setContactPickerOpen(true);
  };

  useEffect(() => {
    if (!selectedToken || !activeAccount?.address || !form.to || !form.amount) {
      setFeePreview(null);
      return;
    }
    let cancelled = false;
    const timer = window.setTimeout(async () => {
      try {
        const preview = await readFeePreview(
          new ethers.JsonRpcProvider(getActiveRpcUrl()),
          activeAccount.address,
          selectedToken.address
        );
        if (!cancelled) setFeePreview(preview);
      } catch {
        if (!cancelled) setFeePreview('unavailable');
      }
    }, 300);
    return () => {
      cancelled = true;
      window.clearTimeout(timer);
    };
  }, [selectedToken, activeAccount?.address, form.to, form.amount, activeNetwork.chainId]);

  // ── Success view ─────────────────────────────────────────────────────
  if (form.transfer && form.transfer.chainId === activeNetwork.chainId) {
    return (
      <TransferSuccess
        key={`${form.transfer.chainId}:${form.transfer.hash}`}
        fromLabel={activeAccount ? activeAccount.name || 'Account' : 'Your wallet'}
        fromAmount={form.transfer.amount}
        fromSymbol={form.transfer.symbol}
        toLabel={`${form.transfer.recipient.slice(0, 6)}...${form.transfer.recipient.slice(-4)}`}
        transfer={form.transfer}
        submissionWarning={form.error}
        explorerUrl={`${activeNetwork.explorerUrl.replace(/\/+$/, '')}/tx/${form.transfer.hash}`}
        onExplorerView={
          activeNetwork.kind === 'root' && onPaxscan
            ? () => onPaxscan(`/tx/${form.transfer!.hash}`)
            : undefined
        }
        onDone={(terminal) => {
          if (terminal) form.clearTransfer();
          onBack();
        }}
      />
    );
  }

  const resolvedContact = form.to ? findByAddress(form.to) : undefined;

  return (
    <div className="min-h-screen flex flex-col px-4 pt-4 safe-area-pt">
      <div className="flex items-center gap-3 mb-6">
        <button type="button" onClick={onBack} aria-label="Back" className="p-2 -ml-2 press-scale">
          <SvgIcon
            name="arrow-left"
            className="w-5 h-5"
            style={{ filter: 'brightness(0) invert(0.6)' }}
          />
        </button>
        <h2 className="text-lg font-bold">Send</h2>
      </div>

      <SendForm
        selectedToken={selectedToken}
        to={form.to}
        amount={form.amount}
        resolvedContact={resolvedContact}
        hasContacts={ownAccounts.length > 0 || recentRecipients.length > 0 || contacts.length > 0}
        onOpenTokenSelector={() => setSelectorOpen(true)}
        onOpenContactPicker={handleOpenPicker}
        onOpenScanner={() => setScannerOpen(true)}
        onChangeTo={form.setTo}
        onChangeAmount={form.setAmount}
        onApplyPercentage={(pct) => form.applyPercentage(pct, selectedToken)}
      />

      {form.error && <p className="text-red-400 text-xs">{form.error}</p>}
      {feePreview === 'unavailable' && (
        <p className="text-red-400 text-xs" role="status" data-role="fee-unavailable">
          The network fee could not be estimated right now.
        </p>
      )}
      {scanError && (
        <p className="text-red-400 text-xs" role="alert">
          {scanError}
        </p>
      )}

      <div className="py-4 pb-24 flex justify-center">
        <SendConfirmation
          key={`${context}:${selectedToken ? assetKey(selectedToken) : ''}`}
          chainId={activeNetwork.chainId}
          networkName={activeNetwork.name}
          triggerLabel={`Send ${selectedToken?.symbol || ''}`}
          token={
            selectedToken
              ? { symbol: selectedToken.symbol, iconUrl: selectedToken.iconUrl }
              : undefined
          }
          amount={form.amount}
          to={form.to}
          loading={form.loading}
          disabled={form.loading || !form.to || !form.amount || !selectedToken || tokensLoading}
          onOpen={validateBeforeConfirm}
          onConfirm={submit}
          fee={feePreview}
        />
      </div>

      <TokenSelectorSheet
        open={selectorOpen}
        onClose={() => setSelectorOpen(false)}
        tokens={tokens}
        loading={tokensLoading}
        selected={selectedToken}
        onSelect={setSelectedToken}
      />

      <QrScanner open={scannerOpen} onClose={() => setScannerOpen(false)} onScan={handleScan} />

      <ContactPickerSheet
        open={contactPickerOpen}
        onClose={() => setContactPickerOpen(false)}
        contacts={contacts}
        ownAccounts={ownAccounts}
        recentRecipients={recentRecipients}
        onSelect={form.setTo}
      />
    </div>
  );
}
