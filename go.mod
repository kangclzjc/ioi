module intel.com/ioi

go 1.19

require (
	github.com/containerd/nri v0.2.0
	github.com/containernetworking/plugins v1.1.1
	github.com/sirupsen/logrus v1.9.0
	github.com/vishvananda/netlink v1.1.1-0.20210330154013-f5de75959ad5
	k8s.io/apimachinery v0.26.0
	k8s.io/klog/v2 v2.80.1
	sigs.k8s.io/yaml v1.3.0
)

require (
	github.com/containerd/ttrpc v1.1.1-0.20220420014843-944ef4a40df3 // indirect
	github.com/containernetworking/cni v1.0.1 // indirect
	github.com/coreos/go-iptables v0.6.0 // indirect
	github.com/go-logr/logr v1.2.3 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/opencontainers/runtime-spec v1.0.3-0.20220825212826-86290f6a00fb // indirect
	github.com/safchain/ethtool v0.0.0-20210803160452-9aa261dae9b1 // indirect
	github.com/vishvananda/netns v0.0.0-20210104183010-2eb08e3e575f // indirect
	golang.org/x/net v0.3.1-0.20221206200815-1e63c2f08a10 // indirect
	golang.org/x/sys v0.3.0 // indirect
	golang.org/x/text v0.5.0 // indirect
	google.golang.org/genproto v0.0.0-20220502173005-c8bf987b8c21 // indirect
	google.golang.org/grpc v1.47.0 // indirect
	google.golang.org/protobuf v1.28.1 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	k8s.io/cri-api v0.25.3 // indirect
)

replace github.com/containerd/nri => /root/go/src/kangclzjc/nri
