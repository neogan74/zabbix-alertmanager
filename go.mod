module github.com/neogan74/zabbix-alertmanager

go 1.13

require (
	github.com/neogan74/zabbix-alertmanager/zabbixprovisioner/provisioner v0.0.0-20191217170800-975e2cd0ebc6
	github.com/neogan74/zabbix-alertmanager/zabbixprovisioner/zabbixclient v0.0.0-20191217165301-3f2b73ce9122
	github.com/pkg/errors v0.8.1
	github.com/povilasv/prommod v0.0.12
	github.com/prometheus/client_golang v1.2.1
	github.com/prometheus/common v0.7.0
	github.com/sirupsen/logrus v1.4.2
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	gopkg.in/yaml.v2 v2.2.7
)
