import { useQuery } from '@tanstack/react-query';
import React from 'react';

import { route } from 'nextjs-routes';

import config from 'configs/app';
import { AccordionItem, AccordionItemContent, AccordionItemTrigger, AccordionRoot, useAccordion } from 'toolkit/chakra/accordion';
import { ContentLoader } from 'toolkit/components/loaders/ContentLoader';
import CopyToClipboard from 'ui/shared/CopyToClipboard';

import SwaggerUI from './SwaggerUI';
import { getCoreApiSpecCandidates, resolveCoreApiSpecUrl, REST_API_SECTIONS } from './utils';

const RestApi = () => {

  const coreApiSpecQuery = useQuery({
    queryKey: [ 'api-docs', 'core-api-spec-url', getCoreApiSpecCandidates() ],
    queryFn: () => resolveCoreApiSpecUrl(getCoreApiSpecCandidates()),
    enabled: config.features.apiDocs.isEnabled && Boolean(config.apis.general),
    refetchOnMount: false,
    staleTime: Infinity,
  });

  const isLoading = config.features.apiDocs.isEnabled && Boolean(config.apis.general) ? coreApiSpecQuery.isPending : false;

  const { value, onValueChange, scrollToItemFromUrl } = useAccordion(REST_API_SECTIONS);

  React.useEffect(() => {
    if (!isLoading) {
      scrollToItemFromUrl();
    }
    // runs only on mount once all queries are resolved
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [ isLoading ]);

  const coreApiSwaggerUrl = coreApiSpecQuery.data;

  const sections = React.useMemo(() => {
    if (!coreApiSwaggerUrl) {
      return REST_API_SECTIONS;
    }

    return REST_API_SECTIONS.map((section) => {
      if (section.id === 'blockscout-core-api') {
        return {
          ...section,
          swagger: {
            ...section.swagger,
            url: coreApiSwaggerUrl,
          },
        };
      }
      return section;
    });
  }, [ coreApiSwaggerUrl ]);

  if (isLoading) {
    return <ContentLoader/>;
  }

  if (sections.length === 0) {
    return null;
  }

  if (sections.length === 1) {
    return <SwaggerUI { ...sections[0].swagger }/>;
  }

  return (
    <AccordionRoot onValueChange={ onValueChange } value={ value }>
      { sections.map((section, index) => (
        <AccordionItem key={ index } value={ section.id }>
          <AccordionItemTrigger>
            <CopyToClipboard
              text={ config.app.baseUrl + route({ pathname: '/api-docs', query: { tab: 'rest_api' }, hash: section.id }) }
              type="link"
              ml={ 0 }
              mr={ 1 }
              as="div"
            />
            { section.title }
          </AccordionItemTrigger>
          <AccordionItemContent>
            <SwaggerUI { ...section.swagger }/>
          </AccordionItemContent>
        </AccordionItem>
      )) }
    </AccordionRoot>
  );
};

export default React.memo(RestApi);
