'use client';

import { setColorMode, type ColorMode } from '@/core/app-foundation/color-mode';
import { useColorMode } from '@/core/app-foundation/use-color-mode';
import { SegmentedControl } from './controls';

export function ColorModeControl({ className }: { className?: string }) {
  const { mode } = useColorMode();
  return (
    <SegmentedControl<ColorMode>
      label="Colour mode"
      options={[
        { value: 'dark', label: 'Dark' },
        { value: 'light', label: 'Light' },
        { value: 'device', label: 'Device' },
      ]}
      value={mode}
      onChange={setColorMode}
      className={className}
    />
  );
}
