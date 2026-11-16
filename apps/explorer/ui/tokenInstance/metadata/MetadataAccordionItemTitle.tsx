import { Box, chakra } from '@chakra-ui/react';
import React from 'react';

import { formatName } from './utils';

interface Props {
  name: string;
  className?: string;
}

const MetadataAccordionItemTitle = ({ name, className }: Props) => {
  return (
    <Box w={{ base: 'auto', lg: 24 }} flexShrink={ 0 } fontWeight="semibold" overflowWrap="anywhere" className={ className }>
      { formatName(name) }
    </Box>
  );
};

export default React.memo(chakra(MetadataAccordionItemTitle));
