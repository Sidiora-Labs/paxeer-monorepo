import React, { useEffect, useState } from "react";
import { styled } from "storybook/theming";

const StyledUl = styled.ul`
  list-style: none;
  padding: var(--oui-space-6, 1.5rem) 0;
  margin: 0;
  display: flex;
  flex-direction: column;
  gap: var(--oui-space-1, 0.25rem);
  align-items: end;
  font-size: var(--oui-font-size-xs, 0.8125rem);
  position: sticky;
  top: var(--oui-space-6, 1.5rem);
`;

const StyledA = styled.a<{ active: boolean }>`
  text-decoration: none;
  color: ${(props) =>
    props.active ? "rgb(var(--oui-color-primary, 176 132 233))" : "inherit"};
  cursor: pointer;
`;

const sections = [
  { id: "colors", label: "Colors" },
  { id: "radius", label: "Radius" },
  { id: "typography", label: "Typography" },
];
export const ThemeMenu = () => {
  const [activeSection, setActiveSection] = useState("");

  useEffect(() => {
    const observer = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          if (entry.isIntersecting) {
            setActiveSection(entry.target.id);
          }
        });
      },
      { threshold: 0.2, rootMargin: "-20px 0px 0px 0px" },
    );

    sections.forEach((section) => {
      const element = document.getElementById(section.id);
      if (element) observer.observe(element);
    });

    return () => observer.disconnect();
  }, []);

  const handleClick = (id: string) => (e: React.MouseEvent) => {
    e.preventDefault();
    document.getElementById(id)?.scrollIntoView({ behavior: "smooth" });
  };

  return (
    <StyledUl>
      {sections.map(({ id, label }) => (
        <li key={id}>
          <StyledA onClick={handleClick(id)} active={activeSection === id}>
            {label}
          </StyledA>
        </li>
      ))}
    </StyledUl>
  );
};
