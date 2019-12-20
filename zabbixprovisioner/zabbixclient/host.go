package zabbixclient

import (
	"reflect"

	reflector "github.com/neogan74/zabbix-alertmanager/zabbixprovisioner/zabbixutil"
)

//AvailableType ...
type AvailableType int

//StatusType ...
type StatusType int

//InventoryType ...
type InventoryType int

//Available ...
const (
	Available   AvailableType = 1
	Unavailable AvailableType = 2

	Monitored   StatusType = 0
	Unmonitored StatusType = 1

	InventoryDisabled  InventoryType = -1
	InventoryManual    InventoryType = 0
	InventoryAutomatic InventoryType = 1
)

//Host https://www.zabbix.com/documentation/4.4/manual/appendix/api/host/definitions
type Host struct {
	HostID    string        `json:"hostid,omitempty"`
	Host      string        `json:"host"`
	Available AvailableType `json:"available"`
	Error     string        `json:"error"`
	Name      string        `json:"name"`
	Status    StatusType    `json:"status"`

	InventoryMode InventoryType     `json:"inventory_mode"`
	Inventory     map[string]string `json:"inventory"`

	// Fields below used only when creating hosts
	GroupIds   HostGroupIDs   `json:"groups,omitempty"`
	Interfaces HostInterfaces `json:"interfaces,omitempty"`
}

//Hosts ...
type Hosts []Host

//HostsGet Wrapper for host.get: https://www.zabbix.com/documentation/2.2/manual/appendix/api/host/get
func (api *API) HostsGet(params Params) (Hosts, error) {
	var res Hosts
	if _, present := params["output"]; !present {
		params["output"] = "extend"
	}
	response, err := api.CallWithError("host.get", params)
	if err != nil {
		return nil, err
	}

	reflector.MapsToStructs2(response.Result.([]interface{}), &res, reflector.Strconv, "json")

	if _, ok := params["selectInventory"]; ok {
		results := response.Result.([]interface{})
		for i := range results {
			host := results[i].(map[string]interface{})
			if reflect.TypeOf(host["inventory"]).Kind() == reflect.Map {
				inventory := host["inventory"].(map[string]interface{})
				res[i].Inventory = make(map[string]string, len(inventory))
				for key, value := range inventory {
					res[i].Inventory[key] = value.(string)
				}
			}
		}
	}
	return res, nil
}

//HostsGetByHostGroupIDs Gets hosts by host group Ids.
func (api *API) HostsGetByHostGroupIDs(IDs []string) (res Hosts, err error) {
	return api.HostsGet(Params{"groupids": IDs})
}

//HostsGetByHostGroups Gets hosts by host groups.
func (api *API) HostsGetByHostGroups(hostGroups HostGroups) (res Hosts, err error) {
	IDs := make([]string, len(hostGroups))
	for i, id := range hostGroups {
		IDs[i] = id.GroupID
	}
	return api.HostsGetByHostGroupIDs(IDs)
}

//HostGetByID Gets host by Id only if there is exactly 1 matching host.
func (api *API) HostGetByID(id string) (*Host, error) {
	var res *Host
	hosts, err := api.HostsGet(Params{"hostids": id})
	if err != nil {
		return nil, err
	}

	if len(hosts) == 1 {
		res = &hosts[0]
	} else {
		e := ExpectedOneResult(len(hosts))
		err = &e
		return nil, err
	}
	return res, nil
}

//HostGetByHost Gets host by Host only if there is exactly 1 matching host.
func (api *API) HostGetByHost(host string) (*Host, error) {
	var res *Host
	hosts, err := api.HostsGet(Params{"filter": map[string]string{"host": host}})
	if err != nil {
		return nil, err
	}

	if len(hosts) == 1 {
		res = &hosts[0]
	} else {
		e := ExpectedOneResult(len(hosts))
		err = &e
		return nil, err
	}
	return res, nil
}

//HostsCreate Wrapper for host.create: https://www.zabbix.com/documentation/2.2/manual/appendix/api/host/create
func (api *API) HostsCreate(hosts Hosts) error {
	response, err := api.CallWithError("host.create", hosts)
	if err != nil {
		return err
	}

	result := response.Result.(map[string]interface{})
	hostids := result["hostids"].([]interface{})
	for i, id := range hostids {
		hosts[i].HostID = id.(string)
	}
	return nil
}

//HostsUpdate Wrapper for host.update: https://www.zabbix.com/documentation/2.2/manual/appendix/api/host/update
func (api *API) HostsUpdate(hosts Hosts) error {
	_, err := api.CallWithError("host.update", hosts)
	if err != nil {
		return err
	}
	return nil
}

//HostsDelete Wrapper for host.delete: https://www.zabbix.com/documentation/2.2/manual/appendix/api/host/delete
// Cleans HostId in all hosts elements if call succeed.
func (api *API) HostsDelete(hosts Hosts) error {
	ids := make([]string, len(hosts))
	for i, host := range hosts {
		ids[i] = host.HostID
	}

	err := api.HostsDeleteByIDs(ids)
	if err != nil {
		return err
	}
	for i := range hosts {
		hosts[i].HostID = ""
	}

	return nil
}

//HostsDeleteByIDs Wrapper for host.delete: https://www.zabbix.com/documentation/2.2/manual/appendix/api/host/delete
func (api *API) HostsDeleteByIDs(IDs []string) error {
	hostIds := make([]map[string]string, len(IDs))
	for i, id := range IDs {
		hostIds[i] = map[string]string{"hostid": id}
	}

	response, err := api.CallWithError("host.delete", hostIds)
	if err != nil {
		// Zabbix 2.4 uses new syntax only
		if e, ok := err.(*Error); ok && e.Code == -32500 {
			response, err = api.CallWithError("host.delete", IDs)
			if err != nil {
				return err
			}
		}
	}
	if err != nil {
		return err
	}

	result := response.Result.(map[string]interface{})
	hostids := result["hostids"].([]interface{})
	if len(IDs) != len(hostids) {
		err = &ExpectedMore{len(IDs), len(hostids)}
		return err
	}
	return nil
}
