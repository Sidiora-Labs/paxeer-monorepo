import React from "react";
import { styled } from "storybook/theming";
import { ColorGradientInput, ColorInput } from "./colorInput";

type ColorItem = {
  name: string;
  value: string;
};

type ColorGroup = {
  name: string;
  description: string;
  colors: ColorItem[];
};

type GradientColorGroup = {
  name: string;
  start: string;
  end: string;
};

const StyledColorList = styled.div`
  display: flex;
  flex-direction: row;
  justify-content: start;
  flex-wrap: wrap;
  gap: var(--oui-space-2, 0.5rem);
`;

const Header = styled.div`
  margin-bottom: var(--oui-space-1, 0.25rem);
  margin-top: var(--oui-space-2, 0.5rem);
`;

const StyledGroupName = styled.h3`
  font-size: var(--oui-font-size-sm, 0.875rem);
  color: rgb(var(--oui-color-text-muted, 132 131 136));
  font-weight: 700;
`;

const StyledGradientGroupName = styled.h3`
  font-size: var(--oui-font-size-sm, 0.875rem);
  color: rgb(var(--oui-color-text-muted, 132 131 136));
  font-weight: 700;
  text-transform: capitalize;
`;

const Description = styled.div`
  font-size: var(--oui-font-size-2xs, 0.75rem);
  color: rgb(var(--oui-color-text-muted, 132 131 136));
  margin-bottom: var(--oui-space-2, 0.5rem);
`;

export const ColorList = ({
  colors,
  onChange,
}: {
  colors: ColorGroup[];
  onChange: (name: string, value: string) => void;
}) => {
  return (
    <div>
      {colors.map((color) => (
        <div key={color.name}>
          <Header>
            <StyledGroupName>{color.name}</StyledGroupName>
            <Description>{color.description}</Description>
          </Header>

          <StyledColorList>
            {color.colors.map((color) => (
              <ColorInput
                key={color.name}
                name={color.name}
                color={color.value}
                onChange={(value) => {
                  onChange(color.name, value);
                }}
              />
            ))}
          </StyledColorList>
        </div>
      ))}
    </div>
  );
};

const GradientGrid = styled.div`
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: var(--oui-space-2, 0.5rem) var(--oui-space-6, 1.5rem);
`;

export const GradientColorList = ({
  colors,
  onChange,
}: {
  colors: GradientColorGroup[];
  onChange: (colors: Record<string, string>) => void;
}) => {
  return (
    <div>
      <Header>
        <StyledGroupName>Gradient Colors</StyledGroupName>
        <Description>
          Gradient colors are used for important components such as the ‘Connect
          Wallet’ button and the ‘Enable Trading’ button. If you don’t need
          gradient colors in your application, you can set both the start and
          stop colors to the same value.
        </Description>
      </Header>
      <GradientGrid>
        {colors.map((color) => (
          <div key={color.name}>
            <Header>
              <StyledGradientGroupName>{color.name}</StyledGradientGroupName>
            </Header>
            <ColorGradientInput
              name={color.name}
              start={color.start}
              end={color.end}
              onChange={(start, end) => {
                onChange({
                  [`--oui-gradient-${color.name}-start`]: start,
                  [`--oui-gradient-${color.name}-end`]: end,
                });
              }}
            />
          </div>
        ))}
      </GradientGrid>
    </div>
  );
};
