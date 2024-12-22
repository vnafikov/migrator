# Migrator

The Go package.

Performs database migrations.

### Help:
```bash
migrator -h
```

### Migration filename format:
```
{datetime}_{title}.up.sql
{datetime}_{title}.down.sql
```

### Creating files (shell):
Migration:
```bash
d=`date +%Y%m%d%H%M%S`
:> ${d}_create_tablename.up.sql
:> ${d}_create_tablename.down.sql
```

Seed:
```bash
:> `date +%Y%m%d%H%M%S`_seed_tablename.sql
```

### Disabling transaction:
At the beginning of the *.sql file:
```sql
-- NO TRANSACTION
```

### Example in the project:
migrations/db/&ast;.sql\
migrations/db/seeds/&ast;.sql\
migrations/migrator.go:
```go
package migrations

import "embed"

//go:embed db/*.sql
//go:embed db/seeds/*.sql
var FS embed.FS
```

cmd/appmigrator/main.go:
```go
package main

// import ...

func main() {
	// ...

	migrator.Init(migrator.Options{
		FS: migrations.FS,
		Databases: map[string]*migrator.Database{
			"db": database(),
		},
	})
	migrator.Run()
}

func database() *migrator.Database {
	// var (
	// 	pool      *pgxpool.Pool
	// 	adminPool *pgxpool.Pool
	// )
	return &migrator.Database{
		Connect: func() {
			// var err error
			// if pool, err = pgxpool.New(ctx, config.App.Database.DSN); err != nil {
			// 	log.Fatal(err)
			// }
		},
		Close: func() {
			// pool.Close()
		},
		AdminConnect: func() {
			// var err error
			// if adminPool, err = pgxpool.New(ctx, adminDSN); err != nil {
			// 	log.Fatal(err)
			// }
		},
		AdminClose: func() {
			// adminPool.Close()
		},
		ExecCreateVersionsTable: func(versionsTable string) {
			// query := `CREATE TABLE IF NOT EXISTS %s (
			// 	version BIGINT PRIMARY KEY,
			// 	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
			// )`
			// query = fmt.Sprintf(query, versionsTable)
			//
			// if _, err := pool.Exec(ctx, query); err != nil {
			// 	log.Fatal(err)
			// }
		},
		ExecIsVersionExists: func(versionsTable string, version int) bool {
			// query := fmt.Sprintf("SELECT 1 FROM %s WHERE version = %d", versionsTable, version)
			// row := pool.QueryRow(ctx, query)
			// var exists int
			// if err := row.Scan(&exists); err != nil {
			// 	if errors.Is(err, pgx.ErrNoRows) {
			// 		return false
			// 	}
			//
			// 	log.Fatal(err)
			// }
			// return true
		},
		ExecQuery: func(queries string, options migrator.ExecQueryOptions) {
			// var updateVersionQuery string
			// if options.IsDown {
			// 	updateVersionQuery = `
			// DELETE FROM %s WHERE version = %d`
			// } else {
			// 	updateVersionQuery = `
			// INSERT INTO %s (version) VALUES (%d)`
			// }
			// queries += fmt.Sprintf(updateVersionQuery, options.VersionsTable, options.Version)
			//
			// if options.InTransaction {
			// 	if _, err := pool.Exec(ctx, strings.TrimSpace(queries)); err != nil {
			// 		log.Fatal(err)
			// 	}
			// 	return
			// }
			//
			// queryList := strings.Split(queries, ";")
			// for _, q := range queryList {
			// 	if _, err := pool.Exec(ctx, strings.TrimSpace(q)); err != nil {
			// 		log.Fatal(err)
			// 	}
			// }
		},
		ExecCreateDB: func() {
			// _, err := adminPool.Exec(ctx, fmt.Sprintf("CREATE DATABASE %s", config.App.Database.Name))
			// if err != nil {
			// 	log.Fatal(err)
			// }
		},
		ExecDropDB: func() {
			// _, err := adminPool.Exec(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS %s", config.App.Database.Name))
			// if err != nil {
			// 	log.Fatal(err)
			// }
		},
	}
}
```
