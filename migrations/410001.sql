SET LOCAL search_path = "gateway", pg_catalog;
ALTER TABLE "gateway"."restrictions"
  ADD COLUMN "initial_visibility" boolean NOT NULL DEFAULT false
  CHECK (NOT initial_visibility OR (revision=1 AND state='approved'));
CREATE TABLE "gateway"."inventory_cursors"(
  bucket text NOT NULL,
  prefix text NOT NULL,
  continuation text NOT NULL,
  PRIMARY KEY(bucket,prefix)
);
REVOKE ALL ON "gateway"."inventory_cursors" FROM PUBLIC;
