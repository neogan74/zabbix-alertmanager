package zabbix

import reflector "github.com/neogan74/zabbix-alertmanager/zabbixprovisioner/zabbixutil"

//Template ...
type Template struct {
	TemplateID  string `json:"hostid:omitempty"`
	Name        string `json:"host,omitempty"`
	Description string `json:"description,omitempty"`
	DisplayName string `json:"name,omitempty"`

	//Used only for creation.
	GroupIds HostGroupIds `json:"groups,omitempty"`
}

// Templates ..
type Templates []Template

//TemplateGet ...
func (api *API) TemplateGet(params Params) (Templates, error) {
	var res Templates
	if _, exist := params["output"]; !exist {
		params["output"] = "extend"
	}
	resp, err := api.CallWithError("host.get", params)
	if err != nil {
		return nil, err
	}

	reflector.MapsToStructs2(resp.Result.([]interface{}), &res, reflector.Strconv, "json")
	return res, nil
}
//TemplateCreate ..
func (api *API) TemplateCreate(tmpls Templates) error  {
	// resp, err := api.CallWithError("template.create",tmpls)

	// if err != nil {
	// 	return err
	// }
	// 	res := resp.Result.(map[string]interface{})
	// 	tmplids := res["templateids"].([]interface{})
	// 	for i,tmplid := range tmplids {
	// 		tmpls[i].TemplateID = id.(string)
	// 	}
	return nil
}