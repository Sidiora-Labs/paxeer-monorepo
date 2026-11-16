import React from 'react';

import { Tooltip } from 'ui/marketplace/NativeTooltip';
import Utilization from 'ui/shared/Utilization/Utilization';

import { Section, Container, SectionHeader, SamplesStack, Sample } from './parts';

const TooltipShowcase = () => {
  return (
    <Container value="tooltip">
      <Section>
        <SectionHeader>Variant</SectionHeader>
        <SamplesStack>
          <Sample label="variant: regular">
            <Tooltip content="Tooltip content">
              <button type="button">Default</button>
            </Tooltip>
            <Tooltip content="Tooltip content" interactive>
              <button type="button" aria-label="Utilization 50%"><Utilization value={ 0.5 }/></button>
            </Tooltip>
          </Sample>
          <Sample label="variant: popover">
            <Tooltip content="Tooltip content" variant="popover">
              <button type="button">Default</button>
            </Tooltip>
          </Sample>
        </SamplesStack>
      </Section>
    </Container>
  );
};

export default React.memo(TooltipShowcase);
