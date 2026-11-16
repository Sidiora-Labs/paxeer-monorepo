import surface from "../../../../surface.module.css";
import { FC, useState, useEffect } from "react";
import { useTranslation } from "@paxeer-network/i18n";
import { cn } from "@paxeer-network/ui";
import { CampaignConfig } from "../type";

interface TimeLeft {
  days: number;
  hours: number;
  minutes: number;
  seconds: number;
}

interface TimeUnitProps {
  value: number;
  label: string;
  isMobile?: boolean;
}

const TimeUnit: FC<TimeUnitProps> = ({ value, label }) => (
  <div className="oui-grid oui-min-w-0 oui-gap-1">
    <dd className="oui-text-2xl oui-font-semibold oui-leading-normal oui-whitespace-nowrap">
      {value.toString().padStart(2, "0")}
    </dd>
    <dt className="oui-text-xs oui-text-content-muted">{label}</dt>
  </div>
);

export const CampaignsCountdown: FC<{
  campaign: CampaignConfig;
  isMobile?: boolean;
}> = ({ campaign, isMobile }) => {
  const { t } = useTranslation();
  const [timeLeft, setTimeLeft] = useState<TimeLeft>({
    days: 0,
    hours: 0,
    minutes: 0,
    seconds: 0,
  });
  const [isStarted, setIsStarted] = useState(false);

  useEffect(() => {
    const calculateTimeLeft = () => {
      const currentTime = new Date().getTime();
      const startTime = new Date(campaign.start_time).getTime();
      const endTime = new Date(campaign.end_time).getTime();

      // Determine if campaign has started
      const hasStarted = currentTime >= startTime;
      setIsStarted(hasStarted);

      // Calculate time difference based on campaign status
      const targetTime = hasStarted ? endTime : startTime;
      const difference = targetTime - currentTime;

      if (difference > 0) {
        const days = Math.floor(difference / (1000 * 60 * 60 * 24));
        const hours = Math.floor(
          (difference % (1000 * 60 * 60 * 24)) / (1000 * 60 * 60),
        );
        const minutes = Math.floor(
          (difference % (1000 * 60 * 60)) / (1000 * 60),
        );
        const seconds = Math.floor((difference % (1000 * 60)) / 1000);

        setTimeLeft({ days, hours, minutes, seconds });
      } else {
        setTimeLeft({ days: 0, hours: 0, minutes: 0, seconds: 0 });
      }
    };

    // Calculate immediately
    calculateTimeLeft();

    // Set up interval to update every second
    const timer = setInterval(calculateTimeLeft, 1000);

    // Cleanup interval on component unmount
    return () => clearInterval(timer);
  }, [campaign.start_time, campaign.end_time]);

  // Check if campaign has ended
  const currentTime = new Date();
  const endTime = new Date(campaign.end_time);

  if (currentTime > endTime) {
    return (
      <p className="oui-w-full oui-p-5 oui-text-center oui-text-lg oui-text-content-muted">
        {t("tradingLeaderboard.batteleHasEnded")}
      </p>
    );
  }

  const titleText = isStarted
    ? t("tradingLeaderboard.battleEndsIn")
    : t("tradingLeaderboard.battleStartsIn");

  // Time units configuration
  const timeUnits = [
    { value: timeLeft.days, label: "Days" },
    { value: timeLeft.hours, label: "Hours" },
    { value: timeLeft.minutes, label: "Minutes" },
    { value: timeLeft.seconds, label: "Seconds" },
  ];

  return (
    <section
      className="oui-grid oui-w-full oui-gap-3 oui-p-4 oui-text-center"
      aria-label={titleText}
    >
      <p className="oui-text-base oui-text-content-muted">{titleText}</p>
      <dl className={surface.countdown}>
        {timeUnits.map((unit) => (
          <TimeUnit key={unit.label} {...unit} />
        ))}
      </dl>
    </section>
  );
};
