import { cn } from '@/lib/utils';

interface IconProps {
  name: string;
  size?: number;
  className?: string;
  alt?: string;
}

export default function Icon({ name, size, className = '', alt }: IconProps) {
  return (
    <img
      src={`/icons/${name}`}
      width={size}
      height={size}
      alt={alt ?? name}
      style={{ width: size, height: size }}
      className={cn('size-icon-lg', className)}
    />
  );
}
