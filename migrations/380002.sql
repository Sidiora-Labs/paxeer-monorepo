-- media/social owner interface for media/livestream: a column-scoped versioned view over the moderation
-- bans (380000) that gate stream listings, viewer admission and broadcaster actions (streams.RestrictionReader).
-- livestream_runtime reads it through its grants.json runtime grant; it never reads chat base tables.
SET LOCAL search_path = "chat", pg_catalog;
CREATE VIEW chat.safety_bans_v1 AS SELECT wallet,scope,resource,expires_at,revoked_at FROM chat.safety_bans;
REVOKE ALL ON chat.safety_bans_v1 FROM PUBLIC;
