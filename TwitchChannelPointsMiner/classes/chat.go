package classes

import (
	"bufio"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"TwitchChannelPointsMiner/TwitchChannelPointsMiner/constants"
	"TwitchChannelPointsMiner/TwitchChannelPointsMiner/privacy"
)

type ChatLogger interface {
	Printf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
	EmojiPrintf(emoji, format string, args ...interface{})
	EmojiEventf(emoji string, event constants.Event, format string, args ...interface{})
}

type ChatClient struct {
	username            string
	channel             string
	token               string
	logger              ChatLogger
	anonymizer          *privacy.Anonymizer
	disableAtInNickname bool
	stop                chan struct{}
	stopped             chan struct{}
	conn                net.Conn
	connMu              sync.Mutex
}

func NewChatClient(username, token, channel string, logger ChatLogger, disableAtInNickname bool, anonymizer *privacy.Anonymizer) *ChatClient {
	return &ChatClient{
		username:            strings.ToLower(strings.TrimSpace(username)),
		channel:             strings.ToLower(strings.TrimSpace(channel)),
		token:               strings.TrimSpace(token),
		logger:              logger,
		anonymizer:          anonymizer,
		disableAtInNickname: disableAtInNickname,
		stop:                make(chan struct{}),
		stopped:             make(chan struct{}),
	}
}

func (c *ChatClient) Start() {
	go c.run()
}

func (c *ChatClient) Stop() {
	select {
	case <-c.stop:
	default:
		close(c.stop)
		c.closeConn()
	}
	<-c.stopped
}

func (c *ChatClient) run() {
	defer close(c.stopped)
	for {
		if err := c.connectAndListen(); err != nil && !c.isStopped() && c.logger != nil {
			if c.anonymizer != nil && c.anonymizer.Enabled() {
				c.logger.Errorf("chat error: %v", err)
			} else {
				c.logger.Errorf("chat #%s error: %v", c.channel, err)
			}
		}
		if c.isStopped() {
			return
		}
		time.Sleep(5 * time.Second)
	}
}

func (c *ChatClient) connectAndListen() error {
	if c.username == "" || c.channel == "" || c.token == "" {
		return fmt.Errorf("missing chat credentials")
	}

	addr := net.JoinHostPort(constants.IRC, strconv.Itoa(constants.IRCPort))
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return err
	}
	c.setConn(conn)
	defer c.closeConn()

	if err := c.write(fmt.Sprintf("PASS oauth:%s\r\n", c.token)); err != nil {
		return err
	}
	if err := c.write(fmt.Sprintf("NICK %s\r\n", c.username)); err != nil {
		return err
	}
	if err := c.write(fmt.Sprintf("JOIN #%s\r\n", c.channel)); err != nil {
		return err
	}

	reader := bufio.NewReader(conn)
	for {
		if err := conn.SetReadDeadline(time.Now().Add(90 * time.Second)); err != nil {
			return err
		}
		line, err := reader.ReadString('\n')
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				if c.isStopped() {
					return nil
				}
				continue
			}
			return err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "PING") {
			payload := strings.TrimPrefix(line, "PING")
			_ = c.write("PONG" + payload + "\r\n")
			continue
		}
		c.handleLine(line)
		if c.isStopped() {
			return nil
		}
	}
}

func (c *ChatClient) handleLine(line string) {
	if strings.Contains(strings.ToLower(line), "authentication failed") {
		if c.logger != nil {
			if c.anonymizer != nil && c.anonymizer.Enabled() {
				c.logger.Printf("chat authentication failed")
			} else {
				c.logger.Printf("chat #%s authentication failed", c.channel)
			}
		}
		c.closeConn()
		return
	}
	if !strings.Contains(line, "PRIVMSG") {
		return
	}
	parts := strings.SplitN(line, " :", 2)
	if len(parts) != 2 {
		return
	}
	msg := parts[1]
	prefix := parts[0]
	nick := ""
	if strings.HasPrefix(prefix, ":") {
		prefix = strings.TrimPrefix(prefix, ":")
		if idx := strings.Index(prefix, "!"); idx > 0 {
			nick = prefix[:idx]
		}
	}

	msgLower := strings.ToLower(msg)
	target := "@" + c.username
	mentioned := strings.Contains(msgLower, target)
	if c.disableAtInNickname && !mentioned {
		mentioned = strings.Contains(msgLower, c.username)
	}
	if !mentioned {
		return
	}
	if c.logger != nil {
		if c.anonymizer != nil && c.anonymizer.Enabled() {
			channel := c.channel
			if channel != "" {
				channel = c.anonymizer.Name(channel)
			}
			c.logger.EmojiEventf(":speech_balloon:", constants.EventChatMention, "Chat mention in %s", channel)
		} else {
			displayNick := nick
			if displayNick == "" {
				displayNick = "unknown"
			}
			c.logger.EmojiEventf(":speech_balloon:", constants.EventChatMention, "%s at #%s wrote: %s", displayNick, c.channel, msg)
		}
	}
}

func (c *ChatClient) write(msg string) error {
	c.connMu.Lock()
	conn := c.conn
	c.connMu.Unlock()
	if conn == nil {
		return fmt.Errorf("no active chat connection")
	}
	_, err := conn.Write([]byte(msg))
	return err
}

func (c *ChatClient) closeConn() {
	c.connMu.Lock()
	if c.conn != nil {
		_ = c.conn.Close()
		c.conn = nil
	}
	c.connMu.Unlock()
}

func (c *ChatClient) setConn(conn net.Conn) {
	c.connMu.Lock()
	c.conn = conn
	c.connMu.Unlock()
}

func (c *ChatClient) isStopped() bool {
	select {
	case <-c.stop:
		return true
	default:
		return false
	}
}
