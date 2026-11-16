import { Flex } from '@chakra-ui/react';
import React from 'react';

import type { FormFields } from '../types';

import { FormFieldCheckbox } from 'toolkit/components/forms/fields/FormFieldCheckbox';
import { FormFieldText } from 'toolkit/components/forms/fields/FormFieldText';

import ContractVerificationFormRow from '../ContractVerificationFormRow';

const ContractVerificationFieldOptimization = () => {
  const [ isEnabled, setIsEnabled ] = React.useState(true);

  const handleCheckboxChange = React.useCallback(() => {
    setIsEnabled(prev => !prev);
  }, []);

  return (
    <ContractVerificationFormRow>
      <Flex columnGap={ 5 } alignItems="start" flexWrap="wrap">
        <FormFieldCheckbox<FormFields, 'is_optimization_enabled'>
          name="is_optimization_enabled"
          label="Optimization enabled"
          onChange={ handleCheckboxChange }
          flexShrink={ 0 }
        />
        { isEnabled && (
          <FormFieldText<FormFields, 'optimization_runs'>
            name="optimization_runs"
            required
            placeholder="Optimization runs"
            inputProps={{
              type: 'number',
            }}
            size="sm"
            minW="24"
            maxW="48"
            flexShrink={ 1 }
          />
        ) }
      </Flex>
    </ContractVerificationFormRow>
  );
};

export default React.memo(ContractVerificationFieldOptimization);
