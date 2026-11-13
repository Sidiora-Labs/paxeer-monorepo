-- In-memory store using the generator-supported Codify schema-14 contract.
-- Scoped Git paths are deliberately unindexed, an ordinary supported inventory state.
CREATE TABLE meta(key TEXT PRIMARY KEY, value TEXT);
INSERT INTO meta VALUES ('schema_version', '14');
INSERT INTO meta VALUES ('last_index_ms', '0');
CREATE TABLE files(  id INTEGER PRIMARY KEY, branch_id INTEGER NOT NULL DEFAULT 0,  path TEXT NOT NULL, lang TEXT,  size INTEGER, mtime INTEGER, mtime_ns INTEGER, hash TEXT, lines INTEGER,  base TEXT,  UNIQUE(branch_id, path));
CREATE TABLE symbols(  id INTEGER PRIMARY KEY, file_id INTEGER NOT NULL REFERENCES files(id),  name TEXT NOT NULL, kind TEXT, line INTEGER, end_line INTEGER, sig TEXT,  decl INTEGER NOT NULL DEFAULT 0);
CREATE TABLE refs(  id INTEGER PRIMARY KEY, file_id INTEGER NOT NULL,  name TEXT NOT NULL, line INTEGER, sym_id INTEGER,  qual TEXT, kind TEXT NOT NULL DEFAULT 'call',  target_id INTEGER DEFAULT NULL,  verdict TEXT DEFAULT NULL,  conf TEXT DEFAULT NULL,  argc INTEGER DEFAULT -1);
CREATE TABLE imports(  id INTEGER PRIMARY KEY, file_id INTEGER NOT NULL,  name TEXT NOT NULL, module TEXT NOT NULL, line INTEGER,  system INTEGER NOT NULL DEFAULT 0,  target_file_id INTEGER DEFAULT NULL,  origin TEXT DEFAULT NULL);
CREATE TABLE routes(  id INTEGER PRIMARY KEY, file_id INTEGER NOT NULL, framework TEXT,  method TEXT, pattern TEXT, handler TEXT, line INTEGER);
CREATE TABLE git_churn(  path TEXT NOT NULL, hash TEXT NOT NULL, PRIMARY KEY(path,hash));
