import { Box, createListCollection, Flex, chakra } from '@chakra-ui/react';
import React from 'react';

import type * as stats from '@blockscout/stats-types';
import type { StatsIntervalIds } from 'types/client/stats';

import { Button } from 'toolkit/chakra/button';
import { Select, SelectContent, SelectControl, SelectItem, SelectRoot, SelectValueText } from 'toolkit/chakra/select';
import { FilterInput } from 'toolkit/components/filters/FilterInput';

import { STATS_INTERVALS } from './constants';

const intervalIds = Object.keys(STATS_INTERVALS) as Array<StatsIntervalIds>;
const intervalCollection = createListCollection({
  items: intervalIds.map((value) => ({ value, label: STATS_INTERVALS[value].title })),
});

type Props = {
  sections?: Array<stats.LineChartSection>;
  currentSection: string;
  onSectionChange: (newSection: string) => void;
  interval: StatsIntervalIds;
  onIntervalChange: (newInterval: StatsIntervalIds) => void;
  onFilterInputChange: (q: string) => void;
  isLoading: boolean;
  initialFilterValue: string;
};

const StatsFilters = ({
  sections,
  currentSection,
  onSectionChange,
  interval,
  onIntervalChange,
  onFilterInputChange,
  isLoading,
  initialFilterValue,
}: Props) => {

  const collection = React.useMemo(() => {
    return createListCollection({
      items: [
        { value: 'all', label: 'All stats' },
        ...(sections || []).map((section) => ({ value: section.id, label: section.title })),
      ],
    });
  }, [ sections ]);
  const inputId = React.useId();
  const sectionId = React.useId();

  const handleItemSelect = React.useCallback(({ value }: { value: Array<string> }) => {
    onSectionChange(value[0]);
  }, [ onSectionChange ]);

  const handleIntervalSelect = React.useCallback(({ value }: { value: Array<string> }) => {
    const selected = intervalIds.find((id) => id === value[0]);
    if (selected) onIntervalChange(selected);
  }, [ onIntervalChange ]);

  const handleIntervalClick = React.useCallback((event: React.MouseEvent<HTMLButtonElement>) => {
    const selected = intervalCollection.items.find((item) => item.value === event.currentTarget.value);
    if (selected) onIntervalChange(selected.value);
  }, [ onIntervalChange ]);

  return (
    <Flex
      data-stats-filters
      css={{ '& button, & input': { minWidth: 'touchTarget', minHeight: 'touchTarget', _motionReduce: { transitionProperty: 'none' } } }}
      gap={{ base: 2, lg: 3 }}
      flexDirection={{ base: 'column', lg: 'row' }}
      flexWrap="wrap"
      alignItems="end"
      bg="bg.surface"
      borderWidth="1"
      borderStyle="solid"
      borderColor="border.divider"
      borderRadius="md"
      boxShadow="card"
      px={{ base: 3, lg: 4 }}
      py={{ base: 3, lg: 3 }}
    >
      <Box
        minW={{ base: 0, lg: 'min(100%, 20ch)' }}
        w={{ base: '100%', lg: 'auto' }}
        flex={{ lg: '1 1 20ch' }}
        data-stats-filter-input
      >
        <chakra.label htmlFor={ inputId } display="block" textStyle="label" mb={ 2 }>Find chart, metric</chakra.label>
        <FilterInput
          key={ initialFilterValue }
          loading={ isLoading }
          onChange={ onFilterInputChange }
          placeholder="Find chart, metric..."
          initialValue={ initialFilterValue }
          size="sm"
          name="q"
          minW={ 0 }
          inputProps={{ id: inputId, type: 'search', autoCapitalize: 'none', spellCheck: false, enterKeyHint: 'search' }}
        />
      </Box>

      <Box
        minW={ 0 }
        w={{ base: '100%', lg: 'auto' }}
        data-stats-filter-section
      >
        <chakra.label htmlFor={ sectionId } display="block" textStyle="label" mb={ 2 }>Section</chakra.label>
        <Select
          ids={{ trigger: sectionId }}
          triggerProps={{ 'aria-label': 'Section' }}
          name="section"
          collection={ collection }
          placeholder="Select section"
          defaultValue={ [ currentSection ] }
          onValueChange={ handleItemSelect }
          w={{ base: '100%', lg: 'var(--chakra-spacing-40)' }}
          loading={ isLoading }
          positioning={{ sameWidth: true }}
          contentProps={{
            minW: 0,
            maxW: 'calc(100dvw - var(--chakra-spacing-8))',
            whiteSpace: 'normal',
            overflowWrap: 'anywhere',
            _motionReduce: { animation: 'none' },
          }}
        />
      </Box>

      <Box
        minW={ 0 }
        maxW="100%"
        w={{ base: '100%', lg: 'auto' }}
        as="fieldset"
        data-stats-filter-interval
      >
        <chakra.legend textStyle="label" mb={ 2 }>Interval</chakra.legend>
        <Box data-chart-interval-select={ interval }>
          <Flex hideBelow="lg" gap={ 1 } flexWrap="wrap" role="group" aria-label="Interval">
            { intervalIds.map((id) => (
              <Button
                key={ id }
                type="button"
                data-id={ id }
                value={ id }
                aria-label={ STATS_INTERVALS[id].title }
                aria-pressed={ interval === id }
                variant="outline"
                color="text.primary"
                bg={ interval === id ? 'selectedOption' : 'canvas' }
                borderColor={ interval === id ? 'accent' : 'border.strong' }
                onClick={ handleIntervalClick }
                disabled={ isLoading }
                minH="touchTarget"
                minW="touchTarget"
              >
                { STATS_INTERVALS[id].shortTitle }
              </Button>
            )) }
          </Flex>
          <SelectRoot
            hideFrom="lg"
            name="interval"
            collection={ intervalCollection }
            value={ [ interval ] }
            onValueChange={ handleIntervalSelect }
            w="full"
          >
            <SelectControl loading={ isLoading } triggerProps={{ 'aria-label': 'Interval' }}>
              <SelectValueText placeholder="Select interval"/>
            </SelectControl>
            <SelectContent _motionReduce={{ animation: 'none' }}>
              { intervalCollection.items.map((item) => (
                <SelectItem key={ item.value } item={ item }>{ item.label }</SelectItem>
              )) }
            </SelectContent>
          </SelectRoot>
        </Box>
      </Box>
    </Flex>
  );
};

export default StatsFilters;
