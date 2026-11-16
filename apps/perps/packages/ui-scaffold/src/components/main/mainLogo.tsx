import { FC } from "react";
import { useAppConfig } from "@paxeer-network/react-app";
import { PaxeerNetworkLogo } from "../icons";
import { Logo } from "@paxeer-network/ui";

type MainLogoProps = {
  src?: string;
  alt?: string;
};

export const MainLogo: FC<MainLogoProps> = (props) => {
  const { appIcons } = useAppConfig();

  if (props.src) {
    return <Logo src={props.src} alt={props.alt} />;
  }

  const { main } = appIcons || {};

  if (main?.img) {
    return (
      <img
        src={main.img}
        alt={props.alt ?? "Paxeer Xtrades"}
        width={160}
        height={32}
        className="oui-max-w-full oui-object-contain"
      />
    );
  }

  if (main?.component) {
    return main.component;
  }

  return <PaxeerNetworkLogo />;
};
