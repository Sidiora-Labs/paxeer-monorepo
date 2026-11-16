import { FC, useId, useRef } from "react";
import surface from "../../surface.module.css";
import { useTranslation } from "@paxeer-network/i18n";
import { cn } from "@paxeer-network/ui";

export const CarouselBackgroundImage: FC<{
  backgroundImages: ReadonlyArray<string>;
  selectedSnap: number;
  setSelectedSnap: (index: number) => void;
}> = ({ backgroundImages, selectedSnap, setSelectedSnap }) => {
  const id = useId();
  const { t } = useTranslation();
  const track = useRef<HTMLDivElement>(null);
  const select = (index: number) => {
    setSelectedSnap(index);
    const element = track.current;
    const child = element?.children[index];
    if (element && child) {
      const bounds = element.getBoundingClientRect();
      const item = child.getBoundingClientRect();
      if (item.left < bounds.left)
        element.scrollBy({ left: item.left - bounds.left });
      else if (item.right > bounds.right)
        element.scrollBy({ left: item.right - bounds.right });
    }
  };
  return (
    <div
      role="group"
      aria-label={t("common.share")}
      className={surface.thumbnailCarousel}
    >
      <button
        type="button"
        aria-label="Previous slide"
        disabled={selectedSnap <= 0}
        onClick={() => select(selectedSnap - 1)}
        className="oui-min-h-12 oui-min-w-12 oui-rounded-md hover:oui-bg-base-5 disabled:oui-opacity-50"
      >
        ‹
      </button>
      <div
        ref={track}
        className={`${surface.thumbnailTrack} oui-flex oui-min-w-0 oui-flex-1 oui-gap-3 oui-overflow-x-auto oui-snap-x oui-snap-mandatory oui-p-1`}
      >
        {backgroundImages.map((src, index) => (
          <button
            type="button"
            key={`${src}-${index}`}
            aria-label={`${index + 1} / ${backgroundImages.length}`}
            aria-pressed={selectedSnap === index}
            aria-current={selectedSnap === index ? "true" : undefined}
            onClick={() => setSelectedSnap(index)}
            className={cn(
              "oui-w-40 oui-shrink-0 oui-snap-center oui-rounded-md oui-border-2 oui-p-1",
              selectedSnap === index
                ? "oui-border-primary"
                : "oui-border-transparent hover:oui-border-base-contrast-80",
            )}
          >
            <img
              src={src}
              alt=""
              width={552}
              height={310}
              loading="lazy"
              className="oui-h-auto oui-w-full oui-rounded-sm"
            />
          </button>
        ))}
      </div>
      <button
        type="button"
        aria-label="Next slide"
        disabled={selectedSnap >= backgroundImages.length - 1}
        onClick={() => select(selectedSnap + 1)}
        className="oui-min-h-12 oui-min-w-12 oui-rounded-md hover:oui-bg-base-5 disabled:oui-opacity-50"
      >
        ›
      </button>
      <span id={id} role="status" className="oui-sr-only">
        {backgroundImages.length ? selectedSnap + 1 : 0} /{" "}
        {backgroundImages.length}
      </span>
    </div>
  );
};
