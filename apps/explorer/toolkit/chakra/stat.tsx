import { Stat as ChakraStat } from '@chakra-ui/react';

import { recipe as statRecipe } from '../theme/recipes/stat.recipe';
import { withRecipe } from './recipe-props';

export const Stat = {
  ...ChakraStat,
  Root: withRecipe(ChakraStat.Root, statRecipe),
};
