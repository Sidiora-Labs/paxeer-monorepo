import { HStack, StackSeparator } from '@chakra-ui/react';
import { Global } from '@emotion/react';
import React from 'react';

import { SECOND } from 'toolkit/utils/consts';

import BlockCountdownTimerItem from './BlockCountdownTimerItem';
import splitSecondsInPeriods from './splitSecondsInPeriods';

interface Props {
  value: number;
  onFinish: () => void;
}

const BlockCountdownTimer = ({ value: initialValue, onFinish }: Props) => {

  const [ value, setValue ] = React.useState(initialValue);

  React.useEffect(() => {
    const intervalId = window.setInterval(() => {
      setValue((prev) => {
        if (prev > 1) {
          return prev - 1;
        }

        onFinish();
        return 0;
      });
    }, SECOND);

    return () => {
      window.clearInterval(intervalId);
    };
  }, [ initialValue, onFinish ]);

  const periods = splitSecondsInPeriods(value);

  return (
    <>
      <Global styles={{
        ':where(div):has(> [data-block-countdown-timer]) :where(div):has(> a[href^="https://www.google.com/calendar/render"])': {
          flexWrap: 'wrap',
          rowGap: 'var(--chakra-spacing-2)',
        },
      }}/>
      <HStack
        data-block-countdown-timer
        role="timer"
        aria-label="Block countdown"
        gap={ 2 }
        w="full"
        bgColor="surface"
        mt={{ base: 6, lg: 8 }}
        p={{ base: 3, lg: 4 }}
        borderRadius="base"
        separator={ <StackSeparator borderColor="border.divider"/> }
      >
        <BlockCountdownTimerItem label="Days" value={ periods.days }/>
        <BlockCountdownTimerItem label="Hours" value={ periods.hours }/>
        <BlockCountdownTimerItem label="Minutes" value={ periods.minutes }/>
        <BlockCountdownTimerItem label="Seconds" value={ periods.seconds }/>
      </HStack>
    </>
  );
};

export default React.memo(BlockCountdownTimer);
