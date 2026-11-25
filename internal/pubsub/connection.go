package pubsub

import (
	"crypto/tls"
	"fmt"
	"github.com/gorilla/websocket"
	"sync"
	"sync/atomic"
	"time"

	"TwitchChannelPointsMiner/internal/random"
	"TwitchChannelPointsMiner/internal/twitchapi"
)

const (
	// messageWorkers handle incoming messages so a slow one does not stall the socket.
	messageWorkers = 4
	// pongTimeout is how long the connection may go without a PONG before it is replaced.
	pongTimeout = 5 * time.Minute
)

// connectAndListen opens one PubSub connection, subscribes it to topics and serves
// it until stop is closed or the connection has to be replaced.
func (p *Client) connectAndListen(connIndex int, topics []string, stop <-chan struct{}) error {
	conn, err := p.dial()
	if err != nil {
		return err
	}

	var lastPong atomic.Pointer[time.Time]
	connectedAt := time.Now()
	lastPong.Store(&connectedAt)
	pingTimer := time.NewTimer(p.randomPingInterval())
	defer pingTimer.Stop()

	msgCh := make(chan []byte, 256)
	workerWG := p.startWorkers(msgCh, stop)

	var readWG sync.WaitGroup
	defer func() {
		conn.Close()
		readWG.Wait()
		close(msgCh)
		workerWG.Wait()
	}()

	if err := p.listenTopics(conn, topics); err != nil {
		return err
	}

	p.logger.Printf("Connected to Twitch PubSub (conn #%d) with %d topic(s)", connIndex, len(topics))

	readErr := make(chan error, 1)
	readWG.Add(1)
	go func() {
		defer readWG.Done()
		p.readMessages(conn, connIndex, msgCh, readErr, &lastPong)
	}()

	return p.keepAlive(conn, pingTimer, stop, readErr, &lastPong)
}

func (p *Client) dial() (*websocket.Conn, error) {
	dialer := *websocket.DefaultDialer
	if p.disableSSL {
		dialer.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec
	}
	endpoint := p.endpoint
	if endpoint == "" {
		endpoint = twitchapi.WebsocketURL
	}
	conn, _, err := dialer.Dial(endpoint, nil)
	return conn, err
}

// startWorkers handles queued messages until msgCh is closed or stop is.
func (p *Client) startWorkers(msgCh <-chan []byte, stop <-chan struct{}) *sync.WaitGroup {
	var workers sync.WaitGroup
	workers.Add(messageWorkers)
	for i := 0; i < messageWorkers; i++ {
		go func() {
			defer workers.Done()
			for {
				select {
				case <-stop:
					return
				case raw, ok := <-msgCh:
					if !ok {
						return
					}
					if err := p.handleMessage(raw, nil); err != nil {
						p.logger.Errorf("PubSub message error: %v", err)
					}
				}
			}
		}()
	}
	return &workers
}

// keepAlive pings on a jittered interval and returns when stop is closed, reading
// failed, or no PONG has arrived for too long.
func (p *Client) keepAlive(conn *websocket.Conn, pingTimer *time.Timer, stop <-chan struct{}, readErr <-chan error, lastPong *atomic.Pointer[time.Time]) error {
	for {
		select {
		case <-stop:
			return nil
		case <-pingTimer.C:
			if err := conn.WriteJSON(map[string]string{"type": "PING"}); err != nil {
				return err
			}
			if time.Since(*lastPong.Load()) > pongTimeout {
				return fmt.Errorf("last PONG >5m ago, reconnecting")
			}
			pingTimer.Reset(p.randomPingInterval())
		case err := <-readErr:
			return err
		}
	}
}

func (p *Client) randomPingInterval() time.Duration {
	return time.Duration(random.Int(25, 30)) * time.Second
}
