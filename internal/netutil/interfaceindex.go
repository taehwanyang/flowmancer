package netutil

import (
	"errors"
	"net"
	"strings"
)

func DetectBridgeInterface() (*net.Interface, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, ifi := range ifaces {
		if ifi.Name == "cni0" {
			return &ifi, nil
		}
	}

	cniPrefixes := []string{
		"cali",    // Calico
		"flannel", // Flannel
		"cilium",  // Cilium
		"weave",   // Weave Net
		"br-",     // generic bridge
	}

	for _, ifi := range ifaces {
		for _, p := range cniPrefixes {
			if strings.HasPrefix(ifi.Name, p) {
				return &ifi, nil
			}
		}
	}

	return nil, errors.New("no suitable interface found")
}
