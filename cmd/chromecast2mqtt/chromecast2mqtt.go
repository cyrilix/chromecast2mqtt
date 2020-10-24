package main

import (
	"encoding/json"
	"flag"
	"github.com/cyrilix/chromecast2mqt/mediaplayer"
	"github.com/cyrilix/mqtt-tools/mqttTooling"
	MQTT "github.com/eclipse/paho.mqtt.golang"
	log "github.com/sirupsen/logrus"
	"github.com/vishen/go-chromecast/application"
	"github.com/vishen/go-chromecast/cast"
	"github.com/vishen/go-chromecast/cast/proto"
	"os"
	"strconv"
)

const (
	defaultChromecastPort = 8009
	defaultClientId       = "chromecast2mqtt"
)

func listenEvents(app *application.Application, client MQTT.Client, topic string, mqttParameters *mqttTooling.MqttCliParameters) error {

	app.MediaStart()

	app.AddMessageFunc(func(msg *api.CastMessage) {
		if msg.GetPayloadType() != api.CastMessage_STRING {
			return
		}
		log.Infof("raw msg: %#v", *msg)
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
	var topic, chromecastAddress string
	var chromecastPort int

	flag.StringVar(&topic, "topic", "", "The topic name to publish")
	flag.StringVar(&chromecastAddress, "chromecast-addr", "", "Chromecast device ip address, if not set, discover from network")
	flag.IntVar(&chromecastPort, "chromecast-port", -1, "Chromecast device ip port, if not set, discover from network")

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

	app := initApp(err, chromecastAddress, chromecastPort)

	err = listenEvents(app, client, topic, &parameters)

}

func initApp(err error, chromecastAddress string, chromecastPort int) *application.Application {
	options := make([]mediaplayer.ApplicationOption, 0)
	if chromecastAddress != "" {
		options = append(options, mediaplayer.WithAddress(chromecastAddress))
	}
	if chromecastPort > 0 {
		options = append(options, mediaplayer.WithPort(chromecastPort))
	} else if chromecastAddress != "" {
		// Address set but not port => use default port
		options = append(options, mediaplayer.WithPort(defaultChromecastPort))
	}
	app, err := mediaplayer.NewApplication(
		options...,
	)

	if err != nil {
		log.Fatalf("unable to connect to chromecast application: %v", err)
	}
	return app
}
