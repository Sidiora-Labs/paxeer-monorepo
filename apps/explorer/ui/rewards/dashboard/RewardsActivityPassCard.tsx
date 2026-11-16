import { Flex, Text } from '@chakra-ui/react';

import config from 'configs/app';
import { useRewardsContext } from 'lib/contexts/rewards';
import { useColorModeValue } from 'toolkit/chakra/color-mode';
import { Heading } from 'toolkit/chakra/heading';
import { Image } from 'toolkit/chakra/image';
import { Link } from 'toolkit/chakra/link';

export default function RewardsActivityPassCard() {
  const { rewardsConfigQuery } = useRewardsContext();
  const backgroundImage = useColorModeValue('/explorer/static/merits/cells.svg', '/explorer/static/merits/cells_dark.svg');

  const activityPassUrl = config.apis.rewards ?
    // eslint-disable-next-line max-len
    `${ config.apis.rewards.endpoint }/?tab=spend&id=${ rewardsConfigQuery.data?.rewards?.blockscout_activity_pass_id }&utm_source=blockscout&utm_medium=tasks` :
    undefined;

  return (
    <Flex
      minW={ 0 }
      p={{ base: 1.5, md: 2 }}
      border="var(--chakra-border-widths-1) solid"
      borderColor="hairline"
      borderRadius="lg"
      gap={{ base: 1, md: 10 }}
      flexDirection={{ base: 'column', md: 'row' }}
    >
      <Flex flex={ 1 } flexDirection="column" p={ 3 } gap={ 2 }>
        <Heading level="3">
          Activity pass
        </Heading>
        <Text textStyle="sm">
          Grab your{ ' ' }
          <Link external href={ activityPassUrl } loading={ rewardsConfigQuery.isLoading }>
            Activity pass
          </Link>{ ' ' }
          then engage with various Blockscout products and features to earn Merits every day!{ ' ' }
          <Link external href="https://docs.blockscout.com/using-blockscout/merits/activity-pass">
            Learn more
          </Link>
        </Text>
      </Flex>
      <Flex
        flex={{ base: 'none', md: 1 }}
        flexDirection={{ base: 'column', md: 'row' }}
        justifyContent="space-between"
        alignItems="center"
        minH={{ base: 'var(--chakra-spacing-40)', md: 'var(--chakra-spacing-30)' }}
        pr={{ base: 0, md: 8 }}
        pl={{ base: 0, md: 'calc(var(--chakra-spacing-1) * 21.5)' }}
        pt={{ base: 4, md: 0 }}
        pb={{ base: 3, md: 0 }}
        borderRadius="base"
        backgroundColor="rewards.pass.bg"
        overflow="hidden"
        position="relative"
      >
        <Image
          src={ backgroundImage }
          alt="Background"
          width="calc(var(--chakra-spacing-1) * 67)"
          height="calc(var(--chakra-spacing-1) * 46)"
          position="absolute"
          top="calc(var(--chakra-spacing-5) * -1)"
          left={{ base: 'calc(50% - calc(var(--chakra-spacing-1) * 33.5))', md: 'calc(var(--chakra-spacing-2) * -1)' }}
        />
        <Image
          src="/explorer/static/merits/activity_pass.svg"
          alt="Activity pass"
          width="calc(var(--chakra-spacing-1) * 19.75)"
          height="calc(var(--chakra-spacing-1) * 21.5)"
          zIndex={ 1 }
        />
        <Link
          external
          href={ activityPassUrl }
          variant="underlaid"
          fontWeight="medium"
          backgroundColor="rewards.pass.badge"
          color="rewards.pass.fg"
          iconColor="rewards.pass.icon"
          _hover={{ color: 'hover' }}
          flexShrink={ 0 }
          zIndex={ 1 }
        >
          Grab Activity pass
        </Link>
      </Flex>
    </Flex>
  );
}
