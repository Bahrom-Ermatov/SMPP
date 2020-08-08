package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"smpp/rabbit"
	"smpp/smsc"

	"github.com/go-pg/pg/v9"
	"gopkg.in/natefinch/lumberjack.v2"
)

var db *pg.DB
var messages chan rabbit.Message

const logFilePath = "logs/smpp.log"
const appVersion = "0.0.1"

func main() {

	lumberjackLogRotate := &lumberjack.Logger{
		Filename:   logFilePath,
		MaxSize:    2,   // Max megabytes before log is rotated
		MaxBackups: 500, // Max number of old log files to keep
		MaxAge:     60,  // Max number of days to retain log files
		Compress:   true,
	}
	log.SetOutput(lumberjackLogRotate)

	db = pg.Connect(&pg.Options{
		Addr:     "localhost:5432",
		Database: "messages",
		User:     "postgres",
		Password: "123",
	})

	sigs := make(chan os.Signal)
	signal.Notify(sigs, os.Interrupt, os.Kill)
	messages := make(chan rabbit.Message)

	s := smsc.NewSession(db)

	rs, err := rabbit.NewSession(db)
	if err != nil {
		log.Fatalf("cannot get rabbit session")
	}

	go s.SendAndReceiveSMS()
	go rs.Consume(messages)
	go s.SubmitSM(messages)

	fmt.Println("awaiting signal")

	<-sigs
	s.Close()
	rs.Close()
}
