import surface from "../../../surface.module.css";
import { Flex } from "@paxeer-network/ui";
import { Faq } from "./components/faq";
import { Hero } from "./components/hero";
import { HowItWorks } from "./components/howItWorks";

export const LandingPage = () => {
  return (
    <Flex
      id="oui-affiliate-landing-page"
      direction="column"
      itemAlign={"stretch"}
      className={`${surface.surface} ${surface.landing} oui-affiliate-landing-page`}
    >
      <Hero />
      <HowItWorks />
      <Faq />
    </Flex>
  );
};
