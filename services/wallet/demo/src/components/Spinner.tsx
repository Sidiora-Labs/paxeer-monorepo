export function Spinner({ label }: { label: string }) {
  return <progress className="icon spinner" aria-label={label} />;
}
