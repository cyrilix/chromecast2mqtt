package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/cyrilix/chromecast2mqt/mediaplayer"
	"github.com/cyrilix/mqtt-tools/mqttTooling"
	MQTT "github.com/eclipse/paho.mqtt.golang"
	log "github.com/sirupsen/logrus"
	"github.com/vishen/go-chromecast/cast"
	"github.com/vishen/go-chromecast/cast/proto"
	"os"
	"strconv"
)

var (
	defaultClientId = "chromecast2mqtt"
)

func listenEvents(client MQTT.Client, topic string, mqttParameters *mqttTooling.MqttCliParameters) error {

	app, err := mediaplayer.NewApplication()
	if err != nil {
		return fmt.Errorf("unable to connect to chromecast application: %v", err)
	}

	app.AddMessageFunc(func(msg *api.CastMessage) {
		if msg.GetPayloadType() != api.CastMessage_STRING {
			return
		}

		payload := msg.GetPayloadUtf8()
		var response cast.ReceiverStatusResponse
		err := json.Unmarshal([]byte(payload), &response)
		if err != nil {
			log.Errorf("unable to marshal json response: %v", err)
		}
		log.Debugf("new payload: %#v", response)

		mute := "OFF"
		if response.Status.Volume.Muted {
			mute = "ON"
		}
		log.Infof("publish to topic %v/volume", topic)
		client.Publish(topic+"/volume", byte(mqttParameters.Qos), mqttParameters.Retain, strconv.Itoa(int(100*response.Status.Volume.Level))).Wait()
		log.Infof("publish to topic %v/mute", topic)
		client.Publish(topic+"/mute", byte(mqttParameters.Qos), mqttParameters.Retain, mute).Wait()

	})
	app.MediaWait()
	return nil
}

func main() {
	var topic string

	flag.StringVar(&topic, "topic", "", "The topic name to publish")

	parameters := mqttTooling.MqttCliParameters{
		ClientId: defaultClientId,
	}
	mqttTooling.InitMqttFlagSet(&parameters)
	flag.Parse()
	if len(os.Args) <= 1 {
		flag.PrintDefaults()
		os.Exit(1)
	}

	if topic == "" {
		log.Fatal("topic is mandatory")
	}

	client, err := mqttTooling.Connect(&parameters)
	if err != nil {
		log.Fatalf("unable to connect to mqtt bus: %v", err)
	}
	defer client.Disconnect(50)

	err = listenEvents(client, topic, &parameters)

}
