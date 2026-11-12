const PERCENT_UNITS = '%';

const UNITS_WITHOUT_SPACE = [ 's', PERCENT_UNITS ];

export function formatCounterUnits(units?: string): string {
  if (!units) {
    return '';
  }

  return UNITS_WITHOUT_SPACE.includes(units) ? units : ' ' + units;
}

export function formatCounterValue(value: string): string {
  const valueNum = Number(value);
  const maximumFractionDigits = valueNum < 10 ** -3 ? undefined : 3;

  return valueNum.toLocaleString(undefined, { maximumFractionDigits, notation: 'compact' });
}
