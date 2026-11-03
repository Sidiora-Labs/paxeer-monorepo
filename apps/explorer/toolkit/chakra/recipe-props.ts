import type { RecipeDefinition, RecipeVariantProps, SlotRecipeDefinition } from '@chakra-ui/react';
import type * as React from 'react';

type Recipe = RecipeDefinition | SlotRecipeDefinition;

export type WithRecipeProps<Props, Definition extends Recipe> =
  Omit<Props, keyof RecipeVariantProps<Definition>> & RecipeVariantProps<Definition>;

export function withRecipe<Props, Definition extends Recipe>(
  component: React.ComponentType<Props>,
  _recipe: Definition,
): React.ComponentType<WithRecipeProps<Props, Definition>> {
  return component as unknown as React.ComponentType<WithRecipeProps<Props, Definition>>;
}
