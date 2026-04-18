package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type NodeConfig struct {
	NodeID      uint32            `yaml:"node_id"`
	ClusterID   string            `yaml:"cluster_id"`
	Threshold   uint32            `yaml:"threshold"`
	TotalNodes  uint32            `yaml:"total_nodes"`
	ListenAddr  string            `yaml:"listen_addr"`
	EnclaveAddr string            `yaml:"enclave_addr"`
	PeerAddrs   map[uint32]string `yaml:"peer_addrs"`
	TLSEnabled  bool              `yaml:"tls_enabled"`
	MetricsAddr string            `yaml:"metrics_addr"`
	ShareFile   string            `yaml:"share_file"`
}

type ClusterConfig struct {
	ClusterID  string            `yaml:"cluster_id"`
	Threshold  uint32            `yaml:"threshold"`
	TotalNodes uint32            `yaml:"total_nodes"`
	Nodes      []NodeInfo        `yaml:"nodes"`
}

type NodeInfo struct {
	NodeID     uint32            `yaml:"node_id"`
	Endpoint   string            `yaml:"endpoint"`
	Attributes map[string]string `yaml:"attributes"`
}

func LoadNodeConfig(path string) (*NodeConfig, error) {
	if path == "" {
		return defaultNodeConfig(), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config NodeConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &config, nil
}

func (c *NodeConfig) Validate() error {
	if c.NodeID == 0 {
		return fmt.Errorf("node_id must be non-zero")
	}
	if c.Threshold == 0 {
		return fmt.Errorf("threshold must be non-zero")
	}
	if c.TotalNodes == 0 {
		return fmt.Errorf("total_nodes must be non-zero")
	}
	if c.Threshold > c.TotalNodes {
		return fmt.Errorf("threshold (%d) cannot exceed total_nodes (%d)", c.Threshold, c.TotalNodes)
	}
	if c.ListenAddr == "" {
		return fmt.Errorf("listen_addr is required")
	}
	if c.EnclaveAddr == "" {
		return fmt.Errorf("enclave_addr is required")
	}
	return nil
}

func (c *NodeConfig) GetPeerAddr(nodeID uint32) (string, bool) {
	addr, ok := c.PeerAddrs[nodeID]
	return addr, ok
}

func (c *NodeConfig) GetAllNodeIDs() []uint32 {
	ids := make([]uint32, 0, len(c.PeerAddrs)+1)
	ids = append(ids, c.NodeID)
	for id := range c.PeerAddrs {
		ids = append(ids, id)
	}
	return ids
}

func defaultNodeConfig() *NodeConfig {
	return &NodeConfig{
		NodeID:      1,
		ClusterID:   "",
		Threshold:   2,
		TotalNodes:  3,
		ListenAddr:  "localhost:8001",
		EnclaveAddr: "localhost:7002",
		PeerAddrs:   make(map[uint32]string),
		TLSEnabled:  false,
	}
}