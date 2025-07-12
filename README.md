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

	if err := migrator.Init(migrator.Options{
		FS: migrations.FS,
		Databases: map[string]migrator.Database{
			"db": database(),
		},
	}); err != nil {
		log.Fatal(err)
	}

	if err := migrator.Run(); err != nil {
		log.Fatal(err)
	}
}

func database() migrator.Database {
	// var (
	// 	pool      *pgxpool.Pool
	// 	adminPool *pgxpool.Pool
	// )
	return migrator.Database{
		Connect: func() error {
			// var err error
			// pool, err = pgxpool.New(ctx, config.App.Database.DSN)
			// return err
		},
		Close: func() error {
			// pool.Close()
			// return nil
		},
		AdminConnect: func() error {
			// var err error
			// adminPool, err = pgxpool.New(ctx, adminDSN)
			// return err
		},
		AdminClose: func() error {
			// adminPool.Close()
			// return nil
		},
		ExecCreateVersionsTable: func(versionsTable string) error {
			// query := `CREATE TABLE IF NOT EXISTS %s (
			// 	version BIGINT PRIMARY KEY,
			// 	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
			// )`
			// query = fmt.Sprintf(query, versionsTable)
			//
			// _, err := pool.Exec(ctx, query)
			// return err
		},
		ExecIsVersionExists: func(versionsTable string, version int64) (bool, error) {
			// query := fmt.Sprintf("SELECT 1 FROM %s WHERE version = %d", versionsTable, version)
			// row := pool.QueryRow(ctx, query)
			// var exists int
			// if err := row.Scan(&exists); err != nil {
			// 	if errors.Is(err, pgx.ErrNoRows) {
			// 		return false, nil
			// 	}
			// 	return false, err
			// }
			// return true, nil
		},
		ExecQuery: func(queries string, options migrator.ExecQueryOptions) error {
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
			// 	_, err := pool.Exec(ctx, strings.TrimSpace(queries))
			// 	return err
			// }
			//
			// queryList := strings.Split(queries, ";")
			// for _, q := range queryList {
			// 	if _, err := pool.Exec(ctx, strings.TrimSpace(q)); err != nil {
			// 	    return err
			//  }
			// }
			// return nil
		},
		ExecCreateDB: func() error {
			// _, err := adminPool.Exec(ctx, fmt.Sprintf("CREATE DATABASE %s", config.App.Database.Name))
			// return err
		},
		ExecDropDB: func() error {
			// _, err := adminPool.Exec(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS %s", config.App.Database.Name))
			// return err
		},
	}
}
```
