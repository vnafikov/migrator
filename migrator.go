package migrator

import (
	"cmp"
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
)

var (
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

type Options struct {
	FS        FS
	Databases map[string]Database
}

type Database struct {
	Connect                 func()
	Close                   func()
	AdminConnect            func()
	AdminClose              func()
	ExecCreateVersionsTable func(string)
	ExecIsVersionExists     func(string, int64) bool
	ExecQuery               func(string, ExecQueryOptions)
	ExecCreateDB            func()
	ExecDropDB              func()
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

func Init(opt Options) {
	options = opt
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
	readFilenames()
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

func readFilenames() {
	for _, database := range databases {
		files[database] = new(filesSt)
		readMigrationFilenames(database)
		readSeedFilenames(database)
	}
}

func readMigrationFilenames(database string) {
	entries := readDir(database)
	sortEntries(entries)
	for _, entry := range entries {
		m, ok := readMigration(entry, database)
		if !ok {
			continue
		}

		name := entry.Name()
		if strings.Contains(name, downMarker) {
			m.isDown = true
			files[database].migrations.down = append(files[database].migrations.down, m)
		} else if strings.Contains(name, upMarker) {
			files[database].migrations.up = append(files[database].migrations.up, m)
		}
	}
}

func readSeedFilenames(database string) {
	path := fmt.Sprintf("%s/%s", database, seedsSubdir)
	entries := readDir(path)
	sortEntries(entries)
	for _, entry := range entries {
		s, ok := readMigration(entry, path)
		if !ok {
			continue
		}

		files[database].seeds = append(files[database].seeds, s)
	}
}

func readDir(name string) []fs.DirEntry {
	entries, err := options.FS.ReadDir(name)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		log.Fatal(err)
	}
	return entries
}

func sortEntries(entries []fs.DirEntry) {
	slices.SortStableFunc(entries, func(a, b fs.DirEntry) int {
		return cmp.Compare(a.Name(), b.Name())
	})
}

func readMigration(entry fs.DirEntry, path string) (m migration, ok bool) {
	name := entry.Name()
	if entry.IsDir() || !strings.HasSuffix(name, sqlSuffix) || len(name) < versionLength {
		return m, false
	}

	var err error
	if m.version, err = strconv.ParseInt(name[:versionLength], 10, 64); err != nil {
		return m, false
	}

	m.filepath = fmt.Sprintf("%s/%s", path, name)
	return m, true
}

func Run() {
	for _, databaseName := range databases {
		log.Println("Database: " + databaseName)

		database := options.Databases[databaseName]
		f := files[databaseName]
		if *flags.bool[flagDropDB] || *flags.bool[flagCreateDB] {
			database.AdminConnect()
			if *flags.bool[flagDropDB] {
				dropDB(database)
			}
			if *flags.bool[flagCreateDB] {
				createDB(database)
			}
			database.AdminClose()
		}
		if *flags.bool[flagRe] {
			database.AdminConnect()
			dropDB(database)
			createDB(database)
			database.AdminClose()
			database.Connect()
			migrate(database, f)
			seed(database, f)
			database.Close()
		}
		if *flags.bool[flagMigrate] || !flags.any {
			database.Connect()
			migrate(database, f)
			database.Close()
		}
		if *flags.string[flagDown] != "" {
			for _, down := range f.migrations.down {
				if *flags.string[flagDown] != strconv.FormatInt(down.version, 10) {
					continue
				}

				database.Connect()

				log.Println("Rollback migration...")

				migrateFile(database, down, schemaMigrations)
				database.Close()
				break
			}
		}
		if *flags.string[flagUp] != "" {
			for _, up := range f.migrations.up {
				if *flags.string[flagUp] != strconv.FormatInt(up.version, 10) {
					continue
				}

				database.Connect()

				log.Println("Migrating...")

				migrateFile(database, up, schemaMigrations)
				database.Close()
				break
			}
		}
		if *flags.bool[flagSeed] || !flags.any {
			database.Connect()
			seed(database, f)
			database.Close()
		}
		if *flags.bool[flagIrr] {
			printIrreversibleMigrations(f)
		}
	}

	log.Println("Done!")
}

func dropDB(database Database) {
	log.Println("Dropping DB...")

	database.ExecDropDB()
}

func createDB(database Database) {
	log.Println("Creating DB...")

	database.ExecCreateDB()
}

func migrate(database Database, f *filesSt) {
	log.Println("Migrating...")

	for _, up := range f.migrations.up {
		migrateFile(database, up, schemaMigrations)
	}
}

func migrateFile(database Database, m migration, versionsTable string) {
	database.ExecCreateVersionsTable(versionsTable)
	if !m.isDown && database.ExecIsVersionExists(versionsTable, m.version) {
		return
	}

	log.Println(m.filepath)

	queriesBytes, err := options.FS.ReadFile(m.filepath)
	if err != nil {
		log.Fatal(err)
	}

	queries := string(queriesBytes)
	inTransaction := !strings.HasPrefix(queries, noTransactionComment)
	database.ExecQuery(queries, ExecQueryOptions{
		IsDown:        m.isDown,
		VersionsTable: versionsTable,
		Version:       m.version,
		InTransaction: inTransaction,
	})
}

func seed(database Database, f *filesSt) {
	log.Println("Seeding...")

	for _, s := range f.seeds {
		migrateFile(database, s, schemaSeeds)
	}
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

	if len(irreversibleMigrations) > 0 {
		log.Println("Irreversible migrations:")
	} else {
		log.Println("No irreversible migrations.")
	}

	for _, irreversibleMigration := range irreversibleMigrations {
		_, _ = fmt.Println(irreversibleMigration.filepath)
	}
}
