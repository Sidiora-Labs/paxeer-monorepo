import React from "react";
import { styled } from "storybook/theming";
import { useTheme } from "./context";

const Wrap = styled.div`
  display: flex;
  flex-direction: row;
  justify-content: center;
  gap: var(--oui-space-2, 0.5rem);
  font-size: var(--oui-font-size-2xs, 0.75rem);
  // padding: 6px 10px;
  border-bottom: 1px solid rgb(var(--oui-color-line, 255 255 255) / 0.12);
  background: rgb(var(--oui-color-surface-raised, 22 18 32));
  position: sticky;
  top: 0;
  z-index: 100;
`;

const TabButton = styled.button<{ active?: boolean }>`
  display: flex;
  align-items: center;
  justify-content: center;
  // border-radius: 4px;
  border: none;
  //   width: 24px;
  padding: var(--oui-space-3, 0.75rem) var(--oui-space-2, 0.5rem);
  background: transparent;
  cursor: pointer;
  min-height: var(--oui-space-8, 2rem);
  color: ${({ active }) =>
    active ? "rgb(var(--oui-color-primary, 176 132 233))" : "inherit"};
  border-bottom: ${(props) =>
    props.active
      ? "2px solid rgb(var(--oui-color-primary, 176 132 233))"
      : "2px solid transparent"};
  &:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
`;

export const Toolbar = () => {
  const { mode, setMode, loading } = useTheme();
  return (
    <Wrap>
      {/* <div style={{ display: "flex", alignItems: "center", gap: "4px" }}>
        <input type="checkbox" id="theme-auto-save" />
        <label htmlFor="theme-auto-save">Save theme to local storage</label>
      </div> */}
      <TabButton
        active={mode === "visual"}
        disabled={loading}
        onClick={() => setMode("visual")}
      >
        <svg
          xmlns="http://www.w3.org/2000/svg"
          width="16"
          height="16"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
          className="lucide lucide-layout-list"
        >
          <rect width="7" height="7" x="3" y="3" rx="1" />
          <rect width="7" height="7" x="3" y="14" rx="1" />
          <path d="M14 4h7" />
          <path d="M14 9h7" />
          <path d="M14 15h7" />
          <path d="M14 20h7" />
        </svg>
      </TabButton>
      <TabButton
        active={mode === "code"}
        disabled={loading}
        onClick={() => setMode("code")}
      >
        <svg
          xmlns="http://www.w3.org/2000/svg"
          width="18"
          height="18"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
          className="lucide lucide-code"
        >
          <polyline points="16 18 22 12 16 6" />
          <polyline points="8 6 2 12 8 18" />
        </svg>
      </TabButton>
    </Wrap>
  );
};
