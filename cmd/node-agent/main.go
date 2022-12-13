/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
	utils "intel.com/ioi/pkg"
	"intel.com/ioi/pkg/agent"
	"os"
	"sigs.k8s.io/yaml"
	"strings"

	"github.com/containerd/nri/pkg/api"
	"github.com/containerd/nri/pkg/stub"
	"github.com/containernetworking/plugins/pkg/ip"
	"github.com/containernetworking/plugins/pkg/ns"
)

type config struct {
	LogFile       string   `json:"logFile"`
	Events        []string `json:"events"`
	AddAnnotation string   `json:"addAnnotation"`
	SetAnnotation string   `json:"setAnnotation"`
	AddEnv        string   `json:"addEnv"`
	SetEnv        string   `json:"setEnv"`
	QosFile       string   `json:"qos"`
}

type plugin struct {
	stub stub.Stub
	mask stub.EventMask
	qos  *agent.QoSClasses
}

const latencyInMillis = 25

var (
	cfg config
	log *logrus.Logger
	_   = stub.ConfigureInterface(&plugin{})
)

func (p *plugin) Configure(config, runtime, version string) (stub.EventMask, error) {
	log.Infof("got configuration data: %q from runtime %s %s", config, runtime, version)
	if config == "" {
		return p.mask, nil
	}

	oldCfg := cfg
	err := yaml.Unmarshal([]byte(config), &cfg)
	if err != nil {
		return 0, fmt.Errorf("failed to parse provided configuration: %w", err)
	}

	p.mask, err = api.ParseEventMask(cfg.Events...)
	if err != nil {
		return 0, fmt.Errorf("failed to parse events in configuration: %w", err)
	}

	if cfg.LogFile != oldCfg.LogFile {
		f, err := os.OpenFile(cfg.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Errorf("failed to open log file %q: %v", cfg.LogFile, err)
			return 0, fmt.Errorf("failed to open log file %q: %w", cfg.LogFile, err)
		}
		log.SetOutput(f)
	}

	return p.mask, nil
}

func time2Tick(time uint32) uint32 {
	return uint32(float64(time) * float64(netlink.TickInUsec()))
}

func buffer(rate uint64, burst uint32) uint32 {
	return time2Tick(uint32(float64(burst) * float64(netlink.TIME_UNITS_PER_SEC) / float64(rate)))
}

func limit(rate uint64, latency float64, buffer uint32) uint32 {
	return uint32(float64(rate)*latency/float64(netlink.TIME_UNITS_PER_SEC)) + buffer
}

func latencyInUsec(latencyInMillis float64) float64 {
	return float64(netlink.TIME_UNITS_PER_SEC) * (latencyInMillis / 1000.0)
}

func createTBF(rateInBits, burstInBits uint64, linkIndex int) error {
	// Equivalent to
	// tc qdisc add dev link root tbf
	//		rate netConf.BandwidthLimits.Rate
	//		burst netConf.BandwidthLimits.Burst
	if rateInBits <= 0 {
		return fmt.Errorf("invalid rate: %d", rateInBits)
	}
	if burstInBits <= 0 {
		return fmt.Errorf("invalid burst: %d", burstInBits)
	}
	rateInBytes := rateInBits / 8
	burstInBytes := burstInBits / 8
	bufferInBytes := buffer(uint64(rateInBytes), uint32(burstInBytes))
	latency := latencyInUsec(latencyInMillis)
	limitInBytes := limit(uint64(rateInBytes), latency, uint32(burstInBytes))

	qdisc := &netlink.Tbf{
		QdiscAttrs: netlink.QdiscAttrs{
			LinkIndex: linkIndex,
			Handle:    netlink.MakeHandle(1, 0),
			Parent:    netlink.HANDLE_ROOT,
		},
		Limit:  uint32(limitInBytes),
		Rate:   uint64(rateInBytes),
		Buffer: uint32(bufferInBytes),
	}
	err := netlink.QdiscAdd(qdisc)
	if err != nil {
		return fmt.Errorf("create qdisc: %s", err)
	}
	return nil
}

func setLimit(netNamespace string) {
	netns, err := ns.GetNS(netNamespace)
	if err != nil {
		log.Errorf("Couldn't open this network namespacce %s", netNamespace)
	}
	defer netns.Close()

	var i int
	_ = netns.Do(func(_ ns.NetNS) error {
		_, i, err = ip.GetVethPeerIfindex("eth0")
		return nil
	})

	if i <= 0 {
		log.Errorf("Couldn't find veth peer")
	}

	// find host interface by index
	link, err := netlink.LinkByIndex(i)
	if err != nil {
		log.Errorf("Couldn't find host NIC of index %d", i)
	}

	log.Infof("------------link-----------%d %s %q", i, link.Attrs().Name, link)
	hostDevice, err := netlink.LinkByName(link.Attrs().Name)
	if err != nil {
		fmt.Errorf("get host device: %s", err)
	}
	err = createTBF(1000000000, 100000000, hostDevice.Attrs().Index)
	if err != nil {
		log.Errorf("-------------err-----------", err)
	}
}

func (p *plugin) Synchronize(pods []*api.PodSandbox, containers []*api.Container) ([]*api.ContainerUpdate, error) {
	//dump("Synchronize", "pods", pods, "containers", containers)
	return nil, nil
}

func (p *plugin) Shutdown() {
	dump("Shutdown")
}

func (p *plugin) RunPodSandbox(pod *api.PodSandbox) error {
	log.Infof("---------%q------\n", pod.Linux.Netns)
	setLimit(pod.Linux.Netns)
	for k, v := range pod.GetAnnotations() {
		log.Infof("----%s: -----%s", k, v)

		//if k == "kubectl.kubernetes.io/last-applied-configuration" {
		//	var vv interface{}
		//	json.Unmarshal([]byte(v), &vv)
		//	data := vv.(map[string]interface{})
		//	for key, value := range data {
		//		if key == "metadata" {
		//			switch v := value.(type) {
		//			case string:
		//				fmt.Println(key, v, "(string)")
		//			case float64:
		//				fmt.Println(key, v, "(float64)")
		//			case []interface{}:
		//				fmt.Println(key, "(array):")
		//				for i, u := range v {
		//					fmt.Println("--------------    ", i, u)
		//				}
		//			default:
		//				fmt.Println(k, v, "(unknown)")
		//			}
		//		}
		//	}
		//}
	}
	if value, ok := pod.Annotations["netio.resources.kubernetes.io/class"]; ok {
		fmt.Printf(value)
	} else {
		fmt.Println("key1 不存在")
	}
	return nil
}

func (p *plugin) StopPodSandbox(pod *api.PodSandbox) error {
	return nil
}

func (p *plugin) RemovePodSandbox(pod *api.PodSandbox) error {
	return nil
}

func (p *plugin) CreateContainer(pod *api.PodSandbox, container *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
	adjust := &api.ContainerAdjustment{}
	if cfg.AddAnnotation != "" {
		adjust.AddAnnotation(cfg.AddAnnotation, fmt.Sprintf("logger-pid-%d", os.Getpid()))
	}
	if cfg.SetAnnotation != "" {
		adjust.RemoveAnnotation(cfg.SetAnnotation)
		adjust.AddAnnotation(cfg.SetAnnotation, fmt.Sprintf("logger-pid-%d", os.Getpid()))
	}
	if cfg.AddEnv != "" {
		adjust.AddEnv(cfg.AddEnv, fmt.Sprintf("logger-pid-%d", os.Getpid()))
	}
	if cfg.SetEnv != "" {
		adjust.RemoveEnv(cfg.SetEnv)
		adjust.AddEnv(cfg.SetEnv, fmt.Sprintf("logger-pid-%d", os.Getpid()))
	}

	return adjust, nil, nil
}

func (p *plugin) PostCreateContainer(pod *api.PodSandbox, container *api.Container) error {
	return nil
}

func (p *plugin) StartContainer(pod *api.PodSandbox, container *api.Container) error {
	return nil
}

func (p *plugin) PostStartContainer(pod *api.PodSandbox, container *api.Container) error {
	return nil
}

func (p *plugin) UpdateContainer(pod *api.PodSandbox, container *api.Container) ([]*api.ContainerUpdate, error) {
	return nil, nil
}

func (p *plugin) PostUpdateContainer(pod *api.PodSandbox, container *api.Container) error {
	return nil
}

func (p *plugin) StopContainer(pod *api.PodSandbox, container *api.Container) ([]*api.ContainerUpdate, error) {
	return nil, nil
}

func (p *plugin) RemoveContainer(pod *api.PodSandbox, container *api.Container) error {
	return nil
}

func (p *plugin) onClose() {
	os.Exit(0)
}

// Dump one or more objects, with an optional global prefix and per-object tags.
func dump(args ...interface{}) {
	var (
		prefix string
		idx    int
	)

	if len(args)&0x1 == 1 {
		prefix = args[0].(string)
		idx++
	}

	for ; idx < len(args)-1; idx += 2 {
		tag, obj := args[idx], args[idx+1]
		msg, err := yaml.Marshal(obj)
		if err != nil {
			log.Infof("%s: %s: failed to dump object: %v", prefix, tag, err)
			continue
		}

		if prefix != "" {
			log.Infof("%s: %s:", prefix, tag)
			for _, line := range strings.Split(strings.TrimSpace(string(msg)), "\n") {
				log.Infof("%s:    %s", prefix, line)
			}
		} else {
			log.Infof("%s:", tag)
			for _, line := range strings.Split(strings.TrimSpace(string(msg)), "\n") {
				log.Infof("  %s", line)
			}
		}
	}
}

func main() {
	var (
		pluginName string
		pluginIdx  string
		events     string
		opts       []stub.Option
		err        error
	)

	log = logrus.StandardLogger()
	log.SetFormatter(&logrus.TextFormatter{
		PadLevelText: true,
	})

	flag.StringVar(&pluginName, "name", "", "plugin name to register to NRI")
	flag.StringVar(&pluginIdx, "idx", "", "plugin index to register to NRI")
	flag.StringVar(&events, "events", "all", "comma-separated list of events to subscribe for")
	flag.StringVar(&cfg.LogFile, "log-file", "", "logfile name, if logging to a file")
	flag.StringVar(&cfg.AddAnnotation, "add-annotation", "", "add this annotation to containers")
	flag.StringVar(&cfg.SetAnnotation, "set-annotation", "", "set this annotation on containers")
	flag.StringVar(&cfg.AddEnv, "add-env", "", "add this environment variable for containers")
	flag.StringVar(&cfg.SetEnv, "set-env", "", "set this environment variable for containers")
	flag.StringVar(&cfg.QosFile, "qos-file", "", "qosfile name")
	flag.Parse()

	if cfg.LogFile != "" {
		f, err := os.OpenFile(cfg.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatalf("failed to open log file %q: %v", cfg.LogFile, err)
		}
		log.SetOutput(f)
	}

	if pluginName != "" {
		opts = append(opts, stub.WithPluginName(pluginName))
	}
	if pluginIdx != "" {
		opts = append(opts, stub.WithPluginIdx(pluginIdx))
	}

	p := &plugin{}
	if cfg.QosFile != "" {
		qos, err := utils.SetNetIOClassConfig(cfg.QosFile)
		if err != nil {
			log.Fatalf("failed to open qos file %q : %v", cfg.QosFile, err)
		}
		p.qos = qos
		log.Infof("---------%q", qos)
	}
	if p.mask, err = api.ParseEventMask(events); err != nil {
		log.Fatalf("failed to parse events: %v", err)
	}
	cfg.Events = strings.Split(events, ",")

	if p.stub, err = stub.New(p, append(opts, stub.WithOnClose(p.onClose))...); err != nil {
		log.Fatalf("failed to create plugin stub: %v", err)
	}

	err = p.stub.Run(context.Background())
	if err != nil {
		log.Errorf("plugin exited with error %v", err)
		os.Exit(1)
	}
}
