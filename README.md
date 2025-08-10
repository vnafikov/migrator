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
migrations/postgres/&ast;.sql\
migrations/postgres/seeds/&ast;.sql\
migrations/migrator.go:
```go
package migrations

import "embed"

//go:embed postgres/*.sql
//go:embed postgres/seeds/*.sql
var FS embed.FS
```

cmd/appmigrator/main.go:
```go
package main

// import ...

func main() {
	// ...

	options := migrator.Options{
		FS:        migrations.FS,
		Databases: map[string]migrator.Database{
			"postgres": new(postgres),
		},
	}
	if err := migrator.Init(options); err != nil {
		log.Fatal(err)
	}

	if err := migrator.Run(ctx); err != nil {
		log.Fatal(err)
	}
}

type postgres struct {
	// pool      *pgxpool.Pool
	// adminPool *pgxpool.Pool
}

func (pg *postgres) Connect(ctx context.Context) error {
	// var err error
	// pg.pool, err = pgxpool.New(ctx, config.App.Postgres.DSN)
	// return err
}

func (pg *postgres) Close(ctx context.Context) error {
	// pg.pool.Close()
	// return nil
}

func (pg *postgres) AdminConnect(ctx context.Context) error {
	// var err error
	// pg.adminPool, err = pgxpool.New(ctx, adminDSN)
	// return err
}

func (pg *postgres) AdminClose(ctx context.Context) error {
	// pg.adminPool.Close()
	// return nil
}

func (pg *postgres) ExecCreateVersionsTable(ctx context.Context, versionsTable string) error {
	// query := `CREATE TABLE IF NOT EXISTS %s (
	// version BIGINT PRIMARY KEY,
	// created_at TIMESTAMPTZ NOT NULL DEFAULT now()
	// )`
	// query = fmt.Sprintf(query, versionsTable)
	//
	// _, err := pg.pool.Exec(ctx, query)
	// return err
}

func (pg *postgres) ExecIsVersionExists(ctx context.Context, versionsTable string, version int64) (bool, error) {
	// query := fmt.Sprintf("SELECT TRUE FROM %s WHERE version = %d", versionsTable, version)
	//
	// row := pg.pool.QueryRow(ctx, query)
	// var exists bool
	// if err := row.Scan(&exists); err != nil {
	// 	if errors.Is(err, pgx.ErrNoRows) {
	// 		return false, nil
	// 	}
	// 	return false, err
	// }
	// return true, nil
}

func (pg *postgres) ExecQuery(ctx context.Context, queries string, options migrator.ExecQueryOptions) error {
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
	// 	_, err := pg.pool.Exec(ctx, queries)
	// 	return err
	// }
	//
	// queryList := strings.Split(queries, ";")
	// for _, query := range queryList {
	// 	if _, err := pg.pool.Exec(ctx, strings.TrimSpace(query)); err != nil {
	// 		return err
	// 	}
	// }
	// return nil
}

func (pg *postgres) ExecCreateDB(ctx context.Context) error {
	// query := fmt.Sprintf("CREATE DATABASE %s", config.App.Postgres.Database)
	//
	// _, err := pg.adminPool.Exec(ctx, query)
	// return err
}

func (pg *postgres) ExecDropDB(ctx context.Context) error {
	// query := fmt.Sprintf("DROP DATABASE IF EXISTS %s", config.App.Postgres.Database)
	//
	// _, err := pg.adminPool.Exec(ctx, query)
	// return err
}
```
