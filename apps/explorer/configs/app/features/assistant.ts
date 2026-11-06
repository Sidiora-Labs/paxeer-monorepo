import type { Feature } from './types';

import { getEnvValue } from '../utils';

const NEXT_PUBLIC_ASSISTANT_ENABLED = getEnvValue('NEXT_PUBLIC_ASSISTANT_ENABLED') === 'true';

const assistant: Feature<{}> = Object.freeze({
  title: 'Ask',
  isEnabled: NEXT_PUBLIC_ASSISTANT_ENABLED,
});

export default assistant;
