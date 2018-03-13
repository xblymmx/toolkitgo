package logger

import (
	"log"
	"os"
	"io"
	"strings"
)

var (
	Info    *log.Logger
	Error   *log.Logger
	Warning *log.Logger
	Default *log.Logger // write to both default log-file and stdout
)

func init() {
	errFile, err := os.OpenFile("error.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalln("Cannot open error file failed:", err)
	}
	logFile, err := os.OpenFile("defaultlog.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalln("Cannot open error file failed:", err)
	}

	Info = log.New(os.Stdout, "[Info]", log.Ldate|log.Ltime|log.Lshortfile)
	Warning = log.New(os.Stdout, "[Warning]", log.Ldate|log.Ltime|log.Lshortfile)
	Error = log.New(io.MultiWriter(os.Stderr, errFile), "[Error]", log.Ldate|log.Ltime|log.Lshortfile)
	Default = log.New(logFile, "[Default]", log.Ldate|log.Ltime|log.Lshortfile)
}

func NewFileLogger(prefix string, filename string, isWriteToStdout bool) *log.Logger {
	if !strings.Contains(filename, ".") {
		filename = filename + ".log"
	}

	logFile, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalln("Cannot open error file failed:", err)
	}

	if isWriteToStdout {
		return log.New(io.MultiWriter(os.Stdout, logFile), prefix, log.Ldate|log.Ltime|log.Lshortfile)
	} else {
		return log.New(logFile, prefix, log.Ldate|log.Ltime|log.Lshortfile)
	}
}
