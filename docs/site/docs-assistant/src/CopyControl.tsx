"use client";

import { useEffect, useRef, useState } from "react";

type CopyStatus = "idle" | "copied" | "error";
type CopyControlProps = {
  targetId: string;
  label: string;
  className?: string;
  title?: string;
  icon?: boolean;
  resetKey?: string;
  errorMessage?: string;
  beforeCopy?: () => boolean;
  onCopy?: (target: HTMLElement) => Promise<void>;
  onFeedback?: (message: string) => void;
};

let lastAnnouncement = "";

export async function writeCopyText(text: string) {
  try {
    if (!navigator.clipboard?.writeText) throw new Error("Clipboard API unavailable");
    await navigator.clipboard.writeText(text);
  } catch {
    const field = document.createElement("textarea");
    const previousFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    field.value = text;
    field.className = "docs-copy-field";
    field.readOnly = true;
    field.setAttribute("aria-hidden", "true");
    field.tabIndex = -1;
    (previousFocus?.closest("dialog") ?? document.body).appendChild(field);
    try {
      field.select();
      if (!document.execCommand("copy")) throw new Error("Clipboard access failed");
    } finally {
      field.remove();
      previousFocus?.focus({ preventScroll: true });
    }
  }
}

export function CopyControl(props: CopyControlProps) {
  const { targetId, label, className, title, icon = true, resetKey } = props;
  const button = useRef<HTMLButtonElement>(null);
  const current = useRef(props);
  current.current = props;
  const timer = useRef<ReturnType<typeof setTimeout>>();
  const sequence = useRef(0);
  const [status, setStatus] = useState<CopyStatus>("idle");
  const [message, setMessage] = useState("");

  useEffect(() => {
    clearTimeout(timer.current);
    sequence.current++;
    setStatus("idle");
    setMessage("");
    current.current.onFeedback?.("");
  }, [targetId, resetKey]);

  useEffect(() => () => { clearTimeout(timer.current); sequence.current++; }, []);

  useEffect(() => {
    const source = button.current;
    if (!source) return;
    const owner = source.ownerDocument;
    const invoke = async (target: EventTarget | null) => {
      if (current.current.beforeCopy && !current.current.beforeCopy()) return;
      const request = ++sequence.current;
      clearTimeout(timer.current);
      setStatus("idle");
      setMessage("");
      current.current.onFeedback?.("");
      let nextStatus: "copied" | "error";
      try {
        if (!(target instanceof HTMLElement)) throw new Error("Copy target unavailable");
        if (current.current.onCopy) await current.current.onCopy(target);
        else await writeCopyText(target.textContent ?? "");
        nextStatus = "copied";
      } catch {
        nextStatus = "error";
      }
      if (request !== sequence.current) return;
      let announcement = nextStatus === "copied" ? "Copied" : current.current.errorMessage ?? "Copy failed";
      if (announcement === lastAnnouncement) announcement += "\u00a0";
      lastAnnouncement = announcement;
      setStatus(nextStatus);
      setMessage(announcement);
      current.current.onFeedback?.(announcement);
      const duration = getComputedStyle(owner.documentElement).getPropertyValue("--duration-feedback").trim();
      const milliseconds = Number.parseFloat(duration) * (duration.endsWith("ms") ? 1 : 1000);
      timer.current = setTimeout(() => {
        setStatus("idle");
        setMessage("");
        current.current.onFeedback?.("");
      }, milliseconds);
    };
    const click = () => { void invoke(owner.getElementById(targetId)); };
    source.addEventListener("click", click);
    return () => source.removeEventListener("click", click);
  }, [targetId]);

  return <span className="docs-copy-control" data-status={status}>
    <button ref={button} type="button" className={className} title={title}>
      {icon && <svg className="docs-copy-icon" viewBox="0 0 24 24" fill="none" aria-hidden="true">
        {status === "copied" ? <path d="m5 13 4 4L19 7" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" /> : <><rect x="9" y="9" width="13" height="13" rx="2" stroke="currentColor" strokeWidth="1.7" /><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2 2v1" stroke="currentColor" strokeWidth="1.7" /></>}
      </svg>}
      <span>{label}</span>
    </button>
    {!props.onFeedback && <span className="docs-copy-feedback" role="status">{message}</span>}
  </span>;
}
