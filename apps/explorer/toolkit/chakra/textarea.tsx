import type { TextareaProps as ChakraTextareaProps } from '@chakra-ui/react';
import { Textarea as ChakraTextarea } from '@chakra-ui/react';

import { recipe as textareaRecipe } from '../theme/recipes/textarea.recipe';
import { withRecipe, type WithRecipeProps } from './recipe-props';

type RecipeProps = WithRecipeProps<ChakraTextareaProps, typeof textareaRecipe>;
const RecipeComponent = withRecipe(ChakraTextarea, textareaRecipe);

export interface TextareaProps extends RecipeProps {}

export const Textarea = RecipeComponent;
