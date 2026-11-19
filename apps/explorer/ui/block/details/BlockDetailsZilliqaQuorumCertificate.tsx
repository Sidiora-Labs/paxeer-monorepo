import { Separator, Grid, GridItem, Flex, Text } from '@chakra-ui/react';
import React from 'react';

import type { ZilliqaNestedQuorumCertificate, ZilliqaQuorumCertificate } from 'types/api/block';

import { AccordionRoot, AccordionItem, AccordionItemTrigger, AccordionItemContent } from 'toolkit/chakra/accordion';
import { Hint } from 'toolkit/components/Hint/Hint';
import { apos, ndash } from 'toolkit/utils/htmlEntities';
import CopyToClipboard from 'ui/shared/CopyToClipboard';
import * as DetailedInfo from 'ui/shared/DetailedInfo/DetailedInfo';

function formatSigners(signers: Array<number>) {
  return `[${ signers.join(', ') }]`;
}

interface Props {
  data: ZilliqaQuorumCertificate & {
    nested_quorum_certificates?: Array<ZilliqaNestedQuorumCertificate>;
  };
}

const BlockDetailsZilliqaQuorumCertificate = ({ data }: Props) => {
  const hint = (isNested?: boolean) => (
    <>
      The iteration of the consensus round in which the block was proposed:<br/><br/>
      View { ndash } the view number of the quorum certificate, indicating the consensus round.<br/><br/>
      Signature { ndash } aggregated BLS signature representing the validators{ apos } agreement.<br/><br/>
      Signers { ndash } an array of integers representing the indices of validators who participated in the quorum (indicated by the cosigned bit vector).
      { isNested && (
        <>
          <br/><br/>
          Proposed by validator { ndash } validator index proposing the nested quorum certificate.
        </>
      ) }
    </>
  );

  return (
    <>
      <DetailedInfo.ItemLabel
        hint={ hint() }
      >
        { data.nested_quorum_certificates ? 'Aggregate quorum certificate' : 'Quorum certificate' }
      </DetailedInfo.ItemLabel>
      <DetailedInfo.ItemValue flexWrap="wrap">
        <Grid
          minW={ 0 }
          w="full"
          textStyle="body"
          gridTemplateColumns={{ base: 'minmax(0, 1fr)', lg: 'min-content minmax(0, 1fr)' }}
          rowGap={ 2 }
          columnGap={ 5 }
          mt={{ base: 2, lg: 1.5 }}
        >
          <GridItem fontWeight="semibold">View</GridItem>
          <GridItem>{ data.view }</GridItem>
          <DetailedInfo.ItemDivider my={{ base: 2, lg: 2 }} colSpan={{ base: 1, lg: 2 }}/>
          <GridItem fontWeight="semibold">Signature</GridItem>
          <GridItem minW={ 0 } display="flex" alignItems="flex-start">
            <Text minW={ 0 } flexGrow={ 1 } whiteSpace="pre-wrap" overflowWrap="anywhere">{ data.signature }</Text>
            <CopyToClipboard text={ data.signature }/>
          </GridItem>
          <DetailedInfo.ItemDivider my={{ base: 2, lg: 2 }} colSpan={{ base: 1, lg: 2 }}/>
          <GridItem fontWeight="semibold">Signers</GridItem>
          <GridItem whiteSpace="pre-wrap">{ formatSigners(data.signers) }</GridItem>
        </Grid>
        { data.nested_quorum_certificates && data.nested_quorum_certificates.length > 0 && (
          <>
            <Separator mt={ 2 } w="100%"/>
            <AccordionRoot
              css={{ '& [data-scope="accordion"][data-part="item-content"]': { _motionReduce: { animation: 'none' } } }}
              multiple
              w="100%"
              textStyle="body"
            >
              <AccordionItem
                value="nested-quorum-certificates"
                borderWidth={ 0 }
                _last={{ borderBottomWidth: 0 }}
              >
                <Flex alignItems="center" gap={ 2 }>
                  <AccordionItemTrigger
                    textStyle="body"
                    fontWeight="semibold"
                  >
                    <span>Nested quorum certificates</span>
                  </AccordionItemTrigger>
                  <Hint label={ hint(true) }/>
                </Flex>
                <AccordionItemContent display="flex" flexDirection="column" rowGap={ 2 } p={ 0 }>
                  { data.nested_quorum_certificates?.map((item, index) => (
                    <Grid
                      key={ index }
                      gridTemplateColumns={{ base: 'minmax(0, 1fr)', lg: 'var(--chakra-spacing-24) minmax(0, 1fr)' }}
                      columnGap={ 3 }
                      rowGap={ 2 }
                      minW={ 0 }
                      bgColor="surface"
                      p={ 4 }
                      borderRadius="md"
                      _first={{ borderTopRightRadius: 0, borderTopLeftRadius: 0 }}
                    >
                      <GridItem>View</GridItem>
                      <GridItem>{ item.view }</GridItem>
                      <GridItem>Signature</GridItem>
                      <GridItem minW={ 0 } display="flex" alignItems="flex-start">
                        <Text minW={ 0 } flexGrow={ 1 } whiteSpace="pre-wrap" overflowWrap="anywhere">{ item.signature }</Text>
                        <CopyToClipboard text={ item.signature }/>
                      </GridItem>
                      <GridItem>Signers</GridItem>
                      <GridItem whiteSpace="pre-wrap">{ formatSigners(item.signers) }</GridItem>
                      <GridItem whiteSpace="pre-wrap">Proposed by validator</GridItem>
                      <GridItem >{ item.proposed_by_validator_index }</GridItem>
                    </Grid>
                  )) }
                </AccordionItemContent>
              </AccordionItem>
            </AccordionRoot>
          </>
        ) }
      </DetailedInfo.ItemValue>
    </>
  );
};

export default React.memo(BlockDetailsZilliqaQuorumCertificate);
