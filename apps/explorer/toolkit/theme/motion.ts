import durations from './foundations/durations';

export function motionDuration(name: keyof typeof durations): number {
  return Number.parseFloat(durations[name].value);
}
