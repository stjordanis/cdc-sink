package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"log"
	"net/http"

	_ "github.com/lib/pq"
)

// Sink holds all the info needed for a specific table.
type Sink struct {
	originalTable string
	resultDBTable string
	sinkDBTable   string
}

// CreateSink creates all the required tables and returns a new Sink.
func CreateSink(
	db *sql.DB, originalTable string, resultDB string, resultTable string,
) (*Sink, error) {
	// Check to make sure the table exists.
	resultDBTable := fmt.Sprintf("%s.%s", resultDB, resultTable)
	exists, err := TableExists(db, resultDB, resultTable)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("Table %s could not be found", resultDBTable)
	}

	// Grab all the columns here.
	sinkDBTable := SinkDBTableName(resultDB, resultTable)
	if err := CreateSinkTable(db, sinkDBTable); err != nil {
		return nil, err
	}

	sink := &Sink{
		originalTable: originalTable,
		resultDBTable: resultDBTable,
		sinkDBTable:   sinkDBTable,
	}

	return sink, nil
}

// HandleRequest is a handler used for this specific sink.
func (s *Sink) HandleRequest(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	scanner := bufio.NewScanner(r.Body)
	defer r.Body.Close()
	for scanner.Scan() {
		line, err := parseLine(scanner.Bytes())
		if err != nil {
			log.Print(err)
			fmt.Fprint(w, err)
			w.WriteHeader(http.StatusBadRequest)
		}
		if err := line.WriteToSinkTable(db, s.sinkDBTable); err != nil {
			log.Print(err)
			fmt.Fprint(w, err)
			w.WriteHeader(http.StatusBadRequest)
		}
	}
}