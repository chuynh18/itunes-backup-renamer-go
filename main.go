package main

import (
	"os"
	"database/sql"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

func main() {
	db, err := sql.Open("sqlite3", "./Manifest.db")
	if err != nil {
		fmt.Println("An error occurred.  Make sure you're running this program from inside the iTunes backup folder.")
		return
	}

	dirList := []string{"files/sms", "files/camera"}
	createDirs(dirList)

	domainList := []string{"CameraRollDomain", "MediaDomain"}
	processDomains(db, domainList)
}

func createDirs(list []string) {
	for _, str := range list {
		os.MkdirAll(str, 0755)
	}
}

func processDomains(db *sql.DB, list []string) {
	for _, str := range list {
		rows, err := query(db, str)
		if err != nil {
			fmt.Println("An error occurred while performing SELECT query on domain " + str)
			fmt.Println(err)
			return
		}
		processFiles(rows)
	}
}

func query(db *sql.DB, domain string) (rows *sql.Rows, err error) {
	rows, err = db.Query("SELECT fileID, domain, relativePath FROM Files WHERE domain = ?", domain)
	return
}

func processFiles(rows *sql.Rows) {
	var fileID, domain, relativePath string
	for rows.Next() {
		rows.Scan(&fileID, &domain, &relativePath)
		fmt.Println("fileID: " + fileID + ", domain: " + domain + ", relativePath: " + relativePath)
  	}
}

