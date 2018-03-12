package logger

import (
	"fmt"
	"time"
	"os"
	"log"
)

type logger struct {
	DebugEnabled	bool
	LogPath			string
	LogFile			*os.File
}

var mlog = logger{DebugEnabled: true, LogPath: ""}

func (l *logger) Write(format string, args ...interface{}) {
	t := time.Now()
	tm := fmt.Sprintf("[%d.%02d.%02d %02d:%02d:%02d]: ", t.Day(), t.Month(), t.Year(), t.Hour(), t.Minute(), t.Second())

	fmt.Printf(tm + format + "\n", args...)
	if mlog.LogFile != nil {
		log.Printf(format+"\n", args...)
	}
}

func (l *logger) WriteAsIs(format string, args ...interface{}){
	if mlog.LogFile != nil {
		//fmt.Printf(format, args...)
		log.Printf(format, args...)
	}
}

func Log(format string, args ...interface{}) {
	mlog.Write("[INFO]: " + format, args...)
}

func DebugAsIs( format string, args ...interface{}) {
	if mlog.DebugEnabled == false {
		return
	}
	mlog.WriteAsIs(format, args...)
}

func Debug(format string, args ...interface{}) {
	if mlog.DebugEnabled == false {
		return
	}
	mlog.Write("[DEBUG]: " + format, args...)
}

func Err(format string, args ...interface{}) {
	mlog.Write("[ERROR]: " + format, args...)
}

func SetDebug(val bool) {
	mlog.DebugEnabled = val
}
func SetPath(path string) {
	mlog.LogPath = path

	if( mlog.LogPath != "" ) {
		var err error
		mlog.LogFile, err = os.OpenFile(mlog.LogPath, os.O_RDWR | os.O_CREATE | os.O_APPEND, 0666)
		if err != nil {
			log.Fatalf("Cannot open logfile %s!", mlog.LogPath)
		}

		log.SetOutput(mlog.LogFile)
	}
}
func CloseLog() {

}
