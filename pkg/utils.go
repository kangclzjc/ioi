package utils

import (
	"fmt"
	"intel.com/ioi/pkg/agent"
	"os"
	"sigs.k8s.io/yaml"
)

func SetNetIOClassConfig(filename string) (*agent.QoSClasses, error) {
	if data, err := os.ReadFile(filename); err == nil {
		fmt.Println("-------%q", data)
		qosConfig := &agent.QoSClasses{}
		if err := yaml.Unmarshal(data, &qosConfig); err != nil {
			return nil, fmt.Errorf("failed to parse config file %q: %v", filename, err)
		}
		fmt.Println(qosConfig.Classes["high-prio"])
		return qosConfig, nil
	} else {
		return nil, fmt.Errorf("failed to read config file %q: %v", filename, err)
	}
}
