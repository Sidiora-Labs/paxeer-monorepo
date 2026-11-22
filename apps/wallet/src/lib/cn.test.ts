import { expect, test } from 'vitest';
import config from '../../tailwind.config';
import { cn } from './cn';

const fontSizes = Object.keys(config.theme!.extend!.fontSize!);
const shadows = Object.keys(config.theme!.extend!.boxShadow!);
const textColors = ['text-pax-accent', 'text-pax-error', 'text-red-500'];
const shadowColors = ['shadow-pax-accent', 'shadow-pax-error', 'shadow-red-500'];

function theme_font_sizes_preserve_text_colors() {
  expect(fontSizes.length).toBeGreaterThan(0);
  for (const name of fontSizes) {
    const size = `text-${name}`;
    for (const color of textColors) {
      expect(cn(color, size)).toBe(`${color} ${size}`);
      expect(cn(size, color)).toBe(`${size} ${color}`);
    }
  }
}

function last_font_size_wins() {
  const sizes = [...fontSizes, '4xl'].map((name) => `text-${name}`);
  for (const first of sizes) {
    for (const last of sizes) {
      expect(cn(first, last)).toBe(last);
    }
  }
}

function last_text_color_wins() {
  for (const first of textColors) {
    for (const last of textColors) {
      expect(cn(first, last)).toBe(last);
    }
  }
}

function theme_shadows_preserve_shadow_colors() {
  expect(shadows.length).toBeGreaterThan(0);
  for (const name of shadows) {
    const shadow = `shadow-${name}`;
    for (const color of shadowColors) {
      expect(cn(color, shadow)).toBe(`${color} ${shadow}`);
      expect(cn(shadow, color)).toBe(`${shadow} ${color}`);
    }
  }
}

function last_shadow_and_shadow_color_win() {
  const shadowClasses = [...shadows, 'lg'].map((name) => `shadow-${name}`);
  for (const classes of [shadowClasses, shadowColors]) {
    for (const first of classes) {
      for (const last of classes) {
        expect(cn(first, last)).toBe(last);
      }
    }
  }
}

test(theme_font_sizes_preserve_text_colors.name, theme_font_sizes_preserve_text_colors);
test(last_font_size_wins.name, last_font_size_wins);
test(last_text_color_wins.name, last_text_color_wins);
test(theme_shadows_preserve_shadow_colors.name, theme_shadows_preserve_shadow_colors);
test(last_shadow_and_shadow_color_win.name, last_shadow_and_shadow_color_win);
