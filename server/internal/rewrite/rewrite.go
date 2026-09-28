package rewrite

import (
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// HostSide maps docker-compose hostnames to addresses that work when this
// process runs on the Mac host instead of inside the compose network.
type HostSide struct {
	Enabled bool
}

func NewHostSide() HostSide {
	if os.Getenv("HOST_SIDE_REWRITES") == "0" {
		return HostSide{Enabled: false}
	}
	if os.Getenv("HOST_SIDE_REWRITES") == "1" {
		return HostSide{Enabled: true}
	}
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return HostSide{Enabled: false}
	}
	return HostSide{Enabled: true}
}

func (h HostSide) ApplyURL(raw string) string {
	if !h.Enabled {
		return raw
	}
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" {
		return raw
	}
	host := parsed.Hostname()
	port := parsed.Port()
	portNum := 0
	if port != "" {
		portNum, _ = strconv.Atoi(port)
	}
	host, portNum = h.ApplyHostPort(host, portNum)
	if portNum > 0 {
		parsed.Host = net.JoinHostPort(host, strconv.Itoa(portNum))
	} else {
		parsed.Host = host
	}
	return parsed.String()
}

func (h HostSide) ApplyHostPort(host string, port int) (string, int) {
	if !h.Enabled {
		return host, port
	}
	switch strings.ToLower(strings.TrimSpace(host)) {
	case "host.docker.internal":
		return "127.0.0.1", port
	case "grok2api":
		if port == 0 || port == 8000 {
			return "127.0.0.1", 37821
		}
		return "127.0.0.1", port
	case "sub2api":
		if port == 0 || port == 8090 {
			return "127.0.0.1", 8090
		}
		return "127.0.0.1", port
	case "minio":
		if port == 0 || port == 9000 {
			return "127.0.0.1", 19100
		}
		return "127.0.0.1", port
	default:
		return host, port
	}
}

func (h HostSide) ApplyEndpoint(endpoint string) string {
	if !h.Enabled {
		return endpoint
	}
	value := strings.TrimSpace(endpoint)
	if value == "" {
		return value
	}
	if strings.Contains(value, "://") {
		return h.ApplyURL(value)
	}
	host, port, err := net.SplitHostPort(value)
	if err != nil {
		host, portNum := h.ApplyHostPort(value, 0)
		if portNum > 0 {
			return net.JoinHostPort(host, strconv.Itoa(portNum))
		}
		return host
	}
	portNum, _ := strconv.Atoi(port)
	host, portNum = h.ApplyHostPort(host, portNum)
	return net.JoinHostPort(host, strconv.Itoa(portNum))
}
