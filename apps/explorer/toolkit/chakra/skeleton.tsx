import type {
  SkeletonProps as ChakraSkeletonProps,
  CircleProps,
} from '@chakra-ui/react';
import { Skeleton as ChakraSkeleton, Circle, Stack } from '@chakra-ui/react';
import * as React from 'react';

import { recipe as skeletonRecipe } from '../theme/recipes/skeleton.recipe';
import { withRecipe, type WithRecipeProps } from './recipe-props';

type RecipeProps = WithRecipeProps<ChakraSkeletonProps, typeof skeletonRecipe>;
const RecipeComponent = withRecipe(ChakraSkeleton, skeletonRecipe);

export interface SkeletonCircleProps extends RecipeProps {
  size?: CircleProps['size'];
}

export const SkeletonCircle = React.forwardRef<
  HTMLDivElement,
  SkeletonCircleProps
>(function SkeletonCircle(props, ref) {
  const { size, ...rest } = props;
  return (
    <Circle size={ size } asChild ref={ ref }>
      <RecipeComponent { ...rest }/>
    </Circle>
  );
});

export interface SkeletonTextProps extends RecipeProps {
  noOfLines?: number;
}

export const SkeletonText = React.forwardRef<HTMLDivElement, SkeletonTextProps>(
  function SkeletonText(props, ref) {
    const { noOfLines = 3, gap, ...rest } = props;
    return (
      <Stack gap={ gap } width="full" ref={ ref }>
        { Array.from({ length: noOfLines }).map((_, index) => (
          <RecipeComponent
            height="4"
            key={ index }
            { ...props }
            _last={{ maxW: '80%' }}
            { ...rest }
          />
        )) }
      </Stack>
    );
  },
);

export interface SkeletonProps extends Omit<RecipeProps, 'loading'> {
  loading: boolean | undefined;
}

export const Skeleton = React.forwardRef<HTMLDivElement, SkeletonProps>(
  function Skeleton(props, ref) {
    const { loading = false, ...rest } = props;
    return (
      <RecipeComponent
        ref={ ref }
        { ...(loading ? { 'data-loading': true, state: 'loading' as const } : { variant: 'none' }) }
        { ...rest }
      />
    );
  },
);
