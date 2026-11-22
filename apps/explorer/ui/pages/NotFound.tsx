import React from 'react';

import AppError from 'ui/shared/AppError/AppError';

const NOT_FOUND_STATUS = 404;
const error = new Error('Not found', { cause: { status: NOT_FOUND_STATUS } });

const NotFound = () => <AppError error={ error }/>;

export default NotFound;
