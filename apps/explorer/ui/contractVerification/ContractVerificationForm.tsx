import { Grid, Text, chakra } from '@chakra-ui/react';
import React from 'react';
import type { SubmitHandler } from 'react-hook-form';
import { useForm, FormProvider } from 'react-hook-form';

import type { FormFields } from './types';
import type { SmartContract, SmartContractVerificationMethodApi } from 'types/api/contract';
import type { SmartContractVerificationConfig } from 'types/client/contract';

import { route } from 'nextjs-routes';

import useApiFetch from 'lib/api/useApiFetch';
import getErrorObjStatusCode from 'lib/errors/getErrorObjStatusCode';
import useRewardsActivity from 'lib/hooks/useRewardsActivity';
import * as mixpanel from 'lib/mixpanel/index';
import { Button } from 'toolkit/chakra/button';
import { toaster } from 'toolkit/chakra/toaster';
import { useUpdateEffect } from 'toolkit/hooks/useUpdateEffect';

import ContractVerificationFieldAddress from './fields/ContractVerificationFieldAddress';
import ContractVerificationFieldLicenseType from './fields/ContractVerificationFieldLicenseType';
import ContractVerificationFieldMethod from './fields/ContractVerificationFieldMethod';
import ContractVerificationFlattenSourceCode from './methods/ContractVerificationFlattenSourceCode';
import ContractVerificationMultiPartFile from './methods/ContractVerificationMultiPartFile';
import ContractVerificationSolidityFoundry from './methods/ContractVerificationSolidityFoundry';
import ContractVerificationSolidityHardhat from './methods/ContractVerificationSolidityHardhat';
import ContractVerificationSourcify from './methods/ContractVerificationSourcify';
import ContractVerificationStandardInput from './methods/ContractVerificationStandardInput';
import ContractVerificationStylusGitHubRepo from './methods/ContractVerificationStylusGitHubRepo';
import ContractVerificationVyperContract from './methods/ContractVerificationVyperContract';
import ContractVerificationVyperMultiPartFile from './methods/ContractVerificationVyperMultiPartFile';
import ContractVerificationVyperStandardInput from './methods/ContractVerificationVyperStandardInput';
import type { VerificationSubmission } from './useVerificationSubmission';
import useVerificationSubmission from './useVerificationSubmission';
import { getDefaultValues, METHOD_LABELS } from './utils';

interface Props {
  method?: SmartContractVerificationMethodApi;
  config: SmartContractVerificationConfig;
  hash?: string;
}

const ContractVerificationForm = ({ method: methodFromQuery, config, hash }: Props) => {
  const formApi = useForm<FormFields>({
    mode: 'onBlur',
    defaultValues: getDefaultValues(methodFromQuery, config, hash, []),
  });
  const { handleSubmit, watch, formState, setError, reset, getFieldState, clearErrors } = formApi;
  const submissionRef = React.useRef<VerificationSubmission>(undefined);
  const methodNameRef = React.useRef<string>(undefined);

  const apiFetch = useApiFetch();
  const { trackContract } = useRewardsActivity();
  const address = watch('address');
  const addressState = getFieldState('address');
  const handleSuccess = React.useCallback((verifiedAddress: string) => {
    toaster.success({
      title: 'Success',
      description: 'Contract is successfully verified.',
    });
    mixpanel.logEvent(
      mixpanel.EventTypes.CONTRACT_VERIFICATION,
      { Status: 'Finished', Method: methodNameRef.current || '' },
      { send_immediately: true },
    );
    window.location.assign('/explorer' + route({ pathname: '/address/[hash]', query: { hash: verifiedAddress, tab: 'contract' } }));
  }, []);
  const submitVerification = useVerificationSubmission({
    submissionRef,
    address,
    isDisabled: !address || Boolean(addressState.error),
    formApi,
    onSuccess: handleSuccess,
  });

  const onFormSubmit: SubmitHandler<FormFields> = React.useCallback(async(data) => {
    if (!hash) {
      try {
        const response = await apiFetch<'general:contract', SmartContract>('general:contract', {
          pathParams: { hash: data.address.toLowerCase() },
        });
        if ('is_verified' in response && response.is_verified && !response.is_partially_verified) {
          setError('address', { message: 'Contract has already been verified' });
          return;
        }
      } catch (error) {
        const statusCode = getErrorObjStatusCode(error);
        setError('address', { message: statusCode === 404 ? 'Address is not a smart contract' : 'Something went wrong' });
        return;
      }
    }
    await trackContract(data.address);
    return submitVerification(data);
  }, [ apiFetch, hash, setError, trackContract, submitVerification ]);

  const handleFormChange = React.useCallback(() => {
    clearErrors('root');
  }, [ clearErrors ]);

  const methods = React.useMemo(() => {
    return {
      'flattened-code': <ContractVerificationFlattenSourceCode config={ config }/>,
      'standard-input': <ContractVerificationStandardInput config={ config }/>,
      sourcify: <ContractVerificationSourcify/>,
      'multi-part': <ContractVerificationMultiPartFile config={ config }/>,
      'vyper-code': <ContractVerificationVyperContract config={ config }/>,
      'vyper-multi-part': <ContractVerificationVyperMultiPartFile config={ config }/>,
      'vyper-standard-input': <ContractVerificationVyperStandardInput config={ config }/>,
      'solidity-hardhat': <ContractVerificationSolidityHardhat config={ config }/>,
      'solidity-foundry': <ContractVerificationSolidityFoundry/>,
      'stylus-github-repository': <ContractVerificationStylusGitHubRepo config={ config }/>,
    };
  }, [ config ]);
  const method = watch('method');
  const methodValue = method?.[0];
  const licenseType = watch('license_type');
  const content = methods[methodValue] || null;

  useUpdateEffect(() => {
    if (methodValue) {
      reset(getDefaultValues(methodValue, config, hash || address, licenseType));

      const methodName = METHOD_LABELS[methodValue];
      mixpanel.logEvent(mixpanel.EventTypes.CONTRACT_VERIFICATION, { Status: 'Method selected', Method: methodName });
      methodNameRef.current = methodName;
    }
  // !!! should run only when method is changed
  }, [ methodValue ]);

  return (
    <FormProvider { ...formApi }>
      <chakra.form
        noValidate
        onSubmit={ handleSubmit(onFormSubmit) }
        onChange={ handleFormChange }
      >
        <Grid as="section" columnGap="30px" rowGap={{ base: 2, lg: 5 }} templateColumns={{ base: '1fr', lg: 'minmax(auto, 680px) minmax(0, 340px)' }}>
          { !hash && <ContractVerificationFieldAddress/> }
          <ContractVerificationFieldLicenseType/>
          <ContractVerificationFieldMethod methods={ config.verification_options }/>
        </Grid>
        { content }
        { formState.errors.root?.message && <Text color="text.error" mt={ 4 } fontSize="sm" whiteSpace="pre-wrap">{ formState.errors.root.message }</Text> }
        { Boolean(method) && methodValue !== 'solidity-hardhat' && methodValue !== 'solidity-foundry' && methodValue !== 'sourcify' && (
          <Button
            size="md"
            type="submit"
            mt={ 12 }
            loading={ formState.isSubmitting }
            loadingText="Verify & publish"
          >
            Verify & publish
          </Button>
        ) }
      </chakra.form>
    </FormProvider>
  );
};

export default React.memo(ContractVerificationForm);
