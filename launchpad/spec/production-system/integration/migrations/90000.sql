SET LOCAL search_path = "api_limits", pg_catalog;
CREATE SCHEMA IF NOT EXISTS api_limits;
CREATE TABLE api_limits.cache_query_budgets_v2 (
 namespace text NOT NULL,
 origin text NOT NULL,
 window_bucket bigint NOT NULL,
 window_ms bigint NOT NULL CHECK (window_ms >= 1 AND window_ms <= 86400000),
 budget_limit bigint NOT NULL CHECK (budget_limit >= 1 AND budget_limit <= 1000000000),
 used bigint NOT NULL CHECK (used > 0 AND used <= budget_limit),
 PRIMARY KEY(namespace,origin,window_bucket,window_ms)
);
REVOKE ALL ON "api_limits"."cache_query_budgets_v2" FROM PUBLIC;
