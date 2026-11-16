import { Flex, Text } from '@chakra-ui/react';

import type { GetAvailableBadgesResponse } from '@blockscout/points-types';

import { Image } from 'toolkit/chakra/image';
import { Link } from 'toolkit/chakra/link';
import { Progress } from 'toolkit/chakra/progress';

const BADGE_BG_COLORS = [ 'rewards.badge.first', 'rewards.badge.second', 'rewards.badge.third' ];

const BADGES = [
  '/explorer/static/merits/streak_30.png',
  '/explorer/static/merits/streak_90.png',
  '/explorer/static/merits/streak_180.png',
] as const;

const GHOST_BADGES = [
  '/explorer/static/merits/streak_30_ghost.png',
  '/explorer/static/merits/streak_90_ghost.png',
  '/explorer/static/merits/streak_180_ghost.png',
] as const;

type Props = {
  badge: GetAvailableBadgesResponse['items'][number];
  currentStreak: number;
  index: number;
};

export default function BadgeCard({ badge, currentStreak, index }: Props) {
  const target = Number(badge.requirements?.streak || 0);
  const isUnlocked = badge.is_whitelisted || badge.is_minted;
  const progress = Math.min(currentStreak, target);

  return (
    <Flex direction={{ base: 'row', lg: 'column' }} flexWrap="wrap" minW={ 0 } alignItems="center" gap={ 3 } flex={ 1 }>
      <Flex
        p={{ base: 2.5, lg: 4 }}
        borderRadius="lg"
        bgColor={ isUnlocked ? BADGE_BG_COLORS[index] : 'surface' }
        alignItems="center"
        justifyContent="center"
        w={{ base: 'min(100%, calc(var(--chakra-spacing-1) * 23))', lg: 'full' }}
        flexShrink={ 0 }
      >
        <Image
          src={ isUnlocked ? BADGES[index] : GHOST_BADGES[index] }
          alt={ `${ target } day streak badge` }
          w="full"
          objectFit="contain"
          h={{ base: 'calc(var(--chakra-spacing-1) * 13.5)', lg: 'calc(var(--chakra-spacing-1) * 20.5)' }}
        />
      </Flex>
      <Flex
        direction="column"
        gap={ 3 }
        w="full"
        minW={ 0 }
        flex={{ base: '1 1 var(--chakra-spacing-30)', lg: undefined }}
        alignItems={{ base: 'flex-start', lg: 'center' }}
      >
        <Text textStyle="sm">{ target } Day streak</Text>
        <Flex
          w="full"
          alignItems="center"
          justifyContent={{ base: 'flex-start', lg: 'center' }}
          gap={ 2 }
          px={{ base: 0, lg: 2 }}
          minH="var(--chakra-spacing-8)"
          flexWrap="wrap"
        >
          { (() => {
            if (badge.is_minted) {
              return (
                <Text textStyle="xs" color="positive">
                  Minted
                </Text>
              );
            }
            if (badge.is_whitelisted) {
              return (
                <Link
                  href={ `https://badges.blockscout.com/mint/${ badge.address }` }
                  external
                  textStyle="sm"
                >
                  Mint a badge
                </Link>
              );
            }
            return (
              <>
                <Text textStyle="xs" color="text.secondary" minW="calc(var(--chakra-spacing-1) * 12.5)">
                  { progress }/{ target }
                </Text>
                <Progress
                  value={ progress }
                  min={ 0 }
                  max={ target }
                  aria-label={ `${ target } day streak progress` }
                  flex={ 1 }
                  minW="var(--chakra-spacing-10)"
                  color="positive"
                />
              </>
            );
          })() }
        </Flex>
      </Flex>
    </Flex>
  );
}
