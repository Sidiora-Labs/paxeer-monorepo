// Package repairdb connects database/sql to the real embedded SQLite engine for
// the service-free sink regression tests. It is not used by the PostgreSQL sink.
package repairdb

/*
#cgo pkg-config: sqlite3
#include <sqlite3.h>
#include <stdlib.h>

static int bind_text(sqlite3_stmt *s, int i, const char *p, int n) {
  return sqlite3_bind_text(s, i, p, n, SQLITE_TRANSIENT);
}
static int bind_blob(sqlite3_stmt *s, int i, const void *p, int n) {
  return sqlite3_bind_blob(s, i, p, n, SQLITE_TRANSIENT);
}
*/
import "C"

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"
	"unsafe"
)

const Name = "repair82-sqlite"

func init() { sql.Register(Name, sqliteDriver{}) }

type sqliteDriver struct{}
type connection struct{ db *C.sqlite3 }
type statement struct {
	conn *connection
	stmt *C.sqlite3_stmt
}
type transaction struct{ conn *connection }
type rows struct{ stmt *statement }
type result struct{ id, count int64 }

// SQLite uses integer primary keys for sequences, and reserves the PostgreSQL
// column name "index". All other DDL, constraints, views and sink SQL run as-is.
var indexIdentifier = regexp.MustCompile(`\bindex\b`)

func Dialect(query string) string {
	query = strings.ReplaceAll(query, "BIGSERIAL PRIMARY KEY", "INTEGER PRIMARY KEY AUTOINCREMENT")
	return indexIdentifier.ReplaceAllString(query, `"index"`)
}

func (sqliteDriver) Open(name string) (driver.Conn, error) {
	path := C.CString(name)
	defer C.free(unsafe.Pointer(path))
	c := &connection{}
	if rc := C.sqlite3_open(path, &c.db); rc != C.SQLITE_OK {
		err := c.err(rc)
		if c.db != nil {
			C.sqlite3_close(c.db)
		}
		return nil, err
	}
	if err := c.exec("PRAGMA foreign_keys = ON"); err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}

func (c *connection) err(rc C.int) error {
	return fmt.Errorf("sqlite code %d: %s", int(rc), C.GoString(C.sqlite3_errmsg(c.db)))
}

func (c *connection) exec(query string) error {
	q := C.CString(query)
	defer C.free(unsafe.Pointer(q))
	if rc := C.sqlite3_exec(c.db, q, nil, nil, nil); rc != C.SQLITE_OK {
		return c.err(rc)
	}
	return nil
}

func (c *connection) Prepare(query string) (driver.Stmt, error) {
	q := C.CString(Dialect(query))
	defer C.free(unsafe.Pointer(q))
	s := &statement{conn: c}
	if rc := C.sqlite3_prepare_v2(c.db, q, -1, &s.stmt, nil); rc != C.SQLITE_OK {
		return nil, c.err(rc)
	}
	if s.stmt == nil {
		return nil, fmt.Errorf("sqlite statement contains no executable SQL")
	}
	return s, nil
}

func (c *connection) Exec(query string, args []driver.Value) (driver.Result, error) {
	if len(args) == 0 {
		if err := c.exec(Dialect(query)); err != nil {
			return nil, err
		}
		return result{id: int64(C.sqlite3_last_insert_rowid(c.db)), count: int64(C.sqlite3_changes(c.db))}, nil
	}
	s, err := c.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer s.Close()
	return s.Exec(args)
}

func (c *connection) Close() error {
	if rc := C.sqlite3_close(c.db); rc != C.SQLITE_OK {
		return c.err(rc)
	}
	return nil
}

func (c *connection) Begin() (driver.Tx, error) {
	if err := c.exec("BEGIN"); err != nil {
		return nil, err
	}
	return &transaction{conn: c}, nil
}

func (tx *transaction) Commit() error   { return tx.conn.exec("COMMIT") }
func (tx *transaction) Rollback() error { return tx.conn.exec("ROLLBACK") }

func (s *statement) Close() error {
	if s.stmt == nil {
		return nil
	}
	rc := C.sqlite3_finalize(s.stmt)
	s.stmt = nil
	if rc != C.SQLITE_OK {
		return s.conn.err(rc)
	}
	return nil
}

func (s *statement) NumInput() int { return int(C.sqlite3_bind_parameter_count(s.stmt)) }

func (s *statement) bind(args []driver.Value) error {
	C.sqlite3_reset(s.stmt)
	C.sqlite3_clear_bindings(s.stmt)
	for i, arg := range args {
		var rc C.int
		pos := C.int(i + 1)
		switch v := arg.(type) {
		case nil:
			rc = C.sqlite3_bind_null(s.stmt, pos)
		case int64:
			rc = C.sqlite3_bind_int64(s.stmt, pos, C.sqlite3_int64(v))
		case float64:
			rc = C.sqlite3_bind_double(s.stmt, pos, C.double(v))
		case bool:
			var n C.int
			if v {
				n = 1
			}
			rc = C.sqlite3_bind_int(s.stmt, pos, n)
		case string:
			p := C.CString(v)
			rc = C.bind_text(s.stmt, pos, p, C.int(len(v)))
			C.free(unsafe.Pointer(p))
		case []byte:
			if v == nil {
				rc = C.sqlite3_bind_null(s.stmt, pos)
			} else if len(v) == 0 {
				rc = C.sqlite3_bind_zeroblob(s.stmt, pos, 0)
			} else {
				p := C.CBytes(v)
				rc = C.bind_blob(s.stmt, pos, p, C.int(len(v)))
				C.free(p)
			}
		case time.Time:
			value := v.Format(time.RFC3339Nano)
			p := C.CString(value)
			rc = C.bind_text(s.stmt, pos, p, C.int(len(value)))
			C.free(unsafe.Pointer(p))
		default:
			return fmt.Errorf("unsupported sqlite parameter %T", arg)
		}
		if rc != C.SQLITE_OK {
			return s.conn.err(rc)
		}
	}
	return nil
}

func (s *statement) Exec(args []driver.Value) (driver.Result, error) {
	if err := s.bind(args); err != nil {
		return nil, err
	}
	if rc := C.sqlite3_step(s.stmt); rc != C.SQLITE_DONE {
		return nil, s.conn.err(rc)
	}
	return result{id: int64(C.sqlite3_last_insert_rowid(s.conn.db)), count: int64(C.sqlite3_changes(s.conn.db))}, nil
}

func (s *statement) Query(args []driver.Value) (driver.Rows, error) {
	if err := s.bind(args); err != nil {
		return nil, err
	}
	return &rows{stmt: s}, nil
}

func (r result) LastInsertId() (int64, error) { return r.id, nil }
func (r result) RowsAffected() (int64, error) { return r.count, nil }

func (r *rows) Columns() []string {
	n := int(C.sqlite3_column_count(r.stmt.stmt))
	names := make([]string, n)
	for i := range names {
		names[i] = C.GoString(C.sqlite3_column_name(r.stmt.stmt, C.int(i)))
	}
	return names
}

func (r *rows) Close() error {
	if rc := C.sqlite3_reset(r.stmt.stmt); rc != C.SQLITE_OK {
		return r.stmt.conn.err(rc)
	}
	return nil
}

func (r *rows) Next(values []driver.Value) error {
	switch rc := C.sqlite3_step(r.stmt.stmt); rc {
	case C.SQLITE_DONE:
		return io.EOF
	case C.SQLITE_ROW:
		for i := range values {
			col := C.int(i)
			switch C.sqlite3_column_type(r.stmt.stmt, col) {
			case C.SQLITE_NULL:
				values[i] = nil
			case C.SQLITE_INTEGER:
				values[i] = int64(C.sqlite3_column_int64(r.stmt.stmt, col))
			case C.SQLITE_FLOAT:
				values[i] = float64(C.sqlite3_column_double(r.stmt.stmt, col))
			case C.SQLITE_TEXT:
				p := C.sqlite3_column_text(r.stmt.stmt, col)
				values[i] = C.GoStringN((*C.char)(unsafe.Pointer(p)), C.sqlite3_column_bytes(r.stmt.stmt, col))
			case C.SQLITE_BLOB:
				values[i] = C.GoBytes(C.sqlite3_column_blob(r.stmt.stmt, col), C.sqlite3_column_bytes(r.stmt.stmt, col))
			}
		}
		return nil
	default:
		return r.stmt.conn.err(rc)
	}
}
