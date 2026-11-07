import { isValidElement, type ReactNode } from "react";

export function spotWidgetElements(node: ReactNode): { type: unknown; props: Record<string, unknown> }[] {
  if (Array.isArray(node)) return node.flatMap(spotWidgetElements);
  if (!isValidElement<{ children?: ReactNode }>(node)) return [];
  return [{ type: node.type, props: node.props as Record<string, unknown> }, ...spotWidgetElements(node.props.children)];
}
