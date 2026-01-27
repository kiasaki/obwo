package log

import (
	"log"
	"os"
)

var logger *log.Logger

func init() {
	Setup("unknown")
}

func Setup(serviceName string) {
	logger = log.New(os.Stderr, serviceName+" ", log.Ldate|log.Ltime|log.LUTC)
}

func Log(format string, v ...any) {
	logger.Printf(format+"\n", v...)
}
