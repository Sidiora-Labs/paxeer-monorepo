'use client';

import { useState } from 'react';
import { Copy } from 'lucide-react';
import { useWalletState } from '@/providers/WalletProvider';
import { SvgIcon } from '@/components/ui/SvgIcon';
import { QRCodeSVG } from 'qrcode.react';
import { qrColors } from '@/theme/catalogue';
import { useLocale } from '@/providers/LocaleProvider';

interface ReceiveWidgetProps {
  onBack: () => void;
}

export function ReceiveWidget({ onBack }: ReceiveWidgetProps) {
  const { activeAccount } = useWalletState();
  const { t } = useLocale();
  const [copied, setCopied] = useState(false);
  const address = activeAccount?.address || '';

  const handleCopy = async () => {
    await navigator.clipboard.writeText(address);
    setCopied(true);
    setTimeout(() => setCopied(false), 2200);
  };

  const handleShare = async () => {
    if (navigator.share) {
      await navigator.share({ title: t.receive.shareTitle, text: address });
    } else {
      handleCopy();
    }
  };

  return (
    <div className="min-h-screen flex flex-col px-4 pt-4 safe-area-pt">
      {/* Copy toast */}
      {copied && (
        <div
          role="status"
          aria-live="polite"
          className="fixed left-1/2 top-16 z-50 -translate-x-1/2"
        >
          <div className="flex items-center gap-2 rounded-full bg-pax-card px-4 py-2 shadow-menu">
            <Copy aria-hidden="true" className="h-4 w-4 text-pax-success" />
            <span className="text-xs font-medium text-pax-success">{t.receive.addressCopied}</span>
          </div>
        </div>
      )}

      <div className="flex items-center gap-3 mb-6">
        <button
          type="button"
          onClick={onBack}
          aria-label="Back"
          className="min-h-[var(--touch-target)] min-w-[var(--touch-target)] -ml-2 p-2 press-scale focus-visible:ring-2 focus-visible:ring-pax-accent"
        >
          <SvgIcon name="arrow-left" className="w-5 h-5" />
        </button>
        <h2 className="text-lg font-bold">{t.receive.title}</h2>
      </div>

      <div className="flex-1 flex flex-col items-center justify-center gap-6 pb-20">
        <div className="glass-card p-6 rounded-3xl">
          <div className="bg-pax-light p-4 rounded-2xl">
            <QRCodeSVG
              value={address}
              size={200}
              level="H"
              bgColor={qrColors.background}
              fgColor={qrColors.foreground}
            />
          </div>
        </div>

        <div className="text-center">
          <p className="text-xs text-pax-muted mb-1">{t.receive.yourAddress}</p>
          <p className="text-sm font-mono text-pax-light break-all max-w-xs px-4">{address}</p>
        </div>

        <div className="flex gap-3">
          <button
            type="button"
            onClick={handleCopy}
            className="min-h-[var(--touch-target)] flex items-center gap-2 px-5 py-2.5 rounded-xl bg-pax-control text-sm press-scale focus-visible:ring-2 focus-visible:ring-pax-accent"
            aria-label={t.common.copy}
          >
            {copied ? (
              <SvgIcon name="check" className="w-4 h-4" />
            ) : (
              <SvgIcon name="copy" className="w-4 h-4" />
            )}
            {copied ? t.common.copied : t.common.copy}
          </button>
          <button
            type="button"
            onClick={handleShare}
            className="min-h-[var(--touch-target)] flex items-center gap-2 px-5 py-2.5 rounded-xl bg-pax-control text-sm press-scale focus-visible:ring-2 focus-visible:ring-pax-accent"
            aria-label={t.receive.share}
          >
            <SvgIcon name="share" className="w-4 h-4" />
            {t.receive.share}
          </button>
        </div>
      </div>
    </div>
  );
}
