import React from 'react';

import type { FormFields } from '../types';

import { Link } from 'toolkit/chakra/link';
import { FormFieldText } from 'toolkit/components/forms/fields/FormFieldText';

import ContractVerificationFormRow from '../ContractVerificationFormRow';

// Plug.Parsers limit in the backend's smart_contracts_api_v2_router.ex.
const MAX_VERIFICATION_PAYLOAD_SIZE = 20_000_000;

const ContractVerificationFieldConstructorArgs = () => {
  return (
    <ContractVerificationFormRow>
      <FormFieldText<FormFields, 'constructor_args'>
        name="constructor_args"
        rules={{
          maxLength: {
            value: MAX_VERIFICATION_PAYLOAD_SIZE,
            message: 'Constructor arguments exceed the verification payload limit of 20,000,000 bytes',
          },
          validate: (value) => !value || (
            value.length % 2 === 0 && !/[^\da-f]/i.test(value.replace(/^0x/i, ''))
          ) || 'Enter an even number of hexadecimal characters, optionally prefixed with 0x',
        }}
        placeholder="ABI-encoded Constructor Arguments"
        asComponent="Textarea"
      />
      <>
        <span>Add arguments in </span>
        <Link href="https://solidity.readthedocs.io/en/develop/abi-spec.html" external noIcon>ABI hex encoded form</Link>
        <span> if required by the contract. Constructor arguments are written right to left, and will be found at the end of the input created bytecode.</span>
        <span> They may also be </span>
        <Link href="https://abi.hashex.org/" external noIcon>parsed here</Link>
        <span>.</span>
      </>
    </ContractVerificationFormRow>
  );
};

export default React.memo(ContractVerificationFieldConstructorArgs);
