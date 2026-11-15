import { cn } from '@/lib/utils';
import { Collapsible as CollapsiblePrimitive } from '@base-ui/react/collapsible';

function Collapsible(props: CollapsiblePrimitive.Root.Props) {
  return <CollapsiblePrimitive.Root data-slot="collapsible" { ...props }/>;
}

function CollapsibleTrigger(props: CollapsiblePrimitive.Trigger.Props) {
  return <CollapsiblePrimitive.Trigger data-slot="collapsible-trigger" { ...props }/>;
}

function CollapsibleContent({ className, ...props }: CollapsiblePrimitive.Panel.Props) {
  return (
    <CollapsiblePrimitive.Panel
      data-slot="collapsible-content"
      className={ cn(
        'h-(--collapsible-panel-height) overflow-hidden transition-[height] duration-(--duration-slow) ease-(--ease-standard) data-ending-style:h-0 data-starting-style:h-0',
        className,
      ) }
      { ...props }
    />
  );
}

export { Collapsible, CollapsibleTrigger, CollapsibleContent };
