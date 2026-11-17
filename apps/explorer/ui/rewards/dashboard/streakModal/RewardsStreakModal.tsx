import { Flex, Text, Separator } from '@chakra-ui/react';
import React from 'react';

import type { GetAvailableBadgesResponse } from '@blockscout/points-types';

import { Heading } from 'toolkit/chakra/heading';
import { DialogBody, DialogContent, DialogHeader, DialogRoot } from 'ui/marketplace/NativeDialog';

import BadgeCard from './BadgeCard';
import ProgressSegment from './ProgressSegment';

type Props = {
  open: boolean;
  onOpenChange: ({ open }: { open: boolean }) => void;
  currentStreak: number;
  badges?: GetAvailableBadgesResponse['items'];
};

const EMPTY_ARRAY: GetAvailableBadgesResponse['items'] = [];

const RewardsStreakModal = ({ open, onOpenChange, currentStreak, badges = EMPTY_ARRAY }: Props) => {

  return (
    <DialogRoot open={ open } onOpenChange={ onOpenChange } size={{ lgDown: 'full', lg: 'md' }}>
      <DialogContent>
        <DialogHeader>Streak progress</DialogHeader>
        <DialogBody>
          <Flex direction="column" gap={ 6 }>
            <Flex direction="column" gap={{ base: 6, lg: 3 }}>
              <Text textStyle={{ base: 'sm', lg: 'md' }}>
                Build your streak day by day and unlock exclusive badges as a reward for staying consistent.
              </Text>
              <Flex
                direction={{ base: 'column', lg: 'row' }}
                gap={{ base: 3, lg: 10 }}
                bgColor="surface"
                p={{ base: 4, lg: 8 }}
                pl={{ base: 4, lg: 5 }}
                borderRadius="lg"
                justifyContent="space-between"
              >
                <Flex direction="column" alignItems="center" gap={{ base: 1, lg: 2 }}>
                  <Heading level="1">{ currentStreak }</Heading>
                  <Text textStyle="xs" color="text.secondary">Day streak</Text>
                </Flex>
                <Flex flex={ 1 } minW={ 0 } flexWrap="wrap" rowGap={ 4 } pl={{ base: 2, lg: 0 }}>
                  { badges.map((badge, i) => {
                    const target = Number(badge.requirements?.streak || 0);
                    const prevTarget = i > 0 ? Number(badges[i - 1]?.requirements?.streak || 0) : 0;
                    const value = (badge.is_whitelisted || badge.is_minted) ? target : currentStreak;
                    return (
                      <ProgressSegment
                        key={ i }
                        value={ value }
                        target={ target }
                        prevTarget={ prevTarget }
                        isFirst={ i === 0 }
                      />
                    );
                  }) }
                </Flex>
              </Flex>
            </Flex>

            <Flex direction="column" gap={ 2 }>
              <Heading level="3">Rewards</Heading>
              <Flex direction={{ base: 'column', lg: 'row' }} gap={{ base: 2, lg: 6 }} justifyContent="space-between">
                { badges.map((badge, i) => (
                  <React.Fragment key={ i }>
                    <BadgeCard
                      badge={ badge }
                      currentStreak={ currentStreak }
                      index={ i }
                    />
                    { i < badges.length - 1 && (
                      <Separator
                        display={{ base: 'none', lg: 'block' }}
                        orientation="vertical"
                      />
                    ) }
                  </React.Fragment>
                )) }
              </Flex>
            </Flex>
          </Flex>
        </DialogBody>
      </DialogContent>
    </DialogRoot>
  );
};

export default RewardsStreakModal;
