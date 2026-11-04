import { chakra } from '@chakra-ui/react';
import React from 'react';

import type { TokenInstance } from 'types/api/token';

import useLoadImageViaIpfs from './useLoadImageViaIpfs';
import useNftMediaInfo from './useNftMediaInfo';
import type { MediaElementProps, Size } from './utils';
import { videoPlayProps } from './utils';

interface Props extends MediaElementProps<'video'> {
  instance: TokenInstance;
  autoPlay?: boolean;
  size?: Size;
}

const POSTER_ALLOWED_TYPES = [ 'image' as const ];

export function loadNftVideoPoster(video: HTMLVideoElement, src: string, onLoad: () => void, onError: () => void) {
  const poster = new Image();
  // Handlers precede src so even a cached poster releases the loading skeleton.
  poster.onload = onLoad;
  poster.onerror = onError;
  poster.src = src;
  video.poster = poster.src;
  return poster;
}

const NftVideo = ({ src, transport, instance, autoPlay = true, onLoad, size = 'original', onError, onClick, ...rest }: Props) => {
  const ref = React.useRef<HTMLVideoElement>(null);

  const mediaInfoQuery = useNftMediaInfo({ data: instance, size, field: 'image_url', isEnabled: true, allowedTypes: POSTER_ALLOWED_TYPES });
  const loadImageViaIpfs = useLoadImageViaIpfs();

  const handleMouseEnter = React.useCallback(() => {
    !autoPlay && ref.current?.play();
  }, [ autoPlay ]);

  const handleMouseLeave = React.useCallback(() => {
    !autoPlay && ref.current?.pause();
  }, [ autoPlay ]);

  const loadViaHttp = React.useCallback(async() => {
    if (!ref.current) {
      return;
    }

    onLoad && (ref.current.oncanplaythrough = onLoad);
    onError && (ref.current.onerror = onError);
    ref.current.src = src;
  }, [ src, onLoad, onError ]);

  React.useEffect(() => {
    switch (transport) {
      case 'ipfs': {
        // Currently we don't support IPFS video loading
        onError?.();
        break;
      }
      case 'http':
        loadViaHttp();
        break;
    }
  }, [ loadViaHttp, onError, transport ]);

  React.useEffect(() => {
    const video = ref.current;
    const mediaInfo = mediaInfoQuery.data?.[0];
    if (autoPlay || !video || mediaInfoQuery.isPending || !mediaInfo) {
      return;
    }

    const controller = new AbortController();
    let poster: HTMLImageElement | undefined;
    const handleLoad = () => {
      if (!controller.signal.aborted) {
        onLoad?.();
      }
    };
    const handleError = () => {
      if (!controller.signal.aborted) {
        onError?.();
      }
    };

    switch (mediaInfo.transport) {
      case 'ipfs': {
        loadImageViaIpfs(mediaInfo.src, controller.signal)
          .then((objectUrl) => {
            if (!controller.signal.aborted) {
              video.poster = objectUrl;
              handleLoad();
            }
          })
          .catch(handleError);
        break;
      }
      case 'http': {
        poster = loadNftVideoPoster(video, mediaInfo.src, handleLoad, handleError);
        break;
      }
    }

    return () => {
      controller.abort();
      if (poster) {
        poster.onload = null;
        poster.onerror = null;
      }
      video.removeAttribute('poster');
    };

  }, [ autoPlay, loadImageViaIpfs, onError, onLoad, mediaInfoQuery.data, mediaInfoQuery.isPending ]);

  return (
    <chakra.video
      ref={ ref }
      { ...videoPlayProps }
      autoPlay={ autoPlay }
      onClick={ onClick }
      onMouseEnter={ handleMouseEnter }
      onMouseLeave={ handleMouseLeave }
      { ...rest }
    />
  );
};

export default React.memo(NftVideo);
