
CREATE SCHEMA IF NOT EXISTS candles;
CREATE TABLE candles.chart_pool_fences(chain_id text NOT NULL,pool text NOT NULL,epoch bigint NOT NULL CHECK(epoch>0),frozen boolean NOT NULL DEFAULT false,PRIMARY KEY(chain_id,pool));
CREATE TABLE candles.chart_inbox(chain_id text NOT NULL,emitter text NOT NULL,code_epoch text NOT NULL,tx_hash text NOT NULL,log_index numeric(20,0) NOT NULL,pool text NOT NULL,digest bytea NOT NULL,fill jsonb NOT NULL,algorithm text NOT NULL,PRIMARY KEY(chain_id,emitter,code_epoch,tx_hash,log_index));
CREATE TABLE candles.chart_exact(chain_id text NOT NULL,pool text NOT NULL,interval text NOT NULL,start bigint NOT NULL,snapshot jsonb NOT NULL,PRIMARY KEY(chain_id,pool,interval,start));
CREATE TABLE candles.chart_traders(chain_id text NOT NULL,pool text NOT NULL,interval text NOT NULL,start bigint NOT NULL,actor text NOT NULL,PRIMARY KEY(chain_id,pool,interval,start,actor));
CREATE TABLE candles.chart_outbox(id bigserial PRIMARY KEY,chain_id text NOT NULL,pool text NOT NULL,interval text NOT NULL,start bigint NOT NULL,revision numeric(20,0) NOT NULL,payload jsonb NOT NULL,published_at timestamptz,UNIQUE(chain_id,pool,interval,start,revision));
CREATE INDEX chart_outbox_pending ON candles.chart_outbox(id) WHERE published_at IS NULL;

CREATE VIEW "candles"."chart_exact_v1" AS SELECT chain_id,pool,interval,start,snapshot FROM "candles"."chart_exact";

CREATE VIEW "candles"."chart_pool_fences_v1" AS SELECT chain_id,pool,frozen FROM "candles"."chart_pool_fences";
