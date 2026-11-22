import { Grid } from '@chakra-ui/react';
import React from 'react';

import { Heading } from 'toolkit/chakra/heading';

import { verificationColumns } from './styles';

interface Props {
  title: string;
  children: React.ReactNode;
  disableScroll?: boolean;
}

const ContractVerificationMethod = ({ title, children, disableScroll }: Props) => {
  const ref = React.useRef<HTMLDivElement>(null);

  React.useEffect(() => {
    !disableScroll && ref.current?.scrollIntoView({ behavior: 'smooth' });
  }, [ disableScroll ]);

  return (
    <section ref={ ref }>
      <Heading level="2" mt={ 12 } mb={ 5 }>{ title }</Heading>
      <Grid
        columnGap={ 7.5 }
        rowGap={{ base: 2, lg: 5 }}
        templateColumns={ verificationColumns }
      >
        { children }
      </Grid>
    </section>
  );
};

export default React.memo(ContractVerificationMethod);
