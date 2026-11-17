import { createListCollection, Flex, chakra } from '@chakra-ui/react';
import React from 'react';

import { Select } from 'toolkit/chakra/select';
import { SCAN_ROWS_PER_PAGE } from 'ui/shared/scan';

const collection = createListCollection({
  items: SCAN_ROWS_PER_PAGE.map((option) => ({ label: String(option), value: String(option) })),
});

interface Props {
  value: number;
  onValueChange: (value: number) => void;
  isLoading?: boolean;
}

const AccountRowsSelect = ({ value, onValueChange, isLoading }: Props) => {
  const labelId = React.useId();
  const triggerId = React.useId();
  const handleChange = React.useCallback(({ value: selected }: { value: Array<string> }) => {
    const option = SCAN_ROWS_PER_PAGE.find((option) => String(option) === selected[0]);
    if (option !== undefined) onValueChange(option);
  }, [ onValueChange ]);

  return (
    <Flex alignItems="center" flexWrap="wrap" columnGap={ 2 } rowGap={ 1 } minW={ 0 } data-scan-show-rows data-wrap-row>
      <chakra.label id={ labelId } htmlFor={ triggerId } textStyle="body" color="text.secondary" data-label>Show rows:</chakra.label>
      <Select
        ids={{ trigger: triggerId }}
        triggerProps={{ 'aria-labelledby': labelId }}
        name="rows"
        collection={ collection }
        placeholder="Show rows"
        value={ [ String(value) ] }
        onValueChange={ handleChange }
        loading={ isLoading }
        size="sm"
        w="fit-content"
        minW={ 20 }
        data-value={ String(value) }
        contentProps={{ _motionReduce: { animation: 'none' } }}
      />
    </Flex>
  );
};

export default React.memo(AccountRowsSelect);
