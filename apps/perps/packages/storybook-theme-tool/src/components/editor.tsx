import React from "react";
import { styled } from "storybook/theming";
import { CodeEditor } from "./codeEditor";
import { ColorList, GradientColorList } from "./colorList";
import { useTheme } from "./context";
import { EditorProvider } from "./editorContext";
import { ThemeMenu } from "./menu";
import { Radius } from "./radius";
import { Toolbar } from "./toolbar";
import { Typography } from "./typography";

const Layout = styled.div`
  display: flex;
  flex-direction: row;
  gap: var(--oui-space-5, 1.25rem);
  max-width: var(--oui-layout-max-width, 75rem);
  margin: 0 auto;
  position: relative;
  @media (max-width: 30rem) {
    // flex-direction: column;
    display: block;
  }
`;

const MenuWrap = styled.div`
  width: var(--oui-menu-width, 12.5rem);
  //   height: 100vh;
  @media (max-width: 30rem) {
    display: none;
  }
`;

const EditorWrap = styled.div`
  flex: 1;
  //   height: 100vh;
`;

const SectionWrap = styled.div`
  padding: var(--oui-space-4, 1rem);
`;

const SectionTitle = styled.h2`
  font-size: var(--oui-font-size-base, 1rem);
  font-weight: 600;
  //   margin-top: 20px;
`;

export const ThemeEditor = () => {
  return (
    <EditorProvider>
      <Toolbar />
      <ViewMode />
    </EditorProvider>
  );
};

const ViewMode = () => {
  const { mode } = useTheme();
  return mode === "visual" ? <VisualEditor /> : <CodeEditor />;
};

const VisualEditor = () => {
  return (
    <Layout>
      <MenuWrap>
        <ThemeMenu />
      </MenuWrap>
      <EditorWrap>
        <SectionWrap id="colors">
          <SectionTitle>Colors</SectionTitle>
          <Colors />
        </SectionWrap>
        <SectionWrap id="radius">
          <SectionTitle>Radius</SectionTitle>
          <Radius />
        </SectionWrap>
        <SectionWrap id="typography">
          <SectionTitle>Typography</SectionTitle>
          <Typography />
        </SectionWrap>
      </EditorWrap>
      <ResetButton />
      <Masker />
    </Layout>
  );
};

const StyledMasker = styled.div`
  position: absolute;
  inset: 0;
  left: 0;
  top: 0;
  right: 0;
  bottom: 0;
  background-color: rgb(var(--oui-color-base-static, 255 255 255) / 0.5);
  backdrop-filter: blur(var(--oui-space-2, 0.5rem));
  z-index: 1000;
  display: flex;
  align-items: center;
  justify-content: center;
  color: rgb(var(--oui-color-text-muted, 132 131 136));
`;

const Masker = () => {
  const { loading } = useTheme();
  return loading ? <StyledMasker>Loading...</StyledMasker> : null;
};

const ResetButtonWrap = styled.div`
  position: fixed;
  bottom: var(--oui-space-4, 1rem);
  right: var(--oui-space-4, 1rem);
  z-index: 10;
`;

const StyledResetButton = styled.button`
  width: var(--oui-space-8, 2rem);
  height: var(--oui-space-8, 2rem);
  background-color: rgb(var(--oui-color-base-static-contrast, 0 0 0) / 0.2);
  border-radius: var(--oui-rounded-full, 9999px);
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  border: 1px solid rgb(var(--oui-color-base-static-contrast, 0 0 0) / 0.6);
`;

const ResetButton = () => {
  const { resetTheme } = useTheme();
  return (
    <ResetButtonWrap>
      <StyledResetButton onClick={resetTheme}>
        <svg
          xmlns="http://www.w3.org/2000/svg"
          width="24"
          height="24"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
        >
          <path d="M21 6H3" />
          <path d="M7 12H3" />
          <path d="M7 18H3" />
          <path d="M12 18a5 5 0 0 0 9-3 4.5 4.5 0 0 0-4.5-4.5c-1.33 0-2.54.54-3.41 1.41L11 14" />
          <path d="M11 10v4h4" />
        </svg>
      </StyledResetButton>
    </ResetButtonWrap>
  );
};

const Colors = () => {
  const { theme, setTheme } = useTheme();

  return (
    <>
      <ColorList
        onChange={(name, value) => {
          setTheme((prev: any) => ({ ...prev, [name]: value }), {
            [name]: value,
          });
        }}
        colors={[
          {
            name: "Primary",
            description:
              "Primary visual color, mainly used for buttons and other elements.",
            colors: [
              {
                name: "--oui-color-primary-light",
                value: theme["--oui-color-primary-light"],
              },
              {
                name: "--oui-color-primary",
                value: theme["--oui-color-primary"],
              },
              {
                name: "--oui-color-primary-darken",
                value: theme["--oui-color-primary-darken"],
              },
              {
                name: "--oui-color-primary-contrast",
                value: theme["--oui-color-primary-contrast"],
              },
            ],
          },
          {
            name: "Success",
            description:
              "Success visual color, mainly used for buttons and other elements.",
            colors: [
              {
                name: "--oui-color-success-light",
                value: theme["--oui-color-success-light"],
              },
              {
                name: "--oui-color-success",
                value: theme["--oui-color-success"],
              },
              {
                name: "--oui-color-success-darken",
                value: theme["--oui-color-success-darken"],
              },
              {
                name: "--oui-color-success-contrast",
                value: theme["--oui-color-success-contrast"],
              },
            ],
          },
          {
            name: "Danger",
            description:
              "Danger visual color, mainly used for buttons and other elements.",
            colors: [
              {
                name: "--oui-color-danger-light",
                value: theme["--oui-color-danger-light"],
              },
              {
                name: "--oui-color-danger",
                value: theme["--oui-color-danger"],
              },
              {
                name: "--oui-color-danger-darken",
                value: theme["--oui-color-danger-darken"],
              },
              {
                name: "--oui-color-danger-contrast",
                value: theme["--oui-color-danger-contrast"],
              },
            ],
          },
          {
            name: "Trade",
            description:
              "Used for trading components, such as profit and loss indicators, buy/sell buttons, etc.",
            colors: [
              {
                name: "--oui-color-trading-loss",
                value: theme["--oui-color-trading-loss"],
              },
              {
                name: "--oui-color-trading-loss-contrast",
                value: theme["--oui-color-trading-loss-contrast"],
              },
              {
                name: "--oui-color-trading-profit",
                value: theme["--oui-color-trading-profit"],
              },
              {
                name: "--oui-color-trading-profit-contrast",
                value: theme["--oui-color-trading-profit-contrast"],
              },
            ],
          },
          {
            name: "Misc",
            description: "",
            colors: [
              {
                name: "--oui-color-base-foreground",
                value: theme["--oui-color-base-foreground"],
              },
              {
                name: "--oui-color-line",
                value: theme["--oui-color-line"],
              },
            ],
          },
          {
            name: "Neutral",
            description:
              "Neutral visual color, mainly used for background and other elements.",
            colors: [
              {
                name: "--oui-color-base-1",
                value: theme["--oui-color-base-1"],
              },
              {
                name: "--oui-color-base-2",
                value: theme["--oui-color-base-2"],
              },
              {
                name: "--oui-color-base-3",
                value: theme["--oui-color-base-3"],
              },
              {
                name: "--oui-color-base-4",
                value: theme["--oui-color-base-4"],
              },
              {
                name: "--oui-color-base-5",
                value: theme["--oui-color-base-5"],
              },
              {
                name: "--oui-color-base-6",
                value: theme["--oui-color-base-6"],
              },
              {
                name: "--oui-color-base-7",
                value: theme["--oui-color-base-7"],
              },
              {
                name: "--oui-color-base-8",
                value: theme["--oui-color-base-8"],
              },
              {
                name: "--oui-color-base-9",
                value: theme["--oui-color-base-9"],
              },
              {
                name: "--oui-color-base-10",
                value: theme["--oui-color-base-10"],
              },
            ],
          },
        ]}
      />
      <GradientColorList
        colors={[
          {
            name: "brand",
            start: theme["--oui-gradient-brand-start"],
            end: theme["--oui-gradient-brand-end"],
          },
          {
            name: "primary",
            start: theme["--oui-gradient-primary-start"],
            end: theme["--oui-gradient-primary-end"],
          },
          {
            name: "secondary",
            start: theme["--oui-gradient-secondary-start"],
            end: theme["--oui-gradient-secondary-end"],
          },
          {
            name: "success",
            start: theme["--oui-gradient-success-start"],
            end: theme["--oui-gradient-success-end"],
          },
          {
            name: "danger",
            start: theme["--oui-gradient-danger-start"],
            end: theme["--oui-gradient-danger-end"],
          },

          {
            name: "warning",
            start: theme["--oui-gradient-warning-start"],
            end: theme["--oui-gradient-warning-end"],
          },
          {
            name: "neutral",
            start: theme["--oui-gradient-neutral-start"],
            end: theme["--oui-gradient-neutral-end"],
          },
        ]}
        onChange={(colors) => {
          setTheme((prev: any) => ({ ...prev, ...colors }), {
            ...colors,
          });
        }}
      />
    </>
  );
};
