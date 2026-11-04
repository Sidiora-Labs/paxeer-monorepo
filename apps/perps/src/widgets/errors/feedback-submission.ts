import { reportError } from "@/core/errors/report";
import type { UserFacingError } from "@/core/errors/types";
import { feedbackWasAccepted } from "@/lib/feedback-submission";

export interface FeedbackDraft {
  comments: string;
  email: string;
  status: "idle" | "sending" | "accepted" | "unavailable" | "failed";
}

export class FeedbackSubmission {
  private draft: FeedbackDraft = { comments: "", email: "", status: "idle" };
  private listeners = new Set<() => void>();

  constructor(readonly error: UserFacingError) {}

  getSnapshot = (): FeedbackDraft => this.draft;

  subscribe = (listener: () => void): (() => void) => {
    this.listeners.add(listener);
    return () => {
      this.listeners.delete(listener);
    };
  };

  private update(patch: Partial<FeedbackDraft>): void {
    this.draft = { ...this.draft, ...patch };
    for (const listener of this.listeners) listener();
  }

  setComments = (comments: string): void => {
    if (this.draft.status === "sending" || this.draft.status === "accepted")
      return;
    this.update({ comments });
  };

  setEmail = (email: string): void => {
    if (this.draft.status === "sending" || this.draft.status === "accepted")
      return;
    this.update({ email });
  };

  submit = async (): Promise<void> => {
    if (this.draft.status === "sending" || this.draft.status === "accepted")
      return;
    this.update({ status: "sending" });
    try {
      const outcome: unknown = await reportError(this.error, {
        feedback: {
          email: this.draft.email.trim() || undefined,
          comments:
            this.draft.comments.trim() || `(no comment) — ${this.error.title}`,
        },
        tags: { reported_by_user: "true" },
      });
      this.update({
        status: feedbackWasAccepted(outcome) ? "accepted" : "unavailable",
      });
    } catch {
      this.update({ status: "failed" });
    }
  };
}
