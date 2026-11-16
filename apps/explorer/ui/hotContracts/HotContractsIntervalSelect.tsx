import { createListCollection } from '@chakra-ui/react';
import React from 'react';

import type { HotContractsInterval } from 'types/api/contracts';

import useIsInitialLoading from 'lib/hooks/useIsInitialLoading';
import type { SelectOption } from 'toolkit/chakra/select';
import { Select } from 'toolkit/chakra/select';
import TagGroupSelect from 'ui/shared/tagGroupSelect/TagGroupSelect';

import { INTERVAL_ITEMS } from './utils';

const intervalCollection = createListCollection<SelectOption<string>>({
  items: INTERVAL_ITEMS.map((item) => ({
    value: item.id,
    label: item.labelFull,
  })),
});

const intervalItems = INTERVAL_ITEMS.map((item) => ({
  id: item.id,
  title: item.labelShort,
}));

interface Props {
  interval: HotContractsInterval;
  onIntervalChange: (newInterval: HotContractsInterval) => void;
  isLoading?: boolean;
};

const HotContractsIntervalSelect = ({ interval, onIntervalChange, isLoading }: Props) => {

  const isInitialLoading = useIsInitialLoading(isLoading);

  const handleItemSelect = React.useCallback(({ value }: { value: Array<string> }) => {
    onIntervalChange(value[0] as HotContractsInterval);
  }, [ onIntervalChange ]);

  return (
    <>
      <TagGroupSelect<HotContractsInterval>
        items={ intervalItems }
        onChange={ onIntervalChange }
        value={ interval }
        tagSize="lg"
        loading={ isInitialLoading }
        disabled={ isLoading }
        hideBelow="lg"
      />
      <Select
        collection={ intervalCollection }
        placeholder="Select interval"
        triggerProps={{ 'aria-label': 'Select interval', minH: 'touchTarget', minW: 'touchTarget' }}
        value={ [ interval ] }
        onValueChange={ handleItemSelect }
        hideFrom="lg"
        flex={ 1 }
        minW={ 0 }
        loading={ isInitialLoading }
        disabled={ isLoading }
      />
    </>
  );
};

export default React.memo(HotContractsIntervalSelect);
