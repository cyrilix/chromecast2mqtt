package mediaplayer

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/vishen/go-chromecast/application"
	castdns "github.com/vishen/go-chromecast/dns"
	"github.com/vishen/go-chromecast/storage"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

func init() {
	log.SetOutput(os.Stdout)
	log.SetLevel(log.DebugLevel)
}

var (
	cache = storage.NewStorage()
)

type CachedDNSEntry struct {
	UUID string `json:"uuid"`
	Name string `json:"name"`
	Addr string `json:"addr"`
	Port int    `json:"port"`
}

func (e CachedDNSEntry) GetUUID() string {
	return e.UUID
}

func (e CachedDNSEntry) GetName() string {
	return e.Name
}

func (e CachedDNSEntry) GetAddr() string {
	return e.Addr
}

func (e CachedDNSEntry) GetPort() int {
	return e.Port
}

type ApplicationOption func(*ApplicationOptions)

type ApplicationOptions struct {
	deviceName        string
	deviceUuid        string
	device            string
	disableCache      bool
	addr              string
	port              int
	ifaceName         string
	dnsTimeoutSeconds int64
	useFirstDevice    bool
}

func WithAddress(addr string) ApplicationOption {
	return func(o *ApplicationOptions) {
		o.addr = addr
	}
}
func WithPort(port int) ApplicationOption {
	return func(o *ApplicationOptions) {
		o.port = port
	}
}

var defaultApplicationOptions = ApplicationOptions{
	deviceName:        "",
	deviceUuid:        "",
	device:            "",
	disableCache:      true,
	addr:              "",
	port:              -1,
	ifaceName:         "",
	dnsTimeoutSeconds: 10,
	useFirstDevice:    true,
}

func NewApplication(opts ...ApplicationOption) (*application.Application, error) {
	options := defaultApplicationOptions
	for _, o := range opts {
		o(&options)
	}

	applicationOptions := []application.ApplicationOption{
		application.WithCacheDisabled(options.disableCache),
	}

	// If we need to look on a specific network interface for mdns or
	// for finding a network ip to host from, ensure that the network
	// interface exists.
	var iface *net.Interface
	if options.ifaceName != "" {
		var err error
		if iface, err = net.InterfaceByName(options.ifaceName); err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("unable to find interface %q", options.ifaceName))
		}
		applicationOptions = append(applicationOptions, application.WithIface(iface))
	}

	var entry castdns.CastDNSEntry
	// If no address was specified, attempt to determine the address of any
	// local chromecast devices.
	if options.addr == "" {
		// If a device name or uuid was specified, check the cache for the ip+port
		found := false
		if !options.disableCache && (options.deviceName != "" || options.deviceUuid != "") {
			entry = findCachedCastDNS(options.deviceName, options.deviceUuid)
			found = entry.GetAddr() != ""
		}
		if !found {
			var err error
			if entry, err = findCastDNS(iface, &options); err != nil {
				return nil, errors.Wrap(err, "unable to find cast dns entry")
			}
		}
		if !options.disableCache {
			cachedEntry := CachedDNSEntry{
				UUID: entry.GetUUID(),
				Name: entry.GetName(),
				Addr: entry.GetAddr(),
				Port: entry.GetPort(),
			}
			cachedEntryJson, _ := json.Marshal(cachedEntry)
			cache.Save(getCacheKey(cachedEntry.UUID), cachedEntryJson)
			cache.Save(getCacheKey(cachedEntry.Name), cachedEntryJson)
		}
		log.WithFields(log.Fields{
			"name": entry.GetName(),
			"addr": entry.GetAddr(),
			"port": entry.GetPort(),
			"uuid": entry.GetUUID(),
		}).Info("device found")
	} else {
		if options.port <= 0 {
			return nil, errors.Errorf("port needs to be a number > 0: port=%v", options.port)
		}
		entry = CachedDNSEntry{
			Addr: options.addr,
			Port: options.port,
		}
	}
	app := application.NewApplication(applicationOptions...)
	if err := app.Start(entry); err != nil {
		// NOTE: currently we delete the dns cache every time we get
		// an error, this is to make sure that if the device gets a new
		// ipaddress we will invalidate the cache.
		cache.Save(getCacheKey(entry.GetUUID()), []byte{})
		cache.Save(getCacheKey(entry.GetName()), []byte{})
		return nil, err
	}
	return app, nil
}

func getCacheKey(suffix string) string {
	return fmt.Sprintf("cmd/utils/dns/%s", suffix)
}

func findCachedCastDNS(deviceName, deviceUuid string) castdns.CastDNSEntry {
	for _, s := range []string{deviceName, deviceUuid} {
		cacheKey := getCacheKey(s)
		if b, err := cache.Load(cacheKey); err == nil {
			cachedEntry := CachedDNSEntry{}
			if err := json.Unmarshal(b, &cachedEntry); err == nil {
				return cachedEntry
			}
		}
	}
	return CachedDNSEntry{}
}

func findCastDNS(iface *net.Interface, options *ApplicationOptions) (castdns.CastDNSEntry, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(options.dnsTimeoutSeconds))
	defer cancel()
	castEntryChan, err := castdns.DiscoverCastDNSEntries(ctx, iface)
	if err != nil {
		return castdns.CastEntry{}, err
	}

	var foundEntries []castdns.CastEntry
	for entry := range castEntryChan {
		if options.useFirstDevice || (options.deviceUuid != "" && entry.UUID == options.deviceUuid) || (options.deviceName != "" && entry.DeviceName == options.deviceName) || (options.device != "" && entry.Device == options.device) {
			return entry, nil
		}
		foundEntries = append(foundEntries, entry)
	}

	if len(foundEntries) == 0 {
		return castdns.CastEntry{}, fmt.Errorf("no cast devices found on network")
	}

	// Always return entries in deterministic order.
	sort.Slice(foundEntries, func(i, j int) bool { return foundEntries[i].DeviceName < foundEntries[j].DeviceName })

	if log.IsLevelEnabled(log.InfoLevel) {
		log.Infof("Found %d cast dns entries, select one:\n", len(foundEntries))
		for i, d := range foundEntries {
			log.Infof("%d) device=%q device_name=%q address=\"%s:%d\" uuid=%q\n", i+1, d.Device, d.DeviceName, d.AddrV4, d.Port, d.UUID)
		}
	}

	reader := bufio.NewReader(os.Stdin)
	for {
		log.Infof("Enter selection: ")
		text, err := reader.ReadString('\n')
		if err != nil {
			log.Errorf("error reading console: %v\n", err)
			continue
		}
		i, err := strconv.Atoi(strings.TrimSpace(text))
		if err != nil {
			continue
		} else if i < 1 || i > len(foundEntries) {
			continue
		}
		return foundEntries[i-1], nil
	}
}
