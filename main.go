package main

import (
	"database/sql"
	"fmt"
	"os"
	"io"
	"strings"
	"strconv"
	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

type processParams struct {
	domain, condition, destination string
	formats           []string
}

func main() {
	// check for Manifest.db, because sql.Open doesn't, sadface
	if _, err := os.Stat("./Manifest.db"); err != nil {
		if os.IsNotExist(err) {
			fmt.Println("Manifest.db was not found.  Are you running this program from inside the iTunes backup folder?")
			fmt.Println("Please move the program itself into the iOS device backup folder that iTunes created, then try running the program again.")
			fmt.Println("Press Enter to quit.")
		} else {
			fmt.Println("Sorry, something unexpectedly went wrong.  Press Enter to quit.")
		}

		fmt.Scanln()
		return
	}

	// open Manifest.db
	db, _ := sql.Open("sqlite3", "./Manifest.db")

	// this stores the domains and relevant info for each type of search/rename we will do
	processParamsList := []processParams{
		processParams{"CameraRollDomain", "Media/DCIM%", "files/camera", []string{"jpg", "mov"}},
		processParams{"MediaDomain", "Library/SMS/Attachments%", "files/sms", []string{"jpg", "jpeg", "gif", "png", "mov", "mp4", "mpg", "mpeg", "ogg", "mp3", "m4v", "webm", "ogv", "avi", "pdf"}},
	}

	// create directories if they don't already exist (os.MkdirAll ignores collisions!)
	createDirs(processParamsList)

	// append uppercase formats to formats slice in each processParam
	appendUppercaseFormats(processParamsList)

	// do the work!
	err := processDomains(db, processParamsList)

	if (err == nil) {
		fmt.Println("All operations completed successfully!  Press Enter to quit.")
	} else {
		fmt.Println("An error has occurred.  Press Enter to quit.")
		fmt.Println(err)
	}

	fmt.Scanln()
}

// takes in list of paths, creates those directories with 755 permissions
func createDirs(processParamsList []processParams) {
	for _, processParam := range processParamsList {
		os.MkdirAll(processParam.destination, 0755)
	}
}

// takes in processParamsList, mutates each processParam.formats slice by appending capitalized strings
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

// kicks off file processing for each separate iOS domain (I organize SQLite queries by domain)
func processDomains(db *sql.DB, processParamsList []processParams) (err error) {
	for _, processParam := range processParamsList {
		rows, err := query(db, processParam.domain, processParam.condition, processParam.formats)
		if err != nil {
			fmt.Println("An error occurred while performing SELECT query on domain " + processParam.domain)
			return err
		}
		err = processFiles(rows, &processParam)
		if err != nil {
			fmt.Println("An error occurred while copying files.")
			return err
		}
	}

	return nil
}

// builds query string then performs the query - leaning on SQLite to do the filtering instead of my own code
func query(db *sql.DB, domain, condition string, formats []string) (rows *sql.Rows, err error) {
	// start building the query string (because we only care about certain file name extensions)
	query := "SELECT fileID, domain, relativePath FROM Files WHERE (domain = ? AND relativePath LIKE ? AND ("
	var queryFormats []string

	for _, format := range formats {
		queryFormat := "relativePath LIKE '%" + format + "'"
		queryFormats = append(queryFormats, queryFormat)
	}

	// query string built
	query = query + strings.Join(queryFormats, " OR ") + "))"

	// execute SQLite query
	rows, err = db.Query(query, domain, condition)
	return
}

// iterate over the query results and perform file operations
func processFiles(rows *sql.Rows, processParams *processParams) (err error) {
	var fileID, domain, relativePath string
	counter := 0
	copyLocation := "./" + processParams.destination
	dupeMap := make(map[string]int)

	fmt.Println("Beginning copy of " + processParams.domain + " files.")
	
	for rows.Next() {
		rows.Scan(&fileID, &domain, &relativePath)

		// path to file with obfuscated name
		backupLocation := "./" + fileID[0:2] + "/" + fileID

		// file's original name
		relPathSlice := strings.Split(relativePath, "/")
		originalName := relPathSlice[len(relPathSlice) - 1]
		var copyPath, rename string

		// check for dupes
		originalNameUpper := strings.ToUpper(originalName)
		_, ok := dupeMap[originalNameUpper]; if ok {
			renameSlice := strings.Split(originalName, ".")
			rename = strings.Join(renameSlice[0:len(renameSlice) - 1], ".") + "-" + strconv.Itoa(dupeMap[originalNameUpper]) + "." + renameSlice[len(renameSlice) - 1]
			copyPath = copyLocation + "/" + rename
			dupeMap[originalNameUpper]++
		} else {
			dupeMap[originalNameUpper] = 1
			copyPath = copyLocation + "/" + originalName
		}

		if dupeMap[originalNameUpper] > 1 {
			fmt.Println("Duplicate filename encountered.  Renaming file to " + rename + ".")
		}

		err := copy(backupLocation, copyPath)
		if err != nil {
			return err
		}

		counter++

		if counter % 100 == 0 {
			fmt.Println("Copied " + strconv.Itoa(counter) + " files...")
		}
	}

	fmt.Println("Copy of " + processParams.domain + "files finished.  Copied " + strconv.Itoa(counter) + " files.")
	return nil
}

func copy (src, dest string) (err error) {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}

	destFile, err := os.Create(dest)
	if err != nil {
		return err
	}

	_, err = io.Copy(destFile, srcFile)
	if err != nil {
		return err
	}

	err = destFile.Sync()
	if err != nil {
		return err
	}

	return nil
}