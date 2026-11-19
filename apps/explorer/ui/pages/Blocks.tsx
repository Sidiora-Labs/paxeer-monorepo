import { Flex } from '@chakra-ui/react';
import { upperFirst } from 'es-toolkit';
import { useRouter } from 'next/router';
import React from 'react';

import type { TabItemRegular } from 'toolkit/components/AdaptiveTabs/types';

import config from 'configs/app';
import useIsMobile from 'lib/hooks/useIsMobile';
import getQueryParamString from 'lib/router/getQueryParamString';
import { BLOCK } from 'stubs/block';
import { generateListStub } from 'stubs/utils';
import RoutedTabs from 'toolkit/components/RoutedTabs/RoutedTabs';
import BlocksContent from 'ui/blocks/BlocksContent';
import BlocksTabSlot from 'ui/blocks/BlocksTabSlot';
import Flashblocks from 'ui/blocks/Flashblocks';
import PageTitle from 'ui/shared/Page/PageTitle';
import useQueryWithPages from 'ui/shared/pagination/useQueryWithPages';

const flashblocksFeature = config.features.flashblocks;

const TAB_LIST_PROPS = {
  marginBottom: 0,
  pt: 0,
  pb: 4,
  marginTop: 0,
};

const BlocksPageContent = () => {
  const router = useRouter();
  const isMobile = useIsMobile();
  const tab = getQueryParamString(router.query.tab);
  const flashblocksTabId = flashblocksFeature.isEnabled ? flashblocksFeature.name + 's' : undefined;
  const isFlashblocksTab = tab === flashblocksTabId && flashblocksTabId !== undefined;

  const blocksQuery = useQueryWithPages({
    resourceName: 'general:blocks',
    filters: { type: 'block' },
    options: {
      enabled: !isFlashblocksTab,
      placeholderData: generateListStub<'general:blocks'>(BLOCK, 50, { next_page_params: {
        block_number: 8988686,
        items_count: 50,
      } }),
    },
  });

  const tabs: Array<TabItemRegular> = [
    { id: 'blocks', title: 'All', component: <BlocksContent type="block" query={ blocksQuery }/> },
    flashblocksFeature.isEnabled && flashblocksTabId && { id: flashblocksTabId, title: upperFirst(flashblocksFeature.name) + 's', component: <Flashblocks/> },
  ].filter(Boolean);

  return (
    <>
      <PageTitle title="Blocks" withTextAd/>
      { flashblocksFeature.isEnabled ? (
        <RoutedTabs
          tabs={ tabs }
          listProps={ isMobile ? undefined : TAB_LIST_PROPS }
          rightSlot={ <BlocksTabSlot/> }
          stickyEnabled={ !isMobile && !isFlashblocksTab }
        />
      ) : (
        <>
          <Flex display={{ base: 'none', lg: 'flex' }} justifyContent="flex-end" alignItems="center" pb={ 4 }
            position="sticky" top={ 0 } zIndex="docked" bgColor="bg.primary">
            <BlocksTabSlot/>
          </Flex>
          <BlocksContent type="block" query={ blocksQuery }/>
        </>
      ) }
    </>
  );
};

export default BlocksPageContent;
