import { Flex, Box } from '@chakra-ui/react';
import React from 'react';

import { stripLeadingSlash } from 'toolkit/utils/url';
import useThemeColors from 'ui/shared/monaco/utils/useThemeColors';

interface Props {
  path: string;
}

const CodeEditorBreadcrumbs = ({ path }: Props) => {
  const chunks = stripLeadingSlash(path).split('/');
  const themeColors = useThemeColors();

  return (
    <Flex
      color={ themeColors['breadcrumbs.foreground'] }
      bgColor={ themeColors['editor.background'] }
      pl={ 4 }
      pr={ 2 }
      flexWrap="wrap"
      textStyle="sm"
      alignItems="center"
    >
      { chunks.map((chunk, index) => {
        return (
          <React.Fragment key={ index }>
            { index !== 0 && (
              <Box
                className="codicon codicon-breadcrumb-separator"
                boxSize={ 4 }
                _before={{
                  content: '"\\eab6"',
                }}/>
            ) }
            <Box>{ chunk }</Box>
          </React.Fragment>
        );
      }) }
    </Flex>
  );
};

export default React.memo(CodeEditorBreadcrumbs);
