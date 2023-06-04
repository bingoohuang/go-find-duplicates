/*
A blazingly-fast simple-to-use tool to find duplicate files (photos, videos, music, documents etc.) on your computer,
portable hard drives etc.
*/
package main

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	set "github.com/deckarep/golang-set/v2"
	"github.com/m-manu/go-find-duplicates/bytesutil"
	"github.com/m-manu/go-find-duplicates/entity"
	"github.com/m-manu/go-find-duplicates/fmte"
	"github.com/m-manu/go-find-duplicates/service"
	"github.com/m-manu/go-find-duplicates/utils"
	"github.com/samber/lo"
	flag "github.com/spf13/pflag"
)

// Exit codes for this program
const (
	exitCodeSuccess = iota
	exitCodeInvalidNumArgs
	exitCodeInvalidExclusions
	exitCodeInputDirectoryNotReadable
	exitCodeExclusionFilesError
	exitCodeErrorFindingDuplicates
	exitCodeErrorCreatingReport
	exitCodeInvalidOutputMode
	exitCodeReportFileCreationFailed
	exitCodeWritingToReportFileFailed
)

const version = "1.7.0"

//go:embed default_exclusions.txt
var defaultExclusionsStr string

var flags struct {
	isHelp             func() bool
	getOutputMode      func() string
	getExcludedFiles   func() set.Set[string]
	getMinSize         func() int64
	getParallelism     func() int
	isThorough         func() bool
	getVersion         func() bool
	isRemoveDuplicates func() bool
}

func setupExclusionsOpt() {
	const exclusionsFlag = "exclusions"
	const exclusionsDefaultValue = ""
	defaultExclusions, defaultExclusionsExamples := utils.LineSeparatedStrToMap(defaultExclusionsStr)
	p := flag.StringP(exclusionsFlag, "x", exclusionsDefaultValue,
		fmt.Sprintf("path to file containing newline-separated list of file/directory names to be excluded\n"+
			"(if this is not set, by default these will be ignored:\n%s etc.)",
			strings.Join(defaultExclusionsExamples, ", ")))
	flags.getExcludedFiles = func() set.Set[string] {
		if *p == exclusionsDefaultValue {
			return defaultExclusions
		}

		if !utils.IsReadableFile(*p) {
			fmte.PrintfErr("error: argument to flag --%s should be a readable file\n", exclusionsFlag)
			flag.Usage()
			os.Exit(exitCodeInvalidExclusions)
		}
		rawContents, err := os.ReadFile(*p)
		if err != nil {
			fmte.PrintfErr("error: unable to read exclusions file: %+v\n", exclusionsFlag, err)
			flag.Usage()
			os.Exit(exitCodeExclusionFilesError)
		}
		contents := strings.ReplaceAll(string(rawContents), "\r\n", "\n") // Windows
		exclusions, _ := utils.LineSeparatedStrToMap(contents)

		return exclusions
	}
}

func setupHelpOpt() {
	p := flag.BoolP("help", "h", false, "display help")
	flags.isHelp = func() bool { return *p }
}

func setupThoroughOpt() {
	p := flag.BoolP("thorough", "t", false,
		"apply thorough check of uniqueness of files\n(caution: this makes the scan very slow!)",
	)
	flags.isThorough = func() bool { return *p }
}

func setupRemoveDuplicates() {
	p := flag.BoolP("remove", "X", false, "remove duplicate files from input directory")
	flags.isRemoveDuplicates = func() bool { return *p }
}

func setupMinSizeOpt() {
	p := flag.Uint64P("minsize", "m", 4,
		"minimum size of file in KiB to consider",
	)
	flags.getMinSize = func() int64 { return int64(*p) * bytesutil.KIBI }
}

func setupParallelismOpt() {
	const defaultParallelismValue = 0
	p := flag.Uint8P("parallelism", "p", defaultParallelismValue,
		"extent of parallelism (defaults to number of cores minus 1)")
	flags.getParallelism = func() int {
		if *p == defaultParallelismValue {
			n := runtime.NumCPU()
			return lo.Ternary(n > 1, n-1, 1)
		}
		return int(*p)
	}
}

func setupOutputModeOpt() {
	var sb strings.Builder
	sb.WriteString("following modes are accepted:\n")
	for outputMode, description := range entity.OutputModes {
		sb.WriteString(fmt.Sprintf("%5s = %s\n", outputMode, description))
	}
	p := flag.StringP("output", "o", entity.OutputModeTextFile, sb.String())
	flags.getOutputMode = func() string {
		outputModeStr := strings.ToLower(strings.TrimSpace(*p))
		if _, exists := entity.OutputModes[outputModeStr]; !exists {
			fmt.Printf("error: invalid output mode '%s'\n", outputModeStr)
			os.Exit(exitCodeInvalidOutputMode)
		}
		return outputModeStr
	}
}

func setupVersionOpt() {
	p := flag.Bool("version", false,
		"Display version ("+version+") and exit (useful for incorporating this in scripts)")
	flags.getVersion = func() bool { return *p }
}

func setupUsage() {
	flag.Usage = func() {
		fmte.PrintfErr("Run \"go-find-duplicates --help\" for usage\n")
	}
}

func readDirectories() (directories []string) {
	if flag.NArg() < 1 {
		fmte.PrintfErr("error: no input directories passed\n")
		flag.Usage()
		os.Exit(exitCodeInvalidNumArgs)
	}
	for i, p := range flag.Args() {
		if !utils.IsReadableDirectory(p) {
			fmte.PrintfErr("error: input #%d \"%v\" isn't a readable directory\n", i+1, p)
			flag.Usage()
			os.Exit(exitCodeInputDirectoryNotReadable)
		}
		abs, _ := filepath.Abs(p)
		directories = append(directories, abs)
	}
	return directories
}

func handlePanic() {
	err := recover()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Program exited unexpectedly. "+
			"Please report the below eror to the author:\n"+
			"%+v\n", err)
		_, _ = fmt.Fprintln(os.Stderr, string(debug.Stack()))
	}
}

func showHelpAndExit() {
	flag.CommandLine.SetOutput(os.Stdout)
	fmt.Printf(`go-find-duplicates is a tool to find duplicate files and directories

Usage:
  go-find-duplicates [flags] <dir-1> <dir-2> ... <dir-n>

where,
  arguments are readable directories that need to be scanned for duplicates

Flags (all optional):
`)
	flag.PrintDefaults()
	fmt.Printf(`
For more details: https://github.com/m-manu/go-find-duplicates
`)
	os.Exit(exitCodeSuccess)
}

func setupFlags() {
	setupExclusionsOpt()
	setupHelpOpt()
	setupRemoveDuplicates()
	setupMinSizeOpt()
	setupOutputModeOpt()
	setupParallelismOpt()
	setupThoroughOpt()
	setupUsage()
	setupVersionOpt()
}

func generateRunID() string {
	return time.Now().Format("060102_150405")
}

func createReportFileIfApplicable(runID string, outputMode string) (reportFileName string) {
	switch outputMode {
	case entity.OutputModeStdOut:
		return
	case entity.OutputModeCsvFile:
		reportFileName = fmt.Sprintf("./duplicates_%s.csv", runID)
	case entity.OutputModeTextFile:
		reportFileName = fmt.Sprintf("./duplicates_%s.txt", runID)
	case entity.OutputModeJSON:
		reportFileName = fmt.Sprintf("./duplicates_%s.json", runID)
	default:
		panic("Bug in code")
	}
	f, err := os.Create(reportFileName)
	if err != nil {
		fmte.PrintfErr("error: couldn't create report file: %+v\n", err)
		os.Exit(exitCodeReportFileCreationFailed)
	}
	_ = f.Close()
	return
}

func main() {
	runID := generateRunID()
	setupFlags()
	flag.Parse()
	if flags.isHelp() {
		showHelpAndExit()
		return
	}
	if flags.getVersion() {
		fmt.Println(version)
		os.Exit(exitCodeSuccess)
	}

	defer handlePanic()

	directories := readDirectories()
	outputMode := flags.getOutputMode()
	reportFileName := createReportFileIfApplicable(runID, outputMode)
	duplicates, duplicateTotalCount, savingsSize, allFiles, fdErr := service.FindDuplicates(directories, flags.getExcludedFiles(), flags.getMinSize(),
		flags.getParallelism(), flags.isThorough())
	if fdErr != nil {
		fmte.PrintfErr("error while finding duplicates: %+v\n", fdErr)
		os.Exit(exitCodeErrorFindingDuplicates)
	}
	if duplicates == nil || duplicates.Size() == 0 {
		if len(allFiles) == 0 {
			fmte.Printf("No actions performed!\n")
		} else {
			fmte.Printf("No duplicates found!\n")
		}
		return
	}
	fmte.Printf("Found %d duplicates. A total of %s can be saved by removing them.\n",
		duplicateTotalCount, bytesutil.BinaryFormat(savingsSize))

	if err := reportDuplicates(duplicates, outputMode, allFiles, runID, reportFileName); err != nil {
		fmte.PrintfErr("error while reporting to file: %+v\n", err)
		os.Exit(exitCodeWritingToReportFileFailed)
	}

	if flags.isRemoveDuplicates() {
		if err := RemoveDuplicates(duplicates); err != nil {
			fmte.PrintfErr("remove duplicates: %+v\n", err)
		}
	}
}
