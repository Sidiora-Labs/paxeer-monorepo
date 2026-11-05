import { Tabs as ChakraTabs, chakra } from '@chakra-ui/react';
import * as React from 'react';

import { recipe as tabsRecipe } from '../theme/recipes/tabs.recipe';
import { withRecipe, type WithRecipeProps } from './recipe-props';

type RecipeRootProps = WithRecipeProps<ChakraTabs.RootProps, typeof tabsRecipe>;
const RecipeRoot = withRecipe(ChakraTabs.Root, tabsRecipe);

export interface TabsProps extends RecipeRootProps {}

export const TabsRoot = React.forwardRef<HTMLDivElement, TabsProps>(
  function TabsRoot(props, ref) {
    const { lazyMount = true, unmountOnExit = true, ...rest } = props;
    return <RecipeRoot ref={ ref } { ...rest } lazyMount={ lazyMount } unmountOnExit={ unmountOnExit }/>;
  },
);

export const TabsList = ChakraTabs.List;

export interface TabsTriggerProps extends ChakraTabs.TriggerProps {}

export const TabsTrigger = React.forwardRef<HTMLButtonElement, TabsTriggerProps>(
  function TabsTrigger(props, ref) {
    return <ChakraTabs.Trigger ref={ ref } className="group" { ...props }/>;
  },
);

export const TabsContent = ChakraTabs.Content;

export interface TabsCounterProps {
  count?: number | null;
}

export const TabsCounter = ({ count }: TabsCounterProps) => {
  const COUNTER_OVERLOAD = 50;

  if (count === undefined || count === null) {
    return null;
  }

  return (
    <chakra.span
      color="text.muted"
      textStyle="number"
    >
      { count > COUNTER_OVERLOAD ? `${ COUNTER_OVERLOAD }+` : count }
    </chakra.span>
  );
};
