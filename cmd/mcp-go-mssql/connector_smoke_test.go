package main

import (
	"database/sql"
	"database/sql/driver"
	"testing"

	"github.com/golang-sql/sqlexp"
	mssql "github.com/microsoft/go-mssqldb"
)

// TestConnectorAPICompiles is a compile-time assertion that the
// go-mssqldb v1.10.0 + sqlexp v0.1.0 APIs the connection refactor
// depends on still exist. If a future driver version removes or
// renames them, this test breaks at compile time before the
// production code does.
func TestConnectorAPICompiles(t *testing.T) {
	// mssql.NewConnector signature
	var _ func(string) (*mssql.Connector, error) = mssql.NewConnector
	// sql.OpenDB takes a driver.Connector; confirm mssql.Connector
	// satisfies the interface by assigning nil to it.
	var _ driverConnector = (*mssql.Connector)(nil)
	// sqlexp.ReturnMessage + helpers
	var rm *sqlexp.ReturnMessage = &sqlexp.ReturnMessage{}
	sqlexp.ReturnMessageInit(rm)
	var _ sqlexp.MsgNotice
	var _ sqlexp.MsgNextResultSet
}

// driverConnector mirrors database/sql/driver.Connector (not directly
// exported by the standard library; aliased here as a named type so
// the test can assert the mssql.Connector satisfies it without
// importing driver.Connector by its long name).
type driverConnector = driver.Connector

// TestConnectorBuildDoesNotPanic constructs an mssql.Connector from a
// syntactically valid DSN and opens a *sql.DB against it, then closes
// it without ever calling PingContext. The point is to verify that the
// refactor from sql.Open("sqlserver", dsn) to
// sql.OpenDB(mssql.NewConnector(dsn)) does not break DSN parsing for
// the URL-form connection string used by legacy SQL Server 2008/2012
// (the documented workaround for TLS 1.0).
func TestConnectorBuildDoesNotPanic(t *testing.T) {
	// ADO-form DSN (the default used by the classic config path).
	dsnADO := "server=localhost;user id=sa;password=;database=master"
	c, err := mssql.NewConnector(dsnADO)
	if err != nil {
		t.Fatalf("mssql.NewConnector(ADO DSN) returned error: %v", err)
	}
	db := sql.OpenDB(c)
	if db == nil {
		t.Fatal("sql.OpenDB returned nil *sql.DB")
	}
	_ = db.Close()

	// URL-form DSN (legacy SQL Server 2008/2012 workaround documented
	// in the env vars section of the project README).
	dsnURL := "sqlserver://sa:@localhost:1433?database=master&encrypt=disable&trustservercertificate=true"
	c2, err := mssql.NewConnector(dsnURL)
	if err != nil {
		t.Fatalf("mssql.NewConnector(URL DSN) returned error: %v", err)
	}
	db2 := sql.OpenDB(c2)
	if db2 == nil {
		t.Fatal("sql.OpenDB returned nil *sql.DB")
	}
	_ = db2.Close()
}

// TestMessageSinkInit verifies that the messageSink constructor wires
// the sqlexp.ReturnMessage correctly. It does not require a live DB.
func TestMessageSinkInit(t *testing.T) {
	s := newMessageSink()
	if s == nil {
		t.Fatal("newMessageSink returned nil")
	}
	if s.rm == nil {
		t.Fatal("messageSink.rm is nil")
	}
	if s.done == nil {
		t.Fatal("messageSink.done is nil")
	}
	if s.ctxCancel != nil {
		t.Fatal("messageSink.ctxCancel should be nil before start()")
	}
	if lines := s.linesSnapshot(); lines == nil {
		t.Fatal("linesSnapshot() returned nil; expected empty slice")
	} else if len(lines) != 0 {
		t.Errorf("linesSnapshot() returned %d entries; want 0", len(lines))
	}
}
