import { FC, useEffect, useRef, useState } from "react";
import jsQR from "jsqr";
import { useTranslation } from "@paxeer-network/i18n";

type ScannerProps = {
  onSuccess?: (data: string) => void;
};

// Capture resolution is independent of the responsive display size.
export const QRCODE_WIDTH = 320;
export const QRCODE_HEIGHT = 320;
const RATIO = 2;

export const QRCodeScanner: FC<ScannerProps> = (props) => {
  const videoRef = useRef<HTMLVideoElement>(null);
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const onSuccessRef = useRef(props.onSuccess);
  onSuccessRef.current = props.onSuccess;
  const [cameraUnavailable, setCameraUnavailable] = useState(false);
  const { t } = useTranslation();

  useEffect(() => {
    const video = videoRef.current;
    const canvas = canvasRef.current;
    if (!video || !canvas) return;
    const ctx = canvas.getContext("2d");
    if (!ctx) {
      setCameraUnavailable(true);
      return;
    }
    let cancelled = false;
    let frame = 0;
    let stream: MediaStream | undefined;

    const tick = () => {
      if (cancelled) return;
      if (video.readyState === video.HAVE_ENOUGH_DATA) {
        const videoWidth = video.videoWidth;
        const videoHeight = video.videoHeight;
        const canvasAspectRatio = canvas.width / canvas.height;
        const videoAspectRatio = videoWidth / videoHeight;
        const sWidth =
          videoAspectRatio > canvasAspectRatio
            ? videoHeight * canvasAspectRatio
            : videoWidth;
        const sHeight =
          videoAspectRatio > canvasAspectRatio
            ? videoHeight
            : videoWidth / canvasAspectRatio;
        ctx.drawImage(
          video,
          (videoWidth - sWidth) / 2,
          (videoHeight - sHeight) / 2,
          sWidth,
          sHeight,
          0,
          0,
          canvas.width,
          canvas.height,
        );
        const imageData = ctx.getImageData(0, 0, canvas.width, canvas.height);
        const result = jsQR(imageData.data, imageData.width, imageData.height, {
          inversionAttempts: "dontInvert",
        });
        if (result?.data) onSuccessRef.current?.(result.data);
      }
      frame = requestAnimationFrame(tick);
    };

    const start = async () => {
      if (!navigator.mediaDevices?.getUserMedia) {
        setCameraUnavailable(true);
        return;
      }
      try {
        stream = await navigator.mediaDevices.getUserMedia({
          video: {
            facingMode: "environment",
            width: { ideal: QRCODE_WIDTH * RATIO },
            height: { ideal: QRCODE_HEIGHT * RATIO },
          },
        });
        if (cancelled) {
          stream.getTracks().forEach((track) => track.stop());
          return;
        }
        video.srcObject = stream;
        await video.play();
        if (!cancelled) frame = requestAnimationFrame(tick);
      } catch {
        stream?.getTracks().forEach((track) => track.stop());
        video.srcObject = null;
        if (!cancelled) setCameraUnavailable(true);
      }
    };
    void start();
    return () => {
      cancelled = true;
      cancelAnimationFrame(frame);
      stream?.getTracks().forEach((track) => track.stop());
      video.srcObject = null;
    };
  }, []);

  return (
    <>
      <video
        ref={videoRef}
        playsInline
        muted
        width={QRCODE_WIDTH}
        height={QRCODE_HEIGHT}
        className="oui-hidden"
      />
      <canvas
        ref={canvasRef}
        width={QRCODE_WIDTH * RATIO}
        height={QRCODE_HEIGHT * RATIO}
        aria-label={t("linkDevice.scanQRCode")}
        className="oui-size-full oui-rounded-2xl oui-bg-base-10"
      />
      {cameraUnavailable && (
        <p
          role="alert"
          className="oui-absolute oui-inset-0 oui-flex oui-items-center oui-justify-center oui-rounded-2xl oui-bg-base-8 oui-p-4 oui-text-center oui-text-xs oui-text-base-contrast"
        >
          {t("linkDevice.scanQRCode.cameraUnavailable")}
        </p>
      )}
    </>
  );
};
