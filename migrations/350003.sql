SET LOCAL search_path = users, pg_catalog;
CREATE VIEW users.account_profiles_v1 WITH (security_barrier=true) AS
 SELECT wallet,profile,deleted,revision FROM users.account_state;
REVOKE ALL ON users.account_profiles_v1 FROM PUBLIC;
