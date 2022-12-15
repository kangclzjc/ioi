package agent

import (
	"fmt"
	"github.com/containerd/nri/pkg/api"
	"k8s.io/apimachinery/pkg/api/resource"
	"os"
	"sigs.k8s.io/yaml"
)

func SetNetIOClassConfig(filename string) (*QoSClasses, error) {
	if data, err := os.ReadFile(filename); err == nil {
		fmt.Println("-------%q", data)
		qosConfig := &QoSClasses{}
		if err := yaml.Unmarshal(data, &qosConfig); err != nil {
			return nil, fmt.Errorf("failed to parse config file %q: %v", filename, err)
		}
		fmt.Println(qosConfig.Classes["high-prio"])
		return qosConfig, nil
	} else {
		return nil, fmt.Errorf("failed to read config file %q: %v", filename, err)
	}
}

func GetNetNSPath(pod *api.PodSandbox) string {
	if pod.Linux.Namespaces == nil {
		return ""
	}

	for _, namespace := range pod.Linux.Namespaces {
		if namespace.Type == "network" {
			return namespace.Path
		}
	}

	return ""
}

func GetPrimaryNIC(netNs string) string {
	return "eth0"
}

func ParseRequestBandwidth(bandwidth string) resource.Quantity {
	return resource.MustParse(bandwidth)
}
