package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net/http"

	_ "github.com/lib/pq"
)

var connectionString = flag.String("conn", "postgresql://root@localhost:26257/defaultdb?sslmode=disable", "cockroach connection string")
var port = flag.Int("port", 26258, "http server listening port")

var sourceTable = flag.String("source_table", "", "Name of the source table sending data")

var resultDB = flag.String("db", "defaultdb", "database for the receiving table")
var resultTable = flag.String("table", "", "receiving table, must exist")

var sinkDB = flag.String("sink_db", "_CDC_SINK", "db for storing temp sink tables")

var dropDB = flag.Bool("drop", false, "Drop the sink db before starting?")

func createHandler(db *sql.DB, sinks *Sinks) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Request: %s", r.RequestURI)
		log.Printf("Header: %s", r.Header)

		// Is it an ndjson url?
		ndjson, ndjsonErr := parseNdjsonURL(r.RequestURI)
		if ndjsonErr == nil {
			sink := sinks.FindSink(ndjson.topic)
			if sink != nil {
				sink.HandleRequest(db, w, r)
				return
			}

			// No sink found, throw an error.
			http.Error(
				w,
				fmt.Sprintf("could not find a sync for %s", ndjson.topic),
				http.StatusInternalServerError,
			)
			return
		}

		// Is it a resolved url?
		resolved, resolvedErr := parseResolvedURL(r.RequestURI)
		if resolvedErr == nil {
			sinks.HandleResolvedRequest(db, resolved, w, r)
			return
		}

		// Could not recognize url.
		http.Error(
			w,
			fmt.Sprintf("URL pattern does not match either an ndjson (%s) or a resolved (%s)",
				ndjsonErr, resolvedErr,
			),
			http.StatusInternalServerError,
		)
		return
	}
}

func main() {
	db, err := sql.Open("postgres", *connectionString)
	if err != nil {
		log.Fatal("error connecting to the database: ", err)
	}
	defer db.Close()

	if *dropDB {
		if err := DropSinkDB(db); err != nil {
			log.Fatal(err)
		}
	}

	if err := CreateSinkDB(db); err != nil {
		log.Fatal(err)
	}

	sinks := CreateSinks()

	// Add all the sinks here
	if err := sinks.AddSink(db, *sourceTable, *resultDB, *resultTable); err != nil {
		log.Fatal(err)
	}

	handler := createHandler(db, sinks)
	http.HandleFunc("/", handler)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *port), nil))
}
