import React, { useMemo } from "react";
import { css } from "@codemirror/lang-css";
import { githubLight } from "@uiw/codemirror-theme-github";
import CodeMirror, { ViewUpdate } from "@uiw/react-codemirror";
import { styled } from "storybook/theming";
import { Button } from "@paxeer-network/ui";
import { object2Css, parseCssToJson } from "../utils";
import { useTheme } from "./context";

const Container = styled.div`
  position: relative;
  max-width: var(--oui-content-max-width, 50rem);
  margin: 0 auto;
  column-gap: var(--oui-space-2, 0.5rem);
  padding: var(--oui-space-4, 1rem);
`;

export const CodeEditor = () => {
  const { theme, setTheme } = useTheme();
  const code = useMemo(() => object2Css(theme), [theme]);

  const onChange = (val: string, viewUpdate: ViewUpdate) => {
    try {
      const newTheme = parseCssToJson(val);
      setTheme(newTheme);
    } catch {
    }
  };

  const copy = () => {
    navigator.clipboard.writeText(code);
  };

  const download = () => {
    const blob = new Blob([code], { type: "text/css" });

    const url = URL.createObjectURL(blob);

    const a = document.createElement("a");
    a.href = url;
    a.download = "theme.css";
    document.body.appendChild(a);
    a.click();

    document.body.removeChild(a);
    URL.revokeObjectURL(url);
  };

  return (
    <Container>
      <CodeMirror
        value={code}
        theme={githubLight}
        extensions={[css()]}
        onChange={onChange}
      />

      <div
        style={{
          position: "fixed",
          right: "var(--oui-space-4, 1rem)",
          bottom: "var(--oui-space-4, 1rem)",
        }}
      >
        <Button
          size="md"
          variant="outlined"
          color="secondary"
          onClick={copy}
          style={{
            color: "rgb(var(--oui-color-base-static-contrast, 0 0 0))",
          }}
        >
          Copy
        </Button>
        <Button
          size="md"
          variant="outlined"
          color="secondary"
          onClick={download}
          style={{
            color: "rgb(var(--oui-color-base-static-contrast, 0 0 0))",
            marginLeft: "var(--oui-space-2, 0.5rem)",
          }}
        >
          Download
        </Button>
      </div>
    </Container>
  );
};
