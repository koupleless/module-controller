package koupleless_mqtt_tunnel

import (
	"github.com/koupleless/virtual-kubelet/common/utils"
	"os"
	"strconv"
)

const (
	DefaultMQTTBroker       = "test-broker"
	DefaultMQTTUsername     = "test-username"
	DefaultMQTTClientPrefix = "koupleless"
	DefaultMQTTPort         = "1883"
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
	c.MqttBroker = utils.GetEnv("MQTT_BROKER", DefaultMQTTBroker)
	portStr := utils.GetEnv("MQTT_PORT", DefaultMQTTPort)
	port, err := strconv.Atoi(portStr)
	if err == nil {
		c.MqttPort = port
	}

	c.MqttUsername = utils.GetEnv("MQTT_USERNAME", DefaultMQTTUsername)
	c.MqttPassword = os.Getenv("MQTT_PASSWORD")
	c.MqttClientPrefix = utils.GetEnv("MQTT_CLIENT_PREFIX", DefaultMQTTClientPrefix)
	c.MqttCAPath = os.Getenv("MQTT_CA_PATH")
	c.MqttClientCrtPath = os.Getenv("MQTT_CLIENT_CRT_PATH")
	c.MqttClientKeyPath = os.Getenv("MQTT_CLIENT_KEY_PATH")
}
