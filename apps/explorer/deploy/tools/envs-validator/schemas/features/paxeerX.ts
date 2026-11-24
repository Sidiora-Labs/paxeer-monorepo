import * as yup from 'yup';
import { urlTest } from '../../utils';

export const paxeerXSchema = yup
  .object()
  .shape({
    NEXT_PUBLIC_PAXEER_X_ENABLED: yup.boolean(),
    NEXT_PUBLIC_PAXEER_X_INDEXER_API_HOST: yup
      .string()
      .when('NEXT_PUBLIC_PAXEER_X_ENABLED', {
        is: (value: boolean) => value,
        then: (schema) => schema.test(urlTest),
        otherwise: (schema) => schema.test(
          'not-exist',
          'NEXT_PUBLIC_PAXEER_X_INDEXER_API_HOST can only be used with NEXT_PUBLIC_PAXEER_X_ENABLED',
          value => value === undefined,
        ),
      }),
    NEXT_PUBLIC_PAXEER_X_INDEXER_API_BASE_PATH: yup
      .string()
      .when('NEXT_PUBLIC_PAXEER_X_ENABLED', {
        is: (value: boolean) => value,
        then: (schema) => schema,
        otherwise: (schema) => schema.test(
          'not-exist',
          'NEXT_PUBLIC_PAXEER_X_INDEXER_API_BASE_PATH can only be used with NEXT_PUBLIC_PAXEER_X_ENABLED',
          value => value === undefined,
        ),
      }),
  });
