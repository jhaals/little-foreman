package inventoryplugins

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"waitron/config"
	"waitron/machine"

	"gopkg.in/yaml.v2"
)

func init() {
	if err := AddMachineInventoryPlugin("file", NewFileInventoryPlugin); err != nil {
		panic(err)
	}
}

type FileInventoryPlugin struct {
	settings      *config.MachineInventoryPluginSettings
	waitronConfig *config.Config
	Log           func(string, int) bool

	machinePath string
}

func NewFileInventoryPlugin(s *config.MachineInventoryPluginSettings, c *config.Config, lf func(string, int) bool) MachineInventoryPlugin {

	p := &FileInventoryPlugin{
		settings:      s, // Plugin settings
		waitronConfig: c, // Global waitron config
		Log:           lf,
	}

	return p

}

func (p *FileInventoryPlugin) Init() error {
	if p.machinePath, _ = p.settings.AdditionalOptions["machinepath"].(string); p.machinePath == "" {
		return fmt.Errorf("machine path not found in config of file plugin")
	}

	p.machinePath = strings.TrimRight(p.machinePath, "/") + "/"

	return nil
}

func (p *FileInventoryPlugin) Deinit() error {
	return nil
}

func (p *FileInventoryPlugin) PutMachine(m *machine.Machine) error {
	return nil
}

func (p *FileInventoryPlugin) GetMachine(hostname string, macaddress string) (*machine.Machine, error) {
	hostname = strings.ToLower(hostname)
	hostSlice := strings.Split(hostname, ".")

	m := &machine.Machine{
		Hostname:  hostname,
		ShortName: hostSlice[0],
		Domain:    strings.Join(hostSlice[1:], "."),
	}

	groupPath, ok := p.settings.AdditionalOptions["grouppath"].(string)

	p.Log(fmt.Sprintf("[%s] looking for %s.[yml|yaml] in %s", p.settings.Name, m.Domain, groupPath), 3)

	// Move the path settings and checks to Init so we can blow up early.
	if ok {
		groupPath = strings.TrimRight(groupPath, "/") + "/"
		// Then, load the domain definition.
		data, err := ioutil.ReadFile(path.Join(groupPath, m.Domain+".yaml")) // apc03.prod.yaml

		if os.IsNotExist(err) {
			data, err = ioutil.ReadFile(path.Join(groupPath, m.Domain+".yml")) // Try .yml
			if err != nil && !os.IsNotExist(err) {                             // We should expect the file to not exist, but if it did exist, err happened for a different reason, then it should be reported.
				return nil, err // Some error beyond just "not found"
			}
		} else {
			return nil, err // Some error beyond just "not found"
		}

		if len(data) > 0 { // Group Files are optional.  So we shouldn't be failing unless they were requested and found.
			if err = yaml.Unmarshal(data, m); err != nil {
				return nil, err
			}
		}

	}

	p.Log(fmt.Sprintf("[%s] looking for %s.[yml|yaml] in %s", p.settings.Name, hostname, p.machinePath), 3)

	// Then load the machine definition.
	data, err := ioutil.ReadFile(path.Join(p.machinePath, hostname+".yaml")) // compute01.apc03.prod.yaml

	p.Log(fmt.Sprintf("[%s] first attempt at slurping %s.[yml|yaml] in %s", p.settings.Name, hostname, p.machinePath), 3)

	if err != nil {
		if os.IsNotExist(err) {

			data, err = ioutil.ReadFile(path.Join(p.machinePath, hostname+".yml")) // One more try but look for .yml
			p.Log(fmt.Sprintf("[%s] second attempt at slurping %s.[yml|yaml] in %s", p.settings.Name, hostname, p.machinePath), 3)

			if err != nil {
				if os.IsNotExist(err) { // Whether the error was due to non-existence or something else, report it.  Machine definitions are must.
					p.Log(fmt.Sprintf("[%s] %s.[yml|yaml] not found in %s", p.settings.Name, hostname, p.machinePath), 3)
					return nil, nil
				} else {
					p.Log(fmt.Sprintf("[%s] %v", p.settings.Name, err), 3)
					return nil, err // Some error beyond just "not found"
				}
			}
		} else {
			p.Log(fmt.Sprintf("[%s] %v", p.settings.Name, err), 3)
			return nil, err // Some error beyond just "not found"
		}
	}

	p.Log(fmt.Sprintf("[%s] %s.[yml|yaml] slurped in from %s", p.settings.Name, hostname, p.machinePath), 3)

	err = yaml.Unmarshal(data, m)
	if err != nil {
		// Don't blow everything up on bad data.  Only truly critical errors should be passed back.
		p.Log(fmt.Sprintf("[%s] unable to unmarshal %s.[yml|yaml]: %v", p.settings.Name, hostname, err), 1)
		return nil, nil
	}

	return m, nil
}