package main

import (
	"toolkitgo/logger"
)

func main() {
	logger.Warning.Println("asfasf")

	logger.Default.Println("something in my heart")

	mylogger := logger.NewFileLogger("{==}", "mytestlogger", true)
	mylogger.Println("asfasfasfafafwwwwwwwwwwwwwwwww")

	mylogger2 := logger.NewFileLogger("{}", "mylog.txt", true)
	mylogger2.Println("mylogger2")
}
