import { getEnvValue } from '../utils';

const paxeerX = Object.freeze({
  title: 'Paxeer X',
  isEnabled: getEnvValue('NEXT_PUBLIC_PAXEER_X_ENABLED') === 'true',
  navigation: {
    text: 'Paxeer X',
    items: [ { text: 'Chains', href: '/chains' } ],
  },
});

export default paxeerX;
