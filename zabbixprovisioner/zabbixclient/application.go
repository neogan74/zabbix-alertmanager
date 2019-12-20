package zabbixclient

import (
	reflector "github.com/neogan74/zabbix-alertmanager/zabbixprovisioner/zabbixutil"
)

//Application struct https://www.zabbix.com/documentation/4.4/manual/appendix/api/application/definitions
type Application struct {
	ApplicationID string `json:"applicationid,omitempty"`
	HostID        string `json:"hostid"`
	Name          string `json:"name"`
	TemplateID    string `json:"templateid,omitempty"`
}

//Applications slice of Application struct
type Applications []Application

//ApplicationsGet ...Wrapper for application.get: https://www.zabbix.com/documentation/4.4/manual/appendix/api/application/get
func (api *API) ApplicationsGet(params Params) (Applications, error) {
	var res Applications
	if _, present := params["output"]; !present {
		params["output"] = "extend"
	}
	response, err := api.CallWithError("application.get", params)
	if err != nil {
		return nil, err
	}

	reflector.MapsToStructs2(response.Result.([]interface{}), &res, reflector.Strconv, "json")
	return res, nil
}

//ApplicationGetByID Gets application by Id only if there is exactly 1 matching application.
func (api *API) ApplicationGetByID(id string) (*Application, error) {
	var res *Application
	apps, err := api.ApplicationsGet(Params{"applicationids": id})
	if err != nil {
		return nil, err
	}

	if len(apps) == 1 {
		res = &apps[0]
	} else {
		e := ExpectedOneResult(len(apps))
		err = &e
		return nil, err
	}
	return res, nil
}

//ApplicationGetByHostIDAndName Gets application by host Id and name only if there is exactly 1 matching application.
func (api *API) ApplicationGetByHostIDAndName(hostID, name string) (*Application, error) {
	var res *Application
	apps, err := api.ApplicationsGet(Params{"hostids": hostID, "filter": map[string]string{"name": name}})
	if err != nil {
		return nil, err
	}

	if len(apps) == 1 {
		res = &apps[0]
	} else {
		e := ExpectedOneResult(len(apps))
		err = &e
		return nil, err
	}
	return res, nil
}

//ApplicationsCreate Wrapper for application.create: https://www.zabbix.com/documentation/2.2/manual/appendix/api/application/create
func (api *API) ApplicationsCreate(apps Applications) error {
	response, err := api.CallWithError("application.create", apps)
	if err != nil {
		return err
	}

	result := response.Result.(map[string]interface{})
	applicationids := result["applicationids"].([]interface{})
	for i, id := range applicationids {
		apps[i].ApplicationID = id.(string)
	}
	return nil
}

//ApplicationsDelete Wrapper for application.delete: https://www.zabbix.com/documentation/2.2/manual/appendix/api/application/delete
// Cleans ApplicationID in all apps elements if call succeed.
func (api *API) ApplicationsDelete(apps Applications) error {
	ids := make([]string, len(apps))
	for i, app := range apps {
		ids[i] = app.ApplicationID
	}

	err := api.ApplicationsDeleteByIDs(ids)
	if err != nil {
		return err
	}

	for i := range apps {
		apps[i].ApplicationID = ""
	}
	return nil

}

//ApplicationsDeleteByIDs Wrapper for application.delete: https://www.zabbix.com/documentation/2.2/manual/appendix/api/application/delete
func (api *API) ApplicationsDeleteByIDs(ids []string) error {
	response, err := api.CallWithError("application.delete", ids)
	if err != nil {
		return err
	}

	result := response.Result.(map[string]interface{})
	applicationids := result["applicationids"].([]interface{})
	if len(ids) != len(applicationids) {
		err = &ExpectedMore{len(ids), len(applicationids)}
		return err
	}
	return nil
}
