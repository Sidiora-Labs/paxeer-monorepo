/* eslint-disable @next/next/no-img-element */
import { Box, Text, Flex } from '@chakra-ui/react';
import Script from 'next/script';
import React from 'react';

import config from 'configs/app';
import { Button } from 'toolkit/chakra/button';
import { Heading } from 'toolkit/chakra/heading';
import { Link } from 'toolkit/chakra/link';
const easterEggBadgeFeature = config.features.easterEggBadge;

interface RunnerInstance extends EventListenerObject {
  canvas: HTMLCanvasElement | null;
  stop: () => void;
}

const CapybaraRunner = () => {
  const [ hasReachedHighScore, setHasReachedHighScore ] = React.useState(false);
  const [ scriptReady, setScriptReady ] = React.useState(false);

  const gameRef = React.useRef<HTMLDivElement>(null);

  React.useEffect(() => {
    const handleHighScore = () => {
      setHasReachedHighScore(true);
    };

    window.addEventListener('reachedHighScore', handleHighScore);

    return () => {
      window.removeEventListener('reachedHighScore', handleHighScore);
    };
  }, []);

  const handleScriptReady = React.useCallback(() => setScriptReady(true), []);

  React.useEffect(() => {
    const container = gameRef.current;
    if (!scriptReady || !container) {
      return;
    }
    let runner: RunnerInstance | undefined;
    const handleKey = (event: KeyboardEvent) => {
      if ([ 'Space', 'ArrowUp', 'ArrowDown', 'Enter' ].includes(event.code)) {
        event.preventDefault();
        runner?.handleEvent(event);
      }
    };
    const handlePointer = (event: MouseEvent) => {
      container.focus();
      runner?.handleEvent(event);
    };
    const handleBlur = (event: FocusEvent) => {
      if (!(event.relatedTarget instanceof Node) || !container.contains(event.relatedTarget)) {
        runner?.stop();
      }
    };
    const bindRunner = () => {
      const instance = (window as Window & { Runner?: { instance_?: RunnerInstance } }).Runner?.instance_;
      if (!instance?.canvas || runner) {
        return;
      }
      runner = instance;
      for (const event of [ 'keydown', 'keyup', 'mousedown', 'mouseup' ]) {
        document.removeEventListener(event, runner);
      }
      container.addEventListener('keydown', handleKey);
      container.addEventListener('keyup', handleKey);
      container.addEventListener('mousedown', handlePointer);
      container.addEventListener('mouseup', handlePointer);
      container.addEventListener('focusout', handleBlur);
    };
    const observer = new MutationObserver(bindRunner);
    observer.observe(container, { childList: true, subtree: true });
    bindRunner();
    return () => {
      observer.disconnect();
      container.removeEventListener('keydown', handleKey);
      container.removeEventListener('keyup', handleKey);
      container.removeEventListener('mousedown', handlePointer);
      container.removeEventListener('mouseup', handlePointer);
      container.removeEventListener('focusout', handleBlur);
      runner?.stop();
    };
  }, [ scriptReady ]);

  return (
    <>
      <Heading level="2" mt={ 12 } mb={ 2 }>Score 1000 to win a special prize!</Heading>
      <Box mb={ 4 }>Tap the game or focus it and press Space to start</Box>
      <Script strategy="lazyOnload" src="/explorer/static/capybara/index.js" onReady={ handleScriptReady }/>
      <Box
        ref={ gameRef }
        tabIndex={ 0 }
        role="region"
        aria-label="Capybara runner game"
        width={{ base: '100%', lg: 'calc(var(--chakra-spacing-1) * 150)' }}
        minH="var(--chakra-spacing-75)"
        p="calc(var(--chakra-spacing-1) * 12.5) 0"
      >
        <Box id="main-frame-error" className="interstitial-wrapper" mt={ 5 }>
          <div id="main-content"></div>
          <div id="offline-resources" style={{ display: 'none' }}>
            <img alt="" width="1200" height="66" id="offline-resources-1x" src="/explorer/static/capybara/capybaraSprite.png"/>
            <img alt="" width="2400" height="131" id="offline-resources-2x" src="/explorer/static/capybara/capybaraSpriteX2.png"/>
          </div>
        </Box>
      </Box>
      { easterEggBadgeFeature.isEnabled && hasReachedHighScore && (
        <Flex flexDirection="column" alignItems="center" justifyContent="center" gap={ 4 } mt={ 10 }>
          <Text fontSize="2xl" fontWeight="bold">You unlocked a hidden badge!</Text>
          <Text fontSize="lg" textAlign="center">Congratulations! You’re eligible to claim an epic hidden badge!</Text>
          <Button asChild>
            <Link href={ easterEggBadgeFeature.badgeClaimLink } external noIcon>Claim</Link>
          </Button>
        </Flex>
      ) }
    </>
  );
};

export default CapybaraRunner;
