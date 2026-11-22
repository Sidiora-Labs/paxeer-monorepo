import type { Metadata, Viewport } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "LayerX Developer",
  other: { "text-scale": "scale" },
  description:
    "API keys, usage, protocol requests, webhook delivery and verified receipts",
};

export const viewport: Viewport = { colorScheme: "light dark" };

export default function RootLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
