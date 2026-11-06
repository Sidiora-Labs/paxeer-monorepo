import { HStack } from '@chakra-ui/react';
import React from 'react';

import { Button } from 'toolkit/chakra/button';

export type TokenCategory = 'all' | 'fungible' | 'nft' | 'bridged';

type Props = {
  value: TokenCategory | undefined;
  onChange: (value: TokenCategory) => void;
  withBridged: boolean;
};

const CATEGORIES = [
  { value: 'all', label: 'All' },
  { value: 'fungible', label: 'Fungible' },
  { value: 'nft', label: 'NFT' },
  { value: 'bridged', label: 'Bridged' },
] as const;

const TokenCategoryFilter = ({ value, onChange, withBridged }: Props) => {
  const handleClick = React.useCallback((event: React.MouseEvent<HTMLButtonElement>) => {
    const selected = CATEGORIES.find((category) => category.value === event.currentTarget.value);
    if (selected) {
      onChange(selected.value);
    }
  }, [ onChange ]);

  return (
    <HStack gap={ 2 } flexWrap="wrap" role="group" aria-label="Token category" mb={ 4 }>
      { CATEGORIES.map((category) => (
        <Button
          key={ category.value }
          value={ category.value }
          size="sm"
          disabled={ category.value === 'bridged' && !withBridged }
          variant="outline"
          borderRadius="full"
          borderColor={ value === category.value ? 'accent' : 'hairline' }
          bg={ value === category.value ? 'accent.soft' : 'canvas' }
          color={ value === category.value ? 'accent' : 'text.secondary' }
          aria-pressed={ value === category.value }
          onClick={ handleClick }
        >
          { category.label }
        </Button>
      )) }
    </HStack>
  );
};

export default TokenCategoryFilter;
