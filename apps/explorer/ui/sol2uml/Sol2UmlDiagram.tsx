import { chakra } from '@chakra-ui/react';
import React from 'react';

import type * as visualizer from '@blockscout/visualizer-types';
import type { SmartContract } from 'types/api/contract';

import type { ResourceError } from 'lib/api/resources';
import useApiQuery from 'lib/api/useApiQuery';
import throwOnAbsentParamError from 'lib/errors/throwOnAbsentParamError';
import throwOnResourceLoadError from 'lib/errors/throwOnResourceLoadError';
import { Tooltip } from 'toolkit/chakra/tooltip';
import { ContentLoader } from 'toolkit/components/loaders/ContentLoader';

interface Props {
  addressHash: string;
}

function composeSources(contract: SmartContract | undefined): visualizer.VisualizeStorageRequest['sources'] {
  if (!contract) {
    return {};
  }
  const additionalSources = contract.additional_sources?.reduce<Record<string, string>>((result, item) => {
    result[item.file_path] = item.source_code;
    return result;
  }, {});

  return {
    [contract.file_path || 'index.sol']: contract.source_code || '',
    ...additionalSources,
  };
}

const Sol2UmlDiagram = ({ addressHash }: Props) => {
  const contractQuery = useApiQuery<'general:contract', ResourceError>('general:contract', {
    pathParams: { hash: addressHash },
    queryOptions: {
      enabled: Boolean(addressHash),
      refetchOnMount: false,
    },
  });

  const umlQuery = useApiQuery('visualize:solidity_contract', {
    fetchParams: {
      method: 'POST',
      body: {
        sources: composeSources(contractQuery.data),
      },
    },
    queryOptions: {
      queryKey: [ 'solidity_contract', addressHash ],
      enabled: Boolean(contractQuery.data),
      refetchOnMount: false,
    },
  });

  const imgUrl = `data:image/svg+xml;base64,${ umlQuery.data?.svg }`;

  const handleClick = React.useCallback(() => {
    const image = new Image();
    image.src = imgUrl;

    const newWindow = window.open(imgUrl, '_blank', 'noopener,noreferrer');
    newWindow?.document.write(image.outerHTML);
  }, [ imgUrl ]);

  throwOnAbsentParamError(addressHash);
  throwOnResourceLoadError(contractQuery);
  throwOnResourceLoadError(umlQuery);

  if (contractQuery.isPending || umlQuery.isPending) {
    return <ContentLoader width={ 800 } maxW="100%" aspectRatio="4 / 3"/>;
  }

  if (!umlQuery.data?.svg || !contractQuery.data) {
    return <span>No data for visualization</span>;
  }

  return (
    <Tooltip content="Click on image to zoom" positioning={{ placement: 'top' }}>
      <chakra.button
        type="button"
        onClick={ handleClick }
        maxW="100%"
        p={ 0 }
        bg="transparent"
        cursor="pointer"
        _focusVisible={{ outline: 'var(--chakra-border-widths-2) solid', outlineColor: 'focusRing', outlineOffset: 'var(--chakra-border-widths-2)' }}
      >
        <chakra.img
          src={ imgUrl }
          alt={ `Contract ${ contractQuery.data.name } UML diagram` }
          width={ 800 }
          height={ 600 }
          loading="lazy"
          decoding="async"
          display="block"
          maxW="100%"
          h="auto"
          aspectRatio="4 / 3"
          objectFit="contain"
          filter={{ _light: 'invert(0)', _dark: 'invert(1)' }}
        />
      </chakra.button>
    </Tooltip>
  );
};

export default React.memo(Sol2UmlDiagram);
