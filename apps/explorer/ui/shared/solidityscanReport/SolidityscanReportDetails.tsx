import { Box, Flex, Text, Grid, chakra } from '@chakra-ui/react';
import React from 'react';

import type { SolidityScanReportSeverityDistribution } from 'lib/solidityScan/schema';

type DistributionItem = {
  id: keyof SolidityScanReportSeverityDistribution;
  name: string;
  color: string;
};

const DISTRIBUTION_ITEMS: Array<DistributionItem> = [
  { id: 'critical', name: 'Critical', color: 'securitySeverity.critical' },
  { id: 'high', name: 'High', color: 'securitySeverity.high' },
  { id: 'medium', name: 'Medium', color: 'securitySeverity.medium' },
  { id: 'low', name: 'Low', color: 'securitySeverity.low' },
  { id: 'informational', name: 'Informational', color: 'securitySeverity.informational' },
  { id: 'gas', name: 'Gas', color: 'securitySeverity.gas' },
];

interface Props {
  vulnerabilities: SolidityScanReportSeverityDistribution;
  vulnerabilitiesCount: number;
}

type ItemProps = {
  item: DistributionItem;
  vulnerabilities: SolidityScanReportSeverityDistribution;
  vulnerabilitiesCount: number;
};

const SolidityScanReportItem = ({ item, vulnerabilities, vulnerabilitiesCount }: ItemProps) => {
  const vulnerability = vulnerabilities[item.id];

  if (vulnerability === undefined) {
    return null;
  }

  return (
    <>
      <Box w={ 3 } h={ 3 } bg={ item.color } borderRadius="6px" mr={ 2 }></Box>
      <Flex justifyContent="space-between" mr={ 3 }>
        <Text>{ item.name }</Text>
        <Text color={ vulnerability > 0 ? 'text.primary' : 'text.secondary' }>{ vulnerabilities[item.id] }</Text>
      </Flex>
      <Box bg={{ _light: 'blackAlpha.50', _dark: 'whiteAlpha.50' }} h="10px" borderRadius="8px">
        <Box bg={ item.color } w={ `${ vulnerability / vulnerabilitiesCount * 100 }%` } h="10px" borderRadius="8px"/>
      </Box>
    </>
  );
};

const SolidityscanReportDetails = ({ vulnerabilities, vulnerabilitiesCount }: Props) => {
  return (
    <Grid templateColumns="20px 1fr 100px" alignItems="center" rowGap={ 2 }>
      { DISTRIBUTION_ITEMS.map(item => (
        <SolidityScanReportItem item={ item } key={ item.id } vulnerabilities={ vulnerabilities } vulnerabilitiesCount={ vulnerabilitiesCount }/>
      )) }
    </Grid>
  );
};

export default chakra(SolidityscanReportDetails);
