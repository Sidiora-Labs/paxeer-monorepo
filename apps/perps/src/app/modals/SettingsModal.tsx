"use client";

import { useTheme } from "next-themes";
import { motion, AnimatePresence } from "framer-motion";
import { XIcon, SunIcon, MoonIcon, Zap, Check } from "lucide-react";
import * as Dialog from "@radix-ui/react-dialog";
import { useState, useEffect } from "react";
import { useDialogFocus } from "./useDialogFocus";
import { useDialogPreferences } from "./dialogPreferences";
import { cn } from "@/ui/utils/cn";
import { useIsMobile } from "@/hooks/ui/useMobile";
import { Switch } from "@/ui/atoms/switch";
import { Label } from "@/ui/atoms/label";
import { Button } from "@/ui/atoms/button";

interface SettingsModalProps {
  isOpen: boolean;
  onClose: () => void;
}

export function SettingsModal({ isOpen, onClose }: SettingsModalProps) {
  const { theme, setTheme, resolvedTheme } = useTheme();
  const [mounted, setMounted] = useState(false);
  const activeTheme = theme === "system" ? resolvedTheme : theme;
  const isMobile = useIsMobile();

  const preferences = useDialogPreferences();
  const focus = useDialogFocus();

  // Handle initial theme setup after mount
  useEffect(() => {
    setMounted(true);
  }, []);

  const themeOptions = [
    {
      label: "Paxify",
      icon: SunIcon,
      value: "greenify",
      gradient: "linear-gradient(135deg, var(--brand-blue) 0%, #0035a8 100%)",
      badgeColor: "#ecfdf5",
      badgeBg: "#13136d",
      border: "240 70% 25%",
    },
    {
      label: "Dark",
      icon: MoonIcon,
      value: "dark",
      bgImage: "/Dark.png",
      gradient:
        "linear-gradient(135deg, #0f172a 0%, #1e293b 50%, #334155 100%)",
      badgeColor: "#f1f5f9",
      badgeBg: "#1e293b",
      border: "#334155",
    },
    {
      label: "Hotline",
      icon: MoonIcon,
      value: "hotline",
      bgImage: "/Hotline.png",
      gradient:
        "linear-gradient(135deg, #701a75 0%, #be185d 50%, #ec4899 100%)",
      badgeColor: "#fdf2f8",
      badgeBg: "#701a75",
      border: "#be185d",
    },
    {
      label: "OLED",
      icon: MoonIcon,
      value: "oled",
      bgImage: "/OLED.png",
      gradient:
        "linear-gradient(135deg, #000000 0%, var(--brand-card) 50%, #1a1a1a 100%)",
      badgeColor: "#ffffff",
      badgeBg: "#000000",
      border: "#333333",
    },
  ];

  // Don't render anything until mounted to prevent hydration mismatch
  if (!mounted) return null;

  return (
    <Dialog.Root
      open={isOpen}
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <AnimatePresence>
        {isOpen && preferences.status !== "loading" && (
          <Dialog.Portal forceMount key="settings">
            <Dialog.Overlay forceMount asChild>
              <motion.div
                initial={preferences.reduceMotion ? false : { opacity: 0 }}
                animate={{ opacity: 1 }}
                exit={{ opacity: 0 }}
                transition={
                  preferences.reduceMotion ? { duration: 0 } : undefined
                }
                className="fixed inset-0 z-[9998] bg-black/50 backdrop-blur-sm"
              />
            </Dialog.Overlay>
            <Dialog.Content forceMount asChild {...focus}>
              {/* Drawer */}
              <motion.div
                initial={
                  preferences.reduceMotion
                    ? false
                    : isMobile
                      ? { y: "100%" }
                      : { x: "100%" }
                }
                animate={isMobile ? { y: 0 } : { x: 0 }}
                exit={isMobile ? { y: "100%" } : { x: "100%" }}
                transition={
                  preferences.reduceMotion
                    ? { duration: 0 }
                    : { type: "spring", damping: 30, stiffness: 300 }
                }
                className={cn(
                  "fixed z-[9999] bg-card bg-surface-1 shadow-2xl flex flex-col",
                  isMobile
                    ? "bottom-0 left-0 right-0 rounded-t-2xl h-[85vh]"
                    : "top-0 right-0 h-screen w-[420px] max-w-[90vw]",
                )}
              >
                {/* Header */}
                <div className="flex items-center justify-between p-6 border-b border-border flex-shrink-0">
                  <div>
                    <Dialog.Title className="text-lg font-semibold text-foreground">
                      Settings
                    </Dialog.Title>
                    <Dialog.Description className="text-sm text-muted-foreground mt-0.5">
                      Customize your appearance and dialog motion
                    </Dialog.Description>
                  </div>
                  <Dialog.Close asChild>
                    <Button
                      aria-label="Close settings"
                      variant="ghost"
                      size="icon"
                      className="h-8 w-8 text-muted-foreground hover:text-foreground"
                    >
                      <XIcon className="w-4 h-4" />
                    </Button>
                  </Dialog.Close>
                </div>

                {/* Scrollable Content */}
                <div
                  className={cn(
                    "flex-1 overflow-y-scroll",
                    isMobile ? "" : "scrollbar-custom",
                  )}
                  style={{
                    WebkitOverflowScrolling: "touch",
                  }}
                >
                  <div className="p-6 space-y-6">
                    {/* Theme Section */}
                    <div className="space-y-4">
                      <div>
                        <h3 className="text-sm font-medium text-foreground mb-1">
                          Appearance
                        </h3>
                        <p className="text-xs text-muted-foreground">
                          Choose your preferred theme
                        </p>
                      </div>
                      <div className="grid grid-cols-1 gap-3">
                        {themeOptions.map((option) => (
                          <button
                            key={option.value}
                            onClick={() => setTheme(option.value)}
                            aria-pressed={option.value === activeTheme}
                            className={cn(
                              // Apple-style interactive card. Adds the
                              // specular hairline + hover lift + sweep on
                              // top of the existing selected-ring affordance.
                              "au-card au-card--interactive",
                              "flex items-center gap-4 !p-3 rounded-xl !border-2 transition-all duration-200",
                              option.value === activeTheme
                                ? "!border-accent bg-accent/10 ring-2 ring-accent/20"
                                : "!border-border hover:!border-accent/50 hover:!bg-accent/5",
                            )}
                          >
                            {/* Real backdrop preview — mirrors what the
                            user will see across the app once selected. */}
                            <div
                              className="w-10 h-10 rounded-lg border flex items-center justify-center relative overflow-hidden shrink-0"
                              style={{
                                backgroundImage: option.bgImage
                                  ? `url(${option.bgImage}), ${option.gradient}`
                                  : option.gradient,
                                backgroundSize: "cover",
                                backgroundPosition: "center",
                                borderColor: option.border,
                              }}
                            >
                              {option.value === activeTheme && (
                                <Check
                                  className="w-5 h-5 text-white drop-shadow-sm"
                                  style={{ color: option.badgeColor }}
                                />
                              )}
                            </div>

                            {/* Theme Info */}
                            <div className="flex-1 text-left">
                              <div className="text-sm font-medium text-foreground">
                                {option.label}
                              </div>
                              <div className="text-xs text-muted-foreground">
                                {option.label === "Paxify" &&
                                  "Fresh and natural"}
                                {option.label === "Dark" && "Easy on the eyes"}
                                {option.label === "Hotline" &&
                                  "Bold and vibrant"}
                                {option.label === "OLED" && "Pure black theme"}
                              </div>
                            </div>

                            {/* Icon */}
                            <option.icon className="w-4 h-4 text-muted-foreground" />
                          </button>
                        ))}
                      </div>
                    </div>

                    <DialogMotionSettings />
                  </div>
                </div>
              </motion.div>
            </Dialog.Content>
          </Dialog.Portal>
        )}
      </AnimatePresence>
    </Dialog.Root>
  );
}

export function DialogMotionSettings() {
  const preferences = useDialogPreferences();
  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between gap-3">
        <div className="flex items-center gap-3">
          <Zap className="h-4 w-4 text-accent" aria-hidden="true" />
          <div>
            <Label
              htmlFor="reduce-dialog-motion"
              className="text-sm font-medium"
            >
              Reduce dialog motion
            </Label>
            <p
              id="dialog-motion-description"
              className="text-xs text-muted-foreground"
            >
              Remove settings drawer movement and wallet panel entrance effects.
            </p>
          </div>
        </div>
        <Switch
          id="reduce-dialog-motion"
          aria-describedby="dialog-motion-description"
          checked={preferences.reduceMotion}
          disabled={preferences.status === "loading"}
          onCheckedChange={preferences.setReduceMotion}
        />
      </div>
      <p
        role={preferences.status === "error" ? "alert" : "status"}
        className="text-xs text-muted-foreground"
      >
        {preferences.status === "loading"
          ? "Loading dialog preference…"
          : preferences.status === "error"
            ? preferences.error
            : preferences.status === "saved"
              ? "Dialog motion preference saved on this device."
              : "Dialog motion uses the default until you save a preference."}
      </p>
      <p className="text-xs text-muted-foreground">
        Themes are managed separately by the application theme provider.
      </p>
    </div>
  );
}
