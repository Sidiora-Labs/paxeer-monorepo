SET LOCAL search_path = narrative_discovery, pg_catalog;
CREATE TABLE IF NOT EXISTS narrative_discovery.invitation_deliveries(squad_id text NOT NULL,account_id text NOT NULL,document jsonb NOT NULL,PRIMARY KEY(squad_id,account_id));
REVOKE ALL ON narrative_discovery.invitation_deliveries FROM PUBLIC;
CREATE TABLE IF NOT EXISTS narrative_discovery.evaluation_cursor(id boolean PRIMARY KEY CHECK(id),last_rule_id text NOT NULL DEFAULT '');
INSERT INTO narrative_discovery.evaluation_cursor(id) VALUES(true) ON CONFLICT DO NOTHING;
REVOKE ALL ON narrative_discovery.evaluation_cursor FROM PUBLIC;
