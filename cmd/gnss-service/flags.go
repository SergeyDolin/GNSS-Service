package main

import (
	"flag"
	"os"
)

var (
	flagRunAddr string
	flagSQL     string
)

func parseFlags() {
	flag.StringVar(&flagRunAddr, "a", "localhost:8080", "address and port to run server")

	flag.StringVar(&flagSQL, "d", "", "DB address")

	flag.Parse()

	if address := os.Getenv("ADDRESS"); address != "" {
		flagRunAddr = address
	}

	if dbName := os.Getenv("DATABASE_DSN"); dbName != "" {
		flagSQL = dbName
	}
}
