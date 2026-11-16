import React from "react";
import { styled } from "storybook/theming";
import { useTheme } from "./context";

export const Wrap = styled.div`
  display: flex;
  flex-direction: row;
  flex-wrap: wrap;
  gap: var(--oui-space-2, 0.5rem);
  margin-top: var(--oui-space-2, 0.5rem);
`;

const StyledGroupName = styled.h3`
  font-size: var(--oui-font-size-sm, 0.875rem);
  color: rgb(var(--oui-color-text-muted, 132 131 136));
  font-weight: 700;
`;

const RADIUS_SIZES = [
  "sm",
  "default",
  "md",
  "lg",
  "xl",
  "2xl",
  "full",
] as const;

export const Radius = () => {
  const { theme, setTheme } = useTheme();

  const handleChange = (name: string) => (value: string) => {
    setTheme({ ...theme, [name]: value });
  };

  return (
    <div>
      <StyledGroupName>Border Radius</StyledGroupName>
      <Wrap>
        {RADIUS_SIZES.map((size) => {
          const suffix = size === "default" ? "" : `-${size}`;
          return (
            <RadiusInput
              key={size}
              name={size}
              value={theme[`--oui-rounded${suffix}`]}
              onChange={handleChange(`--oui-rounded${suffix}`)}
            />
          );
        })}
      </Wrap>
    </div>
  );
};

export const Label = styled.div`
  font-size: var(--oui-font-size-2xs, 0.75rem);
  color: rgb(var(--oui-color-text-muted, 132 131 136));
  margin-bottom: var(--oui-space-1, 0.25rem);
`;

export const StyledInput = styled.input`
  width: 100%;
  border: 1px solid rgb(var(--oui-color-line, 255 255 255) / 0.2);
  background: transparent;
  border-radius: var(--oui-rounded-md, 0.375rem);
  padding: var(--oui-space-1, 0.25rem) var(--oui-space-2, 0.5rem);
  font-size: var(--oui-font-size-2xs, 0.75rem);
`;

const RadiusInput = ({
  name,
  value,
  onChange,
}: {
  name: string;
  value: string;
  onChange: (value: string) => void;
}) => {
  return (
    <div>
      <Label>{name}</Label>
      <div>
        <StyledInput
          type="text"
          value={value}
          onChange={(event) => onChange(event.target.value)}
        />
      </div>
    </div>
  );
};
