package eppserver

import (
	"crypto/tls"
	"net"
	"time"
)

type HandlerFunc func(*Session, []byte) ([]byte, error)
type GreetFunc func(*Session) ([]byte, error)

// Session is an active connection to the EPP server.
type Session struct {
	stopChan chan struct{}
	conn     net.Conn
	handler  HandlerFunc
	greeting GreetFunc

	Data            map[string]interface{}
	SessionID       string
	SessionTimeout  time.Duration
	IdleTimeout     time.Duration
	ConnectionState func() tls.ConnectionState
}

// NewSession will create a new Session.
func NewSession(conn net.Conn, handler HandlerFunc, greeting GreetFunc, tlsStateFunc func() tls.ConnectionState, sessionID string) *Session {
	s := &Session{
		stopChan:        make(chan struct{}),
		conn:            conn,
		handler:         handler,
		greeting:        greeting,
		Data:            map[string]interface{}{},
		SessionID:       sessionID,
		SessionTimeout:  1 * time.Hour,
		IdleTimeout:     10 * time.Minute,
		ConnectionState: tlsStateFunc,
	}

	return s
}

// run will start the session.
func (s *Session) run() error {
	defer s.conn.Close()

	response, err := s.greeting(s)
	if err != nil {
		// TODO: Write response code and message?
		return err
	}

	err = WriteMessage(s.conn, response)
	if err != nil {
		return err
	}

	sessionTimeout := time.After(s.SessionTimeout)
	idleTimeout := time.After(s.IdleTimeout)

	for {
		select {
		case <-s.stopChan:
			return nil
		case <-sessionTimeout:
			return nil
		case <-idleTimeout:
			return nil
		default:
			// Go on...
		}

		err = s.conn.SetDeadline(time.Now().Add(1 * time.Second))
		if err != nil {
			return err
		}

		message, err := ReadMessage(s.conn)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}

			return err
		}

		// Handle Message:
		response, err = s.handler(s, message)
		if err != nil {
			return err
		}

		err = WriteMessage(s.conn, response)
		if err != nil {
			return err
		}

		// Extend the idle timeout.
		idleTimeout = time.After(s.IdleTimeout)
	}
}

// Close will tell the session to close.
func (s *Session) Close() error {
	close(s.stopChan)
	return nil
}