"use client";

import { Button } from "@openai/apps-sdk-ui/components/Button";
import { Markdown } from "@openai/apps-sdk-ui/components/Markdown";
import { Textarea } from "@openai/apps-sdk-ui/components/Textarea";
import { useEffect, useId, useRef, useState, type FormEvent } from "react";

export type DocsAssistantProps = {
  siteName: string;
  suggestions: string[];
  layout?: "floating" | "drawer";
};

type Source = { title: string; url: string };
type Turn = {
  id: number;
  question: string;
  answer: string;
  sources: Source[];
  status: "streaming" | "complete" | "stopped" | "error";
  error?: string;
};
type RequestState = { controller: AbortController; turnId: number };

function sourceHref(value: string): string | undefined {
  if (value.startsWith("/") && !value.startsWith("//")) return value;
  try {
    const url = new URL(value);
    return url.protocol === "https:" || url.protocol === "http:"
      ? url.href
      : undefined;
  } catch {
    return undefined;
  }
}

function ChatIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <path
        d="M5 4h14a2 2 0 0 1 2 2v10a2 2 0 0 1-2 2H9l-6 3V6a2 2 0 0 1 2-2Z"
        stroke="currentColor"
        strokeWidth="1.6"
        strokeLinejoin="round"
      />
      <path
        d="M8 9h8M8 13h5"
        stroke="currentColor"
        strokeWidth="1.6"
        strokeLinecap="round"
      />
    </svg>
  );
}

async function readEvents(
  response: Response,
  receive: (event: string, data: unknown) => boolean,
) {
  const reader = response.body?.getReader();
  if (!reader)
    throw new Error("The answer stream could not be opened. Please try again.");
  const decoder = new TextDecoder();
  let buffer = "";
  let event = "message";
  let data: string[] = [];
  let finished = false;
  const line = (value: string) => {
    if (!value) {
      if (data.length) {
        let parsed: unknown;
        try {
          parsed = JSON.parse(data.join("\n"));
        } catch {
          throw new Error(
            "The answer stream was interrupted. Please try again.",
          );
        }
        finished = receive(event, parsed);
      }
      event = "message";
      data = [];
      return;
    }
    if (value.startsWith(":")) return;
    const separator = value.indexOf(":");
    const field = separator === -1 ? value : value.slice(0, separator);
    let content = separator === -1 ? "" : value.slice(separator + 1);
    if (content.startsWith(" ")) content = content.slice(1);
    if (field === "event") event = content;
    if (field === "data") data.push(content);
  };
  try {
    while (!finished) {
      const chunk = await reader.read();
      buffer += decoder.decode(chunk.value, { stream: !chunk.done });
      let newline: number;
      while (!finished && (newline = buffer.indexOf("\n")) !== -1) {
        line(buffer.slice(0, newline).replace(/\r$/, ""));
        buffer = buffer.slice(newline + 1);
      }
      if (chunk.done) {
        if (!finished && buffer) line(buffer.replace(/\r$/, ""));
        if (!finished && data.length) line("");
        break;
      }
    }
    if (!finished)
      throw new Error(
        "The connection ended before the answer finished. Please retry.",
      );
  } finally {
    await reader.cancel().catch(() => {});
    reader.releaseLock();
  }
}

export function DocsAssistant({ siteName, suggestions, layout = "floating" }: DocsAssistantProps) {
  const dialogId = useId();
  const titleId = useId();
  const descriptionId = useId();
  const inputId = useId();
  const [open, setOpen] = useState(false);
  const [question, setQuestion] = useState("");
  const [turns, setTurns] = useState<Turn[]>([]);
  const [busy, setBusy] = useState(false);
  const [announcement, setAnnouncement] = useState("");
  const dialogRef = useRef<HTMLDialogElement>(null);
  const launcherRef = useRef<HTMLDivElement>(null);
  const scrollRef = useRef<HTMLDivElement>(null);
  const turnsRef = useRef<Turn[]>([]);
  const requestRef = useRef<RequestState | null>(null);
  const nextId = useRef(0);
  const followAnswer = useRef(true);

  function publish(next: Turn[]) {
    turnsRef.current = next;
    setTurns(next);
  }

  function updateTurn(id: number, update: (turn: Turn) => Turn) {
    publish(
      turnsRef.current.map((turn) => (turn.id === id ? update(turn) : turn)),
    );
  }

  useEffect(
    () => () => {
      requestRef.current?.controller.abort();
      requestRef.current = null;
    },
    [],
  );

  useEffect(() => {
    const dialog = dialogRef.current;
    if (!dialog) return;
    if (layout === "drawer") {
      document.documentElement.dataset.docsAssistant = open ? "open" : "closed";
    }
    if (!open) {
      if (dialog.open) {
        dialog.close();
        launcherRef.current?.querySelector("button")?.focus();
      }
      return;
    }
    const previousOverflow = document.body.style.overflow;
    const syncPresentation = () => {
      const modal = layout !== "drawer" || window.innerWidth < 1100;
      if (dialog.open && dialog.matches(":modal") !== modal) dialog.close();
      if (!dialog.open) {
        if (modal) dialog.showModal();
        else dialog.show();
      }
      document.body.style.overflow = modal ? "hidden" : previousOverflow;
    };
    syncPresentation();
    dialog.querySelector("textarea")?.focus({ preventScroll: true });
    const viewport = window.visualViewport;
    const resize = () => {
      syncPresentation();
      const height = viewport?.height ?? window.innerHeight;
      const offset = Math.max(
        0,
        window.innerHeight - height - (viewport?.offsetTop ?? 0),
      );
      dialog.style.setProperty("--da-viewport-height", `${height}px`);
      dialog.style.setProperty("--da-keyboard-offset", `${offset}px`);
    };
    resize();
    viewport?.addEventListener("resize", resize);
    viewport?.addEventListener("scroll", resize);
    window.addEventListener("resize", resize);
    return () => {
      document.body.style.overflow = previousOverflow;
      viewport?.removeEventListener("resize", resize);
      viewport?.removeEventListener("scroll", resize);
      window.removeEventListener("resize", resize);
    };
  }, [layout, open]);

  useEffect(() => () => {
    if (layout === "drawer") delete document.documentElement.dataset.docsAssistant;
  }, [layout]);

  useEffect(() => {
    if (open && followAnswer.current && scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [turns, open]);

  function stop() {
    const request = requestRef.current;
    if (!request) return;
    requestRef.current = null;
    request.controller.abort();
    updateTurn(request.turnId, (turn) => ({ ...turn, status: "stopped" }));
    setBusy(false);
    setAnnouncement("Answer stopped.");
  }

  function clear() {
    requestRef.current?.controller.abort();
    requestRef.current = null;
    publish([]);
    setQuestion("");
    setBusy(false);
    setAnnouncement("Conversation cleared.");
    followAnswer.current = true;
    dialogRef.current?.querySelector("textarea")?.focus();
  }

  async function ask(value: string, previous = turnsRef.current) {
    const text = value.trim();
    if (!text || requestRef.current) return;
    const id = ++nextId.current;
    const request: RequestState = {
      controller: new AbortController(),
      turnId: id,
    };
    requestRef.current = request;
    followAnswer.current = true;
    publish([
      ...previous,
      { id, question: text, answer: "", sources: [], status: "streaming" },
    ]);
    setQuestion("");
    setBusy(true);
    setAnnouncement("Preparing your answer.");
    const current = () => requestRef.current === request;
    try {
      const history = previous
        .filter((turn) => turn.status === "complete")
        .slice(-5)
        .map((turn) => [
          { role: "user" as const, content: turn.question.slice(0, 6000) },
          { role: "assistant" as const, content: turn.answer.slice(0, 6000) },
        ]);
      while (
        history
          .flat()
          .reduce((total, message) => total + message.content.length, 0) > 20000
      )
        history.shift();
      const messages = [
        ...history.flat(),
        { role: "user" as const, content: text },
      ];
      const response = await fetch("/api/docs-assistant", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Accept: "text/event-stream",
        },
        body: JSON.stringify({ messages, page: window.location.pathname }),
        signal: request.controller.signal,
      });
      if (!response.ok) {
        const detail = (await response.json().catch(() => null)) as {
          error?: unknown;
          message?: unknown;
        } | null;
        const message = detail?.message ?? detail?.error;
        throw new Error(
          typeof message === "string"
            ? message
            : "The assistant is unavailable right now. Please try again.",
        );
      }
      await readEvents(response, (event, payload) => {
        if (!current()) return true;
        const data =
          payload && typeof payload === "object"
            ? (payload as Record<string, unknown>)
            : {};
        if (event === "error") {
          throw new Error(
            typeof data["message"] === "string"
              ? data["message"]
              : "The answer could not be completed. Please retry.",
          );
        }
        if (event === "sources" && Array.isArray(data["sources"])) {
          const seen = new Set<string>();
          const sources = data["sources"].flatMap((item: unknown) => {
            if (!item || typeof item !== "object") return [];
            const source = item as Record<string, unknown>;
            if (
              typeof source["title"] !== "string" ||
              typeof source["url"] !== "string"
            )
              return [];
            const url = sourceHref(source["url"]);
            if (!url || seen.has(url)) return [];
            seen.add(url);
            return [{ title: source["title"], url }];
          });
          updateTurn(id, (turn) => ({ ...turn, sources }));
        }
        if (event === "delta" && typeof data["text"] === "string") {
          const delta = data["text"];
          updateTurn(id, (turn) => ({ ...turn, answer: turn.answer + delta }));
        }
        return event === "done";
      });
      if (!current()) return;
      if (!turnsRef.current.find((turn) => turn.id === id)?.answer.trim()) {
        throw new Error(
          "The assistant finished without an answer. Please try again.",
        );
      }
      updateTurn(id, (turn) => ({ ...turn, status: "complete" }));
      setAnnouncement("Answer ready.");
    } catch (error) {
      if (!current()) return;
      const message =
        error instanceof Error
          ? error.message
          : "The answer could not be completed. Please retry.";
      updateTurn(id, (turn) => ({ ...turn, status: "error", error: message }));
      setAnnouncement("The answer could not be completed.");
    } finally {
      if (current()) {
        requestRef.current = null;
        setBusy(false);
      }
    }
  }

  function submit(event: FormEvent) {
    event.preventDefault();
    void ask(question);
  }

  return (
    <div className="docs-assistant" data-layout={layout} data-open={open}>
      <div className="docs-assistant-launcher" ref={launcherRef}>
        <Button
          color="primary"
          size="3xl"
          className="docs-assistant-launch-button"
          onClick={() => setOpen(true)}
          aria-haspopup="dialog"
          aria-expanded={open}
          aria-controls={dialogId}
        >
          <ChatIcon /> Ask AI
        </Button>
      </div>
      <dialog
        id={dialogId}
        ref={dialogRef}
        className="docs-assistant-dialog"
        aria-labelledby={titleId}
        aria-describedby={descriptionId}
        onCancel={(event) => {
          event.preventDefault();
          setOpen(false);
        }}
        onClose={() => { if (!dialogRef.current?.open) setOpen(false); }}
        onKeyDown={(event) => {
          if (event.key === "Escape") {
            event.preventDefault();
            setOpen(false);
            return;
          }
          if (layout === "drawer" && !event.currentTarget.matches(":modal")) return;
          if (
            event.key !== "Tab" ||
            event.altKey ||
            event.ctrlKey ||
            event.metaKey
          )
            return;
          const controls = Array.from(
            event.currentTarget.querySelectorAll<HTMLElement>(
              "button, a[href], textarea, summary, [tabindex]",
            ),
          ).filter(
            (element) =>
              element.tabIndex >= 0 &&
              element.getClientRects().length > 0 &&
              !element.matches('[disabled], [aria-disabled="true"]') &&
              !element.closest("[inert], [hidden]"),
          );
          const first = controls[0];
          const last = controls[controls.length - 1];
          if (event.shiftKey && document.activeElement === first && last) {
            event.preventDefault();
            last.focus();
          } else if (
            !event.shiftKey &&
            document.activeElement === last &&
            first
          ) {
            event.preventDefault();
            first.focus();
          }
        }}
        onClick={(event) => {
          if (event.target !== event.currentTarget) return;
          const rect = event.currentTarget.getBoundingClientRect();
          if (
            event.clientX < rect.left ||
            event.clientX > rect.right ||
            event.clientY < rect.top ||
            event.clientY > rect.bottom
          )
            setOpen(false);
        }}
      >
        <div className="docs-assistant-panel">
          <header className="docs-assistant-header">
            <div>
              <h2 id={titleId}>Ask {siteName}</h2>
              <p id={descriptionId}>Answers from the documentation.</p>
            </div>
            <div className="docs-assistant-header-actions">
              <Button
                color="secondary"
                variant="ghost"
                size="lg"
                onClick={clear}
                disabled={!turns.length && !question}
              >
                Clear
              </Button>
              <Button
                color="secondary"
                variant="ghost"
                size="lg"
                uniform
                onClick={() => setOpen(false)}
                aria-label="Close assistant"
              >
                <svg viewBox="0 0 24 24" fill="none" aria-hidden="true">
                  <path
                    d="m6 6 12 12M18 6 6 18"
                    stroke="currentColor"
                    strokeWidth="1.7"
                    strokeLinecap="round"
                  />
                </svg>
              </Button>
            </div>
          </header>
          <div
            className="docs-assistant-conversation"
            ref={scrollRef}
            role="log"
            aria-label="Conversation"
            aria-live="off"
            onScroll={() => {
              const el = scrollRef.current;
              if (el)
                followAnswer.current =
                  el.scrollHeight - el.scrollTop - el.clientHeight < 80;
            }}
          >
            {!turns.length ? (
              <div className="docs-assistant-empty">
                <div className="docs-assistant-empty-icon">
                  <ChatIcon />
                </div>
                <h3>What would you like to know?</h3>
                <p>
                  Ask a question, find a guide, or get help with your next step.
                </p>
                <div className="docs-assistant-suggestions">
                  {suggestions.slice(0, 4).map((suggestion) => (
                    <Button
                      key={suggestion}
                      color="secondary"
                      variant={layout === "drawer" ? "ghost" : "outline"}
                      pill={false}
                      className="docs-assistant-suggestion"
                      onClick={() => void ask(suggestion)}
                    >
                      {suggestion}
                      <span aria-hidden="true">↗</span>
                    </Button>
                  ))}
                </div>
              </div>
            ) : (
              turns.map((turn, index) => (
                <article className="docs-assistant-turn" key={turn.id}>
                  <div className="docs-assistant-question">
                    <span className="docs-assistant-sr-only">You: </span>
                    {turn.question}
                  </div>
                  <div className="docs-assistant-answer">
                    <p className="docs-assistant-answer-label">
                      {turn.status === "streaming" && <span className="docs-assistant-spinner" aria-hidden="true" />}
                      {turn.status === "streaming" ? "Thinking" : "AI answer"}
                    </p>
                    {turn.answer ? (
                      <Markdown
                        className="docs-assistant-markdown"
                        skipHtml
                        disallowedElements={["img"]}
                      >
                        {turn.answer}
                      </Markdown>
                    ) : turn.status === "streaming" ? (
                      <p className="docs-assistant-waiting" role="status">
                        Preparing your answer<span aria-hidden="true">…</span>
                      </p>
                    ) : null}
                    {turn.sources.length > 0 &&
                      turn.answer.trim() &&
                      turn.status !== "streaming" && (
                        <details className="docs-assistant-sources">
                          <summary>
                            <span>Sources · {turn.sources.length}</span>
                            <svg
                              viewBox="0 0 20 20"
                              fill="none"
                              aria-hidden="true"
                            >
                              <path
                                d="m6 8 4 4 4-4"
                                stroke="currentColor"
                                strokeWidth="1.5"
                                strokeLinecap="round"
                                strokeLinejoin="round"
                              />
                            </svg>
                          </summary>
                          <div>
                            {turn.sources.map((source) => (
                              <a
                                key={source["url"]}
                                href={source["url"]}
                                target="_blank"
                                rel="noopener noreferrer"
                                title="Open source in a new tab"
                              >
                                <span>{source["title"]}</span>
                                <span aria-hidden="true">↗</span>
                                <span className="docs-assistant-sr-only">
                                  {" "}
                                  (opens in a new tab)
                                </span>
                              </a>
                            ))}
                          </div>
                        </details>
                      )}
                    {turn.status === "error" && (
                      <p className="docs-assistant-error" role="alert">
                        {turn.error}
                      </p>
                    )}
                    {turn.status === "stopped" && (
                      <p className="docs-assistant-stopped">Answer stopped.</p>
                    )}
                    {(turn.status === "error" || turn.status === "stopped") &&
                      index === turns.length - 1 &&
                      !busy && (
                        <Button
                          color="secondary"
                          variant="outline"
                          size="lg"
                          className="docs-assistant-retry"
                          onClick={() =>
                            void ask(
                              turn.question,
                              turnsRef.current.slice(0, index),
                            )
                          }
                        >
                          Retry answer
                        </Button>
                      )}
                  </div>
                </article>
              ))
            )}
          </div>
          <form className="docs-assistant-composer" onSubmit={submit}>
            <label className="docs-assistant-sr-only" htmlFor={inputId}>
              Your question
            </label>
            <Textarea
              id={inputId}
              value={question}
              onChange={(event) => setQuestion(event.target.value)}
              placeholder="Ask a question…"
              rows={2}
              maxRows={5}
              autoResize
              maxLength={4000}
              onKeyDown={(event) => {
                if (
                  event.key === "Enter" &&
                  !event.shiftKey &&
                  !event.nativeEvent.isComposing &&
                  event.nativeEvent.keyCode !== 229
                ) {
                  event.preventDefault();
                  if (!busy) void ask(question);
                }
              }}
            />
            <div className="docs-assistant-composer-actions">
              <span className="docs-assistant-key-hint">
                Enter to send · Shift+Enter for a new line
              </span>
              {busy ? (
                <Button
                  color="secondary"
                  variant="outline"
                  size="lg"
                  onClick={stop}
                >
                  <span
                    className="docs-assistant-stop-icon"
                    aria-hidden="true"
                  />
                  Stop
                </Button>
              ) : (
                <Button
                  color="primary"
                  size="lg"
                  type="submit"
                  disabled={!question.trim()}
                >
                  Send
                  <svg viewBox="0 0 20 20" fill="none" aria-hidden="true">
                    <path
                      d="M10 15V5m-4 4 4-4 4 4"
                      stroke="currentColor"
                      strokeWidth="1.7"
                      strokeLinecap="round"
                      strokeLinejoin="round"
                    />
                  </svg>
                </Button>
              )}
            </div>
          </form>
          <p
            className="docs-assistant-sr-only"
            role="status"
            aria-live="polite"
          >
            {announcement}
          </p>
        </div>
      </dialog>
    </div>
  );
}
