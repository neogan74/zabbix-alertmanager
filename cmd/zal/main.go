package main

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/neogan74/zabbix-alertmanager/zabbixprovisioner/provisioner"
	"github.com/neogan74/zabbix-alertmanager/zabbixsender/zabbixsnd"
	"github.com/neogan74/zabbix-alertmanager/zabbixsender/zabbixsvc"
	"github.com/povilasv/prommod"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	ver "github.com/prometheus/common/version"
	log "github.com/sirupsen/logrus"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

func main() {
	app := kingpin.New("zal", "Zabbix and Prometheus integration.")

	app.Version(ver.Print("zal"))
	app.HelpFlag.Short('h')

	send := app.Command("send", "Listens for Alert requests from Alertmanager and sends them to Zabbix.")
	senderAddr := send.Flag("addr", "Server address which will receive alerts from alertmanager.").Default("0.0.0.0:9095").String()
	zabbixAddr := send.Flag("zabbix-addr", "Zabbix address.").Envar("ZABBIX_URL").Required().String()
	hostsFile := send.Flag("hosts-path", "Path to resolver to host mapping file.").String()
	keyPrefix := send.Flag("key-prefix", "Prefix to add to the trapper item key").Default("prometheus").String()
	defaultHost := send.Flag("default-host", "default host to send alerts to").Default("prometheus").String()

	prov := app.Command("prov", "Reads Prometheus Alerting rules and converts them into Zabbix Triggers.")
	provConfig := prov.Flag("config-path", "Path to provisioner hosts config file.").Required().String()
	provUser := prov.Flag("user", "Zabbix json rpc user.").Envar("ZABBIX_USER").Required().String()
	provPassword := prov.Flag("password", "Zabbix json rpc password.").Envar("ZABBIX_PASSWORD").Required().String()
	provURL := prov.Flag("url", "Zabbix json rpc url.").Envar("ZABBIX_URL").Default("http://127.0.0.1/zabbix/api_jsonrpc.php").String()
	provKeyPrefix := prov.Flag("key-prefix", "Prefix to add to the trapper item key.").Default("prometheus").String()
	prometheusURL := prov.Flag("prometheus-url", "Prometheus URL.").Default("").String()

	test := app.Command("test", "Test different things")

	logLevel := app.Flag("log.level", "Log level.").
		Default("info").Enum("error", "warn", "info", "debug")
	logFormat := app.Flag("log.format", "Log format.").
		Default("text").Enum("text", "json")

	cmd := kingpin.MustParse(app.Parse(os.Args[1:]))

	switch strings.ToLower(*logLevel) {
	case "error":
		log.SetLevel(log.ErrorLevel)
	case "warn":
		log.SetLevel(log.WarnLevel)
	case "info":
		log.SetLevel(log.InfoLevel)
	case "debug":
		log.SetLevel(log.DebugLevel)
	}

	switch strings.ToLower(*logFormat) {
	case "json":
		log.SetFormatter(&log.JSONFormatter{})
	case "text":
		log.SetFormatter(&log.TextFormatter{DisableColors: true})
	}
	log.SetOutput(os.Stdout)

	prometheus.MustRegister(ver.NewCollector("zal"))
	prometheus.MustRegister(prommod.NewCollector("zal"))
	switch cmd {
	case send.FullCommand():
		s, err := zabbixsnd.New(*zabbixAddr)
		if err != nil {
			log.Fatalf("error could not create zabbix sender: %v", err)
		}

		hosts := make(map[string]string)

		if hostsFile != nil && *hostsFile != "" {
			hosts, err = zabbixsvc.LoadHostsFromFile(*hostsFile)
			if err != nil {
				log.Errorf("cant load the default hosts file: %v", err)
			}
		}

		h := &zabbixsvc.JSONHandler{
			Sender:      s,
			KeyPrefix:   *keyPrefix,
			DefaultHost: *defaultHost,
			Hosts:       hosts,
		}

		http.Handle("/metrics", promhttp.Handler())
		http.HandleFunc("/alerts", h.HandlePost)

		log.Info("Zabbix sender started, listening on ", *senderAddr)
		if err := http.ListenAndServe(*senderAddr, nil); err != nil {
			log.Fatal(err)
		}

	case prov.FullCommand():
		cfg, err := provisioner.LoadHostConfigFromFile(*provConfig)
		if err != nil {
			log.Fatal(err)
		}
		log.Infof("loaded hosts configuration from '%s'", *provConfig)

		prov, err := provisioner.New(*prometheusURL, *provKeyPrefix, *provURL, *provUser, *provPassword, cfg)
		if err != nil {
			log.Fatalf("error failed to create provisioner: %s", err)
		}

		if err := prov.Run(); err != nil {
			log.Fatalf("error provisioning zabbix items: %s", err)
		}
	case test.FullCommand():
		//get targets from prom
		log.Infof("in testing")

		type TargetsList struct {
			Status string `json:"status"`
			Data   struct {
				ActiveTargets []struct {
					DiscoveredLabels struct {
						Address     string `json:"__address__"`
						MetricsPath string `json:"__metrics_path__"`
						Scheme      string `json:"__scheme__"`
						Group       string `json:"group"`
						Job         string `json:"job"`
					} `json:"discoveredLabels"`
					Labels struct {
						Group    string `json:"group"`
						Instance string `json:"instance"`
						Job      string `json:"job"`
					} `json:"labels"`
					ScrapeURL  string    `json:"scrapeUrl"`
					LastError  string    `json:"lastError"`
					LastScrape time.Time `json:"lastScrape"`
					Health     string    `json:"health"`
				} `json:"activeTargets"`
				DroppedTargets []interface{} `json:"droppedTargets"`
			} `json:"data"`
		}

		/*
			//prom query
			client, err := api.NewClient(api.Config{
				Address: "http://51.15.213.9:9090",
			})
			if err != nil {
				log.Fatalf("Error in request %s", err)
				os.Exit(1)
			}

			v1api := v1.NewAPI(client)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			result, err := v1api.Query(ctx, "up", time.Now())
			if err != nil {
				log.Fatalf("Error in quering to Prometheus: %s", err)
				os.Exit(1)
			}
			data, err := result.Type().MarshalJSON()
			if err != nil {
				log.Fatalf("Error whil unmarshal json from result", err)
			}

			m := string(data[:])

			log.Infof("Result:\n %v\n", m)
			log.Infof("Result:\n %v\n", result.String())
		*/

		resp, err := http.Get("http://51.15.213.9:9090/api/v1/targets")
		if err != nil {
			log.Infof("Error while get targets: %v\n", err)
			os.Exit(1)
		}
		defer resp.Body.Close()

		data, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Fatalf("Error with body: %v\n", err)
		}
		// log.Infof("Targets: %v\n", data)

		var targets TargetsList
		err = json.Unmarshal(data, &targets)
		if err != nil {
			log.Fatalf("Error while JSON unmarshal %s", err)
		}
		log.Infof("Targets: %v\n", targets)
		for _, v := range targets.Data.ActiveTargets {

			log.Infof("%v\n", v.Labels.Instance[:strings.LastIndex(v.Labels.Instance, ":")])
		}

	}
}

func interrupt(logger log.Logger, cancel <-chan struct{}) error {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	select {
	case s := <-c:
		log.Info("caught signal. Exiting.", "signal", s)
		return nil
	case <-cancel:
		return errors.New("canceled")
	}
}
