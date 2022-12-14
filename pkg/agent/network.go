package agent

import (
	"crypto/sha512"
	"fmt"
	"net"
	"syscall"

	"github.com/containernetworking/plugins/pkg/ip"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/vishvananda/netlink"
	"k8s.io/klog/v2"
)

const latencyInMillis = 25
const maxIfbDeviceLength = 15
const ifbDevicePrefix = "bwp"
const MaxHashLen = sha512.Size * 2

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

func getMTU(deviceName string) (int, error) {
	link, err := netlink.LinkByName(deviceName)
	if err != nil {
		return -1, err
	}

	return link.Attrs().MTU, nil
}

func MustFormatHashWithPrefix(length int, prefix string, toHash string) string {
	if len(prefix) >= length || length > MaxHashLen {
		panic("invalid length")
	}

	output := sha512.Sum512([]byte(toHash))
	return fmt.Sprintf("%s%x", prefix, output)[:length]
}

func getIfbDeviceName(networkName string, containerId string) string {
	return MustFormatHashWithPrefix(maxIfbDeviceLength, ifbDevicePrefix, networkName+containerId)
}

func CreateIfb(ifbDeviceName string, mtu int) error {
	err := netlink.LinkAdd(&netlink.Ifb{
		LinkAttrs: netlink.LinkAttrs{
			Name:  ifbDeviceName,
			Flags: net.FlagUp,
			MTU:   mtu,
		},
	})

	if err != nil {
		return fmt.Errorf("adding link: %s", err)
	}

	return nil
}

func CreateEgressQdisc(rateInBits, burstInBits uint64, hostDeviceName string, ifbDeviceName string) error {
	ifbDevice, err := netlink.LinkByName(ifbDeviceName)
	if err != nil {
		return fmt.Errorf("get ifb device: %s", err)
	}
	hostDevice, err := netlink.LinkByName(hostDeviceName)
	if err != nil {
		return fmt.Errorf("get host device: %s", err)
	}

	// add qdisc ingress on host device
	ingress := &netlink.Ingress{
		QdiscAttrs: netlink.QdiscAttrs{
			LinkIndex: hostDevice.Attrs().Index,
			Handle:    netlink.MakeHandle(0xffff, 0), // ffff:
			Parent:    netlink.HANDLE_INGRESS,
		},
	}

	err = netlink.QdiscAdd(ingress)
	if err != nil {
		return fmt.Errorf("create ingress qdisc: %s", err)
	}

	// add filter on host device to mirror traffic to ifb device
	filter := &netlink.U32{
		FilterAttrs: netlink.FilterAttrs{
			LinkIndex: hostDevice.Attrs().Index,
			Parent:    ingress.QdiscAttrs.Handle,
			Priority:  1,
			Protocol:  syscall.ETH_P_ALL,
		},
		ClassId:    netlink.MakeHandle(1, 1),
		RedirIndex: ifbDevice.Attrs().Index,
		Actions: []netlink.Action{
			&netlink.MirredAction{
				ActionAttrs:  netlink.ActionAttrs{},
				MirredAction: netlink.TCA_EGRESS_REDIR,
				Ifindex:      ifbDevice.Attrs().Index,
			},
		},
	}
	err = netlink.FilterAdd(filter)
	if err != nil {
		return fmt.Errorf("add filter: %s", err)
	}

	// throttle traffic on ifb device
	err = createTBF(rateInBits, burstInBits, ifbDevice.Attrs().Index)
	if err != nil {
		return fmt.Errorf("create ifb qdisc: %s", err)
	}
	return nil
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

func SetLimit(podId, netNamespace string) {
	klog.Info("start set limit")

	nic := GetPrimaryNIC(netNamespace)
	netns, err := ns.GetNS(netNamespace)
	if err != nil {
		klog.Errorf("Couldn't open this network namespacce %s", netNamespace)
	}
	defer netns.Close()
	var i int
	_ = netns.Do(func(_ ns.NetNS) error {
		_, i, err = ip.GetVethPeerIfindex(nic)
		return nil
	})

	if i <= 0 {
		klog.Errorf("Couldn't find veth peer")
	}

	// find host interface by index
	link, err := netlink.LinkByIndex(i)
	if err != nil {
		klog.Errorf("Couldn't find host NIC of index %d", i)
	}

	klog.Infof("------------link-----------%d %s %q", i, link.Attrs().Name, link)
	hostDevice, err := netlink.LinkByName(link.Attrs().Name)
	if err != nil {
		klog.Errorf("get host device: %s", err)
	}
	err = createTBF(1000000000, 100000000, hostDevice.Attrs().Index)
	if err != nil {
		klog.Errorf("-------------err-----------", err)
	}

	mtu, err := getMTU(link.Attrs().Name)
	if err != nil {
		klog.Errorf("-------------err-----------", err)
	}

	ifbDeviceName := getIfbDeviceName(podId, podId)
	klog.Infof("================ifbname %s", ifbDeviceName)
	err = CreateIfb(ifbDeviceName, mtu)
	if err != nil {
		klog.Errorf("-------------err-----------", err)
	}

	err = CreateEgressQdisc(1000000000, 1000000000, link.Attrs().Name, ifbDeviceName)
	if err != nil {
		klog.Errorf("-------------err-----------", err)
	}
}