import type { GetServerSideProps, NextPage } from 'next';
import React from 'react';
import { z } from 'zod';

import type { Props as BaseProps } from 'nextjs/getServerSideProps/handlers';
import { base } from 'nextjs/getServerSideProps/main';
import PageNextJs from 'nextjs/PageNextJs';

import config from 'configs/app';
import type { IconInfo } from 'ui/pages/Sprite';
import Sprite from 'ui/pages/Sprite';

type Props = BaseProps<'/sprite'> & { icons: Array<IconInfo> };

const Page: NextPage<Props> = ({ icons }) => {
  return (
    <PageNextJs pathname="/sprite">
      <Sprite initialData={ icons }/>
    </PageNextJs>
  );
};

export default Page;

export const getServerSideProps: GetServerSideProps<Props> = async(context) => {
  const result = await base<'/sprite'>(context);
  if (!('props' in result)) return result;
  const response = await fetch(`${ config.app.baseUrl }/icons/registry.json`);
  if (!response.ok) throw new Error(`Icon registry returned ${ response.status }`);
  const icons = z.array(z.object({ name: z.string(), file_size: z.number().nonnegative() })).parse(await response.json());
  return { props: { ...await result.props, icons } };
};
