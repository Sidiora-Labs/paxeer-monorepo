import { Box, Flex, Text } from '@chakra-ui/react';
import React from 'react';

import useIsMobile from 'lib/hooks/useIsMobile';
import { clearRecentSearchKeywords, getRecentSearchKeywords, removeRecentSearchKeyword } from 'lib/recentSearchKeywords';
import { Button } from 'toolkit/chakra/button';
import { ClearButton } from 'toolkit/components/buttons/ClearButton';
import TextAd from 'ui/shared/ad/TextAd';
import HashStringShortenDynamic from 'ui/shared/HashStringShortenDynamic';

type Props = {
  onClick: (kw: string) => void;
  onClear?: () => void;
};

const SearchBarRecentKeywords = ({ onClick, onClear }: Props) => {
  const isMobile = useIsMobile();

  const [ keywords, setKeywords ] = React.useState<Array<string>>(getRecentSearchKeywords());

  const handleClick = React.useCallback((kw: string) => () => {
    onClick(kw);
  }, [ onClick ]);

  const clearKeywords = React.useCallback(() => {
    clearRecentSearchKeywords();
    setKeywords([]);
    onClear?.();
  }, [ onClear ]);

  const removeKeyword = React.useCallback((kw: string) => (e: React.SyntheticEvent) => {
    e.stopPropagation();
    const result = keywords.filter(item => item !== kw);
    setKeywords(result);
    if (result.length === 0 && onClear) {
      onClear();
    }
    removeRecentSearchKeyword(kw);
  }, [ keywords, onClear ]);

  if (keywords.length === 0) {
    return null;
  }

  return (
    <>
      { !isMobile && (
        <Box
          pb={ 4 }
          mb={ 5 }
          borderColor="hairline"
          borderBottomWidth="var(--chakra-border-widths-1)"
          _empty={{ display: 'none' }}
        >
          <TextAd textStyle={{ lg: 'sm' }}/>
        </Box>
      ) }
      <Flex mb={ 3 } justifyContent="space-between" fontSize="sm">
        <Text textStyle="label">Recent</Text>
        <Button onClick={ clearKeywords } variant="link" textStyle="small">Clear all</Button>
      </Flex>
      <Flex flexDirection="column" overflowY="auto">
        { keywords.map(kw => (
          <Flex
            key={ kw }
            py={ 2 }
            px={{ base: 0, lg: 1 }}
            borderColor="hairline"
            borderBottomWidth="var(--chakra-border-widths-1)"
            _last={{
              borderBottomWidth: '0',
            }}
            _hover={{
              bgColor: 'accent.soft',
            }}
            fontSize="sm"
            _first={{
              mt: 2,
            }}
            alignItems="center"
            justifyContent="space-between"
            cursor="pointer"
            columnGap={ 2 }
            fontWeight="normal"
            color="text.primary"
            minW={ 0 }
            flexGrow={ 1 }
          >
            <Button
              onClick={ handleClick(kw) }
              variant="plain"
              textStyle="body"
              minW={ 0 }
              flex="1"
              justifyContent="flex-start"
              px={ 0 }
              fontWeight="normal"
              aria-label={ kw }
            >
              { kw.startsWith('0x') ? (
                <Box overflow="hidden" whiteSpace="nowrap" fontFamily="mono" fontVariantNumeric="tabular-nums">
                  <HashStringShortenDynamic hash={ kw } noTooltip/>
                </Box>
              ) :
                <Text overflow="hidden" whiteSpace="nowrap" textOverflow="ellipsis">{ kw }</Text>
              }
            </Button>
            <ClearButton color="text.secondary" aria-label={ `Remove ${ kw }` } onClick={ removeKeyword(kw) }/>
          </Flex>
        )) }
      </Flex>
    </>
  );
};

export default SearchBarRecentKeywords;
