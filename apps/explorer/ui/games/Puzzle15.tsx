import { chakra, Grid, Flex, Text } from '@chakra-ui/react';
import React, { useState, useEffect, useCallback, useRef } from 'react';

import config from 'configs/app';
import { Button } from 'toolkit/chakra/button';
import { Link } from 'toolkit/chakra/link';

import Confetti from './Confetti';

const easterEggPuzzleBadgeFeature = config.features.easterEggPuzzleBadge;

const getPossibleMoves = (emptyIndex: number): Array<number> => {

  const moves: Array<number> = [];
  const row = Math.floor(emptyIndex / 4);
  const col = emptyIndex % 4;

  if (row > 0) {
    // Move tile from above into the empty space
    moves.push((row - 1) * 4 + col);
  }
  if (row < 3) {
    // Move tile from below into the empty space
    moves.push((row + 1) * 4 + col);
  }
  if (col > 0) {
    // Move tile from the left into the empty space
    moves.push(row * 4 + (col - 1));
  }
  if (col < 3) {
    // Move tile from the right into the empty space
    moves.push(row * 4 + (col + 1));
  }

  return moves;
};

const shuffleBoard = (initialBoard: Array<number>): Array<number> => {
  const board = initialBoard.slice(); // Create a copy of the board
  let emptyIndex = board.indexOf(15);
  let lastMoveIndex = -1;

  for (let i = 0; i < 100; i++) {
    let possibleMoves = getPossibleMoves(emptyIndex);

    // Prevent immediate reversal of the last move
    if (lastMoveIndex !== -1) {
      possibleMoves = possibleMoves.filter(index => index !== lastMoveIndex);
    }

    // Randomly select a tile to move into the empty space
    const moveIndex = possibleMoves[Math.floor(Math.random() * possibleMoves.length)];

    // Swap the selected tile with the empty space
    [ board[emptyIndex], board[moveIndex] ] = [ board[moveIndex], board[emptyIndex] ];

    // Update indices for the next iteration
    lastMoveIndex = emptyIndex;
    emptyIndex = moveIndex;
  }

  return board;
};

const Puzzle15 = () => {
  const [ tiles, setTiles ] = useState<Array<number>>(Array.from({ length: 16 }, (_, i) => i));
  const [ isWon, setIsWon ] = useState(false);
  const [ image, setImage ] = useState<HTMLImageElement | null>(null);
  const canvasRefs = useRef<Array<(HTMLCanvasElement | null)>>([]);

  const initializeGame = useCallback(() => {
    const newTiles = shuffleBoard(tiles);
    setTiles(newTiles);
    setIsWon(false);
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    initializeGame();
  }, [ initializeGame ]);

  useEffect(() => {
    const img = new Image();
    img.src = '/explorer/static/4x4-easter-game-cut.png';
    img.onload = () => setImage(img);
  }, []);

  useEffect(() => {
    if (image) {
      tiles.forEach((tile, index) => {
        const canvas = canvasRefs.current[index];
        if (canvas) {
          const ctx = canvas.getContext('2d');
          if (ctx) {
            const tileSize = image.width / 4;
            const srcX = (tile % 4) * tileSize;
            const srcY = Math.floor(tile / 4) * tileSize;
            ctx.drawImage(
              image,
              srcX,
              srcY,
              tileSize,
              tileSize,
              0,
              0,
              canvas.width,
              canvas.height,
            );
          }
        }
      });
    }
  }, [ tiles, image ]);

  const isAdjacent = React.useCallback((index1: number, index2: number) => {
    const row1 = Math.floor(index1 / 4);
    const col1 = index1 % 4;
    const row2 = Math.floor(index2 / 4);
    const col2 = index2 % 4;
    return Math.abs(row1 - row2) + Math.abs(col1 - col2) === 1;
  }, []);

  const checkWinCondition = useCallback((currentTiles: Array<number>) => {
    setIsWon(currentTiles.every((tile, index) => tile === index));
  }, []);

  const moveTile = useCallback((index: number) => {
    const emptyIndex = tiles.indexOf(15);
    if (isAdjacent(index, emptyIndex)) {
      const newTiles = [ ...tiles ];
      [ newTiles[index], newTiles[emptyIndex] ] = [ newTiles[emptyIndex], newTiles[index] ];
      setTiles(newTiles);
      checkWinCondition(newTiles);
    }
  }, [ tiles, isAdjacent, checkWinCondition ]);

  const handleTileClick = useCallback((index: number) => () => {
    if (!isWon) {
      moveTile(index);
    }
  }, [ isWon, moveTile ]);

  return (
    <Flex flexDirection="column" alignItems="center" justifyContent="center" my={ 10 }>
      { isWon && <Confetti/> }
      <Grid
        templateColumns="repeat(4, minmax(0, 1fr))"
        w="full"
        maxW="var(--chakra-spacing-100)"
        aspectRatio={ 1 }
        gap={ 0.5 }
        role="group"
        aria-label="Sliding puzzle"
      >
        { tiles.map((tile, index) => tile === 15 ? (
          <chakra.span key={ tile } aria-hidden="true"/>
        ) : (
          <chakra.button
            key={ tile }
            type="button"
            aria-label={ `Move tile ${ tile + 1 }` }
            aria-disabled={ isWon || !isAdjacent(index, tiles.indexOf(15)) }
            onClick={ handleTileClick(index) }
            position="relative"
            minW={ 0 }
            aspectRatio={ 1 }
            borderWidth="1"
            borderColor="hairline"
            borderRadius="control"
            cursor={ isAdjacent(index, tiles.indexOf(15)) ? 'pointer' : 'default' }
            _hover={{ borderColor: 'accent' }}
            _focusVisible={{ outline: 'var(--chakra-border-widths-2) solid', outlineColor: 'focusRing', zIndex: 'docked' }}
          >
            <chakra.canvas
              ref={ (el: HTMLCanvasElement | null) => {
                canvasRefs.current[index] = el;
              } }
              width={ 512 }
              height={ 512 }
              aria-hidden="true"
              display="block"
              w="full"
              h="full"
              css={{ imageRendering: 'pixelated' }}
            />
            <chakra.span
              position="absolute"
              insetInlineStart={ 1 }
              insetBlockEnd={ 1 }
              bg="surface"
              color="text.primary"
              borderRadius="control"
              px={ 1 }
              textStyle="number"
            >
              { tile + 1 }
            </chakra.span>
          </chakra.button>
        )) }
      </Grid>
      <Text role="status" mt={ 4 }>{ isWon ? 'Puzzle complete!' : 'Put the pieces together to complete the puzzle.' }</Text>
      { !isWon && (
        <>
          <Text mt={ 10 }>Put the pieces together and win a prize</Text>
          <Text mb={ 1 }>Choose a tile next to the empty space. Click or press Enter or Space to move it.</Text>
        </>
      ) }
      { isWon && easterEggPuzzleBadgeFeature.isEnabled && (
        <Flex flexDirection="column" alignItems="center" justifyContent="center" gap={ 4 } mt={ 10 }>
          <Text fontSize="2xl" fontWeight="bold">You unlocked a hidden badge!</Text>
          <Text fontSize="lg" textAlign="center">Congratulations! You're eligible to claim an epic hidden badge!</Text>
          <Button asChild>
            <Link href={ easterEggPuzzleBadgeFeature.badgeClaimLink } external noIcon>Claim</Link>
          </Button>
        </Flex>
      ) }
    </Flex>
  );
};

export default Puzzle15;
