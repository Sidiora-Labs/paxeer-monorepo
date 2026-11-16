import React from 'react';

import { Field } from 'toolkit/chakra/field';
import { Input } from 'toolkit/chakra/input';
import { InputGroup } from 'toolkit/chakra/input-group';
import { FilterInput } from 'toolkit/components/filters/FilterInput';
import IconSvg from 'ui/shared/IconSvg';

import { Section, Container, SectionHeader, SamplesStack, Sample, SectionSubHeader } from './parts';

const InputShowcase = () => {
  return (
    <Container value="input">
      <Section>
        <SectionHeader>Size</SectionHeader>
        <SamplesStack>
          <Sample label="size: sm">
            <Input aria-label="Name" name="name" autoComplete="name" type="text" placeholder="Name" size="sm"/>
          </Sample>
          <Sample label="size: md">
            <Input aria-label="Name" name="name" autoComplete="name" type="text" placeholder="Name" size="md"/>
          </Sample>
          <Sample label="size: lg">
            <Input aria-label="Name" name="name" autoComplete="name" type="text" placeholder="Name" size="lg"/>
          </Sample>
        </SamplesStack>
      </Section>

      <Section>
        <SectionHeader>Variant</SectionHeader>
        <SamplesStack>
          <Sample label="variant: outline" maxW="var(--chakra-spacing-75)">
            <Input aria-label="Name" name="name" autoComplete="name" type="text" placeholder="Name"/>
            <Input aria-label="Name" name="name" autoComplete="name" type="text" placeholder="Name (disabled)" disabled/>
            <Input aria-label="Name" name="name" autoComplete="name" type="text" placeholder="Name (readOnly)" readOnly/>
            <Input aria-label="Name" name="name" autoComplete="name" type="text" placeholder="Name (invalid)" data-invalid/>
            <Input aria-label="Name" name="name" autoComplete="name" type="text" placeholder="Name (invalid)" data-invalid defaultValue="duck"/>
          </Sample>
        </SamplesStack>
      </Section>

      <Section>
        <SectionHeader>Input group</SectionHeader>
        <SamplesStack>
          <Sample label="with end element">
            <Field label="Referral code" required floating size="lg" w="full" maxW="var(--chakra-spacing-75)" flexShrink={ 0 }>
              <InputGroup endElement={ <IconSvg name="copy" boxSize={ 5 }/> } endElementProps={{ px: 3 }}>
                <Input name="referral_code" autoComplete="off"/>
              </InputGroup>
            </Field>
          </Sample>
          <Sample label="with start element">
            <InputGroup startElement={ <IconSvg name="collection" boxSize={ 5 }/> } startElementProps={{ px: 2 }}>
              <Input aria-label="Search" name="search" placeholder="Type in something"/>
            </InputGroup>
          </Sample>
        </SamplesStack>
      </Section>

      <Section>
        <SectionHeader>Examples</SectionHeader>
        <SectionSubHeader>Filter input</SectionSubHeader>
        <SamplesStack>
          <Sample>
            <FilterInput placeholder="Search by method name" name="method" minW={ 0 } w="full" inputProps={{ 'aria-label': 'Search by method name' }}/>
            <FilterInput placeholder="Search by method name" name="method" minW={ 0 } w="full" inputProps={{ 'aria-label': 'Search by method name' }} loading/>
          </Sample>
        </SamplesStack>
      </Section>
    </Container>
  );
};

export default React.memo(InputShowcase);
