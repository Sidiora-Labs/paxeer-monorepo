// eslint-disable-next-line no-restricted-imports
import { Box, Dialog as ChakraDialog } from '@chakra-ui/react';
import React from 'react';

import type { TokenInstance } from 'types/api/token';

import { DialogContent, DialogRoot, DialogCloseTrigger } from 'toolkit/chakra/dialog';

import NftHtml from './NftHtml';
import NftImage from './NftImage';
import NftVideo from './NftVideo';
import useNftMediaIdentity from './useNftMediaIdentity';
import useNftMediaInfo from './useNftMediaInfo';
import type { MediaType } from './utils';

interface Props {
  open: boolean;
  onOpenChange: ({ open }: { open: boolean }) => void;
  data: TokenInstance;
  allowedTypes?: Array<MediaType>;
  field: 'animation_url' | 'image_url';
}

const NftMediaFullscreenModal = (props: Props) => {
  const identity = useNftMediaIdentity(props.data);
  return <NftMediaFullscreenViewer key={ JSON.stringify([ ...identity, props.field, props.open ]) } { ...props }/>;
};

const NftMediaFullscreenViewer = ({ open, onOpenChange, data, allowedTypes, field }: Props) => {
  const [ mediaInfoIndex, setMediaInfoIndex ] = React.useState(0);
  const [ isMediaLoading, setIsMediaLoading ] = React.useState(true);
  const [ isMediaLoadingError, setIsMediaLoadingError ] = React.useState(false);

  const mediaInfoQuery = useNftMediaInfo({ data, size: 'original', allowedTypes, field, isEnabled: true });

  const handleMediaLoadError = React.useCallback(() => {
    if (mediaInfoQuery.data) {
      if (mediaInfoIndex < mediaInfoQuery.data.length - 1) {
        setMediaInfoIndex(mediaInfoIndex + 1);
        return;
      }
    }
    setIsMediaLoading(false);
    setIsMediaLoadingError(true);
  }, [ mediaInfoIndex, mediaInfoQuery.data ]);

  const handleMediaLoaded = React.useCallback(() => {
    setIsMediaLoading(false);
  }, []);

  const hasFailed = isMediaLoadingError || mediaInfoQuery.isError || (!mediaInfoQuery.isPending && !mediaInfoQuery.data?.length);

  const content = (() => {
    if (hasFailed) {
      return <Box role="alert" p={ 6 } bg="bg.primary">NFT media is unavailable. Close this preview and reopen it to retry.</Box>;
    }

    const mediaInfo = mediaInfoQuery.data?.[mediaInfoIndex];

    switch (mediaInfo?.mediaType) {
      case 'video':
        return (
          <NftVideo
            src={ mediaInfo.src }
            transport={ mediaInfo.transport }
            onError={ handleMediaLoadError }
            onLoad={ handleMediaLoaded }
            maxW="90vw"
            maxH="90vh"
            objectFit="contain"
            autoPlay
            instance={ data }
          />
        );
      case 'html':
        return (
          <NftHtml
            src={ mediaInfo.src }
            transport={ mediaInfo.transport }
            onError={ handleMediaLoadError }
            onLoad={ handleMediaLoaded }
            w="90vw"
            h="90vh"
            fullscreen
          />
        );
      case 'image':
        return (
          <NftImage
            src={ mediaInfo.src }
            srcSet={ mediaInfo.srcSet }
            transport={ mediaInfo.transport }
            onError={ handleMediaLoadError }
            onLoad={ handleMediaLoaded }
            maxW="90vw"
            maxH="90vh"
            objectFit="contain"
          />
        );
      default:
        return null;
    }
  })();

  return (
    <DialogRoot open={ open } onOpenChange={ onOpenChange } motionPreset="none">
      <DialogContent w="unset" maxW="100vw" p={ 0 } background="none" boxShadow="none">
        { /* FIXME (maybe): if close trigger is outside of the dialog header, it will not hide backdrop after closing the dialog */ }
        <ChakraDialog.Header>
          <DialogCloseTrigger
            position="fixed"
            top={ 6 }
            right={ 6 }
          />
        </ChakraDialog.Header>
        { !hasFailed && isMediaLoading && <Box role="status" p={ 6 } bg="bg.primary">Loading NFT media…</Box> }
        { content }
      </DialogContent>
    </DialogRoot>
  );
};

export default NftMediaFullscreenModal;
