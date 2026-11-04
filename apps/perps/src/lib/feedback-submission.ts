export interface FeedbackCaptureAccepted {
  status: "accepted";
  eventId: string;
  feedbackEventId: string;
}

export function feedbackWasAccepted(
  outcome: unknown,
): outcome is FeedbackCaptureAccepted {
  return (
    typeof outcome === "object" &&
    outcome !== null &&
    "status" in outcome &&
    outcome.status === "accepted" &&
    "eventId" in outcome &&
    typeof outcome.eventId === "string" &&
    outcome.eventId.trim().length > 0 &&
    "feedbackEventId" in outcome &&
    typeof outcome.feedbackEventId === "string" &&
    outcome.feedbackEventId.trim().length > 0
  );
}
