'use client';

import { useEffect, useMemo, useRef, useState } from 'react';
import { useParams } from 'next/navigation';
import { useAccount, useSignMessage } from 'wagmi';
import { useQueryClient } from '@tanstack/react-query';
import type { MetadataV2Data } from '@/core/clients/metadata';
import { metadataApiUrl } from '@/core/sdk-config';
import { MetadataV2Editor } from '@/features/metadata-media/editor';
import { CreatorMetadataMedia } from '@/features/metadata-media/viewer';
import { useMetadataV2 } from '@/hooks/market/use-token-metadata';
import { assertAccount } from '@/widgets/profile/account-transport';
import { useAccountSession } from '@/widgets/profile/use-account-session';
import WalletButton from '@/shell/wallet-button';

export default function TokenMetadataPage() {
  const params = useParams<{ tokenAddress: string }>();
  const token = (typeof params.tokenAddress === 'string' ? params.tokenAddress : '').toLowerCase();
  const { address, isConnected, chainId } = useAccount();
  const { signMessageAsync } = useSignMessage();
  const { session } = useAccountSession();
  const cache = useQueryClient();
  const query = useMetadataV2(token);
  const metadata = query.data;
  const [audience, setAudience] = useState('');
  const signedImageRevision = useRef<string | null>(null);
  const imageIdentity = useRef({ scope: '', revision: '', assetId: null as string | null, key: '' });
  const scope = `${token}:${address}:${session?.epoch}`;
  if (metadata && (imageIdentity.current.scope !== scope || imageIdentity.current.revision !== metadata.revision)) {
    const previous = imageIdentity.current;
    const ownImageCommit = previous.scope === scope && signedImageRevision.current === previous.revision && BigInt(metadata.revision) === BigInt(previous.revision) + 1n && metadata.token.logoAssetId !== previous.assetId;
    imageIdentity.current = { scope, revision: metadata.revision, assetId: metadata.token.logoAssetId, key: ownImageCommit ? previous.key : `${scope}:${metadata.revision}` };
    signedImageRevision.current = null;
  }
  useEffect(() => { setAudience(new URL(metadataApiUrl('/'), window.location.origin).origin); }, []);
  const validToken = /^0x[a-f0-9]{40}$/.test(token);
  const authority = useMemo(() => {
    if (!metadata || !address || !isConnected || chainId !== 125 || !session || !audience) return null;
    return {
      tokenAddress: token,
      wallet: address,
      audience,
      revision: metadata.revision,
      authenticationHeaders: async () => {
        assertAccount(session);
        return { Authorization: `Bearer ${session.accessToken}` };
      },
      signMessage: async (message: string) => {
        assertAccount(session);
        const signature = await signMessageAsync({ message, account: address });
        assertAccount(session);
        signedImageRevision.current = metadata.revision;
        return signature;
      },
    };
  }, [token, metadata, address, isConnected, chainId, session, signMessageAsync, audience]);
  const committed = (value: MetadataV2Data) => {
    cache.setQueryData(['metadata-v2', 125, token], value);
    void cache.invalidateQueries({ queryKey: ['metadata-media-v2', 125, token] });
  };
  const error = !validToken ? 'This is not a valid coin address' : query.isError ? "Can't load coin details right now" : !query.isPending && !metadata ? 'No details found for this coin' : '';
  const identity = `${token}:${address}:${metadata?.revision}:${session?.epoch}`;
  return (
    <div className="mx-auto max-w-2xl px-4 py-8 text-white">
      <h1 className="text-2xl font-semibold">Coin details</h1>
      {validToken && query.isPending && <p role="status">Loading...</p>}
      {error && <div role="alert"><p>{error}</p><button type="button" onClick={() => void query.refetch()}>Try again</button></div>}
      {metadata && <>
        <p>{metadata.token.name} ({metadata.token.symbol})</p><p className="break-all">{metadata.token.address}</p>
        {!isConnected || !address ? <><p>Connect the creator wallet to edit coin details</p><WalletButton /></> : chainId !== 125 ? <><p role="alert">Switch your wallet to Paxeer to edit coin details</p><WalletButton /></> : <>
          <MetadataV2Editor key={identity} tokenAddress={token} wallet={address} audience={audience} metadata={metadata} signMessage={message => signMessageAsync({ message, account: address })} onCommitted={committed} />
          {authority ? <CreatorMetadataMedia key={imageIdentity.current.key} authority={authority} /> : <><p>Sign in with the creator wallet to upload a coin image</p><WalletButton /></>}
        </>}
      </>}
    </div>
  );
}
