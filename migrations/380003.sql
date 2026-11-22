SET LOCAL search_path = "chat", pg_catalog;
CREATE VIEW chat.pool_search_visibility_v2 AS
 SELECT p.pool,p.token,
 COALESCE(vp.status,'restored')<>'removed' AND COALESCE(vt.status,'restored')<>'removed' AS public,
 COALESCE(vp.revision,0)+COALESCE(vt.revision,0) AS revision
 FROM indexer.indexer_canonical_creations_v1 p
 LEFT JOIN chat.safety_visibility vp ON vp.target_type='pool' AND vp.target_id=p.pool
 LEFT JOIN chat.safety_visibility vt ON vt.target_type='token' AND vt.target_id=p.token;
REVOKE ALL ON chat.pool_search_visibility_v2 FROM PUBLIC;
