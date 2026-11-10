SET LOCAL search_path = "ranking", pg_catalog;
ALTER TABLE ranking.snapshots
 DROP CONSTRAINT snapshots_category_check,
 ADD CONSTRAINT snapshots_category_check
 CHECK(category IN ('trending','breakout','volume','gainers','losers','movers','unusual','new'));
