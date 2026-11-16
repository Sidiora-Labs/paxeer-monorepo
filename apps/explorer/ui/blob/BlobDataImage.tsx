import { Center } from '@chakra-ui/react';
import React from 'react';

import { Image } from 'toolkit/chakra/image';

interface Props {
  src: string;
}

const BlobDataImage = ({ src }: Props) => {
  return (
    <Center
      bgColor="bg.surface"
      p={ 4 }
      h="var(--chakra-spacing-80)"
      w="100%"
      borderRadius="md"
    >
      <Image
        src={ src }
        width="full"
        height="full"
        decoding="async"
        objectFit="contain"
        maxW="100%"
        maxH="100%"
        objectPosition="center"
        alt="Blob image representation"
      />
    </Center>
  );
};

export default React.memo(BlobDataImage);
