package rabbit

import (
	"encoding/json"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/go-pg/pg/v9"
	"github.com/pkg/errors"
	"github.com/streadway/amqp"
)

// Channel for sending and receiving sms's
const (
	SMSChannel  string = "SMS_CHANNEL"
	SMSConsumer string = "SMS_CONSUMER"
)

// Session session for rabbitmq
type Session struct {
	connection *amqp.Connection
	queue      amqp.Queue
	channel    *amqp.Channel
	done       chan bool
	db         *pg.DB
}

// NewSession return new rabbitmq session
func NewSession(db *pg.DB) (*Session, error) {
	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	if err != nil {
		return nil, errors.Wrap(err, "cannot connect to rabbitmq")
	}

	ch, err := conn.Channel()
	if err != nil {
		return nil, errors.Wrap(err, "cannot create new channel")
	}

	q, err := ch.QueueDeclare(
		SMSChannel, // name
		true,       // durable
		false,      // delete when unused
		false,      // exclusive
		false,      // no-wait
		nil,        // arguments
	)

	s := &Session{
		connection: conn,
		channel:    ch,
		queue:      q,
		done:       make(chan bool),
		db:         db,
	}

	return s, nil
}

// Consume start consuming from SMSChannel
func (s *Session) Consume(c chan<- Message) {
	msgs, err := s.channel.Consume(
		s.queue.Name, // queue
		SMSConsumer,  // consumer
		false,        // auto-ack
		true,         // exclusive
		false,        // no-local
		false,        // no-wait
		nil,          // args
	)

	if err != nil {
		log.Fatalf("cannot consume from channel %v", err)
	}

	for d := range msgs {
		message := Message{}
		if err := json.Unmarshal(d.Body, &message); err != nil {
			log.Errorf("cannot unmarshal message: %v", err)
			if err := d.Nack(false, true); err != nil {
				log.Errorf("cannot nack message: %v", err)
			}
		}
		log.Info("Step consume")

		if _, err := s.db.Model(&message).
			Set("state = ?, last_updated_date = ?", StateNew, time.Now()).
			Where("id = ?", message.ID).Update(); err != nil {
			log.Errorf("cannot update message: %v", err)
			d.Nack(false, true)
		}

		if err := d.Ack(false); err != nil {
			log.Error("cannot ack message")
		}
		log.Info("Step 1 message = ", message)

		c <- message
	}
	<-s.done
}

// Close the session and cleanup
func (s *Session) Close() {
	s.channel.Close()
	s.connection.Close()
	s.done <- true
}
