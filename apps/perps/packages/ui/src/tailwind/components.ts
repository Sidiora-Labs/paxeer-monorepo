import plugin from "tailwindcss/plugin";

export const componentsPlugin = () =>
  plugin(function ({ addComponents, addBase }) {
    const components = {
      ".card:has(.header-list)": {
        paddingTop: "var(--oui-space-0)",
        paddingBottom: "var(--oui-space-0)",
      },
    };

    addComponents(components);
  });
