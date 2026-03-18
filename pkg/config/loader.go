package config

type NodeConfig struct {
	NodeID      uint32
	ClusterID   string
	Threshold   uint32
	TotalNodes  uint32
	ListenAddr  string
	EnclaveAddr string
	PeerAddrs   map[uint32]string
	TLSEnabled  bool
	MetricsAddr string
}

type ClusterConfig struct {
	ClusterID  string
	Threshold  uint32
	TotalNodes uint32
	Nodes      []NodeInfo
}

type NodeInfo struct {
	NodeID     uint32
	Endpoint   string
	Attributes map[string]string
}

func LoadNodeConfig(path string) (*NodeConfig, error) {
	return &NodeConfig{
		NodeID:      1,
		ClusterID:   "default",
		Threshold:   3,
		TotalNodes:  5,
		ListenAddr:  ":7001",
		EnclaveAddr: "localhost:7002",
		PeerAddrs:   make(map[uint32]string),
		TLSEnabled:  false,
	}, nil
}
