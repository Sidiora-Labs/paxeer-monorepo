import React from 'react';

import config from 'configs/app';
import type { ImageProps } from 'toolkit/chakra/image';
import { Image } from 'toolkit/chakra/image';
import { Link } from 'toolkit/chakra/link';
import { Skeleton } from 'toolkit/chakra/skeleton';
import { Tooltip } from 'toolkit/chakra/tooltip';

interface Props extends ImageProps {
  chainConfig?: typeof config;
  isLoading?: boolean;
}

const RollupStageBadge = ({ chainConfig = config, isLoading, ...props }: Props) => {

  const feature = chainConfig.features.rollup;

  if (!feature.isEnabled || chainConfig.chain.isTestnet) {
    return null;
  }

  if (isLoading) {
    return (
      <Skeleton
        w="calc(var(--chakra-spacing-10) + var(--chakra-spacing-1) / 2)"
        h={ 'var(--chakra-spacing-3\\.5)' }
        { ...props }
        loading
      />
    );
  }

  switch (feature.stageIndex) {
    case '1':
    case '2': {
      const tooltipContent = (
        <>
          The decentralization and maturity of the chain. Learn more at{ ' ' }
          <Link href="https://l2beat.com?utm_source=blockscout&utm_medium=explorer" external>
            l2beat.com
          </Link>
        </>
      );

      return (
        <Tooltip content={ tooltipContent } interactive>
          <Image
            alt={ `Rollup stage ${ feature.stageIndex }` }
            tabIndex={ 0 }
            src={ feature.stageIndex === '1' ? '/explorer/static/labels/stage-1.svg' : '/explorer/static/labels/stage-2.svg' }
            h={ 'var(--chakra-spacing-3\\.5)' }
            w="calc(var(--chakra-spacing-10) + var(--chakra-spacing-1) / 2)"
            { ...props }
          />
        </Tooltip>
      );
    }
    default:
      return null;
  }
};

export default React.memo(RollupStageBadge);
