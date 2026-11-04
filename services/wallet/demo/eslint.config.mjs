import nextVitals from 'eslint-config-next/core-web-vitals';

export default [
  {
    ignores: ['.next/**', 'out/**', 'dist/**', 'coverage/**', 'next-env.d.ts'],
  },
  ...nextVitals,
];
