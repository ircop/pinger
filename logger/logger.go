package logger

import (
	"fmt"
	"log"
	"os"
	"time"
)

type logger struct {
	DebugEnabled bool
	DebugLock    bool
	LogPath      string
	LogFile      *os.File
}

var mlog = logger{
	DebugEnabled: true,
	LogPath:      "",
	LogFile:      nil,
	//DebugLock:	  true,
	DebugLock: false,
}

/*
Write - write log
*/
func (l *logger) Write(format string, args ...interface{}) {
	t := time.Now()
	tm := fmt.Sprintf("[%d.%02d.%02d %02d:%02d:%02d]: ", t.Day(), t.Month(), t.Year(), t.Hour(), t.Minute(), t.Second())

	fmt.Printf(tm+format+"\n", args...)
	if mlog.LogFile != nil {
		log.Printf(format+"\n", args...)
	}
}

/*
WriteAsIs - write log without any formatting
*/
func (l *logger) WriteAsIs(format string, args ...interface{}) {
	if mlog.LogFile != nil {
		//fmt.Printf(format, args...)
		log.Printf(format, args...)
	}
}

/*
Log - normal log (info)
*/
func Log(format string, args ...interface{}) {
	mlog.Write("[INFO]: "+format, args...)
}

/*
DebugAsIs - debug without any formatting
*/
func DebugAsIs(format string, args ...interface{}) {
	if !mlog.DebugEnabled {
		return
	}
	mlog.WriteAsIs(format, args...)
}

/*
Debug - write in debug mode
*/
func Debug(format string, args ...interface{}) {
	if !mlog.DebugEnabled {
		return
	}
	mlog.Write("[DEBUG]: "+format, args...)
}

/*
Err - write error
*/
func Err(format string, args ...interface{}) {
	mlog.Write("[ERROR]: "+format, args...)
}

/*
DebugLock - dedicated debug for locks/unlocks
 */
func DebugLock(format string, args ...interface{}) {
	if mlog.DebugLock {
		mlog.Write("[DEBUG:LOCK]: "+format, args...)
	}
}

/*
SetDebug - enable/disable debug
*/
func SetDebug(val bool) {
	mlog.DebugEnabled = val
}

/*
SetPath - set logfile path
*/
func SetPath(path string) {
	mlog.LogPath = path

	if mlog.LogPath != "" {
		var err error
		mlog.LogFile, err = os.OpenFile(mlog.LogPath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0600)
		if err != nil {
			log.Fatalf("Cannot open logfile %s!", mlog.LogPath)
		}

		log.SetOutput(mlog.LogFile)
	}
}
