import type { PaxeerXCapabilities } from 'types/api/paxeerX';

export const allEnabled: PaxeerXCapabilities = {
  addr: true,
  exchange: true,
  bridge: true,
  launchpad: true,
};

export const addrOnly: PaxeerXCapabilities = {
  addr: true,
  exchange: false,
  bridge: false,
  launchpad: false,
};

export const none: PaxeerXCapabilities = {
  addr: false,
  exchange: false,
  bridge: false,
  launchpad: false,
};
