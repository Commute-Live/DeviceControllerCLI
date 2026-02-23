package device

import (
    "fmt"
    "time"

    pahomqtt "github.com/eclipse/paho.mqtt.golang"
)

type mqttClient struct {
    host     string
    port     int
    username string
    password string
    device   *MockDevice
    client   pahomqtt.Client
}

func newMQTTClient(host string, port int, username, password string, d *MockDevice) *mqttClient {
    return &mqttClient{
        host:     host,
        port:     port,
        username: username,
        password: password,
        device:   d,
    }
}

func (m *mqttClient) presenceTopic() string {
    return "device/" + m.device.DeviceID + "/presence"
}

func (m *mqttClient) connect() error {
    broker := fmt.Sprintf("tcp://%s:%d", m.host, m.port)
    opts := pahomqtt.NewClientOptions().
        AddBroker(broker).
        SetClientID(m.device.DeviceID).
        SetUsername(m.username).
        SetPassword(m.password).
        SetCleanSession(true).
        SetAutoReconnect(true).
        SetConnectTimeout(10 * time.Second).
        SetWill(m.presenceTopic(), "offline", 0, true).
        SetOnConnectHandler(func(c pahomqtt.Client) {
            c.Publish(m.presenceTopic(), 0, true, "online")
        }).
        SetConnectionLostHandler(func(c pahomqtt.Client, err error) {})

    client := pahomqtt.NewClient(opts)
    token := client.Connect()
    if !token.WaitTimeout(10 * time.Second) {
        return fmt.Errorf("mqtt connect timeout")
    }
    if err := token.Error(); err != nil {
        return fmt.Errorf("mqtt connect: %w", err)
    }
    m.client = client
    return nil
}

func (m *mqttClient) subscribe() error {
    if m.client == nil {
        return fmt.Errorf("mqtt not connected")
    }
    topic := "/device/" + m.device.DeviceID + "/commands"
    token := m.client.Subscribe(topic, 0, func(_ pahomqtt.Client, msg pahomqtt.Message) {
        m.device.addMQTTMsg(MQTTMessage{
            Timestamp: time.Now(),
            Topic:     msg.Topic(),
            Payload:   string(msg.Payload()),
        })
    })
    if !token.WaitTimeout(5 * time.Second) {
        return fmt.Errorf("mqtt subscribe timeout")
    }
    return token.Error()
}

func (m *mqttClient) disconnect() {
    if m.client != nil && m.client.IsConnected() {
        m.client.Publish(m.presenceTopic(), 0, true, "offline").Wait()
        m.client.Disconnect(500)
    }
}
