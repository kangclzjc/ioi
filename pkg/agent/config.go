package agent

type QoSClasses struct {
	Classes map[string][]QoSClass `json:",omitempty"`
}

type Device struct {
	Name    string `json:",omitempty"`
	Ingress string `json:",omitempty"`
	Egress  string `json:",omitempty"`
}

type QoSClass struct {
	Devices []Device `json:",omitempty"`
}
