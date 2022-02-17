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
	"os/signal"
	"strconv"
	"syscall"
)

const (
	defaultChromecastPort = 8009
	defaultClientId       = "chromecast2mqtt"
)

func listenEvents(app *application.Application, client MQTT.Client, topic string, mqttParameters *mqttTooling.MqttCliParameters, sigChan chan os.Signal) {

	app.MediaStart()
	defer func() {
		if err := app.Stop(); err != nil {
			log.Errorf("unable to stop application: %v", err)
		}
	}()

	logb := log.WithFields(log.Fields{
		"broker": mqttParameters.Broker,
	})
	app.AddMessageFunc(func(msg *api.CastMessage) {
		if msg.GetPayloadType() != api.CastMessage_STRING {
			return
		}
		logb.WithFields(log.Fields{
			"raw_msg": msg.String(),
		}).Debug("new msg")

		payload := msg.GetPayloadUtf8()
		var raw map[string]interface{}
		err := json.Unmarshal([]byte(payload), &raw)
		if err != nil {
			logb.Errorf("unable parse message %v: %v", payload, err)
		}

		switch raw["type"] {
		case "MEDIA_STATUS":
			onMediaStatusEvent(&payload)
		case "RECEIVER_STATUS":
			onReceiverStatusEvent(client, topic, mqttParameters, &payload)
		default:
			log.Infof("unmanaged even: %v", payload)
		}
	})

	<-sigChan
	log.Infof("exit on sigterm")
}

func onMediaStatusEvent(msg *string) {

}

func onReceiverStatusEvent(client MQTT.Client, topic string, mqttParameters *mqttTooling.MqttCliParameters, msg *string) {
	logr := log.WithField("type", "RECEIVER_STATUS")

	logr.WithFields(log.Fields{
		"payload": msg,
	}).Debug("new payload")

	var response cast.ReceiverStatusResponse
	err := json.Unmarshal([]byte(*msg), &response)
	if err != nil {
		logr.Errorf("unable to marshal json response: %v", err)
	}

	mute := "OFF"
	if response.Status.Volume.Muted {
		mute = "ON"
	}

	vol := strconv.Itoa(int(100 * response.Status.Volume.Level))
	logr.WithFields(log.Fields{
		"topic":  topic + "/volume",
		"volume": vol,
	}).Info("publish volume event")
	client.Publish(topic+"/volume", byte(mqttParameters.Qos), mqttParameters.Retain, vol).Wait()

	logr.WithFields(log.Fields{
		"topic": topic + "/mute",
		"mute":  mute,
	}).Info("publish mute event")
	client.Publish(topic+"/mute", byte(mqttParameters.Qos), mqttParameters.Retain, mute).Wait()

}

func main() {
	var topic, chromecastAddress string
	var chromecastPort int
	var debug bool

	flag.StringVar(&topic, "topic", "", "The topic name to publish")
	flag.StringVar(&chromecastAddress, "chromecast-addr", "", "Chromecast device ip address, if not set, discover from network")
	flag.IntVar(&chromecastPort, "chromecast-port", -1, "Chromecast device ip port, if not set, discover from network")
	flag.BoolVar(&debug, "debug", false, "Display debug logs")
	parameters := mqttTooling.MqttCliParameters{
		ClientId: defaultClientId,
	}

	mqttTooling.InitMqttFlagSet(&parameters)
	flag.Parse()
	if len(os.Args) <= 1 {
		flag.PrintDefaults()
		os.Exit(1)
	}
	log.SetFormatter(&log.TextFormatter{
		DisableLevelTruncation: true,
		DisableTimestamp:       true,
		PadLevelText:           true,
	})
	log.SetOutput(os.Stdout)
	if debug {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.InfoLevel)
	}
	log.SetReportCaller(false)

	if topic == "" {
		log.Fatal("topic is mandatory")
	}

	client, err := mqttTooling.Connect(&parameters)
	if err != nil {
		log.WithFields(log.Fields{
			"broker": parameters.Broker,
		}).Fatalf("unable to connect to mqtt bus: %v", err)
	}
	defer func() {
		log.Infof("disconnect mqtt connection")
		client.Disconnect(50)
	}()

	app := initApp(err, chromecastAddress, chromecastPort)

	signChan := make(chan os.Signal)
	signal.Notify(signChan, syscall.SIGTERM)

	listenEvents(app, client, topic, &parameters, signChan)
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
		log.WithFields(log.Fields{
			"address": chromecastAddress,
			"port":    chromecastPort,
		}).Fatalf("unable to connect to chromecast application: %v", err)
	}
	return app
}
