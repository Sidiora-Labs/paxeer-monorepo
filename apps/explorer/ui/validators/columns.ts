export function hasValidatorField<T>(items: Array<T>, field: keyof T): boolean {
  return items.some((item) => item[field] !== undefined && item[field] !== null);
}
