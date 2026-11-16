import plugin from "tailwindcss/plugin";

export const scrollBarPlugin = () =>
  plugin(function ({ addComponents }) {
    const customVerticalScrollbar = {
      ".custom-scrollbar::-webkit-scrollbar": {
        width: "var(--oui-scrollbar-size, 6px)",
        height: "var(--oui-scrollbar-size, 6px)",
      },

      ".custom-scrollbar::-webkit-scrollbar-track": {
        backgroundColor: "transparent",
        borderRadius: "var(--oui-radius-md)",
      },

      ".custom-scrollbar::-webkit-scrollbar-thumb": {
        borderRadius: "var(--oui-radius-md)",
        borderStyle: "dashed",
        borderColor: "transparent",
        borderWidth: "1px",
        backgroundColor: "rgb(var(--oui-color-base-7))",
        backgroundClip: "padding-box",
      },

      ".custom-scrollbar::-webkit-scrollbar-thumb:hover": {
        backgroundColor: "rgb(var(--oui-color-base-5))",
        borderRadius: "var(--oui-radius-md)",
      },

      ".custom-scrollbar::-webkit-scrollbar-corner": {
        backgroundColor: "transparent",
      },
    };

    addComponents(customVerticalScrollbar);

    const hideScrollBar = {
      ".hide-scrollbar::-webkit-scrollbar": {
        display: "none",
      },
      ".hide-scrollbar::-webkit-scrollbar-thumb": {
        display: "none",
      },
    };

    addComponents(hideScrollBar);
  });
