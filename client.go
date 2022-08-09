// Package pulseaudio is a pure-Go (no libpulse) implementation of the PulseAudio native protocol.
//
// Rather than exposing the PulseAudio protocol directly this library attempts to hide
// the PulseAudio complexity behind a Go interface.
// Some of the things which are deliberately not exposed in the API are:
//
// → backwards compatibility for old PulseAudio servers
//
// → transport mechanism used for the connection (Unix sockets / memfd / shm)
//
// → encoding used in the pulseaudio-native protocol
//
// Working features
//
// Querying and setting the volume.
//
// Listing audio outputs.
//
// Changing the default audio output.
//
// Notifications on config updates.
package pulseaudio

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/user"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/imdario/mergo"
)

const version = 32

var defaultAddr = fmt.Sprintf("/run/user/%d/pulse/native", os.Getuid())

type frame struct {
	buff *bytes.Buffer
	err  error
}

type request struct {
	data     []byte
	response chan<- frame
}

var (
	ErrClientClosed        = errors.New("pulseaudio client was closed")
	ErrCouldNotSendRequest = errors.New("could not send packet")
)

type Error struct {
	Cmd  string
	Code uint32
}

func (err *Error) Error() string {
	return fmt.Sprintf("pulse audio error: %s -> %s", err.Cmd, errorCodes[err.Code])
}

// ClientOpt defines a client modifier routine
type ClientOpt func(*Client)

func WithDialTimeout(timeout time.Duration) ClientOpt {
	return func(client *Client) {
		client.dialer.Timeout = timeout
	}
}

// Client maintains a connection to the PulseAudio server.
type Client struct {
	conn        net.Conn
	err         error
	clientIndex int
	requests    chan request
	updates     chan struct{}
	dialer      net.Dialer
	logger      Logger
	cancel      context.CancelFunc
	opts        Opts
}

// Opts wraps all available config options
type Opts struct {
	DialTimeout    time.Duration
	RequestTimeout time.Duration
	Logger         Logger
	Protocol       string
	Addr           string
	Cookie         string
}

// NewClient establishes a connection to the PulseAudio server.
func NewClient(opts Opts) *Client {
	defaults := Opts{
		Addr:     os.Getenv("PULSE_SERVER"),
		Protocol: "tcp",
		Cookie:   os.Getenv("PULSE_COOKIE"),
	}
	_ = mergo.Merge(opts, defaults)

	c := &Client{
		requests: make(chan request, 16),
		updates:  make(chan struct{}, 1),
		opts:     opts,
	}
	if c.opts.Addr == "" {
		c.opts.Addr = defaultAddr
	}
	if strings.HasPrefix(c.opts.Addr, "unix://") {
		c.opts.Addr = strings.TrimPrefix(c.opts.Addr, "unix://")
		c.opts.Protocol = "unix"
	}
	if c.opts.Cookie == "" {
		c.opts.Cookie = os.Getenv("HOME") + "/.config/pulse/cookie"
	}
	c.dialer.Timeout = c.opts.DialTimeout
	c.logger = c.opts.Logger

	if c.logger == nil {
		c.logger = discardLogger{}
	}
	return c
}

func (c *Client) Connect(ctx context.Context, interval time.Duration, wg *sync.WaitGroup) {
	ctx, c.cancel = context.WithCancel(ctx)
	wg.Add(1)
	go func() {
		defer wg.Done()

		c.logger.Info("starting pulseaudio connection loop")
		// start connecting whenever we are ready
		var timer *time.Timer
		for {
			err := c.connect(ctx, c.logger, wg)
			if err != nil {
				c.logger.Errorf("pulseaudio connection error: %v", err)
			}
			c.logger.Infof("reconnecting pulseaudio connection loop in %s", interval)
			if timer == nil {
				timer = time.NewTimer(interval)
			} else {
				timer.Reset(interval)
			}
			select {
			case <-ctx.Done():
				c.logger.Info("stopping pulseaudio connection loop")
				return
			case <-timer.C:
				continue
			}
		}
	}()
}

func (c *Client) init(ctx context.Context) error {
	err := c.auth(ctx, c.opts.Cookie)
	if err != nil {
		return fmt.Errorf("authentication failure: %w", err)
	}

	err = c.setName(ctx)
	if err != nil {
		return fmt.Errorf("could not send app identification data to server: %w", err)
	}
	return nil
}

func (c *Client) connect(ctx context.Context, logger Logger, wg *sync.WaitGroup) error {
	logger.Infof("dialing pulseaudio server %s://%s", c.opts.Protocol, c.opts.Addr)
	var err error
	c.conn, err = c.dialer.DialContext(ctx, c.opts.Protocol, c.opts.Addr)
	if err != nil {
		return fmt.Errorf("could not dial pulseaudio server %s: %w", c.opts.Addr, err)
	}
	defer func() { _ = c.conn.Close() }()

	// buffer init requests for processing
	initCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	err = c.init(initCtx)
	cancel()
	if err != nil {
		return fmt.Errorf("error during init: %w", err)
	}
	// start receive loop
	recv := c.receive(ctx, wg)

	pending := make(map[uint32]request)
	// cleanup pending
	defer func() {
		for _, p := range pending {
			p.response <- frame{
				buff: nil,
				err:  ErrClientClosed,
			}
		}
	}()
	err = c.handleFrames(recv, c.requests, pending, logger)
	if err != nil {
		return fmt.Errorf("frame handler error: %w", err)
	}
	return nil
}

const frameSizeMaxAllow = 1024 * 1024 * 16

func (c *Client) receive(ctx context.Context, wg *sync.WaitGroup) <-chan frame {
	// the channel will be closed when the goroutine exits
	recv := make(chan frame)
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(recv)
		for {
			if ctx.Err() != nil {
				// context cancelled
				return
			}
			var b bytes.Buffer
			_, err := io.CopyN(&b, c.conn, 4)
			if err != nil {
				recv <- frame{
					buff: &b,
					err:  fmt.Errorf("could not read header from connection: %w", err),
				}
				return
			}
			n := binary.BigEndian.Uint32(b.Bytes())
			if n > frameSizeMaxAllow {
				recv <- frame{
					buff: &b,
					err:  fmt.Errorf("response size %d is too long (only %d allowed)", n, frameSizeMaxAllow),
				}
				_, _ = io.CopyN(io.Discard, c.conn, int64(n))
				return
			}
			// the rest of the header
			b.Grow(int(n) + 20)
			if _, err = io.CopyN(&b, c.conn, int64(n)+16); err != nil {
				recv <- frame{
					buff: &b,
					err:  fmt.Errorf("could not read data from connection: %w", err),
				}
				return
			}
			b.Next(20) // skip the header
			recv <- frame{
				buff: &b,
			}
		}
	}()
	return recv
}

func (c *Client) handleFrames(in <-chan frame, out <-chan request, pending map[uint32]request, logger Logger) error {
	tag := uint32(0)
	for {
		select {
		case p, ok := <-out: // Outgoing request
			if !ok {
				// Client was closed
				logger.Info("outgoing frames channel closed; aborting frame handler routine")
				return nil
			}
			// check if request has valid format
			if len(p.data) < 26 {
				p.response <- frame{err: fmt.Errorf("request too short; minimum is 26 bytes")}
				continue
			}

			tag = nextAvailableTag(tag, pending)

			binary.BigEndian.PutUint32(p.data, uint32(len(p.data))-20)
			binary.BigEndian.PutUint32(p.data[26:], tag) // fix tag
			_, err := c.conn.Write(p.data)
			if err != nil {
				p.response <- frame{err: fmt.Errorf("couldn't send request: %s", err)}
				return fmt.Errorf("could not write to connection: %w", err)
			}
			pending[tag] = p

		case incoming, ok := <-in: // Incoming request
			if !ok {
				// Client was closed
				logger.Info("incoming frames channel closed; aborting frame handler routine")
				return nil
			}
			if incoming.err != nil {
				// this is a circuit breaker
				return fmt.Errorf("error reading incoming frame: %w", incoming.err)
			}
			var tag uint32
			var rsp command
			err := bread(incoming.buff, uint32Tag, &rsp, uint32Tag, &tag)
			if err != nil {
				// we've got a weird request from PulseAudio - that should never happen,
				// we will reset the connection
				return fmt.Errorf("received invalid pulseaudio request: %w", err)
			}
			if rsp == commandSubscribeEvent && tag == 0xffffffff {
				select {
				case c.updates <- struct{}{}:
				default:
				}
				continue
			}
			p, ok := pending[tag]
			if !ok {
				return fmt.Errorf("no pending requests for tag %d (%s)", tag, rsp)
			}
			delete(pending, tag)
			switch rsp {
			case commandError:
				var code uint32
				err = bread(incoming.buff, uint32Tag, &code)
				if err != nil {
					logger.Errorf("could not interpret error frame: %v", err)
				}
				cmd := command(binary.BigEndian.Uint32(p.data[21:]))
				incoming.err = &Error{Cmd: cmd.String(), Code: code}
				p.response <- incoming
				continue
			case commandReply:
				p.response <- incoming
				continue
			default:
				p.response <- frame{err: fmt.Errorf("expected reply (2) or error (0) but got: %s", rsp)}
			}
		}
	}
}

func nextAvailableTag(tag uint32, pending map[uint32]request) uint32 {
	// Find an unused tag
	for {
		_, exists := pending[tag]
		if !exists {
			return tag
		}
		tag++
		if tag == 0xffffffff { // reserved for subscription events
			tag = 0
		}
	}
}

func (c *Client) request(ctx context.Context, cmd command, args ...interface{}) (*bytes.Buffer, error) {
	var b bytes.Buffer
	args = append([]interface{}{uint32(0), // dummy length -- we'll overwrite at the end when we know our final length
		uint32(0xffffffff),   // channel
		uint32(0), uint32(0), // offset high & low
		uint32(0),              // flags
		uint32Tag, uint32(cmd), // command
		uint32Tag, uint32(0), // tag
	}, args...)
	err := bwrite(&b, args...)
	if err != nil {
		return nil, err
	}
	if b.Len() > frameSizeMaxAllow {
		return nil, fmt.Errorf("request size %d is too long (only %d allowed)", b.Len(), frameSizeMaxAllow)
	}
	resp := make(chan frame)

	if c.opts.RequestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.opts.RequestTimeout)
		defer cancel()
	}
	err = c.sendRequest(ctx, request{
		data:     b.Bytes(),
		response: resp,
	})
	if err != nil {
		return nil, err
	}

	select {
	case response := <-resp:
		return response.buff, response.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (c *Client) sendRequest(ctx context.Context, req request) error {
	select {
	case c.requests <- req:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return ErrCouldNotSendRequest
	}
}

func (c *Client) auth(ctx context.Context, cookiePath string) error {
	const protocolVersionMask = 0x0000FFFF
	cookie, err := ioutil.ReadFile(cookiePath)
	if err != nil {
		return err
	}
	const cookieLength = 256
	if len(cookie) != cookieLength {
		return fmt.Errorf("pulseaudio client cookie has incorrect length %d: expected %d (path %#v)",
			len(cookie), cookieLength, cookiePath)
	}
	b, err := c.request(ctx, commandAuth,
		uint32Tag, uint32(version),
		arbitraryTag, uint32(len(cookie)), cookie)
	if err != nil {
		return err
	}
	var serverVersion uint32
	err = bread(b, uint32Tag, &serverVersion)
	if err != nil {
		return err
	}
	serverVersion &= protocolVersionMask
	if serverVersion < version {
		return fmt.Errorf("pulseaudio server supports version %d but minimum required is %d", serverVersion, version)
	}
	return nil
}

func (c *Client) setName(ctx context.Context) error {
	props := map[string]string{
		"application.name":           path.Base(os.Args[0]),
		"application.process.id":     fmt.Sprintf("%d", os.Getpid()),
		"application.process.binary": os.Args[0],
		"application.language":       "en_US.UTF-8",
		"window.x11.display":         os.Getenv("DISPLAY"),
	}
	if current, err := user.Current(); err == nil {
		props["application.process.user"] = current.Username
	}
	if hostname, err := os.Hostname(); err == nil {
		props["application.process.host"] = hostname
	}
	b, err := c.request(ctx, commandSetClientName, props)
	if err != nil {
		return err
	}
	var clientIndex uint32
	err = bread(b, uint32Tag, &clientIndex)
	if err != nil {
		return err
	}
	c.clientIndex = int(clientIndex)
	return nil
}

func (c *Client) Close() {
	close(c.requests)
	close(c.updates)
	// stop main connection loop (this also disconnects current connection)
	if c.cancel != nil {
		c.cancel()
	}
}
