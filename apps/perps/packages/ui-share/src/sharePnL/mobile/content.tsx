import {
  createRef,
  FC,
  useEffect,
  useId,
  useMemo,
  useRef,
  useState,
} from "react";
import { useTranslation } from "@paxeer-network/i18n";
import { Button, Input } from "@paxeer-network/ui";
import {
  PnLDisplayFormat,
  ReferralType,
  ShareEntity,
  ShareOptions,
  SharePnLOptions,
} from "../../types/types";
import { CarouselBackgroundImage } from "../desktop/carousel";
import { PnlFormatView } from "../desktop/pnlFormat";
import { ShareOption } from "../desktop/options";
import { Poster } from "../poster";
import { PosterRef } from "../poster/poster";
import { getPnlInfo, getPnLPosterData, savePnlInfo } from "../utils/utils";
import surface from "../../surface.module.css";

export const MobileSharePnLContent: FC<{
  entity: ShareEntity;
  hide?: () => void;
  baseDp?: number;
  quoteDp?: number;
  referral?: ReferralType;
  shareOptions: SharePnLOptions;
  brokerName?: string;
}> = (props) => {
  const { t } = useTranslation();
  const id = useId();
  const [config] = useState(getPnlInfo);
  const formats: PnLDisplayFormat[] =
    props.entity.roi != null && props.entity.pnl != null
      ? ["roi_pnl", "roi", "pnl"]
      : props.entity.roi != null
        ? ["roi"]
        : props.entity.pnl != null
          ? ["pnl"]
          : [];
  const [pnlFormat, setPnlFormat] = useState<PnLDisplayFormat>(
    formats.length === 1 ? formats[0] : config.pnlFormat,
  );
  const [shareOption, setShareOption] = useState(
    new Set<ShareOptions>(config.options),
  );
  const [message, setMessage] = useState<string>(config.message);
  const [selectIndex, setSelectIndex] = useState(
    Math.max(
      0,
      Math.min(
        config.bgIndex,
        (props.shareOptions.backgroundImages?.length ?? 1) - 1,
      ),
    ),
  );
  const [domain, setDomain] = useState("");
  const [sharing, setSharing] = useState(false);
  const [shareError, setShareError] = useState("");
  const { backgroundImages = [], ...posterOptions } = props.shareOptions;
  const refs = useMemo(
    () => backgroundImages.map(() => createRef<PosterRef>()),
    [backgroundImages],
  );
  const track = useRef<HTMLDivElement>(null);
  useEffect(() => setDomain(window.location.hostname), []);
  useEffect(
    () => savePnlInfo(pnlFormat, shareOption, selectIndex, message),
    [pnlFormat, shareOption, selectIndex, message],
  );
  const selectBackground = (index: number) => {
    setSelectIndex(index);
    const element = track.current;
    const slide = element?.children[index];
    if (element && slide instanceof HTMLElement)
      element.scrollTo({ left: slide.offsetLeft });
  };
  useEffect(() => {
    selectBackground(selectIndex);
  }, [backgroundImages.length]);

  const posterData = getPnLPosterData(
    props.entity,
    message,
    domain,
    pnlFormat,
    shareOption,
    (key) => t(key),
    props.baseDp,
    props.quoteDp,
    props.referral,
    props.brokerName,
  );
  const options: ShareOptions[] = [
    "openPrice",
    "closePrice",
    "openTime",
    "closeTime",
    "leverage",
    "markPrice",
    "quantity",
  ];
  const share = async () => {
    const poster = refs[selectIndex]?.current;
    if (!poster) return;
    setSharing(true);
    setShareError("");
    try {
      const blob = await poster.toBlob();
      if (!blob) throw new Error(t("common.somethingWentWrong"));
      const files = [new File([blob], "image.png", { type: "image/png" })];
      if (navigator.canShare?.({ files })) {
        await navigator.share({ text: message, files });
      } else {
        poster.download("Poster.png");
      }
      props.hide?.();
    } catch (error) {
      if (!(error instanceof DOMException && error.name === "AbortError"))
        setShareError(t("common.somethingWentWrong"));
    } finally {
      setSharing(false);
    }
  };

  return (
    <div className={surface.surface}>
      <div
        ref={track}
        className={surface.posterTrack}
        role="region"
        aria-label={t("common.share")}
        aria-roledescription="carousel"
        tabIndex={0}
        onScroll={(event) => {
          const element = event.currentTarget;
          const slides = Array.from(element.children).filter(
            (slide): slide is HTMLElement => slide instanceof HTMLElement,
          );
          const nearest = slides.reduce(
            (best, slide, index) =>
              Math.abs(slide.offsetLeft - element.scrollLeft) <
              Math.abs(slides[best].offsetLeft - element.scrollLeft)
                ? index
                : best,
            0,
          );
          if (slides.length) setSelectIndex(nearest);
        }}
      >
        {backgroundImages.map((backgroundImg, index) => (
          <div
            key={`${backgroundImg}-${index}`}
            role="group"
            aria-roledescription="slide"
            aria-label={`${index + 1} / ${backgroundImages.length}`}
          >
            <Poster
              width={552}
              height={310}
              data={{
                ...posterOptions,
                backgroundImg,
                direction: "ltr",
                data: posterData,
              }}
              ratio={3}
              ref={refs[index]}
            />
          </div>
        ))}
      </div>
      <CarouselBackgroundImage
        backgroundImages={backgroundImages}
        selectedSnap={selectIndex}
        setSelectedSnap={selectBackground}
      />
      <div className="oui-grid oui-gap-4">
        <fieldset>
          <legend>{t("share.pnl.displayFormat")}</legend>
          <div className="oui-flex oui-flex-wrap oui-gap-3">
            {formats.map((type) => (
              <PnlFormatView
                key={type}
                type={type}
                curType={pnlFormat}
                setPnlFormat={setPnlFormat}
                name={`${id}-format`}
              />
            ))}
          </div>
        </fieldset>
        <fieldset>
          <legend>{t("share.pnl.optionalInfo")}</legend>
          <div className={surface.options}>
            {options
              .filter((key) => !!props.entity[key])
              .map((type) => (
                <ShareOption
                  key={type}
                  type={type}
                  curType={shareOption}
                  setShareOption={setShareOption}
                />
              ))}
          </div>
        </fieldset>
        <div className="oui-grid oui-gap-2">
          <label htmlFor={id} className="oui-text-sm oui-text-content-muted">
            {t("share.pnl.optionalInfo.message")}
          </label>
          <Input
            id={id}
            name="message"
            autoComplete="on"
            maxLength={25}
            size="xl"
            fullWidth
            placeholder={t("share.pnl.optionalInfo.message.placeholder")}
            value={message}
            onChange={(event) => setMessage(event.target.value)}
          />
        </div>
        {shareError && (
          <p role="status" className="oui-text-danger oui-text-sm">
            {shareError}
          </p>
        )}
        <Button fullWidth loading={sharing} disabled={sharing} onClick={share}>
          {t("common.share")}
        </Button>
      </div>
    </div>
  );
};
