import { Grid, GridItem } from '@chakra-ui/react';
import React from 'react';
import type { Path } from 'react-hook-form';

import config from 'configs/app';
import { FormFieldCheckbox } from 'toolkit/components/forms/fields/FormFieldCheckbox';

import type { Inputs as FormFields } from './AddressForm';

const tokenStandardName = config.chain.tokenStandard;

interface NotificationOption {
  readonly id: string;
  readonly label: string;
}

export const NOTIFICATION_OPTIONS: ReadonlyArray<NotificationOption> = [
  { id: 'native', label: config.chain.currency.symbol || '' },
  { id: 'ERC-20', label: `${ tokenStandardName }-20` },
  ...config.chain.additionalTokenTypes.map((item) => ({ id: item.id, label: item.name })),
  { id: 'ERC-721', label: `${ tokenStandardName }-721, ${ tokenStandardName }-1155 (NFT)` },
  { id: 'ERC-404', label: `${ tokenStandardName }-404` },
];

export const NOTIFICATIONS = NOTIFICATION_OPTIONS.map(({ id }) => id);

export default function AddressFormNotifications() {
  return (
    <Grid
      templateColumns="repeat(auto-fit, minmax(min(100%, var(--chakra-sizes-32)), 1fr))"
      maxW="sm"
      columnGap={ 6 }
      rowGap={ 2.5 }
    >
      { NOTIFICATION_OPTIONS.map((notification) => {
        const incomingFieldName = `notification_settings.${ notification.id }.incoming` as Path<FormFields>;
        const outgoingFieldName = `notification_settings.${ notification.id }.outcoming` as Path<FormFields>;
        return (
          <React.Fragment key={ notification.id }>
            <GridItem
              gridColumn="1 / -1"
              _notFirst={{
                mt: { base: 3, lg: 0 },
              }}
            >
              { notification.label }
            </GridItem>
            <GridItem>
              <FormFieldCheckbox<FormFields, typeof incomingFieldName>
                name={ incomingFieldName }
                label="Incoming"
              />
            </GridItem>
            <GridItem>
              <FormFieldCheckbox<FormFields, typeof outgoingFieldName>
                name={ outgoingFieldName }
                label="Outgoing"
              />
            </GridItem>
          </React.Fragment>
        );
      }) }
    </Grid>
  );
}
