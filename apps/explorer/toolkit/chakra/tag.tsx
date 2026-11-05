import { chakra, Tag as ChakraTag } from '@chakra-ui/react';
import * as React from 'react';

import { TruncatedTextTooltip } from '../components/truncation/TruncatedTextTooltip';
import { recipe as tagRecipe } from '../theme/recipes/tag.recipe';
import { nbsp } from '../utils/htmlEntities';
import { CloseButton } from './close-button';
import { withRecipe, type WithRecipeProps } from './recipe-props';
import { Skeleton } from './skeleton';

type RecipeRootProps = WithRecipeProps<ChakraTag.RootProps, typeof tagRecipe>;
const RecipeRoot = withRecipe(ChakraTag.Root, tagRecipe);

export interface TagProps extends RecipeRootProps {
  startElement?: React.ReactNode;
  endElement?: React.ReactNode;
  endElementProps?: ChakraTag.EndElementProps;
  label?: string;
  onClose?: VoidFunction;
  closable?: boolean;
  truncated?: boolean;
  loading?: boolean;
  selected?: boolean;
  disabled?: boolean;
}

export const Tag = React.forwardRef<HTMLSpanElement, TagProps>(
  function Tag(props, ref) {
    const {
      startElement,
      endElement,
      endElementProps,
      label,
      onClose,
      closable = Boolean(onClose),
      children,
      truncated = false,
      loading,
      selected,
      disabled,
      ...rest
    } = props;

    const labelElement = label ? (
      <chakra.span color="text.secondary">{ label }:{ nbsp }</chakra.span>
    ) : null;

    const contentElement = truncated ? (
      <TruncatedTextTooltip label={ children }>
        <ChakraTag.Label>{ labelElement }{ children }</ChakraTag.Label>
      </TruncatedTextTooltip>
    ) : <ChakraTag.Label>{ labelElement }{ children }</ChakraTag.Label>;

    return (
      <Skeleton loading={ loading } asChild>
        <RecipeRoot
          ref={ ref }
          { ...(selected && !loading && { 'data-selected': true }) }
          { ...(disabled && { 'data-disabled': true }) }
          { ...rest }
        >
          { startElement && (
            <ChakraTag.StartElement _empty={{ display: 'none' }}>{ startElement }</ChakraTag.StartElement>
          ) }
          { contentElement }
          { endElement && (
            <ChakraTag.EndElement { ...endElementProps }>{ endElement }</ChakraTag.EndElement>
          ) }
          { closable && (
            <ChakraTag.EndElement>
              <ChakraTag.CloseTrigger onClick={ onClose } asChild>
                <CloseButton/>
              </ChakraTag.CloseTrigger>
            </ChakraTag.EndElement>
          ) }
        </RecipeRoot>
      </Skeleton>
    );
  },
);
