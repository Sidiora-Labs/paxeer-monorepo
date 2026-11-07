import React from 'react';

import BlobData from './BlobData';
import imageBlobWithZeroesBytes from './image_with_zeroes.blob';

const BlobDataNavigation = () => {
  const [ showText, setShowText ] = React.useState(false);

  const navigate = React.useCallback(() => setShowText(true), []);

  return (
    <>
      <button type="button" onClick={ navigate }>Next blob</button>
      <BlobData
        hash={ showText ? `0x${ 'ab'.repeat(32) }` : '0x01' }
        data={ showText ? '0x0048656c6c6f20776f726c6400' : imageBlobWithZeroesBytes }
      />
    </>
  );
};

export default BlobDataNavigation;
