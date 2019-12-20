package zabbixclient

import (
	reflector "github.com/neogan74/zabbix-alertmanager/zabbixprovisioner/zabbixutil"
)

//InternalType ...
type InternalType int

//NotInternal ..
const (
	NotInternal InternalType = 0
	Internal    InternalType = 1
)

//HostGroup https://www.zabbix.com/documentation/4.4/manual/appendix/api/hostgroup/definitions
type HostGroup struct {
	GroupID  string       `json:"groupid,omitempty"`
	Name     string       `json:"name"`
	Internal InternalType `json:"internal,omitempty"`
}

//HostGroups ....
type HostGroups []HostGroup

//HostGroupID ...
type HostGroupID struct {
	GroupID string `json:"groupid"`
}

//HostGroupIDs ...
type HostGroupIDs []HostGroupID

//HostGroupsGet Wrapper for hostgroup.get: https://www.zabbix.com/documentation/4.4/manual/appendix/api/hostgroup/get
func (api *API) HostGroupsGet(params Params) (HostGroups, error) {
	var res HostGroups
	if _, present := params["output"]; !present {
		params["output"] = "extend"
	}
	response, err := api.CallWithError("hostgroup.get", params)
	if err != nil {
		return nil, err
	}

	reflector.MapsToStructs2(response.Result.([]interface{}), &res, reflector.Strconv, "json")
	return res, nil
}

//HostGroupGetByID Gets host group by Id only if there is exactly 1 matching host group.
func (api *API) HostGroupGetByID(id string) (*HostGroup, error) {
	var res *HostGroup
	groups, err := api.HostGroupsGet(Params{"groupids": id})
	if err != nil {
		return nil, err
	}

	if len(groups) == 1 {
		res = &groups[0]
	} else {
		e := ExpectedOneResult(len(groups))
		err = &e
		return nil, err
	}
	return res, nil
}

//HostGroupsCreate Wrapper for hostgroup.create: https://www.zabbix.com/documentation/4.4/manual/appendix/api/hostgroup/create
func (api *API) HostGroupsCreate(hostGroups HostGroups) error {
	response, err := api.CallWithError("hostgroup.create", hostGroups)
	if err != nil {
		return err
	}

	result := response.Result.(map[string]interface{})
	groupids := result["groupids"].([]interface{})
	for i, id := range groupids {
		hostGroups[i].GroupID = id.(string)
	}
	return nil
}

//HostGroupsDelete Wrapper for hostgroup.delete: https://www.zabbix.com/documentation/4.4/manual/appendix/api/hostgroup/delete
// Cleans GroupId in all hostGroups elements if call succeed.
func (api *API) HostGroupsDelete(hostGroups HostGroups) error {
	ids := make([]string, len(hostGroups))
	for i, group := range hostGroups {
		ids[i] = group.GroupID
	}

	err := api.HostGroupsDeleteByIds(ids)
	if err != nil {
		return err
	}

	for i := range hostGroups {
		hostGroups[i].GroupID = ""
	}

	return nil
}

//HostGroupsDeleteByIds Wrapper for hostgroup.delete: https://www.zabbix.com/documentation/4.4/manual/appendix/api/hostgroup/delete
func (api *API) HostGroupsDeleteByIds(ids []string) error {
	response, err := api.CallWithError("hostgroup.delete", ids)
	if err != nil {
		return err
	}

	result := response.Result.(map[string]interface{})
	groupids := result["groupids"].([]interface{})
	if len(ids) != len(groupids) {
		err = &ExpectedMore{len(ids), len(groupids)}
		return err
	}
	return nil
}
