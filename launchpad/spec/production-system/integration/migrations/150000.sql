CREATE TABLE "stats"."reaction_pools"(pool text PRIMARY KEY,revision bigint NOT NULL CHECK(revision>0));
CREATE TABLE "stats"."reaction_wallets"(wallet text PRIMARY KEY,last_change timestamptz);
CREATE TABLE "stats"."reaction_votes"(pool text NOT NULL,wallet text NOT NULL,reaction text NOT NULL CHECK(reaction IN ('bullish','hot','diamond','bearish','trash','warning','clear')),revision bigint NOT NULL CHECK(revision>0),PRIMARY KEY(pool,wallet));
CREATE TABLE "stats"."reaction_nonces"(wallet text NOT NULL,nonce text NOT NULL,digest text NOT NULL,result jsonb NOT NULL,PRIMARY KEY(wallet,nonce));
CREATE TABLE "stats"."reaction_outbox"(id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,pool text NOT NULL,payload jsonb NOT NULL,created_at timestamptz NOT NULL DEFAULT clock_timestamp());