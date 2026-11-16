import { FC, useEffect, useRef } from "react";
import { qrcode as qr } from "@akamfoad/qr";
import { Box, Flex } from "@paxeer-network/ui";

type QRCodeDisplayProps = {
  address?: string;
};

export const QRCodeDisplay: FC<QRCodeDisplayProps> = ({ address }) => {
  return (
    <Flex direction="column" className="oui-mt-5 oui-w-full" itemAlign="center">
      <Box className="oui-relative oui-flex oui-size-36 oui-items-center oui-justify-center oui-rounded-lg oui-bg-base-static oui-p-2">
        {address ? (
          <QRCodeCanvas width={124} height={124} content={address} />
        ) : (
          <Box className="oui-size-full oui-border-8 oui-border-dashed oui-border-base-static-contrast oui-opacity-50" />
        )}
      </Box>
    </Flex>
  );
};

type QRCodeCanvasProps = {
  width: number;
  height: number;
  content: string;
};

const QRCodeCanvas: FC<QRCodeCanvasProps> = ({ width, height, content }) => {
  const canvasRef = useRef<HTMLCanvasElement>(null);

  useEffect(() => {
    if (!canvasRef.current || !content) return;

    const qrcode = qr(content);
    const ctx = canvasRef.current.getContext("2d")!;
    const cells = qrcode.modules!;

    const colors = getComputedStyle(canvasRef.current);
    const ink = `rgb(${colors.getPropertyValue("--oui-color-base-static-contrast")})`;
    const paper = `rgb(${colors.getPropertyValue("--oui-color-base-static")})`;

    const tileW = width / cells.length;
    const tileH = height / cells.length;

    for (let r = 0; r < cells.length; ++r) {
      const row = cells[r];
      for (let c = 0; c < row.length; ++c) {
        ctx.fillStyle = row[c] ? ink : paper;
        const w = Math.ceil((c + 1) * tileW) - Math.floor(c * tileW);
        const h = Math.ceil((r + 1) * tileH) - Math.floor(r * tileH);
        ctx.fillRect(Math.round(c * tileW), Math.round(r * tileH), w, h);
      }
    }
  }, [content, width, height]);

  return (
    <canvas
      width={width}
      height={height}
      ref={canvasRef}
      role="img"
      aria-label={content}
      className="oui-max-w-full oui-h-auto"
    />
  );
};
