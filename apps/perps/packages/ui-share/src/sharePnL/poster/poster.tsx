import { cn } from "@paxeer-network/ui";
import { useTranslation } from "@paxeer-network/i18n";
import { forwardRef, useId, useImperativeHandle } from "react";
import { type DrawOptions, usePoster } from "@paxeer-network/hooks";

export type PosterProps = {
  width: number;
  height: number;
  className?: string;
  ratio?: number;
  data: DrawOptions;
  style?: React.CSSProperties;
};

export type PosterRef = {
  download: (filename: string, type?: string, encoderOptions?: number) => void;
  toDataURL: (type?: string, encoderOptions?: number) => string;
  toBlob: (type?: string, encoderOptions?: number) => Promise<Blob | null>;
  copy: () => Promise<void>;
};

export const Poster = forwardRef<PosterRef, PosterProps>((props, parentRef) => {
  const { width, height, className, data, style } = props;
  const { t } = useTranslation();
  const id = useId();
  const position = data.data?.position;

  const { ref, download, toDataURL, copy, toBlob } = usePoster(data, {
    ratio: props.ratio,
  });

  useImperativeHandle(parentRef, () => ({
    download,
    toDataURL,
    toBlob,
    copy,
  }));

  return (
    <figure className="oui-m-0 oui-w-full oui-min-w-0">
      <canvas
        ref={ref}
        width={width}
        height={height}
        dir="ltr"
        role="img"
        aria-describedby={id}
        aria-label={`${data.data?.position.symbol} ${data.data?.position.side} ${data.data?.position.pnl ?? ""} ${data.data?.position.ROI ?? ""}`}
        className={cn("oui-max-w-full oui-h-auto", className)}
        style={style}
      >
        {t("common.share")} {position?.symbol} {position?.side} {position?.pnl}{" "}
        {position?.ROI}
      </canvas>
      <figcaption id={id} className="oui-sr-only">
        <p>
          {data.data?.message} {position?.symbol} {position?.side}{" "}
          {position?.pnl} {position?.currency} {position?.ROI}%
        </p>
        <dl>
          {position?.informations.map(({ title, value }) => (
            <div key={title}>
              <dt>{title}</dt>
              <dd>{value}</dd>
            </div>
          ))}
        </dl>
        <p>
          {data.data?.updateTime} {data.data?.referral?.code}{" "}
          {data.data?.referral?.link}
        </p>
      </figcaption>
    </figure>
  );
});
