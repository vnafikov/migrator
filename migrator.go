package migrator

import (
	"flag"
	"fmt"
	"log"
)

func Run() {
	seed := flag.Bool("seed", false, "seed the database.")
	re := flag.Bool("re", false, "replay migrations: reset the database and run migrations.")

	usage := flag.Usage
	flag.Usage = func() {
		fmt.Println("Description!\n")
		usage()
	}

	flag.Parse()

	log.Printf("%t", *re)
	log.Printf("%t", *seed)
}

func Info() {
	fmt.Println("MIGRATOR!")
}
