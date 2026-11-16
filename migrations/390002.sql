SET LOCAL search_path = metadata, pg_catalog;
CREATE VIEW metadata.token_search_v1 AS
 SELECT token_address,pool_address,name,symbol,description,created_by,created_at FROM metadata.token_metadata;
REVOKE ALL ON metadata.token_search_v1 FROM PUBLIC;
