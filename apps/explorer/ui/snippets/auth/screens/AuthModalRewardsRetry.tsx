import { Box, Text } from '@chakra-ui/react';
import React from 'react';

import { useRewardsContext } from 'lib/contexts/rewards';
import { Button } from 'toolkit/chakra/button';

interface Props {
  onClose: () => void;
}

const AuthModalRewardsRetry = ({ onClose }: Props) => {
  const { openLoginModal } = useRewardsContext();
  const handleRetry = React.useCallback(() => {
    onClose();
    openLoginModal();
  }, [ onClose, openLoginModal ]);

  return (
    <Box mt={ 6 }>
      <Text>Your account is signed in. Merits login failed. You can try again separately.</Text>
      <Button mt={ 3 } variant="outline" onClick={ handleRetry }>Retry Merits login</Button>
    </Box>
  );
};

export default React.memo(AuthModalRewardsRetry);
