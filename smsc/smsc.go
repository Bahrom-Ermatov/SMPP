package smsc

import (
	"smpp/rabbit"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/go-pg/pg/v9"
	"github.com/linxGnu/gosmpp"
	"github.com/linxGnu/gosmpp/data"
	"github.com/linxGnu/gosmpp/pdu"
)

// Session is main session
type Session struct {
	trans *gosmpp.TransceiverSession
	c     chan *pdu.SubmitSM
	db    *pg.DB
}

// NewSession returns new session object
func NewSession(db *pg.DB) *Session {
	auth := gosmpp.Auth{
		SMSC:       "127.0.0.1:2775", //os.Getenv("SMSC_HOST"),     // ,
		SystemID:   "522241",         //os.Getenv("SMSC_LOGIN"),    // ,
		Password:   "UUDHWB",         //os.Getenv("SMSC_PASSWORD"), // ,
		SystemType: "",
	}

	trans, err := gosmpp.NewTransceiverSession(gosmpp.NonTLSDialer, auth, gosmpp.TransceiveSettings{
		// EnquireLink: 5 * time.Second,

		OnSubmitError: func(p pdu.PDU, err error) {
			log.Errorf("OnSubmitError: %v", err)
		},

		OnReceivingError: func(err error) {
			log.Errorf("OnReceivingError: %v", err)
		},

		OnRebindingError: func(err error) {
			log.Errorf("OnRebindingError: %v", err)
		},

		OnPDU: handlePDU(db),

		OnClosed: func(state gosmpp.State) {
			log.Errorf("OnClosed: %v", state)
		},
	}, 5*time.Second)
	if err != nil {
		log.Fatal(err)
	}

	return &Session{
		trans: trans,
		c:     make(chan *pdu.SubmitSM),
		db:    db,
	}
}

// SendAndReceiveSMS to smsc
func (s *Session) SendAndReceiveSMS() {
	for n := range s.c {
		if err := s.trans.Transceiver().Submit(n); err != nil {
			log.Errorf("cannot submit message: %v", err)
		}
	}
}

func handlePDU(db *pg.DB) func(pdu.PDU, bool) {
	return func(p pdu.PDU, responded bool) {
		switch pd := p.(type) {
		case *pdu.SubmitSMResp:
			log.Info("Step PDU SubmitSMResp")
			message := new(rabbit.Message)

			//Меняем статус сообщения на 2 (доставлено)
			var clntId int32
			var price float32
			if _, err := db.Model(message).
				Set("smsc_message_id = ?, state = ?, last_updated_date = ?", pd.MessageID, rabbit.StateDelivered, time.Now()).
				Where("id = ?", pd.GetSequenceNumber()).
				Returning("clnt_id, price").
				Update(&clntId, &price); err != nil {
				log.Errorf("cannot update message: %v", err)
				return
			}

			//Снимаем деньги с баланса
			if _, err := db.Model((*rabbit.ClientBalance)(nil)).
				Set("balance_sum = balance_sum - ?", price).
				Where("clnt_id = ?", clntId).
				Update(); err != nil {
				log.Errorf("cannot update balance: %v", err)
				return
			}

		case *pdu.GenerickNack:
			log.Info("Step PDU GenerickNack")
			//			log.Infof("SubmitSMResp:%+v sequence number \n", pd.GetSequenceNumber())
			//			log.Info("GenericNack Received")
		case *pdu.EnquireLinkResp:
			log.Info("Step PDU EnquireLinkResp")
			//			log.Info("EnquireLinkResp Received")
		case *pdu.DataSM:
			log.Info("Step PDU DataSM")
			//			log.Infof("DataSM:%+v\n", pd)
		case *pdu.DeliverSM:
			log.Info("Step PDU DeliverSM")
			//			log.Infof("DeliverSM:%+v\n", pd)
		case *pdu.QuerySM:
			log.Info("Step PDU QuerySM")
			//			log.Info("QuerySM")
			//			log.Info(pd)
		case *pdu.QuerySMResp:
			log.Info("Step PDU QuerySMResp")
			//			log.Info("QuerySMResp")
			//			log.Info(pd)
		default:
			log.Info("Step PDU Default")
			//			log.Info("Default ")
			//			log.Info(pd)
		}
	}
}

// SubmitSM submit new short message
func (s *Session) SubmitSM(c <-chan rabbit.Message) {
	for m := range c {
		srcAddr := pdu.NewAddress()
		srcAddr.SetTon(5)
		srcAddr.SetNpi(0)
		_ = srcAddr.SetAddress(m.Src)

		destAddr := pdu.NewAddress()
		destAddr.SetTon(1)
		destAddr.SetNpi(1)
		_ = destAddr.SetAddress(m.Dst)

		submitSM := pdu.NewSubmitSM().(*pdu.SubmitSM)
		submitSM.SourceAddr = srcAddr
		submitSM.DestAddr = destAddr
		_ = submitSM.Message.SetMessageWithEncoding(m.Message, data.UCS2)
		submitSM.ProtocolID = 0
		submitSM.RegisteredDelivery = 1
		submitSM.ReplaceIfPresentFlag = 0
		submitSM.EsmClass = 0
		submitSM.SetSequenceNumber(m.ID)
		s.c <- submitSM
	}
}

// QuerySM make query to smsc about state of sms by message id
func (s *Session) QuerySM() {
	q := pdu.NewQuerySM()
	q.(*pdu.QuerySM).MessageID = "0cacd4da82d8c2df76ebfd60a8a5ffa498d4f7b1259e17133ff5ed2db89befdc"
	a := pdu.NewAddress()
	a.SetTon(5)
	a.SetNpi(0)
	//	a.SetAddress("oasis")

	q.(*pdu.QuerySM).SourceAddr = a
	if err := s.trans.Transceiver().Submit(q); err != nil {
		log.Fatalf("cannot send query sm %v", err)
	}
}

// Close session
func (s *Session) Close() {
	s.trans.Close()
}
