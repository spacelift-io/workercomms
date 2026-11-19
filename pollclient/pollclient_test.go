package pollclient

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/spacelift-io/workercomms/crypto"
	"github.com/spacelift-io/workercomms/messages"
	"github.com/spacelift-io/workercomms/pollcomms"
)

const (
	testPoolULID   = "01POOL"
	testWorkerULID = "01WORKER"
)

// Key generation is slow (4096-bit RSA); share one pair across the suite.
var testKeys struct {
	once    sync.Once
	public  []byte
	private []byte
	err     error
}

func keyPair(t *testing.T) (public, private []byte) {
	t.Helper()
	testKeys.once.Do(func() {
		testKeys.public, testKeys.private, testKeys.err = crypto.GenerateRSAKeyPair()
	})
	require.NoError(t, testKeys.err)
	return testKeys.public, testKeys.private
}

func testRequest() pollcomms.PollRequest {
	buildTimestamp := int64(1234)
	return pollcomms.PollRequest{
		PublicKey:          "worker-public-key",
		BuildTimestamp:     &buildTimestamp,
		Metadata:           map[string]any{"name": "worker-0"},
		LauncherGeneration: 1,
	}
}

// verifyBearer is assert-only: handlers run off the test goroutine.
func verifyBearer(t *testing.T, r *http.Request, publicKey []byte) *pollcomms.Claims {
	t.Helper()
	header := r.Header.Get("Authorization")
	if !assert.True(t, strings.HasPrefix(header, "Bearer "), "missing bearer prefix") {
		return nil
	}
	claims, err := pollcomms.Verify(strings.TrimPrefix(header, "Bearer "), publicKey)
	if !assert.NoError(t, err) {
		return nil
	}
	assert.Equal(t, testPoolULID, claims.PoolULID)
	assert.Equal(t, testWorkerULID, claims.WorkerULID)
	return claims
}

type recordingHandler struct {
	mu        sync.Mutex
	payloads  [][]byte
	onPayload func(workerULID string, payload []byte)
}

func (h *recordingHandler) HandlePayload(workerULID string, payload []byte) {
	h.mu.Lock()
	h.payloads = append(h.payloads, payload)
	onPayload := h.onPayload
	h.mu.Unlock()
	if onPayload != nil {
		onPayload(workerULID, payload)
	}
}

type recordingLogger struct {
	errCh chan error
}

func newRecordingLogger() *recordingLogger {
	return &recordingLogger{errCh: make(chan error, 16)}
}

func (l *recordingLogger) Debug(string) {}
func (l *recordingLogger) Info(string)  {}
func (l *recordingLogger) Warn(string)  {}
func (l *recordingLogger) Error(err error, message string) {
	select {
	case l.errCh <- fmt.Errorf("%s: %w", message, err):
	default:
	}
}

func newClient(t *testing.T, serverURL string, handler messages.PayloadHandler, logger messages.Logger) *Client {
	t.Helper()
	_, private := keyPair(t)
	client, err := New(Options{
		CommsURL:       serverURL,
		PoolULID:       testPoolULID,
		PoolPrivateKey: private,
		LongPoll:       5 * time.Second,
		Logger:         logger,
	}, handler)
	require.NoError(t, err)
	t.Cleanup(client.Close)
	return client
}

func waitFor[T any](t *testing.T, ch <-chan T, what string) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(10 * time.Second):
		t.Fatalf("timed out waiting for %s", what)
		panic("unreachable")
	}
}

func TestDispatchHappensBeforeAckWithValidTokens(t *testing.T) {
	public, _ := keyPair(t)

	var (
		orderMu sync.Mutex
		order   []string
	)
	record := func(event string) {
		orderMu.Lock()
		order = append(order, event)
		orderMu.Unlock()
	}

	ackCh := make(chan struct{}, 1)
	payload := []byte(`{"type":"job_assigned"}`)

	var pollCount int32
	var pollMu sync.Mutex

	mux := http.NewServeMux()
	mux.HandleFunc("POST /worker/poll", func(w http.ResponseWriter, r *http.Request) {
		verifyBearer(t, r, public)
		assert.Equal(t, "5", r.URL.Query().Get("wait_seconds"))

		var request pollcomms.PollRequest
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		assert.Equal(t, testRequest().PublicKey, request.PublicKey)
		assert.Equal(t, testRequest().LauncherGeneration, request.LauncherGeneration)

		pollMu.Lock()
		pollCount++
		first := pollCount == 1
		pollMu.Unlock()

		if !first {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		assert.NoError(t, json.NewEncoder(w).Encode(pollcomms.PollResponse{ULID: "01MSG", Payload: payload}))
	})
	mux.HandleFunc("POST /worker/ack/{messageULID}", func(w http.ResponseWriter, r *http.Request) {
		verifyBearer(t, r, public)
		assert.Equal(t, "01MSG", r.PathValue("messageULID"))
		record("ack")
		w.WriteHeader(http.StatusOK)
		select {
		case ackCh <- struct{}{}:
		default:
		}
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	handler := &recordingHandler{onPayload: func(string, []byte) { record("dispatch") }}
	client := newClient(t, server.URL, handler, nil)

	require.NoError(t, client.StartWorker(testWorkerULID, testRequest()))
	waitFor(t, ackCh, "ack")

	orderMu.Lock()
	defer orderMu.Unlock()
	require.Equal(t, []string{"dispatch", "ack"}, order)
	require.Equal(t, [][]byte{payload}, handler.payloads)
}

func TestErrorBackoffLadderSaturates(t *testing.T) {
	first := nextErrorBackoff(0)
	second := nextErrorBackoff(first)
	require.Greater(t, second, first)

	saturated := second
	for range 10 {
		saturated = nextErrorBackoff(saturated)
	}
	require.Equal(t, maxErrorBackoff, saturated)
	require.Equal(t, maxErrorBackoff, nextErrorBackoff(saturated))
}

func TestIsClientError(t *testing.T) {
	for _, tc := range []struct {
		status int
		want   bool
	}{
		{http.StatusBadRequest, true},
		{http.StatusUnauthorized, true},
		{http.StatusConflict, true},
		{http.StatusInternalServerError, false},
		{http.StatusServiceUnavailable, false},
	} {
		require.Equal(t, tc.want, isClientError(&statusError{op: "poll", status: tc.status}), tc.status)
	}
	require.False(t, isClientError(errors.New("connection reset")))
}

// pollAttemptTimes drives a worker against a server that always answers status, returning when
// three attempts have been made.
func pollAttemptTimes(t *testing.T, status int) []time.Time {
	t.Helper()
	public, _ := keyPair(t)

	thirdPoll := make(chan struct{}, 1)
	var mu sync.Mutex
	var attempts []time.Time

	mux := http.NewServeMux()
	mux.HandleFunc("POST /worker/poll", func(w http.ResponseWriter, r *http.Request) {
		verifyBearer(t, r, public)
		mu.Lock()
		attempts = append(attempts, time.Now())
		count := len(attempts)
		mu.Unlock()

		if count == 3 {
			select {
			case thirdPoll <- struct{}{}:
			default:
			}
		}
		w.WriteHeader(status)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := newClient(t, server.URL, &recordingHandler{}, newRecordingLogger())

	require.NoError(t, client.StartWorker(testWorkerULID, testRequest()))
	waitFor(t, thirdPoll, "third poll attempt")

	mu.Lock()
	defer mu.Unlock()
	return attempts
}

func TestRejectedPollBacksOffProgressively(t *testing.T) {
	attempts := pollAttemptTimes(t, http.StatusUnauthorized)
	require.Greater(t, attempts[2].Sub(attempts[1]), attempts[1].Sub(attempts[0])+time.Second)
}

func TestFailedPollKeepsRetryingAtTheFloor(t *testing.T) {
	attempts := pollAttemptTimes(t, http.StatusInternalServerError)
	require.Less(t, attempts[2].Sub(attempts[1]), attempts[1].Sub(attempts[0])+time.Second)
}

func TestAck404IsSuccessAndLoopContinues(t *testing.T) {
	public, _ := keyPair(t)

	secondPoll := make(chan struct{}, 1)
	var pollCount int
	var mu sync.Mutex

	mux := http.NewServeMux()
	mux.HandleFunc("POST /worker/poll", func(w http.ResponseWriter, r *http.Request) {
		verifyBearer(t, r, public)
		mu.Lock()
		pollCount++
		count := pollCount
		mu.Unlock()

		if count == 1 {
			assert.NoError(t, json.NewEncoder(w).Encode(pollcomms.PollResponse{ULID: "01MSG", Payload: []byte(`{}`)}))
			return
		}
		select {
		case secondPoll <- struct{}{}:
		default:
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /worker/ack/{messageULID}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	logger := newRecordingLogger()
	client := newClient(t, server.URL, &recordingHandler{}, logger)

	require.NoError(t, client.StartWorker(testWorkerULID, testRequest()))
	waitFor(t, secondPoll, "re-poll after 404 ack")

	select {
	case err := <-logger.errCh:
		t.Fatalf("unexpected error logged: %v", err)
	default:
	}
}

func TestStopWorkerStopsLoopAndNotifiesGone(t *testing.T) {
	public, _ := keyPair(t)

	firstPoll := make(chan struct{}, 1)
	goneCh := make(chan struct{}, 1)
	var pollCount int
	var mu sync.Mutex

	mux := http.NewServeMux()
	mux.HandleFunc("POST /worker/poll", func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		pollCount++
		mu.Unlock()
		select {
		case firstPoll <- struct{}{}:
		default:
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /worker/gone", func(w http.ResponseWriter, r *http.Request) {
		verifyBearer(t, r, public)
		w.WriteHeader(http.StatusNoContent)
		select {
		case goneCh <- struct{}{}:
		default:
		}
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := newClient(t, server.URL, &recordingHandler{}, nil)

	require.NoError(t, client.StartWorker(testWorkerULID, testRequest()))
	waitFor(t, firstPoll, "first poll")

	require.NoError(t, client.StopWorker(testWorkerULID))
	waitFor(t, goneCh, "worker_gone")

	mu.Lock()
	stoppedAt := pollCount
	mu.Unlock()
	time.Sleep(200 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, stoppedAt, pollCount, "poll loop kept polling after StopWorker")
}

func TestSetAvailableInterruptsHeldPoll(t *testing.T) {
	firstPollStarted := make(chan struct{}, 1)
	rePolled := make(chan *bool, 1)
	release := make(chan struct{})
	var pollCount int
	var mu sync.Mutex

	mux := http.NewServeMux()
	mux.HandleFunc("POST /worker/poll", func(w http.ResponseWriter, r *http.Request) {
		var request pollcomms.PollRequest
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&request))

		mu.Lock()
		pollCount++
		count := pollCount
		mu.Unlock()

		if count == 1 {
			select {
			case firstPollStarted <- struct{}{}:
			default:
			}
			select {
			case <-r.Context().Done():
			case <-release:
			}
			return
		}
		assert.Equal(t, testRequest().LauncherGeneration, request.LauncherGeneration)
		select {
		case rePolled <- request.Available:
		default:
		}
		w.WriteHeader(http.StatusNoContent)
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	defer close(release)

	client := newClient(t, server.URL, &recordingHandler{}, nil)

	require.NoError(t, client.StartWorker(testWorkerULID, testRequest()))
	waitFor(t, firstPollStarted, "held poll")

	start := time.Now()
	client.SetAvailable(testWorkerULID, false)
	available := waitFor(t, rePolled, "immediate re-poll after interrupt")
	require.Less(t, time.Since(start), 3*time.Second, "re-poll should not wait out the held poll")
	require.NotNil(t, available)
	require.False(t, *available, "the re-poll must carry the new availability")
}

func TestNewValidation(t *testing.T) {
	_, private := keyPair(t)
	handler := &recordingHandler{}

	_, err := New(Options{CommsURL: "https://example.com", PoolPrivateKey: private}, nil)
	require.ErrorContains(t, err, "payload handler")

	_, err = New(Options{CommsURL: "example.com", PoolPrivateKey: private}, handler)
	require.ErrorContains(t, err, "scheme and host")

	_, err = New(Options{CommsURL: "https://example.com"}, handler)
	require.ErrorContains(t, err, "pool private key")

	_, err = New(Options{CommsURL: "https://example.com", PoolPrivateKey: []byte("garbage")}, handler)
	require.ErrorContains(t, err, "pool key")

	_, err = New(Options{CommsURL: "https://example.com", PoolPrivateKey: private, LongPoll: 3 * time.Second}, handler)
	require.ErrorContains(t, err, "at least")

	client, err := New(Options{CommsURL: "https://example.com/", PoolPrivateKey: private}, handler)
	require.NoError(t, err)
	require.Contains(t, client.pollURL, "https://example.com/worker/poll?wait_seconds=55")
}

func TestEveryRequestSignsAFreshToken(t *testing.T) {
	public, _ := keyPair(t)

	tokens := make(chan *pollcomms.Claims, 3)
	captureClaims := func(r *http.Request) {
		header := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		claims, err := pollcomms.Verify(header, public)
		if assert.NoError(t, err) {
			tokens <- claims
		}
	}

	var pollCount int
	var mu sync.Mutex
	mux := http.NewServeMux()
	mux.HandleFunc("POST /worker/poll", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		pollCount++
		first := pollCount == 1
		mu.Unlock()
		if !first {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		captureClaims(r)
		// Push the ack into a later second than the poll token.
		time.Sleep(1100 * time.Millisecond)
		assert.NoError(t, json.NewEncoder(w).Encode(pollcomms.PollResponse{ULID: "01MSG", Payload: []byte(`{}`)}))
	})
	mux.HandleFunc("POST /worker/ack/{messageULID}", func(w http.ResponseWriter, r *http.Request) {
		captureClaims(r)
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("POST /worker/gone", func(w http.ResponseWriter, r *http.Request) {
		captureClaims(r)
		w.WriteHeader(http.StatusNoContent)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := newClient(t, server.URL, &recordingHandler{}, nil)
	require.NoError(t, client.StartWorker(testWorkerULID, testRequest()))

	pollClaims := waitFor(t, tokens, "poll token")
	ackClaims := waitFor(t, tokens, "ack token")
	require.NoError(t, client.StopWorker(testWorkerULID))
	goneClaims := waitFor(t, tokens, "gone token")

	require.True(t, ackClaims.IssuedAt.After(pollClaims.IssuedAt.Time),
		"ack token must be signed after the poll token (fresh token per request)")
	require.False(t, goneClaims.IssuedAt.Before(ackClaims.IssuedAt.Time))
}

func TestSetAvailableFromDispatchReachesNextPoll(t *testing.T) {
	ackCh := make(chan struct{}, 1)
	rePolled := make(chan *bool, 1)
	var pollCount int
	var mu sync.Mutex

	mux := http.NewServeMux()
	mux.HandleFunc("POST /worker/poll", func(w http.ResponseWriter, r *http.Request) {
		var request pollcomms.PollRequest
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&request))

		mu.Lock()
		pollCount++
		first := pollCount == 1
		mu.Unlock()
		if first {
			assert.NoError(t, json.NewEncoder(w).Encode(pollcomms.PollResponse{ULID: "01MSG", Payload: []byte(`{}`)}))
			return
		}
		select {
		case rePolled <- request.Available:
		default:
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /worker/ack/{messageULID}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		select {
		case ackCh <- struct{}{}:
		default:
		}
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	var client *Client
	handler := &recordingHandler{}
	handler.onPayload = func(workerULID string, _ []byte) {
		client.SetAvailable(workerULID, false)
	}
	client = newClient(t, server.URL, handler, nil)

	require.NoError(t, client.StartWorker(testWorkerULID, testRequest()))
	waitFor(t, ackCh, "ack after SetAvailable from dispatch")
	available := waitFor(t, rePolled, "next poll")
	require.NotNil(t, available)
	require.False(t, *available, "a change made during dispatch must reach the next poll")
}

func TestRetireWorkerMapsStatusesAndLeavesLoopAlone(t *testing.T) {
	public, _ := keyPair(t)

	for _, tc := range []struct {
		name   string
		status int
		want   error
	}{
		{name: "idle worker is reaped", status: http.StatusNoContent, want: nil},
		{name: "busy worker is kept", status: http.StatusConflict, want: ErrWorkerBusy},
		{name: "old backend", status: http.StatusNotFound, want: ErrRetireUnsupported},
	} {
		t.Run(tc.name, func(t *testing.T) {
			goneCalls := make(chan struct{}, 1)
			mux := http.NewServeMux()
			mux.HandleFunc("POST /worker/poll", func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			})
			mux.HandleFunc("POST /worker/retire", func(w http.ResponseWriter, r *http.Request) {
				verifyBearer(t, r, public)
				w.WriteHeader(tc.status)
			})
			mux.HandleFunc("POST /worker/gone", func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
				goneCalls <- struct{}{}
			})
			server := httptest.NewServer(mux)
			defer server.Close()

			client := newClient(t, server.URL, &recordingHandler{}, nil)
			require.NoError(t, client.StartWorker(testWorkerULID, testRequest()))

			require.ErrorIs(t, client.RetireWorker(testWorkerULID), tc.want)

			client.mu.Lock()
			_, running := client.workers[testWorkerULID]
			client.mu.Unlock()
			require.True(t, running, "retire must not stop the poll loop; StopWorker does")
			require.Empty(t, goneCalls, "retire must not report gone on its own")
		})
	}
}

func TestRetireWorkerUnexpectedStatusIsAnError(t *testing.T) {
	mux := http.NewServeMux()
	// The body arrives after the headers, so it is only seen if it is read
	// while the request context is still live.
	mux.HandleFunc("POST /worker/retire", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.(http.Flusher).Flush()
		time.Sleep(20 * time.Millisecond)
		_, _ = w.Write([]byte("boom"))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := newClient(t, server.URL, &recordingHandler{}, nil)

	err := client.RetireWorker(testWorkerULID)
	var status *statusError
	require.ErrorAs(t, err, &status)
	require.Equal(t, http.StatusInternalServerError, status.status)
	require.Equal(t, "boom", status.body)
	require.False(t, isClientError(err), "a 5xx must stay retryable")
}

func TestStopWorkerAlwaysNotifiesGone(t *testing.T) {
	public, _ := keyPair(t)

	goneCh := make(chan struct{}, 2)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /worker/gone", func(w http.ResponseWriter, r *http.Request) {
		verifyBearer(t, r, public)
		w.WriteHeader(http.StatusNoContent)
		goneCh <- struct{}{}
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := newClient(t, server.URL, &recordingHandler{}, nil)

	require.NoError(t, client.StopWorker(testWorkerULID))
	waitFor(t, goneCh, "gone for unknown worker")

	client.Close()
	require.NoError(t, client.StopWorker(testWorkerULID))
	waitFor(t, goneCh, "gone after Close")

	require.Error(t, client.StartWorker(testWorkerULID, testRequest()), "StartWorker must be rejected after Close")
}

func TestStartStopStartSpawnsFreshLoop(t *testing.T) {
	events := make(chan string, 16)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /worker/poll", func(w http.ResponseWriter, _ *http.Request) {
		events <- "poll"
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /worker/gone", func(w http.ResponseWriter, _ *http.Request) {
		events <- "gone"
		w.WriteHeader(http.StatusNoContent)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := newClient(t, server.URL, &recordingHandler{}, nil)

	require.NoError(t, client.StartWorker(testWorkerULID, testRequest()))
	require.Equal(t, "poll", waitFor(t, events, "first loop's poll"))

	require.NoError(t, client.StopWorker(testWorkerULID))
	require.Equal(t, "gone", waitFor(t, events, "gone"))

	require.NoError(t, client.StartWorker(testWorkerULID, testRequest()))
	require.Equal(t, "poll", waitFor(t, events, "resurrected loop's poll"))
}

func TestStalledServerHitsClientOwnedDeadline(t *testing.T) {
	release := make(chan struct{})
	pollStarted := make(chan struct{}, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /worker/poll", func(_ http.ResponseWriter, r *http.Request) {
		select {
		case pollStarted <- struct{}{}:
		default:
		}
		select {
		case <-r.Context().Done():
		case <-release:
		}
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	defer close(release)

	_, private := keyPair(t)
	logger := newRecordingLogger()
	client, err := New(Options{
		CommsURL:       server.URL,
		PoolULID:       testPoolULID,
		PoolPrivateKey: private,
		LongPoll:       5 * time.Second,
		// Zero-timeout client: the Client must apply its own deadline.
		HTTPClient: &http.Client{},
		Logger:     logger,
	}, &recordingHandler{})
	require.NoError(t, err)
	defer client.Close()

	start := time.Now()
	require.NoError(t, client.StartWorker(testWorkerULID, testRequest()))
	waitFor(t, pollStarted, "stalled poll")

	select {
	case err := <-logger.errCh:
		require.ErrorContains(t, err, "poll")
		elapsed := time.Since(start)
		require.Greater(t, elapsed, 9*time.Second, "deadline fired too early")
		require.Less(t, elapsed, 14*time.Second, "deadline fired too late")
	case <-time.After(20 * time.Second):
		t.Fatal("stalled poll never hit the client-owned deadline")
	}
}

func TestUnexpectedPollStatusIsLoggedAtError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /worker/poll", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	logger := newRecordingLogger()
	client := newClient(t, server.URL, &recordingHandler{}, logger)

	require.NoError(t, client.StartWorker(testWorkerULID, testRequest()))

	err := waitFor(t, logger.errCh, "error log for unexpected status")
	require.ErrorContains(t, err, "unexpected poll status 500")
	require.ErrorContains(t, err, "boom")
	require.ErrorContains(t, err, testWorkerULID)
}
