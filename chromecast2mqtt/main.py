import logging
import os
import ssl
import time
from contextlib import contextmanager

from pychromecast import Chromecast
import paho.mqtt.client as mqtt
from pychromecast.socket_client import CastStatus

logger = logging.getLogger(__name__)


class MqttStatusListener:

    def __init__(self, host: str, port: int, topic: str, username: str, password: str = None,
                 client_id: str = 'chromecast2mqtt',
                 ca_certs_file: str = None, cert_file: str = None, key_file: str = None) -> None:
        logger.info("Init mqtt connection")
        client = self._connect_mqtt(host, port, client_id, username, password, ca_certs_file, cert_file, key_file)
        self._topic = topic
        self._client = client
        self._mute_state = 'OFF'

    def _connect_mqtt(self, host: str, port: int, client_id: str, username: str, password: str,
                      ca_certs_file: str = None, cert_file: str = None, key_file: str = None) -> mqtt.Client:
        client = mqtt.Client(client_id=client_id, clean_session=False, userdata=self,
                             protocol=mqtt.MQTTv311)
        client.username_pw_set(username=username, password=password)
        if ca_certs:
            client.tls_set(ca_certs=ca_certs_file, certfile=cert_file, keyfile=key_file, cert_reqs=ssl.CERT_REQUIRED,
                           tls_version=ssl.PROTOCOL_TLSv1_2)

        retry = True
        while retry:
            try:
                err = client.connect(host, port)
                if mqtt.MQTT_ERR_SUCCESS == err:
                    retry = False
                    break
                logging.error('Unable to connect to mqtt, return with code %s', err)
                time.sleep(10)
            except (ssl.SSLError, ssl.SSLEOFError) as e:
                logging.error('Unable to connect to mqtt', exc_info=e)
                time.sleep(10)
        client.loop_start()
        return client

    def new_cast_status(self, status: CastStatus):
        logger.debug("New status: %s", status)

        payload_volume = int(100 * status.volume_level)
        payload_mute = "ON" if status.volume_muted else "OFF"
        logger.info("New event: volume=%s, mute=%s", payload_volume, payload_mute)

        self._client.publish(topic=self._topic + "/volume", payload=payload_volume, qos=0,
                             retain=True).wait_for_publish()
        if payload_mute != self._mute_state:
            self._mute_state = payload_mute
            self._client.publish(topic=self._topic + "/mute", payload=payload_mute, qos=0,
                                 retain=True).wait_for_publish()

    def close(self):
        logger.info("Disconnect mqtt")
        self._client.loop_stop()
        self._client.disconnect()

    @staticmethod
    @contextmanager
    def listener(**args):
        mqtt_listener = MqttStatusListener(**args)
        yield mqtt_listener
        logger.error('Close mqtt listener')
        mqtt_listener.close()


class ConfigurationException(Exception):
    pass


def main():
    logging.basicConfig(level=logging.INFO)

    if 'CHROMECAST_HOST' not in os.environ:
        logger.info("Environment variable 'CHROMECAST_HOST' not defined")
        raise ConfigurationException("Environment variable 'CHROMECAST_HOST' not defined")
    host = os.environ['CHROMECAST_HOST']

    if 'MQTT_HOST' not in os.environ:
        logger.info("Environment variable 'MQTT_HOST' not defined")
        raise ConfigurationException("Environment variable 'MQTT_HOST' not defined")
    mqtt_host = os.environ['MQTT_HOST']

    if 'MQTT_PORT' not in os.environ:
        logger.info("Environment variable 'MQTT_PORT' not defined")
        raise ConfigurationException("Environment variable 'MQTT_PORT' not defined")
    mqtt_port = int(os.environ['MQTT_PORT'])

    if 'MQTT_USERNAME' not in os.environ:
        logger.info("Environment variable 'MQTT_USERNAME' not defined")
        raise ConfigurationException("Environment variable 'MQTT_USERNAME' not defined")
    username = os.environ['MQTT_USERNAME']

    if 'MQTT_PASSWORD' not in os.environ:
        logger.info("Environment variable 'MQTT_PASSWORD' not defined")
        raise ConfigurationException("Environment variable 'MQTT_PASSWORD' not defined")
    password = os.environ['MQTT_PASSWORD']

    if 'MQTT_TOPIC_BASE' not in os.environ:
        logger.info("Environment variable 'MQTT_TOPIC_BASE' not defined")
        raise ConfigurationException("Environment variable 'MQTT_TOPIC_BASE' not defined")
    topic_base = os.environ['MQTT_TOPIC_BASE']

    ca_certs = os.environ['CA_CERTS_FILE']
    cert_file = os.environ['CERT_FILE']
    key_file = os.environ['KEY_FILE']

    with MqttStatusListener.listener(host=mqtt_host,
                                     port=mqtt_port,
                                     topic=topic_base,
                                     username=username,
                                     password=password,
                                     ca_certs_file=ca_certs,
                                     cert_file=cert_file,
                                     key_file=key_file) as mqtt_listener:
        cast = Chromecast(host=host)
        cast.register_status_listener(mqtt_listener)
        logger.info("Start chromecast connection")
        cast.start()
        cast.join()
