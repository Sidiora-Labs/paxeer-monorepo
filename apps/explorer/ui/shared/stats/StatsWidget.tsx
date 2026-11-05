import { Box, Flex, Text, chakra } from '@chakra-ui/react';
import React from 'react';

import type { Route } from 'nextjs-routes';
import { route } from 'nextjs-routes';

import { Link } from 'toolkit/chakra/link';
import { Skeleton } from 'toolkit/chakra/skeleton';
import { Hint } from 'toolkit/components/Hint/Hint';
import { TruncatedText } from 'toolkit/components/truncation/TruncatedText';
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

const Container = ({ href, children, className }: { href?: Route; children: React.JSX.Element; className?: string }) => {
  if (href) {
    return (
      <Link href={ route(href) } variant="plain" className={ className }>
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
    <Container href={ !isLoading ? href : undefined } className={ href ? className : undefined }>
      <Flex
        className={ href ? undefined : className }
        alignItems="center"
        bgColor="bg.surface"
        borderWidth="1px"
        borderColor="border.divider"
        p={ 4 }
        borderRadius="8px"
        justifyContent="space-between"
        columnGap={ 2 }
        w="100%"
        h="100%"
      >
        { icon && (
          <IconSvg
            name={ icon }
            p={ 2 }
            boxSize="40px"
            isLoading={ isLoading }
            borderRadius="base"
            display={{ base: 'none', lg: 'block' }}
            flexShrink={ 0 }
            opacity={ isFallback && !isLoading ? 'control.disabled' : 1 }
          />
        ) }
        <Box
          w={{
            base: `calc(100% - ${ hint ? '24px' : '0px' })`,
            lg: `calc(100% - ${ icon ? '48px' : '0px' } - ${ hint ? '24px' : '0px' })`,
          }}
        >
          <Skeleton
            loading={ isLoading }
            color="text.muted"
            textStyle="label"
            w="fit-content"
          >
            <h2>{ label }</h2>
          </Skeleton>
          <Skeleton
            loading={ isLoading }
            display="flex"
            alignItems="baseline"
            data-value
            mt={ 2 }
            textStyle="statValue"
            color={ isFallback || value == null ? 'text.muted' : 'text.primary' }
          >
            { valuePrefix && <chakra.span whiteSpace="pre">{ valuePrefix }</chakra.span> }
            { typeof value === 'string' ? (
              <TruncatedText text={ value } loading={ isLoading }/>
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
            { period && <Text color="text.secondary" textStyle="xs" fontWeight={ 400 } ml={ 1 }>({ period })</Text> }
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
