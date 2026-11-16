import React from 'react';

import { EmptyState } from 'toolkit/chakra/empty-state';

import { Section, Container, SectionHeader, SamplesStack, Sample } from './parts';

const EmptyStateShowcase = () => {
  return (
    <Container value="empty-state">
      <Section>
        <SectionHeader>Type</SectionHeader>
        <SamplesStack>
          <Sample label="type: query">
            <EmptyState term="transaction" outline="var(--chakra-border-widths-1) dashed var(--chakra-colors-hairline)"/>
          </Sample>
          <Sample label="type: stats">
            <EmptyState type="stats" term="transactions" outline="var(--chakra-border-widths-1) dashed var(--chakra-colors-hairline)"/>
          </Sample>
          <Sample label="type: coming_soon">
            <EmptyState type="coming_soon" outline="var(--chakra-border-widths-1) dashed var(--chakra-colors-hairline)"/>
          </Sample>
        </SamplesStack>
      </Section>
    </Container>
  );
};

export default React.memo(EmptyStateShowcase);
