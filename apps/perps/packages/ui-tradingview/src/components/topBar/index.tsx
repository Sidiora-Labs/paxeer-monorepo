import React from "react";

const TopBar: React.FC<React.PropsWithChildren> = (props) => {
  return (
    <div className="top-toolbar oui-flex oui-h-11 oui-w-full oui-items-center oui-justify-between oui-p-2 md:oui-justify-start md:oui-px-3 md:oui-py-3">
      {props.children}
    </div>
  );
};

export default TopBar;
