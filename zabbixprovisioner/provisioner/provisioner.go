package provisioner

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	zabbix "github.com/neogan74/zabbix-alertmanager/zabbixprovisioner/zabbixclient"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	yaml "gopkg.in/yaml.v2"
)

//HostConfig structure
type HostConfig struct {
	Name                    string            `yaml:"name"`
	HostGroups              []string          `yaml:"hostGroups"`
	Tag                     string            `yaml:"tag"`
	DeploymentStatus        string            `yaml:"deploymentStatus"`
	ItemDefaultApplication  string            `yaml:"itemDefaultApplication"`
	ItemDefaultHistory      string            `yaml:"itemDefaultHistory"`
	ItemDefaultTrends       string            `yaml:"itemDefaultTrends"`
	ItemDefaultTrapperHosts string            `yaml:"itemDefaultTrapperHosts"`
	HostAlertsDir           string            `yaml:"alertsDir"`
	TriggerTags             map[string]string `yaml:"triggerTags"`
}

//Targets structure for Prometheus api/v1/targets resposce
type Targets struct {
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

//Provisioner structure for syncronization objects between zabbix and prometheus alerts rules.
type Provisioner struct {
	api           *zabbix.API
	keyPrefix     string
	hosts         []HostConfig
	prometheusURL string
	*CustomZabbix
}

/*New function create provisioning object
gets parameters:
- prometheusUrl - URL for access to prometheus
  keyPrefix - zabbix item key prefix like prom.metric
  ur; - Zabbix API URL
  user,password -  Zabbix API credentails
  hosts - list of hosts which will be created, updated in zabbix
*/
func New(prometheusURL, keyPrefix, url, user, password string, hosts []HostConfig) (*Provisioner, error) {
	transport := http.DefaultTransport
	//Zabbix API init
	api := zabbix.NewAPI(url)
	api.SetClient(&http.Client{
		Transport: transport,
	})
	// try to login
	_, err := api.Login(user, password)
	if err != nil {
		return nil, errors.Wrap(err, "error while login to zabbix api")
	}
	return &Provisioner{
		api:           api,
		keyPrefix:     keyPrefix,
		hosts:         hosts,
		prometheusURL: prometheusURL,
	}, nil
}

//LoadHostConfigFromFile function
func LoadHostConfigFromFile(filename string) ([]HostConfig, error) {
	configFile, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, errors.Wrapf(err, "can't open the config file: %s", filename)
	}

	hosts := []HostConfig{}

	err = yaml.Unmarshal(configFile, &hosts)
	if err != nil {
		return nil, errors.Wrapf(err, "can't read the config file: %s", filename)
	}

	return hosts, nil
}

//Run main function for start provisioning
func (p *Provisioner) Run() error {
	p.CustomZabbix = &CustomZabbix{
		Hosts:      map[string]*CustomHost{},
		Templates:  map[string]*CustomTemplate{},
		HostGroups: map[string]*CustomHostGroup{},
	}

	//

	//All hosts will have the rules which were only written for them
	for _, host := range p.hosts {
		if err := p.LoadRulesFromPrometheus(host); err != nil {
			return errors.Wrapf(err, "error loading prometheus rules, file: %s", host.HostAlertsDir)
		}
		if err := p.LoadTargetsFromPrometheus(host); err != nil {
			return errors.Wrapf(err, "error loading prometheus targets from given URL: %s", p.prometheusURL)
		}
	}

	if err := p.LoadDataFromZabbix(); err != nil {
		return errors.Wrap(err, "error loading zabbix rules")
	}

	if err := p.ApplyChanges(); err != nil {
		return errors.Wrap(err, "error applying changes")
	}

	return nil
}

//LoadTargetsFromPrometheus ...
func (p *Provisioner) LoadTargetsFromPrometheus(hostConfig HostConfig) error {
	targetsPath := "/api/v1/targets"
	promURL := fmt.Sprintf("%s%s", p.prometheusURL, targetsPath)
	resp, err := http.Get(promURL)
	if err != nil {
		log.Infof("Error while get targets: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Error with body: %v\n", err)
	}
	var targets []string
	var targetsjs Targets
	err = json.Unmarshal(data, &targetsjs)
	if err != nil {
		log.Fatalf("Error while JSON unmarshal %s", err)
	}
	for _, v := range targetsjs.Data.ActiveTargets {
		targets = append(targets, v.Labels.Instance[:strings.LastIndex(v.Labels.Instance, ":")])
	}
	log.Infof("targets list: %v", targets)
	for _, trg := range targets {
		newHost := &CustomHost{
			State: StateNew,
			Host: zabbix.Host{
				Host:          trg,
				Available:     1,
				Name:          trg,
				Status:        0,
				InventoryMode: zabbix.InventoryManual,
				Inventory: map[string]string{
					"tag": "Prom2zbx",
				},
				Interfaces: zabbix.HostInterfaces{
					zabbix.HostInterface{
						DNS:   "",
						IP:    "127.0.0.1",
						Main:  1,
						Port:  "10050",
						Type:  1,
						UseIP: 1,
					},
				},
			},
			HostGroups: make(map[string]struct{}, 1),
		}
		hostGroupName := hostConfig.HostGroups[0]
		p.AddHostGroup(&CustomHostGroup{
			State: StateNew,
			HostGroup: zabbix.HostGroup{
				Name: hostGroupName,
			}})
		newHost.HostGroups[hostGroupName] = struct{}{}
		log.Debugf("Host from Prometheus: %+v", newHost)
		p.AddHost(newHost)
	}
	return nil
}

//LoadRulesFromPrometheus function Creates hosts structures and populate them from Prometheus rules
func (p *Provisioner) LoadRulesFromPrometheus(hostConfig HostConfig) error {
	rules, err := LoadPrometheusRulesFromDir(hostConfig.HostAlertsDir)
	if err != nil {
		return errors.Wrap(err, "error loading rules")
	}

	log.Infof("Prometheus Rules for template - %v loaded: %v", hostConfig.Name, len(rules))

	newTemplate := &CustomTemplate{
		State: StateNew,
		Template: zabbix.Template{
			Name:        hostConfig.Name,
			DisplayName: hostConfig.Name,
		},
		HostGroups:   make(map[string]struct{}, 1),
		Items:        map[string]*CustomItem{},
		Applications: map[string]*CustomApplication{},
		Triggers:     map[string]*CustomTrigger{},
	}
	for _, hostGroupName := range hostConfig.HostGroups {
		p.AddHostGroup(&CustomHostGroup{
			State: StateNew,
			HostGroup: zabbix.HostGroup{
				Name: hostGroupName,
			},
		})

		newTemplate.HostGroups[hostGroupName] = struct{}{}
	}

	// Parse Prometheus rules and create corresponding items/triggers and applications for this host
	for _, rule := range rules {
		key := fmt.Sprintf("%s.%s", strings.ToLower(p.keyPrefix), strings.ToLower(rule.Name))

		var triggerTags []zabbix.Tag
		for k, v := range hostConfig.TriggerTags {
			triggerTags = append(triggerTags, zabbix.Tag{Tag: k, Value: v})
		}

		newItem := &CustomItem{
			State: StateNew,
			Item: zabbix.Item{
				Name:         rule.Name,
				Key:          key,
				HostID:       "", //To be filled when the host will be created
				Type:         2,  //Trapper
				ValueType:    3,
				History:      hostConfig.ItemDefaultHistory,
				Trends:       hostConfig.ItemDefaultTrends,
				TrapperHosts: hostConfig.ItemDefaultTrapperHosts,
			},
			Applications: map[string]struct{}{},
		}

		newTrigger := &CustomTrigger{
			State: StateNew,
			Trigger: zabbix.Trigger{
				Description: rule.Name,
				Expression:  fmt.Sprintf("{%s:%s.last()}<>0", newTemplate.Name, key),
				ManualClose: 1,
				Tags:        triggerTags,
			},
		}

		if p.prometheusURL != "" {
			newTrigger.URL = p.prometheusURL + "/alerts"

			url := p.prometheusURL + "/graph?g0.expr=" + url.QueryEscape(rule.Expression)
			if len(url) < 255 {
				newTrigger.URL = url
			}
		}

		if v, ok := rule.Annotations["summary"]; ok {
			newTrigger.Comments = v
		} else if v, ok := rule.Annotations["message"]; ok {
			newTrigger.Comments = v
		} else if v, ok := rule.Annotations["description"]; ok {
			newTrigger.Comments = v
		}

		if v, ok := rule.Labels["severity"]; ok {
			newTrigger.Priority = GetZabbixPriority(v)
		}

		// Add the special "No Data" trigger if requested
		if delay, ok := rule.Annotations["zabbix_trigger_nodata"]; ok {
			newTrigger.Trigger.Description = fmt.Sprintf("%s - no data for the last %s seconds", newTrigger.Trigger.Description, delay)
			newTrigger.Trigger.Expression = fmt.Sprintf("{%s:%s.nodata(%s)}", newTemplate.Name, key, delay)
		}

		// If no applications are found in the rule, add the default application declared in the configuration
		if len(newItem.Applications) == 0 {
			newTemplate.AddApplication(&CustomApplication{
				State: StateNew,
				Application: zabbix.Application{
					Name: hostConfig.ItemDefaultApplication,
				},
			})
			newItem.Applications[hostConfig.ItemDefaultApplication] = struct{}{}
		}

		log.Debugf("Loading item from Prometheus: %+v", newItem)
		newTemplate.AddItem(newItem)

		log.Debugf("Loading trigger from Prometheus: %+v", newTrigger)
		newTemplate.AddTrigger(newTrigger)

	}
	log.Debugf("Host from Prometheus: %+v", newTemplate)
	p.AddTemplate(newTemplate)

	return nil
}

//LoadDataFromZabbix Update created hosts with the current state in Zabbix
func (p *Provisioner) LoadDataFromZabbix() error {
	hostNames := make([]string, len(p.hosts))
	templateNames := []string{}
	hostGroupNames := []string{}
	for i := range p.hosts {
		hostNames[i] = p.hosts[i].Name
		templateNames = append(templateNames, p.hosts[i].Name)
		hostGroupNames = append(hostGroupNames, p.hosts[i].HostGroups...)
	}

	if len(hostNames) == 0 {
		return errors.Errorf("error no hosts are defined")
	}
	// Getting ZABBIX HOSTGROUPS //
	zabbixHostGroups, err := p.api.HostGroupsGet(zabbix.Params{
		"output": "extend",
		"filter": map[string][]string{
			"name": hostGroupNames,
		},
	})
	if err != nil {
		return errors.Wrapf(err, "error getting hostgroups: %v", hostGroupNames)
	}

	for _, zabbixHostGroup := range zabbixHostGroups {
		p.AddHostGroup(&CustomHostGroup{
			State:     StateOld,
			HostGroup: zabbixHostGroup,
		})
	}

	// Getting ZABBIX TEMPLATES //

	if len(templateNames) == 0 {
		return errors.Errorf("error no templates are defined")
	}
	zabbixTemplates, err := p.api.TemplateGet(zabbix.Params{
		"output": "extend",
		"filter": map[string][]string{
			"host": templateNames,
		},
	})

	for _, zabbixTemplate := range zabbixTemplates {
		zabbixHostGroups, err := p.api.HostGroupsGet(zabbix.Params{
			"output":      "extend",
			"templateids": zabbixTemplate.TemplateID,
		})
		if err != nil {
			return errors.Wrapf(err, "error getting hostgroup, hostid: %v", zabbixTemplate.TemplateID)
		}
		hostGroups := make(map[string]struct{}, len(zabbixHostGroups))
		for _, zabbixHostGroup := range zabbixHostGroups {
			hostGroups[zabbixHostGroup.Name] = struct{}{}
		}

		oldTemplate := p.AddTemplate(&CustomTemplate{
			State:        StateOld,
			Template:     zabbixTemplate,
			HostGroups:   hostGroups,
			Items:        map[string]*CustomItem{},
			Applications: map[string]*CustomApplication{},
			Triggers:     map[string]*CustomTrigger{},
		})
		log.Debugf("Load template from Zabbix: %+v", oldTemplate)
		zabbixApplications, err := p.api.ApplicationsGet(zabbix.Params{
			"output":      "extend",
			"templateids": oldTemplate.TemplateID,
		})
		if err != nil {
			return errors.Wrapf(err, "error getting application, hostid: %v", oldTemplate.TemplateID)
		}

		for _, zabbixApplication := range zabbixApplications {
			oldTemplate.AddApplication(&CustomApplication{
				State:       StateOld,
				Application: zabbixApplication,
			})
		}

		zabbixItems, err := p.api.ItemsGet(zabbix.Params{
			"output":      "extend",
			"templateids": oldTemplate.TemplateID,
		})
		if err != nil {
			return errors.Wrapf(err, "error getting item, hostid: %v", oldTemplate.Template.TemplateID)
		}

		for _, zabbixItem := range zabbixItems {
			newItem := &CustomItem{
				State: StateOld,
				Item:  zabbixItem,
			}

			zabbixApplications, err := p.api.ApplicationsGet(zabbix.Params{
				"output":  "extend",
				"itemids": zabbixItem.ItemID,
			})
			if err != nil {
				return errors.Wrapf(err, "error getting item, itemid: %v", oldTemplate.Template.TemplateID)
			}

			newItem.Applications = make(map[string]struct{}, len(zabbixApplications))
			for _, zabbixApplication := range zabbixApplications {
				newItem.Applications[zabbixApplication.Name] = struct{}{}
			}

			log.Debugf("Loading item from Zabbix: %+v", newItem)
			oldTemplate.AddItem(newItem)
		}

		zabbixTriggers, err := p.api.TriggersGet(zabbix.Params{
			"output":           "extend",
			"templateids":      oldTemplate.TemplateID,
			"expandExpression": true,
		})
		if err != nil {
			return errors.Wrapf(err, "error getting zabbix triggers, hostids: %v", oldTemplate.Template.TemplateID)
		}

		for _, zabbixTrigger := range zabbixTriggers {
			newTrigger := &CustomTrigger{
				State:   StateOld,
				Trigger: zabbixTrigger,
			}

			log.Debugf("Loading trigger from Zabbix: %+v", newTrigger)
			oldTemplate.AddTrigger(newTrigger)
		}
	}
	/// Geting ZABBIX HOSTS
	hgids := make([]string, 1)
	for _, hg := range p.HostGroups {
		log.Debugf("HG: %v %s ", hg, hg.GroupID)
		hgids = append(hgids, hg.GroupID)
	}

	zabbixHosts, err := p.api.HostsGet(zabbix.Params{
		"output":   "extend",
		"groupids": hgids[1],
	})
	if err != nil {
		return errors.Wrapf(err, "error getting hosts: %v", hostNames)
	}
	log.Debugf("HOSTS from ZABBIXfor HGids: %+v %+v\n", hgids, zabbixHosts)

	for _, zabbixHost := range zabbixHosts {
		zabbixHostGroups, err := p.api.HostGroupsGet(zabbix.Params{
			"output":  "extend",
			"hostids": zabbixHost.HostID,
		})
		if err != nil {
			return errors.Wrapf(err, "error getting hostgroup, hostid: %v", zabbixHost.HostID)
		}

		hostGroups := make(map[string]struct{}, len(zabbixHostGroups))
		for _, zabbixHostGroup := range zabbixHostGroups {
			hostGroups[zabbixHostGroup.Name] = struct{}{}
		}

		// Remove hostid because the Zabbix api add it automatically and it breaks the comparison between new/old hosts
		delete(zabbixHost.Inventory, "hostid")

		oldHost := p.AddHost(&CustomHost{
			State:        StateOld,
			Host:         zabbixHost,
			HostGroups:   hostGroups,
			Items:        map[string]*CustomItem{},
			Applications: map[string]*CustomApplication{},
			Triggers:     map[string]*CustomTrigger{},
		})
		log.Debugf("Load host from Zabbix: %+v", oldHost)
	}
	return nil
}

//ApplyChanges ...
func (p *Provisioner) ApplyChanges() error {
	hostGroupsByState := p.GetHostGroupsByState()
	if len(hostGroupsByState[StateNew]) != 0 {
		log.Debugf("Creating HostGroups: %+v\n", hostGroupsByState[StateNew])
		err := p.api.HostGroupsCreate(hostGroupsByState[StateNew])
		if err != nil {
			return errors.Wrap(err, "Failed in creating hostgroups")
		}
	}

	// Make sure we update ids for the newly created host groups
	p.PropagateCreatedHostGroups(hostGroupsByState[StateNew])

	templatesByState := p.GetTemplatesByState()
	if len(templatesByState[StateNew]) != 0 {
		log.Debug("Creating Templates: %+v\n", templatesByState[StateNew])
		err := p.api.TemplateCreate(templatesByState[StateNew])
		if err != nil {
			return errors.Wrap(err, "Failed in creating tempalte")
		}
	}
	hostsByState := p.GetHostsByState()
	if len(hostsByState[StateNew]) != 0 {
		log.Debugf("Creating Hosts: %+v\n", hostsByState[StateNew])
		err := p.api.HostsCreate(hostsByState[StateNew])
		if err != nil {
			return errors.Wrap(err, "Failed in creating host")
		}
	}

	// Make sure we update ids for the newly created hosts
	p.PropagateCreatedHosts(hostsByState[StateNew])

	if len(hostsByState[StateUpdated]) != 0 {
		log.Debugf("Updating Hosts: %+v\n", hostsByState[StateUpdated])
		err := p.api.HostsUpdate(hostsByState[StateUpdated])
		if err != nil {
			return errors.Wrap(err, "Failed in updating host")
		}
	}

	for _, host := range p.Hosts {
		log.Debugf("Updating host, hostName: %s", host.Name)

		applicationsByState := host.GetApplicationsByState()
		if len(applicationsByState[StateOld]) != 0 {
			log.Debugf("Deleting applications: %+v\n", applicationsByState[StateOld])
			err := p.api.ApplicationsDelete(applicationsByState[StateOld])
			if err != nil {
				return errors.Wrap(err, "Failed in deleting applications")
			}
		}

		if len(applicationsByState[StateNew]) != 0 {
			log.Debugf("Creating applications: %+v\n", applicationsByState[StateNew])
			err := p.api.ApplicationsCreate(applicationsByState[StateNew])
			if err != nil {
				return errors.Wrap(err, "Failed in creating applications")
			}
		}
		host.PropagateCreatedApplications(applicationsByState[StateNew])

		itemsByState := host.GetItemsByState()
		triggersByState := host.GetTriggersByState()

		if len(triggersByState[StateOld]) != 0 {
			log.Debugf("Deleting triggers: %+v\n", triggersByState[StateOld])
			err := p.api.TriggersDelete(triggersByState[StateOld])
			if err != nil {
				return errors.Wrap(err, "Failed in deleting triggers")
			}
		}

		if len(itemsByState[StateOld]) != 0 {
			log.Debugf("Deleting items: %+v\n", itemsByState[StateOld])
			err := p.api.ItemsDelete(itemsByState[StateOld])
			if err != nil {
				return errors.Wrap(err, "Failed in deleting items")
			}
		}

		if len(itemsByState[StateUpdated]) != 0 {
			log.Debugf("Updating items: %+v\n", itemsByState[StateUpdated])
			err := p.api.ItemsUpdate(itemsByState[StateUpdated])
			if err != nil {
				return errors.Wrap(err, "Failed in updating items")
			}
		}

		if len(triggersByState[StateUpdated]) != 0 {
			log.Debugf("Updating triggers: %+v\n", triggersByState[StateUpdated])
			err := p.api.TriggersUpdate(triggersByState[StateUpdated])
			if err != nil {
				return errors.Wrap(err, "Failed in updating triggers")
			}
		}

		if len(itemsByState[StateNew]) != 0 {
			log.Debugf("Creating items: %+v\n", itemsByState[StateNew])
			err := p.api.ItemsCreate(itemsByState[StateNew])
			if err != nil {
				return errors.Wrap(err, "Failed in creating items")
			}
		}

		if len(triggersByState[StateNew]) != 0 {
			log.Debugf("Creating triggers: %+v\n", triggersByState[StateNew])
			err := p.api.TriggersCreate(triggersByState[StateNew])
			if err != nil {
				return errors.Wrap(err, "Failed in creating triggers")
			}
		}
	}
	return nil
}
