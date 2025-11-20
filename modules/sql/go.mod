module github.com/risor-io/risor/modules/sql

go 1.24.0

replace github.com/risor-io/risor => ../..

require (
	github.com/go-sql-driver/mysql v1.9.2
	github.com/lib/pq v1.10.9
	github.com/mattn/go-sqlite3 v1.14.28
	github.com/microsoft/go-mssqldb v1.8.1
	github.com/risor-io/risor v1.8.0
	github.com/xo/dburl v0.23.7
)

require (
	filippo.io/edwards25519 v1.1.0 // indirect
	github.com/golang-sql/civil v0.0.0-20220223132316-b832511892a9 // indirect
	github.com/golang-sql/sqlexp v0.1.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	golang.org/x/crypto v0.45.0 // indirect
	golang.org/x/text v0.31.0 // indirect
)
