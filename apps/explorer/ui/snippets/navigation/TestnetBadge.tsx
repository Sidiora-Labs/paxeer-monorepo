import { chakra } from '@chakra-ui/react';
import React from 'react';

import config from 'configs/app';
import { Image } from 'toolkit/chakra/image';

interface Props {
  className?: string;
}

const TestnetBadge = ({ className }: Props) => {
  if (!config.chain.isTestnet) {
    return null;
  }

  return (
    <Image
      className={ className }
      src="/explorer/static/labels/testnet.svg"
      alt="Test network"
      h={ 'var(--chakra-spacing-3\\.5)' }
      w="calc(var(--chakra-spacing-9) + var(--chakra-spacing-1) / 4)"
      color="negative"
    />
  );
};

export default React.memo(chakra(TestnetBadge));
