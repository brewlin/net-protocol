package logging

import (
	"log"
	"os"
)

// Setup initialize the log instance
func Setup() {
	file := "./log.txt"
	logFile, err := os.OpenFile(file, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0766)
	if err != nil {
		panic(err)
	}
	log.SetOutput(logFile)
	log.SetFlags(log.Lshortfile)
	return
}
func init(){
	Setup()
}
