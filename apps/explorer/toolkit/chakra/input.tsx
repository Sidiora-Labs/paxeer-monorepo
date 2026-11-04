import type { InputProps as ChakraInputProps } from '@chakra-ui/react';
import { Input as ChakraInput } from '@chakra-ui/react';

import { recipe as inputRecipe } from '../theme/recipes/input.recipe';
import { withRecipe, type WithRecipeProps } from './recipe-props';

type RecipeProps = WithRecipeProps<ChakraInputProps, typeof inputRecipe>;
const RecipeComponent = withRecipe(ChakraInput, inputRecipe);

export interface InputProps extends RecipeProps {}

export const Input = RecipeComponent;
