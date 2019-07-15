package main

import (
	"database/sql"
	"fmt"
	"os"
	"io"
	"strings"
	"strconv"
	"encoding/csv"
	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

// domain:  iOS domain to query against.  These are defined by Apple (e.g. CameraRollDomain)
// condition:  the iOS filesystem path that we're interested in.  Will be used in SQL query as part of a LIKE condition.
// destination:  the relative path where the files will be copied to
// formats:  LOWERCASE ONLY []string of file extensions to search for (see appendUppercaseFormats())
type domainParams struct {
	domain, condition, destination string
	formats           []string
}

// friendlyName:  user-facing name of db (used for fmt.Println and naming of csv file)
// dbPath:  relative path to SQLite db as a string
// query:  SQL query to run
// csvHeadings:  []string of headings to use for the CSV file
type dbParams struct {
	friendlyName, dbPath, query string
	csvHeadings []string
}

func main() {
	// open Manifest.db
	db, err := sql.Open("sqlite3", "./Manifest.db")

	if err != nil {
		handleError("Error opening Manifest.db.  Did you place this program inside the folder that iTunes created when it backed up your iOS device?", err)
		fmt.Println("Press Enter to quit.")
		fmt.Scanln()
		return
	}
	defer db.Close()

	// this stores the domains and relevant info for each type of search/rename we will do
	domainParamsList := []domainParams{
		domainParams{"CameraRollDomain", "Media/DCIM%", "files/camera", []string{"jpg", "mov"}},
		domainParams{"MediaDomain", "Library/SMS/Attachments%", "files/sms", []string{"jpg", "jpeg", "gif", "png", "mov", "mp4", "mpg", "mpeg", "ogg", "mp3", "m4v", "webm", "ogv", "avi", "pdf"}},
	}

	dbParamsList := []dbParams{
		dbParams{"Contacts", "./31/31bb7ba8914766d4ba40d6dfb6113c8b614be442", "SELECT c0First, c1Last, c2Middle, c6Organization, c16Phone, c17Email, c18Address FROM 'ABPersonFullTextSearch_content'", []string{"First name", "Last name", "Middle", "Organization", "Phone number", "E-mail address", "Address"}},
	}

	// create directories if they don't already exist (os.MkdirAll does not recreate extant directories, how convenient)
	createDirs(domainParamsList)

	// append uppercase formats to formats slice in each processParam
	appendUppercaseFormats(domainParamsList)

	// do the work!
	err = processDomains(db, domainParamsList)

	if (err == nil) {
		fmt.Println("Camera images/videos and SMS attachments backed up successfully!")
		fmt.Println() // fine, have it your way, go-vet
	} else {
		handleError("An error has occurred.  Not all camera images/videos and SMS attachments may have been saved.", err)
	}

	// Process information held in databases such as contacts, perhaps text messages someday
	for _, dbParam := range dbParamsList {
		err = processDb(dbParam)
		
		if err == nil {
			fmt.Println("Processed " + dbParam.friendlyName + " successfully.")
		}
	}

	fmt.Println("Press Enter to quit.")
	fmt.Scanln()
}

func handleError(msg string, err error) {
	if err != nil {
		fmt.Println(msg)
		fmt.Println(err)
	}
}

// takes in list of paths, creates those directories with 755 permissions
func createDirs(domainParamsList []domainParams) {
	for _, processParam := range domainParamsList {
		os.MkdirAll(processParam.destination, 0755)
	}
}

// takes in domainParamsList, mutates each processParam.formats slice by appending capitalized strings
func appendUppercaseFormats(domainParamsList []domainParams) {
	for i, processParam := range domainParamsList {
		capitalList := []string{}

		for _, ext := range processParam.formats {
			capitalList = append(capitalList, strings.ToUpper(ext))
		}

		// actually mutate the formats key in each processParam struct
		domainParamsList[i].formats = append(processParam.formats, capitalList...)
	}
}

// kicks off file processing for each separate iOS domain (I organize SQLite queries by domain)
func processDomains(db *sql.DB, domainParamsList []domainParams) (err error) {
	for _, processParam := range domainParamsList {
		rows, err := query(db, processParam.domain, processParam.condition, processParam.formats)
		if err != nil {
			return err
		}
		err = processFiles(rows, &processParam)
		if err != nil {
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
func processFiles(rows *sql.Rows, domainParams *domainParams) (err error) {
	var fileID, domain, relativePath string
	counter := 0
	copyLocation := "./" + domainParams.destination
	dupeMap := make(map[string]int)

	fmt.Println("Beginning copy of " + domainParams.domain + " files.")
	
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

	fmt.Println("Copy of " + domainParams.domain + "files finished.  Copied " + strconv.Itoa(counter) + " files.\n")
	return nil
}

// copies files
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

// execute query against SQLite db and save results to csv
func processDb(params dbParams) (err error) {
	db, err := sql.Open("sqlite3", params.dbPath)

	if err != nil {
		handleError("Error opening " + params.friendlyName + " database.", err)
		return
	}

	defer db.Close()

	rows, err := db.Query(params.query)

	if err != nil {
		handleError("Unable to query " + params.friendlyName + " database.", err)
		return
	}

	cols, err := rows.Columns()
	numCols := len(cols)
	csvString := [][]string{params.csvHeadings}
	fileName := params.friendlyName + ".csv"

	file, err := os.Create("files/" + fileName)

	if err != nil {
		handleError("Unable to create " + fileName, err)
		return
	}

	valuesList := make([]interface{}, numCols)

	for i := range valuesList {
		var iface interface{}
		valuesList[i] = &iface
	}

	for rows.Next() {
		prettyLine := []string{}

		rows.Scan(valuesList...)
		for i := range cols {
			value := *(valuesList[i].(*interface{})) // but it's actually a string

			if value == nil {
				prettyLine = append(prettyLine, "")
			} else {
				prettyLine = append(prettyLine, value.(string))
			}
		}

		csvString = append(csvString, prettyLine)
	}

	fmt.Println("Saving contacts.")
	csvWriter := csv.NewWriter(file)
	csvWriter.WriteAll(csvString)
	csvWriter.Flush()

	return nil
}