// Package pollclient implements the worker side of the Spacelift HTTP
// long-poll transport. One Client serves one worker pool and runs a poll loop
// per worker. Messages are dispatched before they are acked (at-least-once),
// and every request carries a freshly signed pool-key JWT.
package pollclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/spacelift-io/workercomms/messages"
	"github.com/spacelift-io/workercomms/pollcomms"
)

// MinLongPoll is the shortest long-poll hold.
const MinLongPoll = 5 * time.Second

const (
	errorBackoff = 2 * time.Second
	// A worker that keeps failing must not out-poll a healthy idle one.
	maxErrorBackoff    = pollcomms.MaxPollHold
	pollDeadlineBuffer = 5 * time.Second
	ackTimeout         = 10 * time.Second
	shutdownTimeout    = 3 * time.Second
	maxErrorBody       = 1024
)

// ErrWorkerBusy is returned when the backend answers a retire with 409: a
// workload is attached and is delivered on the worker's next poll.
var ErrWorkerBusy = errors.New("worker holds a workload")

// ErrRetireUnsupported is returned when the backend answers a retire with 404.
var ErrRetireUnsupported = errors.New("backend does not support retiring workers")

// Options configures a Client for a single worker pool.
type Options struct {
	CommsURL string
	PoolULID string
	// PoolPrivateKey is the pool's RSA private key in PEM form.
	PoolPrivateKey []byte
	// LongPoll is the requested poll hold; zero means pollcomms.MaxPollHold.
	LongPoll time.Duration
	// HTTPClient is optional; when nil the Client owns a tuned one.
	HTTPClient *http.Client
	// Logger is optional; poll failures log at the Error level.
	Logger messages.Logger
}

// Client runs the poll loops for one worker pool's workers.
type Client struct {
	poolULID string
	poolKey  []byte
	longPoll time.Duration
	handler  messages.PayloadHandler
	logger   messages.Logger

	client     *http.Client
	ownsClient bool

	pollURL   string
	ackURL    string
	goneURL   string
	retireURL string

	mu      sync.Mutex
	workers map[string]*poller
	closed  bool
}

// New validates the options and builds a Client. It performs no I/O.
func New(opts Options, handler messages.PayloadHandler) (*Client, error) {
	if handler == nil {
		return nil, errors.New("poll client requires a payload handler")
	}

	base, err := url.Parse(opts.CommsURL)
	if err != nil {
		return nil, errors.Wrap(err, "could not parse poll comms URL")
	}
	if base.Scheme == "" || base.Host == "" {
		return nil, errors.Errorf("poll comms URL %q must include a scheme and host", opts.CommsURL)
	}

	longPoll := opts.LongPoll
	if longPoll == 0 {
		longPoll = pollcomms.MaxPollHold
	}
	if longPoll < MinLongPoll {
		return nil, errors.Errorf("long-poll timeout must be at least %s, got %s", MinLongPoll, longPoll)
	}

	if len(opts.PoolPrivateKey) == 0 {
		return nil, errors.New("poll client requires the pool private key")
	}
	if _, err := pollcomms.Sign(opts.PoolPrivateKey, opts.PoolULID, "validation"); err != nil {
		return nil, errors.Wrap(err, "could not sign poll token with pool key")
	}

	logger := opts.Logger
	if logger == nil {
		logger = nopLogger{}
	}

	client, ownsClient := opts.HTTPClient, false
	if client == nil {
		client, ownsClient = defaultClient(longPoll), true
	}

	commsURL := strings.TrimSuffix(opts.CommsURL, "/")

	return &Client{
		poolULID:   opts.PoolULID,
		poolKey:    opts.PoolPrivateKey,
		longPoll:   longPoll,
		handler:    handler,
		logger:     logger,
		client:     client,
		ownsClient: ownsClient,
		pollURL:    commsURL + pollcomms.URLPoll + "?wait_seconds=" + strconv.FormatInt(int64(longPoll/time.Second), 10),
		ackURL:     commsURL + pollcomms.URLAck,
		goneURL:    commsURL + pollcomms.URLGone,
		retireURL:  commsURL + pollcomms.URLRetire,
		workers:    make(map[string]*poller),
	}, nil
}

func defaultClient(longPoll time.Duration) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = 0
	transport.MaxIdleConnsPerHost = 256
	transport.IdleConnTimeout = 90 * time.Second
	transport.TLSHandshakeTimeout = 10 * time.Second
	return &http.Client{
		Timeout:   longPoll + pollDeadlineBuffer,
		Transport: transport,
	}
}

// StartWorker starts the poll loop for workerULID and does nothing if one is
// already running. Errors after Close.
func (c *Client) StartWorker(workerULID string, request pollcomms.PollRequest) error {
	if _, err := json.Marshal(request); err != nil {
		return errors.Wrap(err, "could not marshal poll request")
	}

	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return errors.New("poll client is closed")
	}
	if _, ok := c.workers[workerULID]; ok {
		c.mu.Unlock()
		return nil
	}

	loopCtx, cancel := context.WithCancel(context.Background())
	p := &poller{
		client:     c,
		workerULID: workerULID,
		request:    request,
		cancel:     cancel,
	}
	c.workers[workerULID] = p
	c.mu.Unlock()

	go p.run(loopCtx)
	return nil
}

// SetAvailable sets whether workerULID can take work. A change interrupts the
// in-flight poll so the next poll reports it at once; the same value sends
// nothing. Safe to call from inside HandlePayload.
func (c *Client) SetAvailable(workerULID string, available bool) {
	c.mu.Lock()
	p, ok := c.workers[workerULID]
	c.mu.Unlock()
	if !ok {
		return
	}

	p.mu.Lock()
	changed := p.request.Available == nil || *p.request.Available != available
	p.request.Available = &available
	cancel := p.pollCancel
	p.mu.Unlock()

	if changed && cancel != nil {
		cancel()
	}
}

// StopWorker cancels workerULID's poll loop, if any, and always reports the
// shutdown via /worker/gone — even for unknown workers and after Close.
func (c *Client) StopWorker(workerULID string) error {
	c.mu.Lock()
	p, ok := c.workers[workerULID]
	delete(c.workers, workerULID)
	c.mu.Unlock()

	if ok && p != nil {
		p.cancel()
	}

	return c.notifyGone(workerULID)
}

// RetireWorker has the backend reap workerULID only if no workload is attached.
// The poll loop is left running either way: on nil, StopWorker ends it, on
// ErrWorkerBusy it delivers the workload. StopWorker alone reaps the worker
// even when a run is attached and fails that run, so a scale-down retires
// first.
func (c *Client) RetireWorker(workerULID string) error {
	status, body, err := c.postSigned(workerULID, c.retireURL, "worker_retire")
	if err != nil {
		return err
	}

	switch status {
	case http.StatusNoContent:
		return nil
	case http.StatusConflict:
		return ErrWorkerBusy
	case http.StatusNotFound:
		return ErrRetireUnsupported
	}
	return &statusError{op: "worker_retire", status: status, body: string(body)}
}

// Close cancels all poll loops without sending /worker/gone; StopWorker each
// worker first for graceful deregistration. Idempotent.
func (c *Client) Close() {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	pollers := make([]*poller, 0, len(c.workers))
	for workerULID, p := range c.workers {
		pollers = append(pollers, p)
		delete(c.workers, workerULID)
	}
	c.mu.Unlock()

	for _, p := range pollers {
		p.cancel()
	}
	if c.ownsClient {
		c.client.CloseIdleConnections()
	}
}

func (c *Client) signToken(workerULID string) (string, error) {
	token, err := pollcomms.Sign(c.poolKey, c.poolULID, workerULID)
	return token, errors.Wrap(err, "could not sign poll token")
}

func (c *Client) doPoll(ctx context.Context, token string, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.pollURL, bytes.NewReader(body))
	if err != nil {
		return nil, errors.Wrap(err, "could not build poll request")
	}
	req.Header.Set(pollcomms.APITokenHeader, "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "could not perform poll request")
	}
	return resp, nil
}

func (c *Client) ack(ctx context.Context, workerULID, messageULID string) error {
	token, err := c.signToken(workerULID)
	if err != nil {
		return err
	}

	ackCtx, cancel := context.WithTimeout(ctx, ackTimeout)
	defer cancel()

	ackURL := strings.Replace(c.ackURL, "{messageULID}", messageULID, 1)
	req, err := http.NewRequestWithContext(ackCtx, http.MethodPost, ackURL, nil)
	if err != nil {
		return errors.Wrap(err, "could not build ack request")
	}
	req.Header.Set(pollcomms.APITokenHeader, "Bearer "+token)

	resp, err := c.client.Do(req)
	if err != nil {
		return errors.Wrap(err, "could not perform ack request")
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNotFound {
		// 404 = already acked: keeps a retried ack idempotent.
		return nil
	}
	return unexpectedStatus("ack", resp)
}

func (c *Client) notifyGone(workerULID string) error {
	status, body, err := c.postSigned(workerULID, c.goneURL, "worker_gone")
	if err != nil {
		return err
	}

	if status != http.StatusNoContent {
		return &statusError{op: "worker_gone", status: status, body: string(body)}
	}
	return nil
}

// postSigned sends an empty signed POST under the shutdown timeout. The body is
// read before the request context is cancelled, since the transport fails any
// read after that.
func (c *Client) postSigned(workerULID, url, op string) (int, []byte, error) {
	token, err := c.signToken(workerULID)
	if err != nil {
		return 0, nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return 0, nil, errors.Wrapf(err, "could not build %s request", op)
	}
	req.Header.Set(pollcomms.APITokenHeader, "Bearer "+token)

	resp, err := c.client.Do(req)
	if err != nil {
		return 0, nil, errors.Wrapf(err, "could not perform %s request", op)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))
	return resp.StatusCode, body, nil
}

type poller struct {
	client     *Client
	workerULID string
	cancel     context.CancelFunc

	mu         sync.Mutex
	request    pollcomms.PollRequest
	pollCancel context.CancelFunc // cancels only the in-flight poll
}

func (p *poller) run(loopCtx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			p.client.logger.Error(errors.Errorf("panic: %v", r), fmt.Sprintf("Poll loop for worker %s panicked.", p.workerULID))
			panic(r)
		}
	}()
	var backoff time.Duration
	for loopCtx.Err() == nil {
		start := time.Now()
		idle, err := p.pollOnce(loopCtx)
		switch {
		case err != nil && loopCtx.Err() == nil:
			p.client.logger.Error(err, fmt.Sprintf("Poll for worker %s failed.", p.workerULID))
			// Only a self-inflicted failure escalates; a backend blip must not delay recovery.
			if isClientError(err) {
				backoff = nextErrorBackoff(backoff)
			} else {
				backoff = errorBackoff
			}
			p.sleep(loopCtx, backoff)
		case idle:
			backoff = 0
			if wait := MinLongPoll - time.Since(start); wait > 0 {
				p.sleep(loopCtx, wait)
			}
		default:
			backoff = 0
		}
	}
}

func nextErrorBackoff(previous time.Duration) time.Duration {
	if previous <= 0 {
		return errorBackoff
	}
	return min(previous*2, maxErrorBackoff)
}

func (p *poller) pollOnce(loopCtx context.Context) (idle bool, err error) {
	token, err := p.client.signToken(p.workerULID)
	if err != nil {
		return false, err
	}

	pollCtx, interrupt := context.WithCancel(loopCtx)
	defer interrupt()

	// One lock for both, so a SetAvailable either lands in this poll or interrupts it.
	p.mu.Lock()
	p.pollCancel = interrupt
	request := p.request
	p.mu.Unlock()

	body, err := json.Marshal(request)
	if err != nil {
		return false, errors.Wrap(err, "could not marshal poll request")
	}

	// The Client owns this deadline; an injected HTTPClient may have none.
	reqCtx, cancelReq := context.WithTimeout(pollCtx, p.client.longPoll+pollDeadlineBuffer)
	defer cancelReq()

	resp, err := p.client.doPoll(reqCtx, token, body)
	if err != nil {
		// Interrupted by SetAvailable (loop still live): re-poll, not a failure.
		if loopCtx.Err() == nil && pollCtx.Err() != nil {
			return false, nil
		}
		return false, err
	}
	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case http.StatusNoContent:
		return true, nil
	case http.StatusOK:
		return false, p.handlePollResponse(loopCtx, resp.Body)
	default:
		return false, unexpectedStatus("poll", resp)
	}
}

func (p *poller) handlePollResponse(loopCtx context.Context, body io.Reader) error {
	var pollResp pollcomms.PollResponse
	if err := json.NewDecoder(body).Decode(&pollResp); err != nil {
		return errors.Wrap(err, "could not decode poll response")
	}

	// The response is in hand, so an interrupt now could only drop its connection.
	p.setPollCancel(nil)
	p.client.handler.HandlePayload(p.workerULID, pollResp.Payload)

	return p.client.ack(loopCtx, p.workerULID, pollResp.ULID)
}

func (p *poller) setPollCancel(cancel context.CancelFunc) {
	p.mu.Lock()
	p.pollCancel = cancel
	p.mu.Unlock()
}

func (p *poller) sleep(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}

type statusError struct {
	op     string
	status int
	body   string
}

func (e *statusError) Error() string {
	return fmt.Sprintf("unexpected %s status %d: %s", e.op, e.status, e.body)
}

func unexpectedStatus(op string, resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))
	return &statusError{op: op, status: resp.StatusCode, body: string(body)}
}

// A 4xx says the request itself is wrong, so repeating it unchanged cannot help.
func isClientError(err error) bool {
	var status *statusError
	return errors.As(err, &status) && status.status >= 400 && status.status < 500
}

type nopLogger struct{}

func (nopLogger) Debug(string)        {}
func (nopLogger) Info(string)         {}
func (nopLogger) Warn(string)         {}
func (nopLogger) Error(error, string) {}
