package zabbixclient

import (
	"fmt"

	reflector "github.com/neogan74/zabbix-alertmanager/zabbixprovisioner/zabbixutil"
)

//Template ...
type Template struct {
	TemplateID  string `json:"templateid,omitempty"`
	Name        string `json:"host,omitempty"`
	Description string `json:"description,omitempty"`
	DisplayName string `json:"name,omitempty"`

	//Used only for creation.
	GroupIds HostGroupIDs `json:"groups,omitempty"`
}

// Templates ..
type Templates []Template

//TemplateGet ...
func (api *API) TemplateGet(params Params) (Templates, error) {
	var res Templates
	if _, exist := params["output"]; !exist {
		params["output"] = "extend"
	}
	resp, err := api.CallWithError("template.get", params)
	if err != nil {
		return nil, err
	}

	reflector.MapsToStructs2(resp.Result.([]interface{}), &res, reflector.Strconv, "json")
	return res, nil
}

//TemplateCreate ..
func (api *API) TemplateCreate(tmpls Templates) error {
	resp, err := api.CallWithError("template.create", tmpls)
	fmt.Println(resp)
	if err != nil {
		return err
	}
	res := resp.Result.(map[string]interface{})
	tmplids := res["templateids"].([]interface{})
	fmt.Println(res)
	fmt.Println(tmplids)
	for i, tmplid := range tmplids {
		tmpls[i].TemplateID = tmplid.(string)
	}
	return nil
}

//TemplatesUpdate ...
func (api *API) TemplatesUpdate(tmpls Templates) error {
	_, err := api.CallWithError("template.update", tmpls)
	if err != nil {
		return err
	}
	return nil
}
