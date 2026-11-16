import { FC } from "react";
import { cn } from "@paxeer-network/ui";
import surface from "../../../../surface.module.css";

export interface TimelinePoint {
  title: string;
  type: "past" | "active" | "future";
  time: string;
}
export interface CampaignsAxisProps {
  points: TimelinePoint[];
}
export const CampaignsAxis: FC<CampaignsAxisProps> = ({ points }) => {
  if (!points.length) return null;
  return (
    <ol className={surface.timeline}>
      {points.map((point, index) => (
        <li
          key={`${point.title}-${index}`}
          className="oui-flex oui-min-w-0 oui-flex-col oui-items-center oui-gap-3 oui-text-center"
          aria-current={point.type === "active" ? "step" : undefined}
        >
          <AxisPoint type={point.type} />
          <p className="oui-text-sm oui-font-medium oui-text-content-muted">
            {point.title}
          </p>
          {point.type !== "active" && (
            <p className="oui-text-xs oui-text-content-muted">{point.time}</p>
          )}
        </li>
      ))}
    </ol>
  );
};
export const CampaignsAxisMobile = CampaignsAxis;

const AxisPoint: FC<{ type: "past" | "active" | "future" }> = ({ type }) => {
  return (
    <div
      className={cn([
        "oui-size-5 oui-shrink-0 oui-rounded-full",
        type === "past" && "oui-bg-base-contrast-80",
        type === "active" &&
          "oui-bg-gradient-to-r oui-from-primary oui-to-primary",
        type === "future" &&
          "oui-border-2 oui-border-solid oui-border-base-contrast-80 oui-bg-base-10",
      ])}
    />
  );
};
