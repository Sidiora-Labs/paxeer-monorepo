import { chakra, createListCollection, HStack } from '@chakra-ui/react';
import React from 'react';

import type { StatsInterval, StatsIntervalIds } from 'types/client/stats';

import { Button } from 'toolkit/chakra/button';
import { Select } from 'toolkit/chakra/select';
import { Skeleton } from 'toolkit/chakra/skeleton';
import type { TagProps } from 'toolkit/chakra/tag';
import { STATS_INTERVALS } from 'ui/stats/constants';

const intervalCollection = createListCollection({
  items: Object.keys(STATS_INTERVALS).map((id: string) => ({
    value: id,
    label: STATS_INTERVALS[id as StatsIntervalIds].title,
  })),
});

const intervalListShort = Object.keys(STATS_INTERVALS).map((id: string) => ({
  id: id,
  title: STATS_INTERVALS[id as StatsIntervalIds].shortTitle,
})) as Array<StatsInterval>;

type Props = {
  interval: StatsIntervalIds;
  onIntervalChange: (newInterval: StatsIntervalIds) => void;
  isLoading?: boolean;
  selectTagSize?: TagProps['size'];
  className?: string;
};

const ChartIntervalSelect = ({ interval, onIntervalChange, isLoading, selectTagSize, className }: Props) => {

  const handleItemSelect = React.useCallback(({ value }: { value: Array<string> }) => {
    onIntervalChange(value[0] as StatsIntervalIds);
  }, [ onIntervalChange ]);

  const handleButtonClick = React.useCallback((event: React.MouseEvent<HTMLButtonElement>) => {
    onIntervalChange(event.currentTarget.value as StatsIntervalIds);
  }, [ onIntervalChange ]);

  return (
    <chakra.div className={ className } data-chart-interval-select={ interval } w={{ base: '100%', lg: 'auto' }}>
      <Skeleton hideBelow="lg" borderRadius="base" loading={ isLoading }>
        <HStack
          role="group"
          aria-label="Chart interval"
          gap={ 1 }
          flexWrap="wrap"
          bg="bg.surface"
          borderWidth="var(--chakra-border-widths-1)"
          borderStyle="solid"
          borderColor="border.divider"
          borderRadius="md"
          px={ 1 }
          py={ 1 }
        >
          { intervalListShort.map((item) => (
            <Button
              key={ item.id }
              type="button"
              value={ item.id }
              data-id={ item.id }
              aria-pressed={ interval === item.id }
              onClick={ handleButtonClick }
              variant="plain"
              size={ selectTagSize === 'lg' ? 'sm' : 'xs' }
              h="auto"
              bg={ interval === item.id ? 'selected.option.bg' : 'transparent' }
              color="text.primary"
              _hover={{ bg: 'bg.raised', color: 'text.primary' }}
              disabled={ isLoading }
            >
              { item.title }
            </Button>
          )) }
        </HStack>
      </Skeleton>
      <Select
        triggerProps={{ 'aria-label': 'Chart interval' }}
        collection={ intervalCollection }
        placeholder="Select interval"
        value={ [ interval ] }
        onValueChange={ handleItemSelect }
        hideFrom="lg"
        w="100%"
        loading={ isLoading }
      />
    </chakra.div>
  );
};

export default React.memo(ChartIntervalSelect);
