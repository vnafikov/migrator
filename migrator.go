package migrator

import (
	"cmp"
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"slices"
	"strconv"
	"strings"
)

const (
	flagDatabases = "databases"
	flagMigrate   = "migrate"
	flagSeed      = "seed"
	flagCreateDB  = "createdb"
	flagDropDB    = "dropdb"
	flagRe        = "re"
	flagUp        = "up"
	flagDown      = "down"
	flagIrr       = "irr"

	versionLength        = 14
	upMarker             = ".up."
	downMarker           = ".down."
	sqlSuffix            = ".sql"
	noTransactionComment = "-- NO TRANSACTION"
	seedsSubdir          = "seeds"

	schemaMigrations = "schema_migrations"
	schemaSeeds      = "schema_seeds"

	openFileErrorFormat = `cannot open file "%s/%s": %w`
)

var (
	ErrDir              = errors.New("entry is a directory")
	ErrInvalidExtension = fmt.Errorf("invalid file extension: expected %q", sqlSuffix)
	ErrFilenameTooShort = fmt.Errorf("filename is too short: timestamp must be %d digits", versionLength)

	ErrNoMarker = fmt.Errorf("no marker found: expected %q or %q", upMarker, downMarker)

	ErrDuplicateVersion = errors.New("duplicate version detected")

	options Options
	flags   = flagsSt{
		bool:   make(map[string]*bool),
		string: make(map[string]*string),
	}
	databases []string
	files     = make(map[string]*filesSt)
)

type FS interface {
	fs.ReadDirFS
	fs.ReadFileFS
}

type Database interface {
	Connect(ctx context.Context) error
	Close(ctx context.Context) error
	AdminConnect(ctx context.Context) error
	AdminClose(ctx context.Context) error
	ExecCreateVersionsTable(ctx context.Context, versionsTable string) error
	ExecIsVersionExists(ctx context.Context, versionsTable string, version int64) (bool, error)
	ExecQuery(ctx context.Context, queries string, options ExecQueryOptions) error
	ExecCreateDB(ctx context.Context) error
	ExecDropDB(ctx context.Context) error
}

type Options struct {
	FS        FS
	Databases map[string]Database
}

type ExecQueryOptions struct {
	IsDown        bool
	VersionsTable string
	Version       int64
	InTransaction bool
}

type flagsSt struct {
	bool   map[string]*bool
	string map[string]*string
	any    bool
}

type filesSt struct {
	migrations struct {
		down []migration
		up   []migration
	}
	seeds []migration
}

type migration struct {
	version  int64
	filepath string
	isDown   bool
}

func Init(opt Options) error {
	options = opt
	if err := validateDatabases(options.Databases); err != nil {
		return err
	}

	optionDatabaseNames := mapKeys(options.Databases)
	dbFlag := flag.String(flagDatabases, "", fmt.Sprintf(`options: %s. All by default.`, strings.Join(optionDatabaseNames, ", ")))
	flags.bool[flagMigrate] = flag.Bool(flagMigrate, false, "run migrations.")
	flags.bool[flagSeed] = flag.Bool(flagSeed, false, "seed the database.")
	flags.bool[flagCreateDB] = flag.Bool(flagCreateDB, false, "create the database.")
	flags.bool[flagDropDB] = flag.Bool(flagDropDB, false, "drop the database.")
	flags.bool[flagRe] = flag.Bool(flagRe, false, "replay migrations: reset the database and run migrations.")
	flags.string[flagUp] = flag.String(flagUp, "", "run migration by version (datetime).")
	flags.string[flagDown] = flag.String(flagDown, "", "rollback migration by version (datetime).")
	flags.bool[flagIrr] = flag.Bool(flagIrr, false, "list of irreversible migrations (without *.down.sql files).")

	usage := flag.Usage
	flag.Usage = func() {
		_, _ = fmt.Print(`Performs database migrations.

Runs migrations and seeding by default.
The flags can be combined.

`)
		usage()
	}

	flag.Parse()
	setDatabases(optionDatabaseNames, dbFlag)
	setFlagsAny()
	return readFilenames()
}

func validateDatabases(databases map[string]Database) error {
	for databaseName, database := range databases {
		if database == nil {
			return fmt.Errorf("database %q is nil", databaseName)
		}
	}
	return nil
}

func mapKeys[M ~map[K]V, K comparable, V any](m M) []K {
	r := make([]K, 0, len(m))
	for k := range m {
		r = append(r, k)
	}
	return r
}

func setDatabases(optionDatabaseNames []string, dbFlag *string) {
	if *dbFlag == "" {
		databases = optionDatabaseNames
		return
	}

	databaseNames := strings.Split(*dbFlag, ",")
	for i := range databaseNames {
		databaseNames[i] = strings.TrimSpace(databaseNames[i])
	}
	databases = slicesIntersection(optionDatabaseNames, databaseNames)
}

func slicesIntersection[T comparable](a, b []T) []T {
	m := make(map[T]struct{}, len(a))
	for i := range a {
		m[a[i]] = struct{}{}
	}
	intersection := make([]T, 0, len(b))
	for i := range b {
		if _, ok := m[b[i]]; ok {
			intersection = append(intersection, b[i])
		}
	}
	return intersection
}

func setFlagsAny() {
	for _, value := range flags.bool {
		if *value {
			flags.any = true
			return
		}
	}

	for _, value := range flags.string {
		if *value != "" {
			flags.any = true
			return
		}
	}
}

func readFilenames() error {
	for _, database := range databases {
		files[database] = new(filesSt)
		if err := readMigrationFilenames(database); err != nil {
			return err
		}

		if err := readSeedFilenames(database); err != nil {
			return err
		}

		if err := validateVersions(database); err != nil {
			return err
		}
	}
	return nil
}

func readMigrationFilenames(database string) error {
	entries, err := readDir(database)
	if err != nil {
		return err
	}

	sortEntries(entries)
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() && name == seedsSubdir {
			continue
		}

		m, err := readMigration(entry, database)
		if err != nil {
			return err
		}

		switch {
		case strings.Contains(name, downMarker):
			m.isDown = true
			files[database].migrations.down = append(files[database].migrations.down, m)
		case strings.Contains(name, upMarker):
			files[database].migrations.up = append(files[database].migrations.up, m)
		default:
			return fmt.Errorf(openFileErrorFormat, database, name, ErrNoMarker)
		}
	}
	return nil
}

func readSeedFilenames(database string) error {
	path := fmt.Sprintf("%s/%s", database, seedsSubdir)
	entries, err := readDir(path)
	if err != nil {
		return err
	}

	sortEntries(entries)
	for _, entry := range entries {
		s, err := readMigration(entry, path)
		if err != nil {
			return err
		}

		files[database].seeds = append(files[database].seeds, s)
	}
	return nil
}

func readDir(path string) ([]fs.DirEntry, error) {
	entries, err := options.FS.ReadDir(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("cannot read directory %q: %w", path, err)
	}
	return entries, nil
}

func sortEntries(entries []fs.DirEntry) {
	slices.SortStableFunc(entries, func(a, b fs.DirEntry) int {
		return cmp.Compare(a.Name(), b.Name())
	})
}

func readMigration(entry fs.DirEntry, path string) (migration, error) {
	var m migration
	name := entry.Name()
	if entry.IsDir() {
		return m, fmt.Errorf(openFileErrorFormat, path, name, ErrDir)
	}
	if !strings.HasSuffix(name, sqlSuffix) {
		return m, fmt.Errorf(openFileErrorFormat, path, name, ErrInvalidExtension)
	}
	if len(name) < versionLength+len(sqlSuffix) {
		return m, fmt.Errorf(openFileErrorFormat, path, name, ErrFilenameTooShort)
	}

	var err error
	if m.version, err = strconv.ParseInt(name[:versionLength], 10, 64); err != nil {
		return m, fmt.Errorf(openFileErrorFormat, path, name, err)
	}

	m.filepath = fmt.Sprintf("%s/%s", path, name)
	return m, nil
}

func validateVersions(database string) error {
	if err := validateMigrationVersions(files[database].migrations.down); err != nil {
		return err
	}

	if err := validateMigrationVersions(files[database].migrations.up); err != nil {
		return err
	}
	return validateMigrationVersions(files[database].seeds)
}

func validateMigrationVersions(migrations []migration) error {
	versions := make(map[int64]string, len(migrations))
	for _, m := range migrations {
		if previousFilepath, ok := versions[m.version]; ok {
			return fmt.Errorf("%w in files %q and %q", ErrDuplicateVersion, previousFilepath, m.filepath)
		}

		versions[m.version] = m.filepath
	}
	return nil
}

func Run(ctx context.Context) error {
	for _, databaseName := range databases {
		log.Println("Database: " + databaseName)

		database := options.Databases[databaseName]
		f := files[databaseName]
		if *flags.bool[flagDropDB] || *flags.bool[flagCreateDB] {
			if err := adminConnect(ctx, database); err != nil {
				return err
			}

			if *flags.bool[flagDropDB] {
				if err := dropDB(ctx, database); err != nil {
					return err
				}
			}

			if *flags.bool[flagCreateDB] {
				if err := createDB(ctx, database); err != nil {
					return err
				}
			}

			if err := adminCloseConnection(ctx, database); err != nil {
				return err
			}
		}

		if *flags.bool[flagRe] {
			if err := adminConnect(ctx, database); err != nil {
				return err
			}

			if err := dropDB(ctx, database); err != nil {
				return err
			}

			if err := createDB(ctx, database); err != nil {
				return err
			}

			if err := adminCloseConnection(ctx, database); err != nil {
				return err
			}

			if err := connect(ctx, database); err != nil {
				return err
			}

			if err := migrate(ctx, database, f); err != nil {
				return err
			}

			if err := seed(ctx, database, f); err != nil {
				return err
			}

			if err := closeConnection(ctx, database); err != nil {
				return err
			}
		}

		if *flags.bool[flagMigrate] || !flags.any {
			if err := connect(ctx, database); err != nil {
				return err
			}

			if err := migrate(ctx, database, f); err != nil {
				return err
			}

			if err := closeConnection(ctx, database); err != nil {
				return err
			}
		}

		if *flags.string[flagDown] != "" {
			for _, down := range f.migrations.down {
				if *flags.string[flagDown] != strconv.FormatInt(down.version, 10) {
					continue
				}

				if err := connect(ctx, database); err != nil {
					return err
				}

				log.Println("Rollback migration...")

				if err := migrateFile(ctx, database, down, schemaMigrations); err != nil {
					return err
				}

				if err := closeConnection(ctx, database); err != nil {
					return err
				}
				break
			}
		}

		if *flags.string[flagUp] != "" {
			for _, up := range f.migrations.up {
				if *flags.string[flagUp] != strconv.FormatInt(up.version, 10) {
					continue
				}

				if err := connect(ctx, database); err != nil {
					return err
				}

				log.Println("Migrating...")

				if err := migrateFile(ctx, database, up, schemaMigrations); err != nil {
					return err
				}

				if err := closeConnection(ctx, database); err != nil {
					return err
				}
				break
			}
		}

		if *flags.bool[flagSeed] || !flags.any {
			if err := connect(ctx, database); err != nil {
				return err
			}

			if err := seed(ctx, database, f); err != nil {
				return err
			}

			if err := closeConnection(ctx, database); err != nil {
				return err
			}
		}

		if *flags.bool[flagIrr] {
			printIrreversibleMigrations(f)
		}
	}

	log.Println("Done!")

	return nil
}

func adminConnect(ctx context.Context, database Database) error {
	if err := database.AdminConnect(ctx); err != nil {
		return fmt.Errorf("failed to connect as admin: %w", err)
	}
	return nil
}

func dropDB(ctx context.Context, database Database) error {
	log.Println("Dropping DB...")

	if err := database.ExecDropDB(ctx); err != nil {
		return fmt.Errorf("failed to drop database: %w", err)
	}
	return nil
}

func createDB(ctx context.Context, database Database) error {
	log.Println("Creating DB...")

	if err := database.ExecCreateDB(ctx); err != nil {
		return fmt.Errorf("failed to create database: %w", err)
	}
	return nil
}

func adminCloseConnection(ctx context.Context, database Database) error {
	if err := database.AdminClose(ctx); err != nil {
		return fmt.Errorf("failed to close connection as admin: %w", err)
	}
	return nil
}

func connect(ctx context.Context, database Database) error {
	if err := database.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	return nil
}

func migrate(ctx context.Context, database Database, f *filesSt) error {
	log.Println("Migrating...")

	for _, up := range f.migrations.up {
		if err := migrateFile(ctx, database, up, schemaMigrations); err != nil {
			return err
		}
	}
	return nil
}

func migrateFile(ctx context.Context, database Database, m migration, versionsTable string) error {
	if err := database.ExecCreateVersionsTable(ctx, versionsTable); err != nil {
		return fmt.Errorf("failed to create table %q: %w", versionsTable, err)
	}

	exists, err := database.ExecIsVersionExists(ctx, versionsTable, m.version)
	if err != nil {
		return fmt.Errorf("failed to check version %d in table %q: %w", m.version, versionsTable, err)
	}
	if !m.isDown && exists {
		return nil
	}

	log.Println(m.filepath)

	b, err := options.FS.ReadFile(m.filepath)
	if err != nil {
		return fmt.Errorf("cannot read file %q: %w", m.filepath, err)
	}

	queries := strings.TrimSpace(string(b))
	inTransaction := !strings.HasPrefix(queries, noTransactionComment)
	queryOptions := ExecQueryOptions{
		IsDown:        m.isDown,
		VersionsTable: versionsTable,
		Version:       m.version,
		InTransaction: inTransaction,
	}
	if err := database.ExecQuery(ctx, queries, queryOptions); err != nil {
		return fmt.Errorf("failed to execute query from %q: %w", m.filepath, err)
	}
	return nil
}

func seed(ctx context.Context, database Database, f *filesSt) error {
	log.Println("Seeding...")

	for _, s := range f.seeds {
		if err := migrateFile(ctx, database, s, schemaSeeds); err != nil {
			return err
		}
	}
	return nil
}

func closeConnection(ctx context.Context, database Database) error {
	if err := database.Close(ctx); err != nil {
		return fmt.Errorf("failed to close connection: %w", err)
	}
	return nil
}

func printIrreversibleMigrations(f *filesSt) {
	migrations := make(map[int64]struct{}, len(f.migrations.down))
	for _, down := range f.migrations.down {
		migrations[down.version] = struct{}{}
	}
	var irreversibleMigrations []migration
	for _, up := range f.migrations.up {
		if _, ok := migrations[up.version]; !ok {
			irreversibleMigrations = append(irreversibleMigrations, up)
		}
	}

	if len(irreversibleMigrations) == 0 {
		log.Println("No irreversible migrations.")
		return
	}

	log.Println("Irreversible migrations:")

	var sb strings.Builder
	for _, irreversibleMigration := range irreversibleMigrations {
		sb.WriteString(irreversibleMigration.filepath + "\n")
	}
	log.Print(sb.String())
}
