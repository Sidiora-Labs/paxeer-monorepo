import { Box, Center } from '@chakra-ui/react';
import React from 'react';

import AddressIdenticon from 'ui/shared/entities/address/AddressIdenticon';
import IconSvg from 'ui/shared/IconSvg';

type Props = {
  address: string;
  isAutoConnectDisabled?: boolean;
};

const UserIdenticon = ({ address, isAutoConnectDisabled }: Props) => {
  return (
    <Box position="relative">
      <AddressIdenticon size={ 20 } hash={ address }/>
      { isAutoConnectDisabled && (
        <Center
          boxSize={ 'var(--chakra-spacing-3\\.5)' }
          position="absolute"
          bottom={ 'calc(-1 * var(--chakra-spacing-0\\.25))' }
          right="calc(-1 * (var(--chakra-spacing-1) / 1.3333333333333333))"
          backgroundColor="identicon.overlay"
          borderRadius="full"
          borderWidth="var(--chakra-border-widths-1)"
          borderColor="warningSoft"
        >
          <IconSvg aria-hidden={ true }
            name="integration/partial"
            color="white"
            boxSize={ 2 }
          />
        </Center>
      ) }
    </Box>
  );
};

export default React.memo(UserIdenticon);
