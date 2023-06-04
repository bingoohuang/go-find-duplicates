package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/m-manu/go-find-duplicates/entity"
	"github.com/m-manu/go-find-duplicates/fmte"
	"go.uber.org/multierr"
)

const bytesPerLineGuess = 500

func reportDuplicates(duplicates *entity.DigestToFiles, outputMode string, allFiles entity.FilePathToMeta,
	runID string, reportFileName string,
) error {
	var err error
	switch outputMode {
	case entity.OutputModeStdOut:
		reportBytes := getReportAsText(duplicates)
		printReportToStdOut(runID, reportBytes)
	case entity.OutputModeTextFile:
		reportBytes := getReportAsText(duplicates)
		createTextFileReport(reportFileName, reportBytes)
	case entity.OutputModeCsvFile:
		createCsvReport(duplicates, allFiles, reportFileName)
	case entity.OutputModeJSON:
		err = createJSONReport(duplicates, reportFileName)
	}
	return err
}

func createTextFileReport(reportFileName string, report bytes.Buffer) {
	rcErr := os.WriteFile(reportFileName, report.Bytes(), 0o644)
	if rcErr != nil {
		fmte.PrintfErr("error while creating report file %s: %+v\n", reportFileName, rcErr)
		os.Exit(exitCodeErrorCreatingReport)
	}
	fmte.Printf("View duplicates report here: %s\n", reportFileName)
}

func RemoveDuplicates(duplicates *entity.DigestToFiles) (err error) {
	for iter := duplicates.Iterator(); iter.HasNext(); {
		_, paths := iter.Next()
		for i, path := range paths {
			if i > 0 {
				err = multierr.Append(err, os.Remove(path))
			}
		}
	}

	return
}

func getReportAsText(duplicates *entity.DigestToFiles) bytes.Buffer {
	var bb bytes.Buffer
	bb.Grow(duplicates.Size() * bytesPerLineGuess)
	for iter := duplicates.Iterator(); iter.HasNext(); {
		digest, paths := iter.Next()
		sort.Strings(paths)
		bb.WriteString(fmt.Sprintf("%s: %d duplicate(s)\n", digest, len(paths)-1))
		for _, path := range paths {
			bb.WriteString(fmt.Sprintf("\t%s\n", path))
		}
	}
	return bb
}

func printReportToStdOut(runID string, bb bytes.Buffer) {
	fmt.Printf(`
==========================
Report (run id %s)
==========================
`, runID)
	fmt.Printf(bb.String())
}

func createCsvReport(duplicates *entity.DigestToFiles, allFiles entity.FilePathToMeta, reportFileName string) {
	var bb bytes.Buffer
	bb.Grow(duplicates.Size() * bytesPerLineGuess)
	cf := csv.NewWriter(&bb)
	cf.Write([]string{"file hash", "file size", "last modified", "file path"})
	for iter := duplicates.Iterator(); iter.HasNext(); {
		digest, paths := iter.Next()
		for _, path := range paths {
			cf.Write([]string{
				digest.FileHash,
				strconv.FormatInt(digest.FileSize, 10),
				time.Unix(allFiles[path].ModifiedTimestamp, 0).Format("02-Jan-2006 03:04:05 PM"),
				path,
			})
		}
	}
	cf.Flush()
	os.WriteFile(reportFileName, bb.Bytes(), 0o644)
	fmte.Printf("View duplicates report here: %s\n", reportFileName)
}

func createJSONReport(duplicates *entity.DigestToFiles, reportFileName string) error {
	type duplicateFile struct {
		entity.FileDigest
		Paths []string `json:"paths"`
	}
	var duplicatesToMarshall []duplicateFile
	for iter := duplicates.Iterator(); iter.HasNext(); {
		digest, paths := iter.Next()
		duplicatesToMarshall = append(duplicatesToMarshall, duplicateFile{
			*digest,
			paths,
		})
	}
	jsonBytes, err := json.Marshal(duplicatesToMarshall)
	if err != nil {
		return err
	}
	os.WriteFile(reportFileName, jsonBytes, 0o644)
	fmte.Printf("View duplicates report here: %s\n", reportFileName)
	return nil
}
