-- Compatibility check for the 828 fixtures, executed after store.Migrate.
-- The sole table definitions live in migrations/0016_spot_board_omissions.sql.
-- Resolve every required column without creating tables or changing board data.
SELECT account, updated_block FROM perps.spot_board_omissions WHERE false;
SELECT singleton, generated_at, omitted_accounts FROM perps.spot_board_generation WHERE false;
