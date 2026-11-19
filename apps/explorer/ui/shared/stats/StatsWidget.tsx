import { Box, Flex, Text, chakra } from '@chakra-ui/react';
import React from 'react';

import type { Route } from 'nextjs-routes';
import { route } from 'nextjs-routes';

import { Link } from 'toolkit/chakra/link';
import { Skeleton } from 'toolkit/chakra/skeleton';
import { Hint } from 'toolkit/components/Hint/Hint';
import IconSvg, { type IconName } from 'ui/shared/IconSvg';

export type Props = {
  className?: string;
  label: string;
  value: string | React.ReactNode;
  valuePrefix?: string;
  valuePostfix?: string;
  hint?: string | React.ReactNode;
  isLoading?: boolean;
  diff?: string | number;
  diffFormatted?: string;
  diffPeriod?: '24h';
  period?: '1h' | '24h' | '30min';
  href?: Route;
  icon?: IconName;
  isFallback?: boolean;
};

const Container = ({ href, isLoading, children, className }: { href?: Route; isLoading?: boolean; children: React.JSX.Element; className?: string }) => {
  if (href) {
    return (
      <Link href={ isLoading ? undefined : route(href) } inert={ isLoading } variant="plain" className={ className }
        display="block" w="100%" h="100%" minW={ 0 } maxW="100%">
        { children }
      </Link>
    );
  }

  return children;
};

const StatsWidget = ({
  className,
  icon,
  label,
  value,
  valuePrefix,
  valuePostfix,
  isLoading,
  hint,
  diff,
  diffPeriod = '24h',
  diffFormatted,
  period,
  href,
  isFallback,
}: Props) => {
  let diffColor = 'text.muted';
  if (diff != null && Number(diff) !== 0) {
    diffColor = Number(diff) > 0 ? 'positive' : 'negative';
  }
  return (
    <Container href={ href } isLoading={ isLoading } className={ href ? className : undefined }>
      <Flex
        className={ href ? undefined : className }
        alignItems="center"
        bgColor="bg.surface"
        borderWidth="var(--chakra-border-widths-1)"
        borderColor="border.divider"
        p={ 4 }
        borderRadius="card"
        justifyContent="space-between"
        columnGap={ 2 }
        w="100%"
        h="100%"
      >
        { icon && (
          <IconSvg
            name={ icon }
            p={ 2 }
            boxSize={ 10 }
            isLoading={ isLoading }
            borderRadius="base"
            display={{ base: 'none', lg: 'block' }}
            flexShrink={ 0 }
            opacity={ isFallback && !isLoading ? 'control.disabled' : 1 }
          />
        ) }
        <Box flex={ 1 } minW={ 0 } css={{ containerType: 'inline-size' }}>
          <Skeleton
            loading={ isLoading }
            color="text.muted"
            textStyle="label"
            w="fit-content"
            maxW="100%"
            overflowWrap="anywhere"
          >
            <h2>{ label }</h2>
          </Skeleton>
          <Skeleton
            loading={ isLoading }
            display="flex"
            flexWrap="wrap"
            minW={ 0 }
            maxW="100%"
            alignItems="baseline"
            data-value
            mt={ 2 }
            textStyle="statValue"
            color={ isFallback || value == null ? 'text.muted' : 'text.primary' }
          >
            { valuePrefix && <chakra.span whiteSpace="pre">{ valuePrefix }</chakra.span> }
            { typeof value === 'string' || typeof value === 'number' ? (
              <chakra.span
                minW={ 0 }
                maxW="100%"
                whiteSpace="nowrap"
                overflow="hidden"
                textOverflow="ellipsis"
                css={{ '& wbr': { display: 'contents' } }}
                wordBreak="normal"
                overflowWrap="anywhere"
                fontSize="statValue"
              >
                { typeof value === 'string' ? value.split(/(?<=,)/).map((group, index) => (
                  <React.Fragment key={ index }>{ index > 0 && <wbr/> }{ group }</React.Fragment>
                )) : value }
              </chakra.span>
            ) : (
              value ?? '—'
            ) }
            { valuePostfix && <chakra.span whiteSpace="pre">{ valuePostfix }</chakra.span> }
            { diff != null && Number.isFinite(Number(diff)) && (
              <>
                <Text ml={ 2 } mr={ 1 } textStyle="number" color={ diffColor }>
                  { Number(diff) > 0 ? '+' : '' }{ diffFormatted || Number(diff).toLocaleString() }
                </Text>
                <Text color="text.secondary" textStyle="sm">({ diffPeriod })</Text>
              </>
            ) }
            { period && <Text color="text.secondary" textStyle="xs" fontWeight="normal" ml={ 1 }>({ period })</Text> }
          </Skeleton>
        </Box>
        { typeof hint === 'string' ? (
          <Skeleton loading={ isLoading } alignSelf="center" borderRadius="base" flexShrink={ 0 }>
            <Hint label={ hint } boxSize={ 5 } color="icon.secondary"/>
          </Skeleton>
        ) : hint }
      </Flex>
    </Container>
  );
};

export default chakra(StatsWidget);
