CREATE SCHEMA IF NOT EXISTS "pnl";
 CREATE TABLE IF NOT EXISTS "pnl".objects(kind text NOT NULL,id text NOT NULL,body jsonb NOT NULL,PRIMARY KEY(kind,id));
 CREATE TABLE IF NOT EXISTS "pnl".audit(seq bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,at timestamptz NOT NULL DEFAULT clock_timestamp(),actor text NOT NULL,role text NOT NULL,operation text NOT NULL,resource text NOT NULL,reason text NOT NULL,result text NOT NULL);
 CREATE FUNCTION "pnl".immutable_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'audit is append only'; END $$;
 CREATE TRIGGER immutable_audit BEFORE UPDATE OR DELETE OR TRUNCATE ON "pnl".audit FOR EACH STATEMENT EXECUTE FUNCTION "pnl".immutable_audit();