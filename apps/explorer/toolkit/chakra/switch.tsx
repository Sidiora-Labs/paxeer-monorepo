import { Switch as ChakraSwitch } from '@chakra-ui/react';
import * as React from 'react';

import { recipe as switchRecipe } from '../theme/recipes/switch.recipe';
import { withRecipe, type WithRecipeProps } from './recipe-props';

type RecipeRootProps = WithRecipeProps<ChakraSwitch.RootProps, typeof switchRecipe>;
const RecipeRoot = withRecipe(ChakraSwitch.Root, switchRecipe);

export interface SwitchProps extends RecipeRootProps {
  inputProps?: React.InputHTMLAttributes<HTMLInputElement>;
  labelProps?: ChakraSwitch.LabelProps;
  rootRef?: React.Ref<HTMLLabelElement>;
  trackLabel?: { on: React.ReactNode; off: React.ReactNode };
  thumbLabel?: { on: React.ReactNode; off: React.ReactNode };
}

export const Switch = React.forwardRef<HTMLInputElement, SwitchProps>(
  function Switch(props, ref) {
    const { inputProps, children, rootRef, trackLabel, thumbLabel, labelProps, ...rest } =
      props;

    return (
      <RecipeRoot ref={ rootRef } { ...rest }>
        <ChakraSwitch.HiddenInput ref={ ref } { ...inputProps }/>
        <ChakraSwitch.Control>
          <ChakraSwitch.Thumb>
            { thumbLabel && (
              <ChakraSwitch.ThumbIndicator fallback={ thumbLabel?.off }>
                { thumbLabel?.on }
              </ChakraSwitch.ThumbIndicator>
            ) }
          </ChakraSwitch.Thumb>
          { trackLabel && (
            <ChakraSwitch.Indicator fallback={ trackLabel.off }>
              { trackLabel.on }
            </ChakraSwitch.Indicator>
          ) }
        </ChakraSwitch.Control >
        { children != null && (
          <ChakraSwitch.Label { ...labelProps }>{ children }</ChakraSwitch.Label>
        ) }
      </RecipeRoot>
    );
  },
);
