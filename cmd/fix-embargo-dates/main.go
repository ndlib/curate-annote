// Fix-embargo-dates is simple utility to efficiently update an existing
// database with missing embargo dates. The fedora embargo date harvesting was
// fixed in commit b79b799cf7ea867b8b7e52410c77288368235ff5
//
// There should be no need for anyone to ever run this command again.
//
// Usage:
//     env FEDORA_PATH="..." MYSQL_PATH="..." ./fix-embargo-dates
package main

import (
	"database/sql"
	"log"
	"os"

	_ "github.com/go-sql-driver/mysql"

	"github.com/ndlib/curate-annote/internal/annote"
)

var (
	db *sql.DB
)

func main() {
	var err error
	fedoraPath := os.Getenv("FEDORA_PATH")
	if fedoraPath == "" {
		log.Println("FEDORA_PATH not set")
		return
	}
	fedora := annote.NewRemote(fedoraPath)
	dbPath := os.Getenv("MYSQL_PATH")
	if dbPath == "" {
		log.Println("MYSQL_PATH not set")
		return
	}
	db, err = sql.Open("mysql", dbPath)
	if err != nil {
		log.Println(err)
		return
	}

	harvestAllEmbargoDates(fedora)
}

func harvestAllEmbargoDates(remote *annote.RemoteFedora) {
	var nItems, nErr int
	token := ""
	query := "pid~und:*"
	var err error

	for {
		// get a page of search results
		var ids []string
		ids, token, err = remote.SearchObjects(query, token)
		if err != nil {
			log.Println(err)
			break
		}
		for _, pid := range ids {
			nItems++
			result := &annote.CurateItem{PID: pid}
			err = annote.ReadRightsMetadata(remote, pid, result)
			if err != nil && err != annote.ErrNotFound {
				log.Println(pid, err)
				nErr++
				continue
			}
			embargoDate := result.FirstField("embargo-date")
			if embargoDate != "" {
				log.Println("updating", pid, embargoDate)
				err = saveEmbargo(pid, embargoDate)
				if err != nil {
					log.Println(pid, err)
					nErr++
				}
			}
		}
		// no token is returned on the last results page
		if token == "" {
			break
		}
	}
	log.Println("Update Embargo Date:", nItems, "items with", nErr, "errors")
}

func saveEmbargo(pid, embargoDate string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.Exec(
		`DELETE FROM triples WHERE subject = ? AND predicate = "embargo-date"`,
		pid,
	)
	if err != nil {
		return err
	}
	_, err = tx.Exec(
		`INSERT INTO triples (subject, predicate, object) VALUES (?, ?, ?)`,
		pid,
		"embargo-date",
		embargoDate,
	)
	if err != nil {
		return err
	}
	return tx.Commit()
}
