package koupleless_mqtt_tunnel

import (
	"os"
	"strconv"
)

const (
	DefaultMQTTBroker       = "test-broker"
	DefaultMQTTUsername     = "test-username"
	DefaultMQTTClientPrefix = "koupleless"
	DefaultMQTTPort         = 1883
)

type MqttConfig struct {
	MqttBroker        string
	MqttPort          int
	MqttUsername      string
	MqttPassword      string
	MqttClientPrefix  string
	MqttCAPath        string
	MqttClientCrtPath string
	MqttClientKeyPath string
}

func (c *MqttConfig) init() {
	c.MqttBroker = os.Getenv("MQTT_BROKER")
	if c.MqttBroker == "" {
		c.MqttBroker = DefaultMQTTBroker
	}
	portStr := os.Getenv("MQTT_PORT")
	port, err := strconv.Atoi(portStr)
	if err == nil {
		c.MqttPort = port
	}
	if c.MqttPort == 0 {
		c.MqttPort = DefaultMQTTPort
	}

	c.MqttUsername = os.Getenv("MQTT_USERNAME")
	if c.MqttUsername == "" {
		c.MqttUsername = DefaultMQTTUsername
	}
	c.MqttPassword = os.Getenv("MQTT_PASSWORD")
	c.MqttClientPrefix = os.Getenv("MQTT_CLIENT_PREFIX")
	if c.MqttClientPrefix == "" {
		c.MqttClientPrefix = DefaultMQTTClientPrefix
	}
	c.MqttCAPath = os.Getenv("MQTT_CA_PATH")
	c.MqttClientCrtPath = os.Getenv("MQTT_CLIENT_CRT_PATH")
	c.MqttClientKeyPath = os.Getenv("MQTT_CLIENT_KEY_PATH")
}
