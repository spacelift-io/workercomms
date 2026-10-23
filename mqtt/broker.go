package mqtt

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"strings"
	"sync/atomic"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/eclipse/paho.mqtt.golang/packets"
	"github.com/pkg/errors"

	"github.com/spacelift-io/workercomms/messages"
)

type mqttQoSLevel byte

const (
	qoSAtMostOnce mqttQoSLevel = iota
	qoSAtLeastOnce
	qoSExactlyOnce

	defaultMaxBackoffSeconds = 16
)

type MQTTBrokerOptions struct {
	AdditionalCACerts       []string
	BrokerEndpoint          string
	ClientID                string
	ConnectionRetryCount    uint
	LastWill                *messages.Message
	LauncherULID            string
	Logger                  messages.Logger
	MaxBackoffSeconds       *uint
	MaxReconnectionAttempts *uint
	OnReconnectFailed       func(err error)
	OrderMatters            *bool
	PoolKeyPair             tls.Certificate
	PublishChannelFormat    string
	SubscribeChannelFormat  string
}

type MQTTBroker struct {
	connectionRetryCount    uint
	mqttClient              mqtt.Client
	mqttOpts                *mqtt.ClientOptions
	subscribeTopic          string
	publishChannelFormat    string
	messageHandler          messages.PayloadHandler
	logger                  messages.Logger
	reconnecting            *atomic.Bool
	maxReconnectionAttempts *uint
	maxBackoffSeconds       uint
	onReconnectFailed       func(err error)
}

func NewMQTTBroker(opts MQTTBrokerOptions, messageHandler messages.PayloadHandler) (*MQTTBroker, error) {
	broker := &MQTTBroker{
		// Default to subscribing to messages for all Launchers. If the broker is being created
		// for a specific Launcher ULID, we'll override this later to just subscribe to that
		// single Launcher.
		subscribeTopic:          fmt.Sprintf(opts.SubscribeChannelFormat, "#"),
		publishChannelFormat:    opts.PublishChannelFormat,
		connectionRetryCount:    opts.ConnectionRetryCount,
		mqttOpts:                mqtt.NewClientOptions(),
		logger:                  opts.Logger,
		reconnecting:            &atomic.Bool{}, // false by default
		maxReconnectionAttempts: opts.MaxReconnectionAttempts,
		maxBackoffSeconds:       defaultMaxBackoffSeconds,
		onReconnectFailed:       opts.OnReconnectFailed,
		messageHandler:          messageHandler,
	}

	if opts.ConnectionRetryCount == 0 {
		broker.connectionRetryCount = 5
	}

	if opts.MaxBackoffSeconds != nil {
		broker.maxBackoffSeconds = *opts.MaxBackoffSeconds
	}

	if opts.OrderMatters != nil {
		broker.mqttOpts.SetOrderMatters(*opts.OrderMatters)
	}

	rootCAs, err := getMQTTCertPool(opts.AdditionalCACerts)
	if err != nil {
		return nil, errors.Wrap(err, "could not retrieve root CAs")
	}

	broker.mqttOpts.SetClientID(opts.ClientID)
	broker.mqttOpts.SetTLSConfig(&tls.Config{
		Certificates: []tls.Certificate{opts.PoolKeyPair},
		RootCAs:      rootCAs,
		NextProtos:   []string{"x-amzn-mqtt-ca"}, // Forces to use MQTT on this connection. Would use HTTP otherwise.
		MinVersion:   tls.VersionTLS12,
	})

	endpoint := opts.BrokerEndpoint
	if !strings.Contains(endpoint, ":") {
		endpoint = fmt.Sprintf("ssl://%s:443", endpoint)
	}
	broker.mqttOpts.AddBroker(endpoint)

	broker.mqttOpts.SetConnectTimeout(time.Second * 10)
	broker.mqttOpts.SetAutoReconnect(false)
	broker.mqttOpts.SetConnectionLostHandler(broker.onConnectionLost)

	if opts.LastWill != nil {
		poolPublishTopic := fmt.Sprintf(opts.PublishChannelFormat, opts.LauncherULID)
		workerGoneJSON, err := json.Marshal(opts.LastWill)
		if err != nil {
			return nil, errors.Wrap(err, "could not marshal last will and testament payload")
		}

		broker.mqttOpts.SetWill(poolPublishTopic, string(workerGoneJSON), byte(qoSAtLeastOnce), false)
	}

	if opts.LauncherULID != "" {
		// Let's only subscribe to messages for that individual launcher
		broker.subscribeTopic = fmt.Sprintf(opts.SubscribeChannelFormat, opts.LauncherULID)
	}

	mqtt.DEBUG = newMQTTLogger(opts.Logger, mqttLogLevelDebug)
	mqtt.WARN = newMQTTLogger(opts.Logger, mqttLogLevelWarn)
	mqtt.CRITICAL = newMQTTLogger(opts.Logger, mqttLogLevelError)
	mqtt.ERROR = newMQTTLogger(opts.Logger, mqttLogLevelError)

	return broker, nil
}

func (c *MQTTBroker) Connect() error {
	if c.mqttClient != nil && c.mqttClient.IsConnected() {
		return errors.New("broker is already started")
	}

	c.mqttClient = mqtt.NewClient(c.mqttOpts)
	if err := c.connectWithRetry(); err != nil {
		return errors.Wrap(err, "could not connect to MQTT broker")
	}

	if err := c.subscribeToMessages(); err != nil {
		return errors.Wrap(err, "couldn't subscribe to messages")
	}

	return nil
}

func (c *MQTTBroker) connectWithRetry() error {
	var err error
	var retryCount uint

	for {
		if token := c.mqttClient.Connect(); token.Wait() && token.Error() != nil {
			err = token.Error()
			retryCount++

			if retryCount >= c.connectionRetryCount {
				return errors.Wrapf(err, "could not connect to MQTT broker after %d retries", retryCount)
			}

			sleepTime := time.Duration(math.Pow(2, float64(retryCount))) * time.Second
			c.logger.Error(err, fmt.Sprintf("Could not connect to MQTT broker - retrying in %s", sleepTime))
			time.Sleep(sleepTime)

			continue
		}

		return nil
	}
}

// IsNetworkError reports whether err is a connection failure the MQTT client
// blamed on the network rather than on a refusal from the broker.
func IsNetworkError(err error) bool {
	var netErr net.Error

	return errors.Is(err, packets.ErrorNetworkError) || errors.As(err, &netErr)
}

func (c *MQTTBroker) IsConnected() bool {
	return c.mqttClient != nil && c.mqttClient.IsConnected()
}

func (c *MQTTBroker) Disconnect(quiesce uint) {
	client := c.mqttClient
	if client != nil {
		client.Disconnect(quiesce)
		c.mqttClient = nil
	}
}

func (c *MQTTBroker) Publish(workerULID string, message messages.Message) error {
	if !c.IsConnected() {
		return errors.New("broker not connected - please call Connect() before attempting to publish")
	}

	jsonMsg, err := json.Marshal(message)
	if err != nil {
		return errors.Wrap(err, "could not marshal IoT payload")
	}

	// TODO Investigate if we should change that to qoSExactlyOnce
	//   qoSAtLeastOnce mean that we will allow message to be delivered more than once, that's probably not exactly what we want
	token := c.mqttClient.Publish(fmt.Sprintf(c.publishChannelFormat, workerULID), byte(qoSAtLeastOnce), false, jsonMsg)
	if token.Wait() && token.Error() != nil {
		return errors.Wrapf(token.Error(), `couldn't Publish "%s" message`, message.MessageType)
	}

	return nil
}

func (c *MQTTBroker) handleMessage(client mqtt.Client, message mqtt.Message) {
	// It turns out that message handling should happen in a Goroutine to avoid blocking the broker,
	// but currently the launcher doesn't do that. We may want to change this, but let's not change
	// too many things at once.
	workerULID, err := workerULIDFromTopic(message.Topic())
	if err != nil {
		c.logger.Error(err, "could not handle MQTT message")
		return
	}
	c.messageHandler.HandlePayload(workerULID, message.Payload())
}

// workerULIDFromTopic extracts the worker ULID from a
// {namespace}/{readonly|writeonly}/{pool-ulid}/{worker-ulid} topic.
func workerULIDFromTopic(topic string) (string, error) {
	splitTopic := strings.Split(topic, "/")
	if len(splitTopic) != 4 {
		return "", errors.Errorf("unable to parse topic: %s", topic)
	}
	return splitTopic[3], nil
}

func (c *MQTTBroker) subscribeToMessages() error {
	if token := c.mqttClient.Subscribe(c.subscribeTopic, 0, c.handleMessage); token.Wait() && token.Error() != nil {
		return errors.Wrapf(token.Error(), "could not subscribe to MQTT topic (%s)", c.subscribeTopic)
	}
	return nil
}

func (c *MQTTBroker) onConnectionLost(client mqtt.Client, err error) {
	if !c.reconnecting.CompareAndSwap(false, true) {
		return
	}

	c.logger.Error(err, "Connection lost. Reconnecting...")

	for i := uint(0); c.maxReconnectionAttempts == nil || i < *c.maxReconnectionAttempts; i++ {
		if c.mqttClient == nil {
			c.logger.Info("Stopping reconnection attempts because client has disconnected")
			return
		}

		if err := c.attemptToReconnect(client); err == nil {
			c.reconnecting.CompareAndSwap(true, false)
			return
		}

		sleepDuration := int(math.Min(float64(c.maxBackoffSeconds), math.Pow(2, float64(i))))
		c.logger.Warn(fmt.Sprintf("Reconnection attempt failed - sleeping for %d seconds", sleepDuration))
		time.Sleep(time.Duration(sleepDuration) * time.Second)
	}

	if c.onReconnectFailed != nil {
		c.onReconnectFailed(err)
	}
}

func (c *MQTTBroker) attemptToReconnect(client mqtt.Client) error {
	// It's possible that reconnecting succeeded, but subscribing to messages failed. In this
	// case we shouldn't try to connect again because it will fail if we're already connected.
	// We can just immediately attempt to subscribe again.
	if !client.IsConnected() {
		if token := client.Connect(); token.Wait() && token.Error() != nil {
			c.logger.Error(token.Error(), "Could not reconnect")
			return errors.Wrap(token.Error(), "could not reconnect")
		}

		c.logger.Info("Successfully reconnected to broker")
	}

	if err := c.subscribeToMessages(); err != nil {
		c.logger.Error(err, "could not subscribe to messages")
		return errors.Wrap(err, "could not subscribe to messages")
	}

	c.logger.Info("Successfully subscribed to messages")

	return nil
}
