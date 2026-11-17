import React from 'react';

import { Field } from 'toolkit/chakra/field';
import { Input } from 'toolkit/chakra/input';

import { Section, Container, SectionHeader, SamplesStack, Sample, SectionSubHeader } from './parts';

const FieldShowcase = () => {
  return (
    <Container value="field">
      <Section>
        <SectionHeader>Size</SectionHeader>
        <SectionSubHeader>Default</SectionSubHeader>
        <SamplesStack>
          { ([ 'sm', 'md', 'lg' ] as const).map((size) => (
            <Sample label={ `size: ${ size }` } w="100%" key={ size } alignItems="flex-start">
              <Field label="Email" required size={ size } helperText="Helper text" maxWidth="calc(var(--chakra-spacing-1) * 50)">
                <Input name="email" type="email" autoComplete="email"/>
              </Field>
              <Field label="Email (disabled)" required size={ size } maxWidth="calc(var(--chakra-spacing-1) * 50)">
                <Input name="email" type="email" autoComplete="email" disabled value="me@example.com"/>
              </Field>
              <Field label="Email (readOnly)" required size={ size } maxWidth="calc(var(--chakra-spacing-1) * 50)">
                <Input name="email" type="email" autoComplete="email" readOnly value="me@example.com"/>
              </Field>
              <Field
                label="Email (invalid)"
                required
                size={ size }
                errorText="Something went wrong"
                invalid
                maxWidth="calc(var(--chakra-spacing-1) * 50)"
              >
                <Input name="email" type="email" autoComplete="email" defaultValue="duck"/>
              </Field>
            </Sample>
          )) }

          <Sample label="size: xl" w="100%" alignItems="flex-start">
            <Field label="Email" required floating size="lg" helperText="Helper text" maxWidth="var(--chakra-spacing-75)">
              <Input name="email" type="email" autoComplete="email"/>
            </Field>
            <Field label="Email (disabled)" required floating disabled size="lg" maxWidth="var(--chakra-spacing-75)">
              <Input name="email" type="email" autoComplete="email" value="me@example.com"/>
            </Field>
            <Field label="Email (readOnly)" required floating readOnly size="lg" maxWidth="var(--chakra-spacing-75)">
              <Input name="email" type="email" autoComplete="email" value="me@example.com"/>
            </Field>
            <Field label="Email (invalid)" required floating size="lg" errorText="Something went wrong" invalid maxWidth="var(--chakra-spacing-75)">
              <Input name="email" type="email" autoComplete="email" defaultValue="duck"/>
            </Field>
          </Sample>
        </SamplesStack>

        <SectionSubHeader>On custom background</SectionSubHeader>
        <SamplesStack>
          <Sample label="no floating label" p={ 4 } bgColor="hairline" alignItems="flex-start">
            <Field label="Email" required helperText="Helper text" maxWidth="calc(var(--chakra-spacing-1) * 50)">
              <Input name="email" type="email" autoComplete="email"/>
            </Field>
            <Field label="Email (disabled)" required disabled maxWidth="calc(var(--chakra-spacing-1) * 50)">
              <Input name="email" type="email" autoComplete="email" value="me@example.com"/>
            </Field>
            <Field label="Email (readOnly)" required readOnly maxWidth="calc(var(--chakra-spacing-1) * 50)">
              <Input name="email" type="email" autoComplete="email" value="me@example.com"/>
            </Field>
            <Field label="Email (invalid)" required errorText="Something went wrong" invalid maxWidth="calc(var(--chakra-spacing-1) * 50)">
              <Input name="email" type="email" autoComplete="email" defaultValue="duck"/>
            </Field>
          </Sample>
          <Sample label="floating label" p={ 4 } bgColor="hairline" alignItems="flex-start">
            <Field label="Email" required floating size="lg" helperText="Helper text" maxWidth="var(--chakra-spacing-75)">
              <Input name="email" type="email" autoComplete="email"/>
            </Field>
            <Field label="Email (disabled)" required disabled floating size="lg" maxWidth="var(--chakra-spacing-75)">
              <Input name="email" type="email" autoComplete="email" value="me@example.com"/>
            </Field>
            <Field label="Email (readOnly)" required readOnly floating size="lg" maxWidth="var(--chakra-spacing-75)">
              <Input name="email" type="email" autoComplete="email" value="me@example.com"/>
            </Field>
            <Field label="Email (invalid)" required floating size="lg" errorText="Something went wrong" invalid maxWidth="var(--chakra-spacing-75)">
              <Input name="email" type="email" autoComplete="email" defaultValue="duck"/>
            </Field>
          </Sample>
        </SamplesStack>
      </Section>
    </Container>
  );
};

export default React.memo(FieldShowcase);
