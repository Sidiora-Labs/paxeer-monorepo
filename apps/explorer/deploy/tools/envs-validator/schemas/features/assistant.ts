import * as yup from 'yup';

import { urlTest } from '../../utils';

export const assistantSchema = yup.object().shape({
  NEXT_PUBLIC_ASSISTANT_ENABLED: yup.boolean(),
  EXPLORER_ASSISTANT_BASE_URL: yup.string().test(urlTest),
  EXPLORER_ASSISTANT_API_KEY: yup.string(),
  EXPLORER_ASSISTANT_MODEL: yup.string(),
});
