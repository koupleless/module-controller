package mqtt

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"github.com/koupleless/module_controller/common/zaplogger"
	"os"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type ClientInitFunc func(*mqtt.ClientOptions) mqtt.Client

// Constants for MQTT QoS levels
const (
	// Qos0 means message only published once
	Qos0 = iota

	// Qos1 means message must be consumed
	Qos1

	// Qos2 means message must be consumed only once
	Qos2
)

// Client represents an MQTT client
type Client struct {
	client mqtt.Client
}

// ClientConfig holds configuration for an MQTT client
type ClientConfig struct {
	Broker                string
	Port                  int
	ClientID              string
	Username              string
	Password              string
	CAPath                string
	ClientCrtPath         string
	ClientKeyPath         string
	CleanSession          bool
	KeepAlive             time.Duration
	DefaultMessageHandler mqtt.MessageHandler
	OnConnectHandler      mqtt.OnConnectHandler
	ConnectionLostHandler mqtt.ConnectionLostHandler
	ClientInitFunc        ClientInitFunc
}

// DefaultMqttClientInitFunc is the default function to initialize an MQTT client
var DefaultMqttClientInitFunc ClientInitFunc = mqtt.NewClient

// Default message handlers
var defaultMessageHandler mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	zaplogger.GetLogger().Info(fmt.Sprintf("Received message: %s from topic: %s\n", msg.Payload(), msg.Topic()))
}

var defaultOnConnectHandler mqtt.OnConnectHandler = func(client mqtt.Client) {
	zaplogger.GetLogger().Info("Connected")
}

var defaultConnectionLostHandler mqtt.ConnectionLostHandler = func(client mqtt.Client, err error) {
	zaplogger.GetLogger().Info(fmt.Sprintf("Connection lost %v\n", err))
}

// newTlsConfig creates a TLS configuration using the client configuration
func newTlsConfig(cfg *ClientConfig) (*tls.Config, error) {
	config := tls.Config{
		InsecureSkipVerify: true,
	}

	certpool := x509.NewCertPool()
	ca, err := os.ReadFile(cfg.CAPath)
	if err != nil {
		return nil, err
	}
	certpool.AppendCertsFromPEM(ca)
	config.RootCAs = certpool
	if cfg.ClientCrtPath != "" {
		// Import client certificate/key pair
		clientKeyPair, err := tls.LoadX509KeyPair(cfg.ClientCrtPath, cfg.ClientKeyPath)
		if err != nil {
			return nil, err
		}
		config.Certificates = []tls.Certificate{clientKeyPair}
		config.ClientAuth = tls.NoClientCert
	}

	return &config, nil
}

// NewMqttClient creates a new MQTT client using the provided configuration
func NewMqttClient(cfg *ClientConfig) (*Client, error) {
	opts := mqtt.NewClientOptions()
	broker := ""
	opts.SetClientID(cfg.ClientID)
	opts.SetUsername(cfg.Username)
	opts.SetPassword(cfg.Password)
	if cfg.CAPath != "" {
		// TLS configuration
		tlsConfig, err := newTlsConfig(cfg)
		if err != nil {
			return nil, err
		}
		opts.SetTLSConfig(tlsConfig)
		broker = fmt.Sprintf("ssl://%s:%d", cfg.Broker, cfg.Port)
	} else {
		broker = fmt.Sprintf("tcp://%s:%d", cfg.Broker, cfg.Port)
	}

	opts.AddBroker(broker)

	if cfg.DefaultMessageHandler == nil {
		cfg.DefaultMessageHandler = defaultMessageHandler
	}

	if cfg.OnConnectHandler == nil {
		cfg.OnConnectHandler = defaultOnConnectHandler
	}

	if cfg.ConnectionLostHandler == nil {
		cfg.ConnectionLostHandler = defaultConnectionLostHandler
	}

	if cfg.ClientInitFunc == nil {
		cfg.ClientInitFunc = DefaultMqttClientInitFunc
	}

	if cfg.KeepAlive == 0 {
		cfg.KeepAlive = time.Minute * 3
	}

	opts.SetDefaultPublishHandler(cfg.DefaultMessageHandler)
	opts.SetAutoReconnect(true)
	opts.SetKeepAlive(cfg.KeepAlive)
	opts.SetCleanSession(cfg.CleanSession)
	opts.SetOnConnectHandler(cfg.OnConnectHandler)
	opts.SetConnectionLostHandler(cfg.ConnectionLostHandler)
	client := cfg.ClientInitFunc(opts)
	return &Client{
		client: client,
	}, nil
}

// Connect attempts to connect the MQTT client
func (c *Client) Connect() error {
	token := c.client.Connect()
	token.Wait()
	return token.Error()
}

// Pub publishes a message to a specified topic
func (c *Client) Pub(topic string, qos byte, msg []byte) error {
	zaplogger.GetLogger().Info(fmt.Sprintf("Publishing message: %s to topic: %s\n", msg, topic))
	token := c.client.Publish(topic, qos, true, msg)
	token.Wait()
	return token.Error()
}

// Sub subscribes to a topic with a callback
func (c *Client) Sub(topic string, qos byte, callBack mqtt.MessageHandler) error {
	zaplogger.GetLogger().Info(fmt.Sprintf("Subscribing to topic: %s\n", topic))
	token := c.client.Subscribe(topic, qos, callBack)
	token.Wait()
	return token.Error()
}

// UnSub unsubscribes from a topic
func (c *Client) UnSub(topic string) error {
	zaplogger.GetLogger().Info(fmt.Sprintf("Unsubscribing from topic: %s\n", topic))
	c.client.Unsubscribe(topic)
	return nil
}

// Disconnect disconnects the MQTT client
func (c *Client) Disconnect() {
	c.client.Disconnect(200)
}
