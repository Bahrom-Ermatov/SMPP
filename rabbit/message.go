package rabbit

import "time"

type state int

// Message states
const (
	StateNew state = iota + 1
	StateDelivered
	StateNotDelivered
)

// Message format to share between sender and consumer
type Message struct {
	ID              int32
	ExternalID      int32
	Dst             string
	Message         string
	Src             string
	State           state
	CreatedAt       *time.Time
	LastUpdatedDate *time.Time
	SMSCMessageID   string
	Price           float32
	ClntId          int32
}

type ClientBalance struct {
	ClntId     int
	BalanceSum float32
}
