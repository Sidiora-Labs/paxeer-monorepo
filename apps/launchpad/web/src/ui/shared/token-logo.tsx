'use client';

import { useState } from 'react';
import { metadataApiUrl } from '@/core/sdk-config';

interface TokenLogoProps {
  tokenAddress: string;
  symbol?: string;
  size?: number;
  className?: string;
}

export default function TokenLogo({ tokenAddress, symbol, size = 32, className = '' }: TokenLogoProps) {
  const [logo, setLogo] = useState({ tokenAddress, failed: false });
  const sameToken = logo.tokenAddress === tokenAddress;

  if (!sameToken) {
    setLogo({ tokenAddress, failed: false });
  }

  const src = tokenAddress && (!sameToken || !logo.failed)
    ? metadataApiUrl(`/logo/${tokenAddress}.png`)
    : null;

  if (!src) {
    return (
      <img
        src="/shadcn.png"
        alt={symbol || ''}
        width={size}
        height={size}
        className={`rounded-full object-cover ${className}`}
      />
    );
  }

  return (
    <img
      src={src}
      alt={symbol || ''}
      width={size}
      height={size}
      className={`rounded-full object-cover ${className}`}
      onError={() => setLogo((current) => (
        current.tokenAddress === tokenAddress ? { tokenAddress, failed: true } : current
      ))}
    />
  );
}
