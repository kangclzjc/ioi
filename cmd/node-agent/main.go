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
	"math"
	"os"
	"strings"

	"github.com/containerd/nri/pkg/api"
	"github.com/containerd/nri/pkg/stub"
	"github.com/sirupsen/logrus"
	"intel.com/ioi/pkg/agent"
	"sigs.k8s.io/yaml"
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

const QosClassPrefix = "netio.resources.kubernetes.io"

var (
	cfg  config
	log  *logrus.Logger
	_    = stub.ConfigureInterface(&plugin{})
	ifbs = make(map[string]string)
)

func (p *plugin) Configure(config, runtime, version string) (stub.EventMask, error) {
	log.Infof("got configuration dataq from runtime %s %s", config, runtime, version)
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

func (p *plugin) Synchronize(pods []*api.PodSandbox, containers []*api.Container) ([]*api.ContainerUpdate, error) {
	//dump("Synchronize", "pods", pods, "containers", containers)
	for _, pod := range pods {
		for k, _ := range pod.GetAnnotations() {
			if strings.HasPrefix(k, QosClassPrefix) {
				if _, ok := ifbs[pod.Id]; !ok {
					ifb := agent.GenerateIfbName(pod.Id)
					log.Infof("--------ifb ---------- is %s", ifb)
					ifbs[pod.Id] = ifb
				}
			}
		}
	}
	return nil, nil
}

func (p *plugin) Shutdown() {
	dump("Shutdown")
}

func (p *plugin) parseAnnotation(annotation string) *agent.RequestBandwidth {
	log.Infof("---------%q------\n", p.qos.Classes["high-prio"])
	request := &agent.RequestBandwidth{}
	for _, v := range p.qos.Classes[annotation] {
		log.Infof("----------%q", v.Devices[0].Ingress)
		request.DeviceName = v.Devices[0].Name
		ingress, _ := agent.ParseRequestBandwidth(v.Devices[0].Ingress).AsInt64()
		egress, _ := agent.ParseRequestBandwidth(v.Devices[0].Egress).AsInt64()
		request.IngressBandwidth = ingress
		request.IngressBurst = math.MaxUint32
		request.EgressBandwidth = egress
		request.EgressBurst = math.MaxUint32
	}
	return request
}

func (p *plugin) RunPodSandbox(pod *api.PodSandbox) error {
	log.Infof("---------%q------\n", pod.Linux.Netns)

	//request := agent.RequestBandwidth{}
	for k, v := range pod.GetAnnotations() {
		log.Infof("----%s: -----%s", k, v)
		if strings.HasPrefix(k, QosClassPrefix) {

			request := p.parseAnnotation(v)
			netNsPath := agent.GetNetNSPath(pod)

			log.Infof("------netNsPath---%s------ %q\n", netNsPath, *request)
			ifb := agent.SetLimit(pod.Id, netNsPath, request)
			if ifb != "" {
				ifbs[pod.Id] = ifb
			}
		}
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
	return nil
}

func (p *plugin) StopPodSandbox(pod *api.PodSandbox) error {
	log.Infof("StopPodSandbox---------%q------\n", pod.Linux)
	for k, v := range pod.GetAnnotations() {
		log.Infof("----%s: -----%s", k, v)
	}
	return nil
}

func (p *plugin) RemovePodSandbox(pod *api.PodSandbox) error {
	log.Infof("RemovePodSandbox---------%q------\n", pod)
	for k, v := range pod.GetAnnotations() {
		log.Infof("----%s: -----%s", k, v)
	}

	ifb := agent.GenerateIfbName(pod.Id)
	err := agent.DelIfb(ifb)
	if err != nil {
		log.Errorf("Can't delete ifb %s, %w", ifb, err)
	}
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
	log.Infof("RemoveContainer---------%q------\n", pod.Linux)
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
		qos, err := agent.SetNetIOClassConfig(cfg.QosFile)
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
