package migrator

import (
	"cmp"
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"slices"
	"strconv"
	"strings"
)

type Options struct {
	FS        embed.FS
	Databases map[string]*Database
}

type Database struct {
	Connect                 func()
	Close                   func()
	AdminConnect            func()
	AdminClose              func()
	ExecCreateVersionsTable func(string)
	ExecIsVersionExists     func(string, int) bool
	ExecQuery               func(string, ExecQueryOptions)
	ExecCreateDB            func()
	ExecDropDB              func()
}

type ExecQueryOptions struct {
	IsDown        bool
	VersionsTable string
	Version       int
	InTransaction bool
}

type flagsSt struct {
	bool   map[string]*bool
	string map[string]*string
	any    bool
}

type filesSt struct {
	migrations struct {
		down []*migration
		up   []*migration
	}
	seeds []*migration
}

type migration struct {
	version  int
	filepath string
	isDown   bool
}

const (
	schemaMigrations     = "schema_migrations"
	schemaSeeds          = "schema_seeds"
	versionLength        = 14
	noTransactionComment = "-- NO TRANSACTION"
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

func Init(opt Options) {
	options = opt
	optionDatabaseNames := mapKeys(options.Databases)
	dbFlag := flag.String("databases", "", fmt.Sprintf(`options: %s. All by default.`, strings.Join(optionDatabaseNames, ", ")))
	flags.bool["migrate"] = flag.Bool("migrate", false, "run migrations.")
	flags.bool["seed"] = flag.Bool("seed", false, "seed the database.")
	flags.bool["createdb"] = flag.Bool("createdb", false, "create the database.")
	flags.bool["dropdb"] = flag.Bool("dropdb", false, "drop the database.")
	flags.bool["re"] = flag.Bool("re", false, "replay migrations: reset the database and run migrations.")
	flags.string["up"] = flag.String("up", "", "run migration by version (datetime).")
	flags.string["down"] = flag.String("down", "", "rollback migration by version (datetime).")
	flags.bool["irr"] = flag.Bool("irr", false, "list of irreversible migrations (without *.down.sql files).")

	usage := flag.Usage
	flag.Usage = func() {
		fmt.Print(`Performs database migrations.

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
	for key := range flags.bool {
		if *flags.bool[key] {
			flags.any = true
			break
		}
	}
	if !flags.any {
		for key := range flags.string {
			if *flags.string[key] != "" {
				flags.any = true
				break
			}
		}
	}
}

func readFilenames() {
	for i := range databases {
		files[databases[i]] = new(filesSt)
		readMigrationFilenames(databases[i])
		readSeedFilenames(databases[i])
	}
}

func readMigrationFilenames(database string) {
	entries := readDir(database)
	sortEntries(entries)
	for _, entry := range entries {
		m := readMigration(entry, database)
		if m == nil {
			continue
		}

		if strings.Contains(entry.Name(), ".down.") {
			m.isDown = true
			files[database].migrations.down = append(files[database].migrations.down, m)
		} else if strings.Contains(entry.Name(), ".up.") {
			files[database].migrations.up = append(files[database].migrations.up, m)
		}
	}
}

func readSeedFilenames(database string) {
	path := fmt.Sprintf("%s/seeds", database)
	entries := readDir(path)
	sortEntries(entries)
	for _, entry := range entries {
		s := readMigration(entry, path)
		if s == nil {
			continue
		}

		files[database].seeds = append(files[database].seeds, s)
	}
}

func readDir(name string) []fs.DirEntry {
	entries, err := options.FS.ReadDir(name)
	if err != nil {
		log.Fatal(err)
	}
	return entries
}

func sortEntries(entries []fs.DirEntry) {
	slices.SortStableFunc(entries, func(a, b fs.DirEntry) int {
		return cmp.Compare(a.Name(), b.Name())
	})
}

func readMigration(entry fs.DirEntry, path string) *migration {
	if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
		return nil
	}
	version, err := strconv.Atoi(entry.Name()[:versionLength])
	if err != nil {
		return nil
	}

	return &migration{
		version:  version,
		filepath: fmt.Sprintf("%s/%s", path, entry.Name()),
	}
}

func Run() {
	for i := range databases {
		log.Println("Database: " + databases[i])

		database := options.Databases[databases[i]]
		f := files[databases[i]]
		if *flags.bool["dropdb"] || *flags.bool["createdb"] {
			database.AdminConnect()
			if *flags.bool["dropdb"] {
				dropDB(database)
			}
			if *flags.bool["createdb"] {
				createDB(database)
			}
			database.AdminClose()
		}
		if *flags.bool["re"] {
			database.AdminConnect()
			dropDB(database)
			createDB(database)
			database.AdminClose()
			database.Connect()
			migrate(database, f)
			seed(database, f)
			database.Close()
		}
		if *flags.bool["migrate"] || !flags.any {
			database.Connect()
			migrate(database, f)
			database.Close()
		}
		if *flags.string["down"] != "" {
			var m *migration
			for _, down := range f.migrations.down {
				if *flags.string["down"] == strconv.Itoa(down.version) {
					m = down
					break
				}
			}
			if m != nil {
				database.Connect()

				log.Println("Rollback migration...")

				migrateFile(database, m, schemaMigrations)
				database.Close()
			}
		}
		if *flags.string["up"] != "" {
			var m *migration
			for _, up := range f.migrations.up {
				if *flags.string["up"] == strconv.Itoa(up.version) {
					m = up
					break
				}
			}
			if m != nil {
				database.Connect()

				log.Println("Migrating...")

				migrateFile(database, m, schemaMigrations)
				database.Close()
			}
		}
		if *flags.bool["seed"] || !flags.any {
			database.Connect()
			seed(database, f)
			database.Close()
		}
		if *flags.bool["irr"] {
			printIrreversibleMigrations(f)
		}
	}

	log.Println("Done!")
}

func dropDB(database *Database) {
	log.Println("Dropping DB...")

	database.ExecDropDB()
}

func createDB(database *Database) {
	log.Println("Creating DB...")

	database.ExecCreateDB()
}

func migrate(database *Database, f *filesSt) {
	log.Println("Migrating...")

	for _, m := range f.migrations.up {
		migrateFile(database, m, schemaMigrations)
	}
}

func migrateFile(database *Database, m *migration, versionsTable string) {
	database.ExecCreateVersionsTable(versionsTable)
	if !m.isDown && database.ExecIsVersionExists(versionsTable, m.version) {
		return
	}

	log.Println(m.filepath)

	queriesB, err := options.FS.ReadFile(m.filepath)
	if err != nil {
		log.Fatal(err)
	}

	queries := string(queriesB)
	inTransaction := !strings.Contains(queries, noTransactionComment)
	database.ExecQuery(queries, ExecQueryOptions{
		IsDown:        m.isDown,
		VersionsTable: versionsTable,
		Version:       m.version,
		InTransaction: inTransaction,
	})
}

func seed(database *Database, f *filesSt) {
	log.Println("Seeding...")

	for _, s := range f.seeds {
		migrateFile(database, s, schemaSeeds)
	}
}

func printIrreversibleMigrations(f *filesSt) {
	migrations := make(map[int]*migration, len(f.migrations.down))
	for _, m := range f.migrations.down {
		migrations[m.version] = m
	}
	var irreversibleMigrations []*migration
	for _, m := range f.migrations.up {
		if _, ok := migrations[m.version]; !ok {
			irreversibleMigrations = append(irreversibleMigrations, m)
		}
	}

	if len(irreversibleMigrations) > 0 {
		log.Println("Irreversible migrations:")
	} else {
		log.Println("No irreversible migrations.")
	}

	for _, irreversibleMigration := range irreversibleMigrations {
		fmt.Println(irreversibleMigration.filepath)
	}
}
