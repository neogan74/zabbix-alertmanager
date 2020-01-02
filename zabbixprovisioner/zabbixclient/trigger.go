package zabbixclient

import (
	reflector "github.com/devopyio/zabbix-alertmanager/zabbixprovisioner/zabbixutil"
)

//PriorityType ...
type PriorityType int

//NotClassified ...
const (
	NotClassified PriorityType = 0
	Information   PriorityType = 1
	Warning       PriorityType = 2
	Average       PriorityType = 3
	High          PriorityType = 4
	Critical      PriorityType = 5

	Enabled  StatusType = 0
	Disabled StatusType = 1

	OK      ValueType = 0
	Problem ValueType = 1
)

//Trigger https://www.zabbix.com/documentation/4.4/manual/appendix/api/item/definitions
type Trigger struct {
	TriggerID   string       `json:"triggerid,omitempty"`
	Description string       `json:"description"`
	Expression  string       `json:"expression"`
	Comments    string       `json:"comments"`
	URL         string       `json:"url"`
	ManualClose int32        `json:"manual_close"`
	Priority    PriorityType `json:"priority"`
	Status      StatusType   `json:"status"`
}

//Tag ...
type Tag struct {
	Tag   string `json:"tag"`
	Value string `json:"value"`
}

//Triggers ...
type Triggers []Trigger

//TriggersGet Wrapper for item.get https://www.zabbix.com/documentation/4.4/manual/appendix/api/item/get
func (api *API) TriggersGet(params Params) (Triggers, error) {
	var res Triggers
	if _, present := params["output"]; !present {
		params["output"] = "extend"
	}
	response, err := api.CallWithError("trigger.get", params)
	if err != nil {
		return nil, err
	}

	reflector.MapsToStructs2(response.Result.([]interface{}), &res, reflector.Strconv, "json")
	return res, nil
}

//TriggersCreate Wrapper for item.create: https://www.zabbix.com/documentation/4.4/manual/appendix/api/item/create
func (api *API) TriggersCreate(triggers Triggers) error {
	response, err := api.CallWithError("trigger.create", triggers)
	if err != nil {
		return err
	}

	result := response.Result.(map[string]interface{})
	triggerids := result["triggerids"].([]interface{})
	for i, id := range triggerids {
		triggers[i].TriggerID = id.(string)
	}
	return nil
}

//TriggersUpdate Wrapper for item.update: https://www.zabbix.com/documentation/4.4/manual/appendix/api/item/update
func (api *API) TriggersUpdate(triggers Triggers) error {
	_, err := api.CallWithError("trigger.update", triggers)
	if err != nil {
		return err
	}
	return nil
}

//TriggersDelete Wrapper for item.delete: https://www.zabbix.com/documentation/4.4/manual/appendix/api/item/delete
// Cleans ItemId in all items elements if call succeed.
func (api *API) TriggersDelete(triggers Triggers) error {
	ids := make([]string, len(triggers))
	for i, trigger := range triggers {
		ids[i] = trigger.TriggerID
	}

	err := api.TriggersDeleteByIDs(ids)
	if err != nil {
		return err
	}

	for i := range triggers {
		triggers[i].TriggerID = ""
	}

	return nil
}

//TriggersDeleteByIDs Wrapper for item.delete: https://www.zabbix.com/documentation/2.2/manual/appendix/api/item/delete
func (api *API) TriggersDeleteByIDs(ids []string) error {
	response, err := api.CallWithError("trigger.delete", ids)
	if err != nil {
		return err
	}

	result := response.Result.(map[string]interface{})
	triggerids1, ok := result["triggerids"].([]interface{})
	l := len(triggerids1)
	if !ok {
		// some versions actually return map there
		triggerids2 := result["triggerids"].(map[string]interface{})
		l = len(triggerids2)
	}
	if len(ids) != l {
		err = &ExpectedMore{len(ids), l}
		return err
	}
	return nil
}
