import { Box } from '@chakra-ui/react';
import React, { useState, useEffect } from 'react';

import durations from 'toolkit/theme/foundations/durations';

const Confetti = () => {
  const [ shouldGenerate, setShouldGenerate ] = useState(true);

  useEffect(() => {
    const timer = setTimeout(() => {
      setShouldGenerate(false);
    }, Number.parseInt(durations.cycle.value, 10) * 5);

    return () => clearTimeout(timer);
  }, []);

  const getRandomShape = () => {
    const shapes = [
      // Circle
      { borderRadius: 'var(--chakra-radii-full)' },
      // Triangle
      { clipPath: 'polygon(50% 0%, 0% 100%, 100% 100%)' },
      // Star
      { clipPath: 'polygon(50% 0%, 61% 35%, 98% 35%, 68% 57%, 79% 91%, 50% 70%, 21% 91%, 32% 57%, 2% 35%, 39% 35%)' },
      // Diamond
      { clipPath: 'polygon(50% 0%, 100% 50%, 50% 100%, 0% 50%)' },
      // Square (default)
      { borderRadius: '0' },
    ];
    return shapes[Math.floor(Number(Math.random()) * shapes.length)];
  };

  const createSplash = (centerX: number, centerY: number, delay: number) => {
    const confettiCount = 30;
    return Array.from({ length: confettiCount }).map((_, i) => {
      const angle = Number(Math.random()) * 360;
      const distance = Number(Math.random()) * 200 + 100;
      const shape = getRandomShape();
      return {
        id: `${ centerX }-${ centerY }-${ i }`,
        left: `${ centerX }%`,
        top: `${ centerY }%`,
        angle,
        distance,
        animationDuration: 'var(--chakra-durations-shimmer)',
        animationDelay: `calc(var(--chakra-durations-cycle) * ${ delay })`,
        size: `calc(var(--chakra-spacing-1) * ${ Number(Math.random()) * 2 + 1 })`,
        color: [ 'accent', 'positive', 'warning', 'negative' ][i % 4],
        transform: `translate(
          calc(-50% + calc(var(--chakra-spacing-1) * ${ Math.cos(angle * Math.PI / 180) * distance / 4 })),
          calc(-50% + calc(var(--chakra-spacing-1) * ${ Math.sin(angle * Math.PI / 180) * distance / 4 }))
        )`,
        shape,
      };
    });
  };

  const splashes = shouldGenerate ? [
    createSplash(50, 50, 0), // center
    createSplash(20, 30, Number(Math.random()) * 3), // top left
    createSplash(80, 30, Number(Math.random()) * 3), // top right
    createSplash(20, 70, Number(Math.random()) * 3), // bottom left
    createSplash(80, 70, Number(Math.random()) * 3), // bottom right
  ].flat() : [];

  return (
    <>
      <style>
        { `
          @keyframes splash {
            0% {
              transform: translate(-50%, -50%) scale(0);
              opacity: 0;
            }
            1% {
              opacity: 1;
            }
            80% {
              opacity: 0.9;
            }
            100% {
              transform: var(--final-transform) scale(1);
              opacity: 0;
            }
          }
        ` }
      </style>
      <Box
        position="fixed"
        top="0"
        left="0"
        width="100%"
        height="100%"
        pointerEvents="none"
        zIndex="overlay"
        aria-hidden="true"
        overflow="clip"
        _motionReduce={{ display: 'none' }}
      >
        { splashes.map((piece) => (
          <Box
            key={ piece.id }
            position="absolute"
            left={ piece.left }
            top={ piece.top }
            width={ piece.size }
            height={ piece.size }
            backgroundColor={ piece.color }
            opacity={ 0 }
            animation={ `splash ${ piece.animationDuration } var(--chakra-easings-exit) ${ piece.animationDelay } forwards` }
            style={{
              transformOrigin: 'center',
              '--final-transform': piece.transform,
              ...piece.shape,
            } as unknown as React.CSSProperties}
          />
        )) }
      </Box>
    </>
  );
};

export default Confetti;
