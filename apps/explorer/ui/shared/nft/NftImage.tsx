import React from 'react';

import { Image } from 'toolkit/chakra/image';

import useLoadImageViaIpfs from './useLoadImageViaIpfs';
import type { MediaElementProps } from './utils';

interface Props extends MediaElementProps<'img'> {}

const NftImage = ({ src, srcSet, onLoad, onError, transport, onClick, ...rest }: Props) => {
  const ref = React.useRef<HTMLImageElement>(null);
  const [ isLoaded, setIsLoaded ] = React.useState(false);

  const loadImageViaIpfs = useLoadImageViaIpfs();

  React.useEffect(() => {
    const image = ref.current;
    if (!image) {
      return;
    }

    const controller = new AbortController();
    setIsLoaded(false);

    const handleLoad = () => {
      if (!controller.signal.aborted) {
        setIsLoaded(true);
        onLoad?.();
      }
    };
    const handleError = () => {
      if (!controller.signal.aborted) {
        onError?.();
      }
    };

    switch (transport) {
      case 'ipfs': {
        loadImageViaIpfs(src, controller.signal)
          .then((objectUrl) => {
            if (!controller.signal.aborted) {
              image.src = objectUrl;
              handleLoad();
            }
          })
          .catch(handleError);
        break;
      }
      case 'http': {
        image.onload = handleLoad;
        image.onerror = handleError;
        image.srcset = srcSet || '';
        image.src = src;
        break;
      }
    }

    return () => {
      controller.abort();
      image.onload = null;
      image.onerror = null;
      image.removeAttribute('src');
      image.removeAttribute('srcset');
    };
  }, [ loadImageViaIpfs, onError, onLoad, src, srcSet, transport ]);

  return (
    <Image
      key={ `${ transport }:${ src }` }
      ref={ ref }
      w="100%"
      h="100%"
      opacity={ isLoaded ? 1 : 0 }
      alt="Token instance image"
      onClick={ onClick }
      { ...rest }
    />
  );
};

export default React.memo(NftImage);
