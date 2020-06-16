// Copyright (C) 2020 Markus L. Noga
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.


package internal

import (
	"bufio"
	"fmt"
	"os"
)

// Singleton log writer. Writes to stdout, and optionally to a file.
// Does not add prefixes, or force newlines.

// The optional additional file to log into
var logFile   *bufio.Writer
var logFileOS *os.File

// Enables logging to file
func LogAlsoToFile(fileName string) (err error) {
	if logFile!=nil { 
		err=logFile.Flush() 
		if err!=nil { return err }
		err=logFileOS.Close() 
		if err!=nil { return err }
	}
	logFileOS, err = os.OpenFile(fileName, os.O_CREATE | os.O_TRUNC | os.O_WRONLY, 0666)
	logFile=bufio.NewWriter(logFileOS)
	return nil
}

func LogPrint(args ...interface{}) (n int, err error) {
	n, err=fmt.Print(args...)
	if err!=nil || logFile==nil { return n, err }
	return fmt.Fprint(logFile, args...)
}

func LogPrintln(args ...interface{}) (n int, err error) {
	n, err=fmt.Println(args...)
	if err!=nil || logFile==nil { return n, err }
	return fmt.Fprintln(logFile, args...)
}

func LogPrintf(format string, args ...interface{}) (n int, err error) {
	n, err=fmt.Printf(format, args...)
	if err!=nil || logFile==nil { return n, err }
	return fmt.Fprintf(logFile, format, args...)
}

func LogFatal(args ...interface{}) {
	fmt.Println(args...)
	if logFile!=nil { 
		fmt.Fprint(logFile, args...)
		logFile.Flush()
		logFileOS.Close()
	}
	os.Exit(1)
}

func LogFatalf(format string, args ...interface{}) {
	fmt.Printf(format, args...)
	if logFile!=nil { 
		fmt.Fprintf(logFile, format, args...)
		logFile.Flush()
		logFileOS.Close()
	}
	os.Exit(1)
}

func LogSync() {
	logFile.Flush()
	logFileOS.Sync()
}
