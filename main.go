package main

import (
	"os"
	"database/sql"
	"fmt"
	"strings"
	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

type processParams struct {
	domain, condition string
	formats []string
}

func main() {
	// open Manifest.db
	db, err := sql.Open("sqlite3", "./Manifest.db")
	if err != nil {
		fmt.Println("An error occurred.  Make sure you're running this program from inside the iTunes backup folder.")
		return
	}

	// create directories if they don't already exist (os.MkdirAll ignores collisions!)
	dirList := []string{"files/sms", "files/camera"}
	createDirs(dirList)

	// this stores the domains and relevant info for each type of search/rename we will do
	processParamsList := []processParams{
		processParams{"CameraRollDomain", "Media/DCIM%", []string{"jpg", "mov"}},
		processParams{"MediaDomain", "Library/SMS/Attachments%", []string{"jpg", "jpeg", "gif", "png", "mov", "mp4", "mpg", "mpeg", "ogg", "mp3", "m4v", "webm", "ogv", "avi", "pdf"}},
	}

	// append uppercase formats to formats slice in each processParam
	appendUppercaseFormats(processParamsList)

	// do the work!
	processDomains(db, processParamsList)
}

func createDirs(dirList []string) {
	for _, str := range dirList {
		os.MkdirAll(str, 0755)
	}
}

func appendUppercaseFormats(processParamsList []processParams) {
	for i, processParam := range processParamsList {
		capitalList := []string{}

		for _, ext := range processParam.formats {
			capitalList = append(capitalList, strings.ToUpper(ext))
		}

		// actually mutate the formats key in each processParam struct
		processParamsList[i].formats = append(processParam.formats, capitalList...)
	}
}

func processDomains(db *sql.DB, processParamsList []processParams) {
	for _, processParam := range processParamsList {
		rows, err := query(db, processParam.domain, processParam.condition)
		if err != nil {
			fmt.Println("An error occurred while performing SELECT query on domain " + processParam.domain)
			fmt.Println(err)
			return
		}
		processFiles(rows, &processParam)
	}
}

func query(db *sql.DB, domain, condition string) (rows *sql.Rows, err error) {
	rows, err = db.Query("SELECT fileID, domain, relativePath FROM Files WHERE (domain = ? AND relativePath LIKE ?)", domain, condition)
	return
}

func processFiles(rows *sql.Rows, processParams *processParams) {
	var fileID, domain, relativePath string
	for rows.Next() {
		rows.Scan(&fileID, &domain, &relativePath)
		fmt.Println("fileID: " + fileID + ", domain: " + domain + ", relativePath: " + relativePath)
	}
	
	fmt.Println(processParams)
}