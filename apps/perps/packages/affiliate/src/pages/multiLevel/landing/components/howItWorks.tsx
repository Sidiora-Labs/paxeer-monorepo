import surface from "../../../../surface.module.css";
import { useMemo } from "react";
import { useTranslation } from "@paxeer-network/i18n";
import { Flex, Text, Grid, Box } from "@paxeer-network/ui";
import { IconType, StepCard } from "./stepCard";

type StepData = {
  step: number;
  icon: IconType;
  title: string;
  description: string;
};

export const HowItWorks = () => {
  const { t } = useTranslation();
  const steps = useMemo<StepData[]>(() => {
    return [
      {
        step: 1,
        icon: "wallet" as IconType,
        title: t("affiliate.howItWorks.step1.title"),
        description: t("affiliate.howItWorks.step1.description"),
      },
      {
        step: 2,
        icon: "settings" as IconType,
        title: t("affiliate.howItWorks.step2.title"),
        description: t("affiliate.howItWorks.step2.description"),
      },
      {
        step: 3,
        icon: "rocket" as IconType,
        title: t("affiliate.howItWorks.step3.title"),
        description: t("affiliate.howItWorks.step3.description"),
      },
    ];
  }, [t]);

  return (
    <Box
      id="oui-affiliate-landing-how-it-works"
      className="oui-affiliate-howItWorks oui-mb-24"
    >
      {/* Title and subtitle */}
      <Flex direction="column" itemAlign="start">
        <Text asChild weight="semibold" className="oui-text-4xl">
          <h2>{t("affiliate.howItWorks.title")}</h2>
        </Text>
        <Box py={4}>
          <Text intensity={54}>{t("affiliate.howItWorks.description")}</Text>
        </Box>
      </Flex>

      {/* Step cards */}
      <Grid className={`${surface.steps} oui-howItWorks-steps`} gap={6}>
        {steps.map((step) => (
          <StepCard
            key={step.step}
            step={step.step}
            icon={step.icon}
            title={step.title}
            description={step.description}
          />
        ))}
      </Grid>
    </Box>
  );
};
