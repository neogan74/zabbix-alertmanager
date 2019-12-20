package zabbixclient

//InterfaceType ...
type InterfaceType int

//Agent ...
const (
	Agent InterfaceType = 1
	SNMP  InterfaceType = 2
	IPMI  InterfaceType = 3
	JMX   InterfaceType = 4
)

//HostInterface https://www.zabbix.com/documentation/4.4/manual/appendix/api/hostinterface/definitions
type HostInterface struct {
	DNS   string        `json:"dns"`
	IP    string        `json:"ip"`
	Main  int           `json:"main"`
	Port  string        `json:"port"`
	Type  InterfaceType `json:"type"`
	UseIP int           `json:"useip"`
}

//HostInterfaces ...
type HostInterfaces []HostInterface
