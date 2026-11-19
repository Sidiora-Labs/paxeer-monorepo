import { Box, Center, Flex, Grid } from '@chakra-ui/react';
import { useRouter } from 'next/router';
import React from 'react';

import { route } from 'nextjs/routes';

import useApiQuery from 'lib/api/useApiQuery';
import { useMultichainContext } from 'lib/contexts/multichain';
import dayjs from 'lib/date/dayjs';
import throwOnResourceLoadError from 'lib/errors/throwOnResourceLoadError';
import getQueryParamString from 'lib/router/getQueryParamString';
import { Button } from 'toolkit/chakra/button';
import { Heading } from 'toolkit/chakra/heading';
import { Image } from 'toolkit/chakra/image';
import { Link } from 'toolkit/chakra/link';
import { ContentLoader } from 'toolkit/components/loaders/ContentLoader';
import { downloadBlob } from 'toolkit/utils/file';
import BlockCountdownTimer from 'ui/blockCountdown/BlockCountdownTimer';
import createGoogleCalendarLink from 'ui/blockCountdown/createGoogleCalendarLink';
import createIcsFileBlob from 'ui/blockCountdown/createIcsFileBlob';
import ChainIcon from 'ui/shared/externalChains/ChainIcon';
import IconSvg from 'ui/shared/IconSvg';
import StatsWidget from 'ui/shared/stats/StatsWidget';
import Time from 'ui/shared/time/Time';

import CapybaraRunner from '../games/CapybaraRunner';

type Props = {
  hideCapybaraRunner?: boolean;
};

const BlockCountdown = ({ hideCapybaraRunner }: Props) => {
  const multichainContext = useMultichainContext();
  const router = useRouter();
  const height = getQueryParamString(router.query.height);

  const { data, isPending, isError, error } = useApiQuery('general:block_countdown', {
    queryParams: {
      module: 'block',
      action: 'getblockcountdown',
      blockno: height,
    },
  });

  const handleAddToAppleCalClick = React.useCallback(() => {
    if (!data?.result?.EstimateTimeInSec) {
      return;
    }
    const fileBlob = createIcsFileBlob({ blockHeight: height, date: dayjs().add(Number(data.result.EstimateTimeInSec), 's'), multichainContext });
    downloadBlob(fileBlob, `Block #${ height } creation event.ics`);
  }, [ data?.result?.EstimateTimeInSec, height, multichainContext ]);

  const handleTimerFinish = React.useCallback(() => {
    const target = route({ pathname: '/block/[height_or_hash]', query: { height_or_hash: height } }, multichainContext);
    window.location.assign(target.startsWith('/') ? '/explorer' + target : target);
  }, [ height, multichainContext ]);

  React.useEffect(() => {
    if (!isError && !isPending && !data.result) {
      handleTimerFinish();
    }
  }, [ data?.result, handleTimerFinish, isError, isPending ]);

  if (isError) {
    throwOnResourceLoadError({ isError, error, resource: 'general:block_countdown' });
  }

  if (isPending || !data?.result) {
    return <Center h="100%"><ContentLoader/></Center>;
  }

  return (
    <Center h="100%" alignItems={{ base: 'flex-start', lg: 'center' }}>
      <Flex flexDir="column" w="fit-content" maxW={{ base: '100%', lg: 'var(--chakra-spacing-182)', xl: 'var(--chakra-spacing-200)' }}>
        <Flex gap={ 8 } alignItems="flex-start" justifyContent="space-between" w="100%" flexWrap="wrap">
          <Box flex="1 1 max-content" minW={ 0 } maxW="full">
            <Heading
              level="1"
            >
              { `Block #${ height }` }
            </Heading>
            <Box mt={ 2 } color="text.secondary">
              <Box fontWeight="semibold">Estimated target date</Box>
              <Time timestamp={ dayjs().add(Number(data.result.EstimateTimeInSec), 's').valueOf() }/>
            </Box>
            <Flex gap={ 2 } mt={ 3 } flexWrap="wrap">
              <Link
                external
                variant="underlaid"
                textStyle="sm"
                minH="touchTarget"
                px={ 2 }
                display="inline-flex"
                href={ createGoogleCalendarLink({ blockHeight: height, timeFromNow: Number(data.result.EstimateTimeInSec), multichainContext }) }
              >
                <Image src="/explorer/static/google_calendar.svg" alt="Google calendar logo" boxSize={ 5 } mr={ 2 }/>
                <span>Google</span>
              </Link>
              <Button
                variant="plain"
                px={ 2 }
                size="sm"
                fontWeight="normal"
                color="link.primary"
                _hover={{ color: 'link.primary.hover' }}
                bgColor="link.underlaid.bg"
                borderWidth="var(--chakra-border-widths-1)"
                borderStyle="solid"
                borderColor="border.strong"
                display="inline-flex"
                onClick={ handleAddToAppleCalClick }
              >
                <Image src="/explorer/static/apple_calendar.svg" alt="Apple calendar logo" boxSize={ 5 }/>
                <span>Apple</span>
              </Button>
            </Flex>
          </Box>
          <Box position="relative" flexShrink={ 0 }>
            <IconSvg
              name="block"
              w={{ base: 'var(--chakra-spacing-16)', lg: 'var(--chakra-spacing-30)' }}
              h="auto"
              aspectRatio={ 1 }
              color="icon.secondary"
              flexShrink={ 0 }
            />
            { multichainContext?.chain && (
              <ChainIcon
                data={ multichainContext.chain }
                position="absolute"
                bottom={{ base: 'var(--chakra-spacing-1\\.25)', lg: 'var(--chakra-spacing-1\\.5)' }}
                right={{ base: 'var(--chakra-spacing-11)', lg: 'var(--chakra-spacing-20)' }}
                boxSize={{ lg: 'var(--chakra-spacing-15)' }}
                bgColor="bg.primary"
                borderRadius="full"
              />
            ) }
          </Box>
        </Flex>
        { data.result.EstimateTimeInSec && (
          <BlockCountdownTimer
            value={ Math.ceil(Number(data.result.EstimateTimeInSec)) }
            onFinish={ handleTimerFinish }
          />
        ) }
        <Grid gridTemplateColumns="repeat(auto-fit, minmax(min(100%, var(--chakra-spacing-65)), 1fr))" columnGap={ 2 } mt={ 2 }>
          <StatsWidget label="Remaining blocks" value={ data.result.RemainingBlock } icon="apps"/>
          <StatsWidget label="Current block" value={ data.result.CurrentBlock } icon="block"/>
        </Grid>
        { !hideCapybaraRunner && <CapybaraRunner/> }
      </Flex>
    </Center>
  );
};

export default React.memo(BlockCountdown);
