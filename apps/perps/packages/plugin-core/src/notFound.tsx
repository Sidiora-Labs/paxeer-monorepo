import { FC } from "react";

export const NotFound: FC<{ position: string }> = (props) => {
  const { position } = props;
  return (
    <div role="status" style={{ color: "rgb(var(--oui-color-danger))" }}>
      <strong>{`[${position}] `}</strong>
      <span>Not found!</span>
    </div>
  );
};
