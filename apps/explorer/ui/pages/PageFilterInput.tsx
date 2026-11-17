import { Box, chakra } from '@chakra-ui/react';
import React from 'react';

import type { FilterInputProps } from 'toolkit/components/filters/FilterInput';
import { FilterInput } from 'toolkit/components/filters/FilterInput';

export function PageFilterInput(props: FilterInputProps) {
  const generatedId = React.useId();
  const {
    onChange, onFocus, onBlur, loading, size, placeholder, initialValue, type, name, inputProps,
    ...layoutProps
  } = props;
  const id = inputProps?.id ?? generatedId;

  return (
    <Box minW={ 0 } maxW="100%" { ...layoutProps }>
      <chakra.label htmlFor={ id } display="block" textStyle="small" mb={ 2 }>{ placeholder }</chakra.label>
      <FilterInput
        minW={ 0 }
        w="100%"
        onChange={ onChange }
        onFocus={ onFocus }
        onBlur={ onBlur }
        loading={ loading }
        size={ size }
        placeholder={ placeholder }
        initialValue={ initialValue }
        type={ type }
        name={ name ?? 'search' }
        inputProps={{ id, autoCorrect: 'off', autoCapitalize: 'none', spellCheck: false, enterKeyHint: 'search', ...inputProps }}
      />
    </Box>
  );
}
