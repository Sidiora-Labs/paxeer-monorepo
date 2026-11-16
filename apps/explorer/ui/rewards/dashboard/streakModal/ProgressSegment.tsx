import { Flex, Text } from '@chakra-ui/react';
import { clamp } from 'es-toolkit';

import { Progress } from 'toolkit/chakra/progress';
import IconSvg from 'ui/shared/IconSvg';

type Props = {
  value: number;
  target: number;
  prevTarget: number;
  isFirst: boolean;
};

export default function ProgressSegment({ value, target, prevTarget, isFirst }: Props) {
  const isDone = value >= target;
  const progress = clamp(value, prevTarget, target);

  return (
    <Flex gap={ 0 } flex="1 1 var(--chakra-spacing-20)" minW={ 0 }>
      <Flex flex={ 1 } alignItems="center" h={{ base: 'var(--chakra-spacing-8)', lg: 'var(--chakra-spacing-10)' }} mx={ -2.5 }>
        <Progress
          value={ progress }
          min={ prevTarget }
          max={ target }
          aria-label={ `${ target } day streak progress` }
          w="full"
          color="positive"
          trackProps={{
            borderStartRadius: isFirst ? undefined : 0,
            borderEndRadius: 0,
          }}
        />
      </Flex>
      <Flex
        direction="column"
        alignItems="center"
        gap={ 2 }
        flexShrink={ 0 }
        w="var(--chakra-spacing-15)"
      >
        <Flex h={{ base: 'var(--chakra-spacing-8)', lg: 'var(--chakra-spacing-10)' }} alignItems="center">
          <Flex
            w="var(--chakra-spacing-10)"
            h="var(--chakra-spacing-8)"
            borderRadius="lg"
            bgColor={ isDone ? 'positive' : 'progress.track' }
            alignItems="center"
            justifyContent="center"
            zIndex={ 1 }
          >
            { isDone ? (
              <IconSvg name="check" boxSize={ 5 } color="onAccent"/>
            ) : (
              <IconSvg name="hexagon" boxSize={ 4 } color="icon.secondary"/>
            ) }
          </Flex>
        </Flex>
        <Text textStyle="xs" color="text.secondary">{ target } Days</Text>
      </Flex>
    </Flex>
  );
}
