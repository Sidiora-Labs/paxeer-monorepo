import { Spinner, Center } from '@chakra-ui/react';
import React from 'react';

import config from 'configs/app';
import { Alert } from 'toolkit/chakra/alert';
import { DialogBody, DialogContent, DialogHeader, DialogRoot } from 'toolkit/chakra/dialog';
import ReCaptcha from 'ui/shared/reCaptcha/ReCaptcha';

import { useMetadataUpdateContext } from './contexts/metadataUpdate';
import useTokenInstanceMetadataRefresh from './useTokenInstanceMetadataRefresh';

interface Props {
  hash: string;
  id: string;
}

const TokenInstanceMetadataFetcher = ({ hash, id }: Props) => {
  const { status, setStatus } = useMetadataUpdateContext() || {};
  const recaptcha = useTokenInstanceMetadataRefresh(hash, id);

  const handleModalClose = React.useCallback(({ open }: { open: boolean }) => {
    if (!open) {
      setStatus?.('INITIAL');
    }
  }, [ setStatus ]);

  if (status !== 'MODAL_OPENED') {
    return null;
  }

  return (
    <DialogRoot
      open={ status === 'MODAL_OPENED' }
      onOpenChange={ handleModalClose }
      size={{ lgDown: 'full', lg: 'sm' }}
      trapFocus={ false }
      preventScroll={ false }
      modal={ false }
      closeOnInteractOutside={ false }
    >
      <DialogContent>
        <DialogHeader mb={ 4 }>Sending request</DialogHeader>
        <DialogBody mb={ 0 } minH="78px">
          { config.services.reCaptchaV2.siteKey ? (
            <>
              <Center h="80px">
                <Spinner size="lg"/>
              </Center>
              <ReCaptcha { ...recaptcha } hideWarning/>
            </>
          ) : (
            <Alert status="error">
              Metadata refresh is not available at the moment since reCaptcha is not configured for this application.
              Please contact the service maintainer to make necessary changes in the service configuration.
            </Alert>
          ) }
        </DialogBody>
      </DialogContent>
    </DialogRoot>
  );
};

export default React.memo(TokenInstanceMetadataFetcher);
