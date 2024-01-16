package migrator

import (
	"flag"
	"fmt"
)

var (
	bFlags map[string]*bool
	sFlags map[string]*string
)

func Init() {
	bFlags["migrate"] = flag.Bool("migrate", true, "run migrations. True by default.")
	bFlags["seed"] = flag.Bool("seed", false, "seed the database.")
	bFlags["createdb"] = flag.Bool("", false, "create the database.")
	bFlags["dropdb"] = flag.Bool("", false, "drop the database.")
	bFlags["re"] = flag.Bool("re", false, "replay migrations: reset the database and run migrations.")
	sFlags["up"] = flag.String("up", "", "run migration by version (datetime).")
	sFlags["down"] = flag.String("down", "", "rollback migration by version (datetime).")
	bFlags["irr"] = flag.Bool("irr", false, "list of irreversible migrations (without *.down.sql files).")

	usage := flag.Usage
	flag.Usage = func() {
		fmt.Print(`Performs database migrations.

Runs only migrations by default.
The flags can be combined.

`)
		usage()
	}

	flag.Parse()
}

func Run() {
}

func SetFS() {
}

func Info() {
	fmt.Println("MIGRATOR!")
}
