import type { verifiedFetch } from '@helia/verified-fetch';
import React from 'react';

type IpfsFetch = typeof verifiedFetch;

export const NftIpfsFetchContext = React.createContext<IpfsFetch | null>(null);

export default function useLoadImageViaIpfs() {
  const client = React.useContext(NftIpfsFetchContext);
  const objectUrls = React.useRef(new Set<string>());
  const generation = React.useRef(0);

  React.useEffect(() => {
    const urls = objectUrls.current;
    return () => {
      generation.current += 1;
      urls.forEach((src) => URL.revokeObjectURL(src));
      urls.clear();
    };
  }, []);

  return React.useCallback(async(url: string, signal?: AbortSignal) => {
    const requestGeneration = generation.current;
    const checkActive = () => {
      if (signal?.aborted || requestGeneration !== generation.current) {
        throw new DOMException('Image request was abandoned', 'AbortError');
      }
    };
    checkActive();
    // The verified-fetch client carries a full IPFS stack, which only the few pages that fall back to
    // it need, so it is fetched when the fallback is taken rather than with the page.
    const verifiedFetch = client || (await import('@helia/verified-fetch')).verifiedFetch;

    const response = await verifiedFetch(url);

    if (response.status !== 200) {
      throw new Error('Failed to load image');
    }

    const blob = await response.blob();
    checkActive();
    const src = URL.createObjectURL(blob);
    // The hook owns the URL; a consumer's request signal releases it on replacement.
    objectUrls.current.add(src);
    signal?.addEventListener('abort', () => {
      if (objectUrls.current.delete(src)) {
        URL.revokeObjectURL(src);
      }
    }, { once: true });
    return src;
  }, [ client ]);
}
