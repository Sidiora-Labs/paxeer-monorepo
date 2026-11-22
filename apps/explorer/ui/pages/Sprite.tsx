import { Flex, Box, chakra } from '@chakra-ui/react';
import { useQuery } from '@tanstack/react-query';
import React from 'react';

import useFetch from 'lib/hooks/useFetch';
import { EmptyState } from 'toolkit/chakra/empty-state';
import { Tooltip } from 'toolkit/chakra/tooltip';
import { ContentLoader } from 'toolkit/components/loaders/ContentLoader';
import { useClipboard } from 'toolkit/hooks/useClipboard';
import DataFetchAlert from 'ui/shared/DataFetchAlert';
import type { IconName } from 'ui/shared/IconSvg';
import IconSvg from 'ui/shared/IconSvg';
import PageTitle from 'ui/shared/Page/PageTitle';

import { PageFilterInput } from './PageFilterInput';

const formatFileSize = (fileSizeInBytes: number) => `${ (fileSizeInBytes / 1_024).toFixed(2) } Kb`;

export interface IconInfo {
  name: string;
  file_size: number;
}

const Item = ({ name, file_size: fileSize }: IconInfo) => {
  const { hasCopied, copy, disclosure } = useClipboard(name);

  return (
    <Tooltip content={ hasCopied ? 'Copied' : 'Copy to clipboard' } { ...disclosure }>
      <chakra.button
        type="button"
        display="flex"
        flexDir="column"
        alignItems="center"
        whiteSpace="normal"
        overflowWrap="anywhere"
        minW={ 0 }
        w="calc(var(--chakra-spacing-100) / 4)"
        maxW="100%"
        textAlign="center"
        onClick={ copy }
        cursor="pointer"
        borderRadius="control"
        p={ 0 }
        _hover={{ bg: 'raised' }}
        _active={{ bg: 'accent.soft' }}
      >
        <IconSvg name={ name.replace('.svg', '') as IconName } boxSize="calc(var(--chakra-spacing-100) / 4)" bgColor="surface" borderRadius="base"/>
        <Box as="span" maxW="100%" fontWeight="medium" mt={ 2 }>{ name }</Box>
        <Box as="span" maxW="100%" color="text.secondary" role="status">{ hasCopied ? 'Copied' : formatFileSize(fileSize) }</Box>
      </chakra.button>
    </Tooltip>
  );
};

const Sprite = ({ initialData }: { initialData?: Array<IconInfo> }) => {
  const [ searchTerm, setSearchTerm ] = React.useState('');

  const fetch = useFetch();
  const { data, isFetching, isPending, isError } = useQuery({
    queryKey: [ 'sprite' ],
    initialData,
    queryFn: () => {
      return fetch<Array<IconInfo>, unknown>('/explorer/icons/registry.json');
    },
  });

  const content = (() => {
    if (isPending) {
      return <ContentLoader/>;
    }

    if (isError || !data || !Array.isArray(data)) {
      return <DataFetchAlert/>;
    }

    const items = data
      .filter((icon) => icon.name.includes(searchTerm))
      .sort((a, b) => a.name.localeCompare(b.name));

    if (items.length === 0) {
      return <EmptyState description="No icons found"/>;
    }

    return (
      <Flex flexWrap="wrap" fontSize="sm" columnGap={ 5 } rowGap={ 5 } justifyContent="flex-start">
        { items.map((item) => <Item key={ item.name } { ...item }/>) }
      </Flex>
    );
  })();

  const total = React.useMemo(() => {
    if (!data || !Array.isArray(data)) {
      return;
    }
    return data.reduce((result, item) => {
      result.num++;
      result.fileSize += item.file_size;
      return result;
    }, { num: 0, fileSize: 0 });
  }, [ data ]);

  const searchInput = (
    <PageFilterInput
      placeholder="Search by name..."
      onChange={ setSearchTerm }
      loading={ isFetching }
      minW={{ base: '100%', lg: 'var(--chakra-spacing-75)' }}
    />
  );
  const totalEl = total ? <Box ml="auto">Items: { total.num } / Size: { formatFileSize(total.fileSize) }</Box> : null;

  const contentAfter = (
    <>
      { totalEl }
      { searchInput }
    </>
  );

  return (
    <div>
      <PageTitle title="SVG sprite 🥤" contentAfter={ contentAfter }/>
      { content }
    </div>
  );
};

export default React.memo(Sprite);
