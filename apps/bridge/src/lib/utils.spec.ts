import { badgeVariants } from '@/components/ui/badge';
import { buttonVariants } from '@/components/ui/button';

import { expect, test } from 'vitest';

import { cn } from './utils';

function surfaceTypographyPreservesControlColorRoles() {
  for (const size of [ 'xs', 'sm', 'default', 'lg' ] as const) {
    expect(cn(buttonVariants({ size })).split(' ')).toContain('text-primary-foreground');
    expect(cn(buttonVariants({ size, variant: 'destructive' })).split(' ')).toContain('text-destructive-foreground');
  }
  for (const variant of [ 'success', 'warning', 'info', 'destructive' ] as const) {
    const classes = cn(badgeVariants({ variant })).split(' ');
    expect(classes).toContain('text-label');
    expect(classes).toContain(`text-${ variant }`);
  }
}

function surfaceTypographyOverridesSizeWithoutChangingInk() {
  for (const size of [ 'caption', 'label', 'body-sm', 'body', 'title-sm', 'title', 'display', 'unit', 'icon-2xs', 'icon-xs', 'icon-sm', 'icon-md' ]) {
    const classes = cn('text-primary-foreground text-sm', `text-${ size }`).split(' ');
    expect(classes).toContain('text-primary-foreground');
    expect(classes).toContain(`text-${ size }`);
    expect(classes).not.toContain('text-sm');
  }
}

test(surfaceTypographyPreservesControlColorRoles);
test(surfaceTypographyOverridesSizeWithoutChangingInk);
