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
