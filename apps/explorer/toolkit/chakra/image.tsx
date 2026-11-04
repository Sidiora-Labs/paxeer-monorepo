import type { BoxProps, ImageProps as ChakraImageProps } from '@chakra-ui/react';
import { Image as ChakraImage } from '@chakra-ui/react';
import React from 'react';

import { Skeleton } from './skeleton';

export interface ImageProps extends ChakraImageProps {
  fallback?: React.ReactNode;
  // for the case where the image dimensions are not known before the image is loaded
  skeletonWidth?: BoxProps['width'];
  skeletonHeight?: BoxProps['height'];
}

export const Image = React.forwardRef<HTMLImageElement, ImageProps>(
  function Image(props, ref) {
    const { fallback, src, srcSet, onLoad, onError, skeletonWidth, skeletonHeight, alt, ...rest } = props;

    const [ loading, setLoading ] = React.useState(true);
    const [ error, setError ] = React.useState(false);

    React.useEffect(() => {
      setLoading(true);
      setError(false);
    }, [ src, srcSet ]);

    const handleLoadError = React.useCallback((event: React.SyntheticEvent<HTMLImageElement>) => {
      setError(true);
      setLoading(false);
      onError?.(event);
    }, [ onError ]);

    const handleLoadSuccess = React.useCallback((event: React.SyntheticEvent<HTMLImageElement>) => {
      setLoading(false);
      onLoad?.(event);
    }, [ onLoad ]);

    if (!src && fallback) {
      if (React.isValidElement(fallback)) {
        return React.cloneElement(fallback, rest);
      }
      return fallback;
    }

    if (error) {
      if (React.isValidElement(fallback)) {
        return React.cloneElement(fallback, rest);
      }
      return fallback;
    }

    const skeletonProps: BoxProps = {
      ...rest as BoxProps,
      ...(skeletonWidth !== undefined && { width: skeletonWidth }),
      ...(skeletonHeight !== undefined && { height: skeletonHeight }),
    };

    return (
      <>
        { loading && <Skeleton loading { ...skeletonProps }/> }
        <ChakraImage
          key={ JSON.stringify([ src, srcSet ]) }
          ref={ ref }
          src={ src }
          srcSet={ srcSet }
          alt={ alt }
          onError={ handleLoadError }
          onLoad={ handleLoadSuccess }
          { ...rest }
          display={ loading ? 'none' : rest.display || 'block' }
        />
      </>
    );
  },
);
