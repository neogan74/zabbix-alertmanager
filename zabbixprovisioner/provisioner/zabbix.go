package provisioner

import (
	"strings"

	zabbix "github.com/neogan74/zabbix-alertmanager/zabbixprovisioner/zabbixclient"
	log "github.com/sirupsen/logrus"
)

//State ..
type State int

// StateNew ...
const (
	StateNew State = iota
	StateUpdated
	StateEqual
	StateOld
)

//StateName ..
var StateName = map[State]string{
	StateNew:     "New",
	StateUpdated: "Updated",
	StateEqual:   "Equal",
	StateOld:     "Old",
}

//CustomApplication ...
type CustomApplication struct {
	State State
	zabbix.Application
}

//CustomTrigger ...
type CustomTrigger struct {
	State State
	zabbix.Trigger
}

//CustomHostGroup ...
type CustomHostGroup struct {
	State State
	zabbix.HostGroup
}

//CustomItem ...
type CustomItem struct {
	State State
	zabbix.Item
	Applications map[string]struct{}
}

//CustomTemplate ..
type CustomTemplate struct {
	State State
	zabbix.Template
	HostGroups   map[string]struct{}
	Applications map[string]*CustomApplication
	Items        map[string]*CustomItem
	Triggers     map[string]*CustomTrigger
}

//CustomHost ...
type CustomHost struct {
	State State
	zabbix.Host
	HostGroups   map[string]struct{}
	Applications map[string]*CustomApplication
	Items        map[string]*CustomItem
	Triggers     map[string]*CustomTrigger
}

//CustomZabbix ...
type CustomZabbix struct {
	Hosts      map[string]*CustomHost
	Templates  map[string]*CustomTemplate
	HostGroups map[string]*CustomHostGroup
}

//AddTemplate ...
func (z *CustomZabbix) AddTemplate(tmpl *CustomTemplate) (updatedTemplate *CustomTemplate) {
	updatedTemplate = tmpl
	if existing, ok := z.Templates[tmpl.Name]; ok {
		log.Debugf("===In ADDTEMPALTE ===: %+v ||| %+v", existing, tmpl)
		if existing.Equal(tmpl) {
			if tmpl.State == StateOld {
				existing.TemplateID = tmpl.TemplateID
				existing.State = StateEqual
				updatedTemplate = existing
			}
		} else {
			if tmpl.State == StateOld {
				existing.TemplateID = tmpl.TemplateID
			}
			existing.State = StateUpdated
			updatedTemplate = existing
		}
	}
	z.Templates[tmpl.Name] = updatedTemplate
	return updatedTemplate
}

//AddItem CustomTemplate method
func (tmpl *CustomTemplate) AddItem(item *CustomItem) {

	updatedItem := item

	if existing, ok := tmpl.Items[item.Key]; ok {
		if existing.Equal(item) {
			if item.State == StateOld {
				existing.ItemID = item.ItemID
				existing.State = StateEqual
				updatedItem = existing
			}
		} else {
			if item.State == StateOld {
				existing.ItemID = item.ItemID
			}
			existing.State = StateUpdated
			updatedItem = existing
		}
	}

	tmpl.Items[item.Key] = updatedItem
}

//AddTrigger CustomTemplate method
func (tmpl *CustomTemplate) AddTrigger(trigger *CustomTrigger) {

	updatedTrigger := trigger

	if existing, ok := tmpl.Triggers[trigger.Expression]; ok {
		if existing.Equal(trigger) {
			if trigger.State == StateOld {
				existing.TriggerID = trigger.TriggerID
				existing.State = StateEqual
				updatedTrigger = existing
			}
		} else {
			if trigger.State == StateOld {
				existing.TriggerID = trigger.TriggerID
			}
			existing.State = StateUpdated
			updatedTrigger = existing
		}
	}

	tmpl.Triggers[trigger.Expression] = updatedTrigger
}

//AddApplication ...
func (tmpl *CustomTemplate) AddApplication(application *CustomApplication) {
	if _, ok := tmpl.Applications[application.Name]; ok {
		if application.State == StateOld {
			application.State = StateEqual
		}
	}
	tmpl.Applications[application.Name] = application
}

//Equal ...
func (tmpl *CustomTemplate) Equal(j *CustomTemplate) bool {
	if tmpl.Name != j.Name {
		return false
	}

	if len(tmpl.HostGroups) != len(j.HostGroups) {
		return false
	}

	for hostGroupName := range tmpl.HostGroups {
		if _, ok := j.HostGroups[hostGroupName]; !ok {
			return false
		}
	}

	return true
}

//GetTemplatesByState ...
func (z *CustomZabbix) GetTemplatesByState() (templateByState map[State]zabbix.Templates) {

	templateByState = map[State]zabbix.Templates{
		StateNew:     zabbix.Templates{},
		StateOld:     zabbix.Templates{},
		StateUpdated: zabbix.Templates{},
		StateEqual:   zabbix.Templates{},
	}

	newTemplateAmmount := 0
	for _, tmpl := range z.Templates {
		log.Debugf("ZTMPL: %+v", tmpl)
		for hostGroupName := range tmpl.HostGroups {
			tmpl.GroupIds = append(tmpl.GroupIds, zabbix.HostGroupID{GroupID: z.HostGroups[hostGroupName].GroupID})
		}
		templateByState[tmpl.State] = append(templateByState[tmpl.State], tmpl.Template)
		if StateName[tmpl.State] == "New" || StateName[tmpl.State] == "Updated" {
			newTemplateAmmount++
			log.Infof("GetTemplatesByState = State: %s, Name: %s", StateName[tmpl.State], tmpl.Template)
		} else {
			log.Debugf("GetTemplatesByState = State: %s, Name: %s", StateName[tmpl.State], tmpl.Template)
		}
	}

	log.Infof("TEMPLATES, total: %v, new or updated: %v", len(z.Templates), newTemplateAmmount)
	return templateByState
}

//GetApplicationsByState ...
func (tmpl *CustomTemplate) GetApplicationsByState() (applicationsByState map[State]zabbix.Applications) {

	applicationsByState = map[State]zabbix.Applications{
		StateNew:     zabbix.Applications{},
		StateOld:     zabbix.Applications{},
		StateUpdated: zabbix.Applications{},
		StateEqual:   zabbix.Applications{},
	}
	newAppAmmount := 0
	log.Debugf("Application template obj: %+v", tmpl)
	for _, application := range tmpl.Applications {
		application.Application.HostID = tmpl.TemplateID
		applicationsByState[application.State] = append(applicationsByState[application.State], application.Application)
		if StateName[application.State] == "New" || StateName[application.State] == "Updated" {
			newAppAmmount++
			log.Infof("GetApplicationsByState = State: %s, Name: %s", StateName[application.State], application.Name)
		} else {
			log.Debugf("GetApplicationsByState = State: %s, Name: %s", StateName[application.State], application.Name)
		}
	}

	log.Infof("APPLICATIONS, total: %v, new or updated: %v", len(tmpl.Applications), newAppAmmount)
	return applicationsByState
}

//PropagateCreatedApplications ...
func (tmpl *CustomTemplate) PropagateCreatedApplications(applications zabbix.Applications) {

	for _, application := range applications {
		tmpl.Applications[application.Name].ApplicationID = application.ApplicationID
	}
}

//GetItemsByState ...
func (tmpl *CustomTemplate) GetItemsByState() (itemsByState map[State]zabbix.Items) {

	itemsByState = map[State]zabbix.Items{
		StateNew:     zabbix.Items{},
		StateOld:     zabbix.Items{},
		StateUpdated: zabbix.Items{},
		StateEqual:   zabbix.Items{},
	}

	newItemAmmount := 0
	for _, item := range tmpl.Items {
		item.HostID = tmpl.TemplateID
		item.Item.ApplicationIds = []string{}
		for appName := range item.Applications {
			item.Item.ApplicationIds = append(item.Item.ApplicationIds, tmpl.Applications[appName].ApplicationID)
		}
		itemsByState[item.State] = append(itemsByState[item.State], item.Item)
		if StateName[item.State] == "New" || StateName[item.State] == "Updated" {
			newItemAmmount++
			log.Infof("GetItemsByState = State: %s, Key: %s, Applications: %+v", StateName[item.State], item.Key, item.Applications)
		} else {
			log.Debugf("GetItemsByState = State: %s, Key: %s, Applications: %+v", StateName[item.State], item.Key, item.Applications)
		}
	}

	log.Infof("ITEMS, total: %v, new or updated: %v", len(tmpl.Items), newItemAmmount)
	return itemsByState
}

//GetTriggersByState ...
func (tmpl *CustomTemplate) GetTriggersByState() (triggersByState map[State]zabbix.Triggers) {

	triggersByState = map[State]zabbix.Triggers{
		StateNew:     zabbix.Triggers{},
		StateOld:     zabbix.Triggers{},
		StateUpdated: zabbix.Triggers{},
		StateEqual:   zabbix.Triggers{},
	}

	newTriggerAmmount := 0
	for _, trigger := range tmpl.Triggers {
		triggersByState[trigger.State] = append(triggersByState[trigger.State], trigger.Trigger)
		if StateName[trigger.State] == "New" || StateName[trigger.State] == "Updated" {
			newTriggerAmmount++
			log.Infof("GetTriggersByState = State: %s, Expression: %s", StateName[trigger.State], trigger.Expression)
		} else {
			log.Debugf("GetTriggersByState = State: %s, Expression: %s", StateName[trigger.State], trigger.Expression)
		}
	}

	log.Infof("TRIGGERS, total: %v, new or updated: %v", len(tmpl.Triggers), newTriggerAmmount)
	return triggersByState
}

//AddHost ...
func (z *CustomZabbix) AddHost(host *CustomHost) (updatedHost *CustomHost) {
	updatedHost = host

	if existing, ok := z.Hosts[host.Name]; ok {
		if existing.Equal(host) {
			if host.State == StateOld {
				existing.HostID = host.HostID
				existing.State = StateEqual
				updatedHost = existing
			}
		} else {
			if host.State == StateOld {
				existing.HostID = host.HostID
			}
			existing.State = StateUpdated
			updatedHost = existing
		}
	}

	z.Hosts[host.Name] = updatedHost
	return updatedHost
}

//AddItem CusomHost method
func (host *CustomHost) AddItem(item *CustomItem) {

	updatedItem := item

	if existing, ok := host.Items[item.Key]; ok {
		if existing.Equal(item) {
			if item.State == StateOld {
				existing.ItemID = item.ItemID
				existing.State = StateEqual
				updatedItem = existing
			}
		} else {
			if item.State == StateOld {
				existing.ItemID = item.ItemID
			}
			existing.State = StateUpdated
			updatedItem = existing
		}
	}

	host.Items[item.Key] = updatedItem
}

//AddTrigger ...
func (host *CustomHost) AddTrigger(trigger *CustomTrigger) {

	updatedTrigger := trigger

	if existing, ok := host.Triggers[trigger.Expression]; ok {
		if existing.Equal(trigger) {
			if trigger.State == StateOld {
				existing.TriggerID = trigger.TriggerID
				existing.State = StateEqual
				updatedTrigger = existing
			}
		} else {
			if trigger.State == StateOld {
				existing.TriggerID = trigger.TriggerID
			}
			existing.State = StateUpdated
			updatedTrigger = existing
		}
	}

	host.Triggers[trigger.Expression] = updatedTrigger
}

//AddApplication ...
func (host *CustomHost) AddApplication(application *CustomApplication) {
	if _, ok := host.Applications[application.Name]; ok {
		if application.State == StateOld {
			application.State = StateEqual
		}
	}
	host.Applications[application.Name] = application
}

//AddHostGroup ...
func (z *CustomZabbix) AddHostGroup(hostGroup *CustomHostGroup) {
	if _, ok := z.HostGroups[hostGroup.Name]; ok {
		if hostGroup.State == StateOld {
			hostGroup.State = StateEqual
		}
	}
	z.HostGroups[hostGroup.Name] = hostGroup
}

//Equal ...
func (host *CustomHost) Equal(j *CustomHost) bool {
	if host.Name != j.Name {
		return false
	}

	if len(host.HostGroups) != len(j.HostGroups) {
		return false
	}

	for hostGroupName := range host.HostGroups {
		if _, ok := j.HostGroups[hostGroupName]; !ok {
			return false
		}
	}

	if len(host.Inventory) != len(j.Inventory) {
		return false
	}

	for key, valueI := range host.Inventory {
		if valueJ, ok := j.Inventory[key]; !ok {
			return false
		} else if valueJ != valueI {
			return false
		}
	}

	return true
}

//Equal ...
func (i *CustomItem) Equal(j *CustomItem) bool {
	if i.Name != j.Name {
		return false
	}

	if i.Description != j.Description {
		return false
	}

	if i.Trends != j.Trends {
		return false
	}

	if i.History != j.History {
		return false
	}

	if i.TrapperHosts != j.TrapperHosts {
		return false
	}

	if len(i.Applications) != len(j.Applications) {
		return false
	}

	for appName := range i.Applications {
		if _, ok := j.Applications[appName]; !ok {
			return false
		}
	}

	return true
}

//Equal ...
func (i *CustomTrigger) Equal(j *CustomTrigger) bool {
	if i.Expression != j.Expression {
		return false
	}

	if i.Description != j.Description {
		return false
	}

	if i.Priority != j.Priority {
		return false
	}

	if i.Comments != j.Comments {
		return false
	}

	if i.URL != j.URL {
		return false
	}

	if i.ManualClose != j.ManualClose {
		return false
	}

	return true
}

//GetHostsByState ...
func (z *CustomZabbix) GetHostsByState() (hostByState map[State]zabbix.Hosts) {

	hostByState = map[State]zabbix.Hosts{
		StateNew:     zabbix.Hosts{},
		StateOld:     zabbix.Hosts{},
		StateUpdated: zabbix.Hosts{},
		StateEqual:   zabbix.Hosts{},
	}

	newHostAmmount := 0
	for _, host := range z.Hosts {
		for hostGroupName := range host.HostGroups {
			host.GroupIds = append(host.GroupIds, zabbix.HostGroupID{GroupID: z.HostGroups[hostGroupName].GroupID})
		}
		hostByState[host.State] = append(hostByState[host.State], host.Host)
		if StateName[host.State] == "New" || StateName[host.State] == "Updated" {
			newHostAmmount++
			log.Infof("GetHostByState = State: %s, Name: %s", StateName[host.State], host.Name)
		} else {
			log.Debugf("GetHostByState = State: %s, Name: %s", StateName[host.State], host.Name)
		}
	}

	log.Infof("HOSTS, total: %v, new or updated: %v", len(z.Hosts), newHostAmmount)
	return hostByState
}

//GetHostGroupsByState ...
func (z *CustomZabbix) GetHostGroupsByState() (hostGroupsByState map[State]zabbix.HostGroups) {

	hostGroupsByState = map[State]zabbix.HostGroups{
		StateNew:     zabbix.HostGroups{},
		StateOld:     zabbix.HostGroups{},
		StateUpdated: zabbix.HostGroups{},
		StateEqual:   zabbix.HostGroups{},
	}

	newHostGroupAmmount := 0
	for _, hostGroup := range z.HostGroups {
		hostGroupsByState[hostGroup.State] = append(hostGroupsByState[hostGroup.State], hostGroup.HostGroup)
		if StateName[hostGroup.State] == "New" || StateName[hostGroup.State] == "Updated" {
			newHostGroupAmmount++
			log.Infof("GetHostGroupsByState = State: %s, Name: %s", StateName[hostGroup.State], hostGroup.Name)
		} else {
			log.Debugf("GetHostGroupsByState = State: %s, Name: %s", StateName[hostGroup.State], hostGroup.Name)
		}
	}

	log.Infof("HOSTGROUPS, total: %v, new or updated: %v", len(z.HostGroups), newHostGroupAmmount)

	return hostGroupsByState
}

//PropagateCreatedTemplates ...
func (z *CustomZabbix) PropagateCreatedTemplates(templates zabbix.Templates) {
	for _, newTemplate := range templates {
		if tmpl, ok := z.Templates[newTemplate.Name]; ok {
			tmpl.TemplateID = newTemplate.TemplateID
		}
	}
}

//PropagateCreatedHosts ...
func (z *CustomZabbix) PropagateCreatedHosts(hosts zabbix.Hosts) {
	for _, newHost := range hosts {
		if host, ok := z.Hosts[newHost.Name]; ok {
			host.HostID = newHost.HostID
		}
	}
}

//PropagateCreatedHostGroups ...
func (z *CustomZabbix) PropagateCreatedHostGroups(hostGroups zabbix.HostGroups) {
	for _, newHostGroup := range hostGroups {
		if hostGroup, ok := z.HostGroups[newHostGroup.Name]; ok {
			hostGroup.GroupID = newHostGroup.GroupID
		}
	}
}

//PropagateCreatedApplications ...
func (host *CustomHost) PropagateCreatedApplications(applications zabbix.Applications) {

	for _, application := range applications {
		host.Applications[application.Name].ApplicationID = application.ApplicationID
	}
}

//GetItemsByState ...
func (host *CustomHost) GetItemsByState() (itemsByState map[State]zabbix.Items) {

	itemsByState = map[State]zabbix.Items{
		StateNew:     zabbix.Items{},
		StateOld:     zabbix.Items{},
		StateUpdated: zabbix.Items{},
		StateEqual:   zabbix.Items{},
	}

	newItemAmmount := 0
	for _, item := range host.Items {
		item.HostID = host.HostID
		item.Item.ApplicationIds = []string{}
		for appName := range item.Applications {
			item.Item.ApplicationIds = append(item.Item.ApplicationIds, host.Applications[appName].ApplicationID)
		}
		itemsByState[item.State] = append(itemsByState[item.State], item.Item)
		if StateName[item.State] == "New" || StateName[item.State] == "Updated" {
			newItemAmmount++
			log.Infof("GetItemsByState = State: %s, Key: %s, Applications: %+v", StateName[item.State], item.Key, item.Applications)
		} else {
			log.Debugf("GetItemsByState = State: %s, Key: %s, Applications: %+v", StateName[item.State], item.Key, item.Applications)
		}
	}

	log.Infof("ITEMS, total: %v, new or updated: %v", len(host.Items), newItemAmmount)
	return itemsByState
}

//GetTriggersByState ...
func (host *CustomHost) GetTriggersByState() (triggersByState map[State]zabbix.Triggers) {

	triggersByState = map[State]zabbix.Triggers{
		StateNew:     zabbix.Triggers{},
		StateOld:     zabbix.Triggers{},
		StateUpdated: zabbix.Triggers{},
		StateEqual:   zabbix.Triggers{},
	}

	newTriggerAmmount := 0
	for _, trigger := range host.Triggers {
		triggersByState[trigger.State] = append(triggersByState[trigger.State], trigger.Trigger)
		if StateName[trigger.State] == "New" || StateName[trigger.State] == "Updated" {
			newTriggerAmmount++
			log.Infof("GetTriggersByState = State: %s, Expression: %s", StateName[trigger.State], trigger.Expression)
		} else {
			log.Debugf("GetTriggersByState = State: %s, Expression: %s", StateName[trigger.State], trigger.Expression)
		}
	}

	log.Infof("TRIGGERS, total: %v, new or updated: %v", len(host.Triggers), newTriggerAmmount)
	return triggersByState
}

//GetApplicationsByState ...
func (host *CustomHost) GetApplicationsByState() (applicationsByState map[State]zabbix.Applications) {

	applicationsByState = map[State]zabbix.Applications{
		StateNew:     zabbix.Applications{},
		StateOld:     zabbix.Applications{},
		StateUpdated: zabbix.Applications{},
		StateEqual:   zabbix.Applications{},
	}
	newAppAmmount := 0
	for _, application := range host.Applications {
		application.Application.HostID = host.HostID
		applicationsByState[application.State] = append(applicationsByState[application.State], application.Application)
		if StateName[application.State] == "New" || StateName[application.State] == "Updated" {
			newAppAmmount++
			log.Infof("GetApplicationsByState = State: %s, Name: %s", StateName[application.State], application.Name)
		} else {
			log.Debugf("GetApplicationsByState = State: %s, Name: %s", StateName[application.State], application.Name)
		}
	}

	log.Infof("APPLICATIONS, total: %v, new or updated: %v", len(host.Applications), newAppAmmount)
	return applicationsByState
}

//GetZabbixPriority ...
func GetZabbixPriority(severity string) zabbix.PriorityType {

	switch strings.ToLower(severity) {
	case "information":
		return zabbix.Information
	case "warning":
		return zabbix.Warning
	case "average":
		return zabbix.Average
	case "high":
		return zabbix.High
	case "critical":
		return zabbix.Critical
	default:
		return zabbix.NotClassified
	}
}
