package network

import (
	"context"
	"fmt"
	"multinic-agent/internal/domain/entities"
	"multinic-agent/internal/domain/errors"
	"multinic-agent/internal/domain/interfaces"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

// NetplanAdapter is a NetworkConfigurer and NetworkRollbacker implementation using Ubuntu Netplan
type NetplanAdapter struct {
	commandExecutor interfaces.CommandExecutor
	fileSystem      interfaces.FileSystem
	logger          *logrus.Logger
	configDir       string
}

// NewNetplanAdapter creates a new NetplanAdapter
func NewNetplanAdapter(
	executor interfaces.CommandExecutor,
	fs interfaces.FileSystem,
	logger *logrus.Logger,
) *NetplanAdapter {
	return &NetplanAdapter{
		commandExecutor: executor,
		fileSystem:      fs,
		logger:          logger,
		configDir:       "/etc/netplan",
	}
}

// GetConfigDir returns the directory path where configuration files are stored
func (a *NetplanAdapter) GetConfigDir() string {
	return a.configDir
}

// Configure configures a network interface
func (a *NetplanAdapter) Configure(ctx context.Context, iface entities.NetworkInterface, name entities.InterfaceName) error {
	// Generate configuration file path
	index := extractInterfaceIndex(name.String())
	configPath := filepath.Join(a.configDir, fmt.Sprintf("9%d-%s.yaml", index, name.String()))

	// Backup logic removed - overwrite existing configuration file if it exists

	// Generate Netplan configuration
	config := a.generateNetplanConfig(iface, name.String())
	configData, err := yaml.Marshal(config)
	if err != nil {
		return errors.NewSystemError("failed to marshal Netplan configuration", err)
	}

	// Save configuration file
	if err := a.fileSystem.WriteFile(configPath, configData, 0644); err != nil {
		return errors.NewSystemError("failed to save Netplan configuration file", err)
	}

	a.logger.WithFields(logrus.Fields{
		"interface":   name.String(),
		"config_path": configPath,
	}).Info("Netplan configuration file created")

	// Test Netplan (try command)
	if err := a.testNetplan(ctx); err != nil {
		// Remove configuration file on failure
		if removeErr := a.fileSystem.Remove(configPath); removeErr != nil {
			a.logger.WithError(removeErr).WithField("config_path", configPath).Error("Failed to remove config file after Netplan test failure")
		}
		return errors.NewNetworkError("Netplan configuration test failed", err)
	}

	// Apply Netplan
	if err := a.applyNetplan(ctx); err != nil {
		// Rollback on failure
		if rollbackErr := a.Rollback(ctx, name.String()); rollbackErr != nil {
			a.logger.WithError(rollbackErr).Error("Rollback failed")
		}
		return errors.NewNetworkError("failed to apply Netplan configuration", err)
	}

	return nil
}

// Validate verifies that the configured interface is working properly
func (a *NetplanAdapter) Validate(ctx context.Context, name entities.InterfaceName) error {
	// Check if interface exists
	interfacePath := fmt.Sprintf("/sys/class/net/%s", name.String())
	if !a.fileSystem.Exists(interfacePath) {
		return errors.NewValidationError("network interface does not exist", nil)
	}

	// Check if interface is UP
	_, err := a.commandExecutor.ExecuteWithTimeout(ctx, 10*time.Second, "ip", "link", "show", name.String(), "up")
	if err != nil {
		return errors.NewValidationError("network interface is not UP", err)
	}

	return nil
}

// Rollback reverts the interface configuration to the previous state
func (a *NetplanAdapter) Rollback(ctx context.Context, name string) error {
	index := extractInterfaceIndex(name)
	configPath := filepath.Join(a.configDir, fmt.Sprintf("9%d-%s.yaml", index, name))

	// Remove configuration file
	if a.fileSystem.Exists(configPath) {
		if err := a.fileSystem.Remove(configPath); err != nil {
			return errors.NewSystemError("failed to remove configuration file", err)
		}
	}

	// Backup restore logic removed - simply remove configuration file

	// Reapply Netplan
	if err := a.applyNetplan(ctx); err != nil {
		return errors.NewNetworkError("failed to apply Netplan after rollback", err)
	}

	a.logger.WithField("interface", name).Info("network configuration rollback completed")
	return nil
}

// testNetplan tests the configuration with netplan try command
func (a *NetplanAdapter) testNetplan(ctx context.Context) error {
	// Direct execution in host daemon mode
	_, err := a.commandExecutor.ExecuteWithTimeout(
		ctx,
		120*time.Second,
		"netplan", "try", "--timeout=120",
	)
	return err
}

// applyNetplan applies the configuration with netplan apply command
func (a *NetplanAdapter) applyNetplan(ctx context.Context) error {
	// Direct execution in host daemon mode
	_, err := a.commandExecutor.ExecuteWithTimeout(
		ctx,
		30*time.Second,
		"netplan", "apply",
	)
	return err
}

// generateNetplanConfig generates Netplan configuration
func (a *NetplanAdapter) generateNetplanConfig(iface entities.NetworkInterface, interfaceName string) map[string]interface{} {
	ethernetConfig := map[string]interface{}{
		"match": map[string]interface{}{
			"macaddress": iface.MacAddress,
		},
		"set-name": interfaceName,
	}

	// Static IP configuration: Both Address and CIDR must be present
	if iface.Address != "" && iface.CIDR != "" {
		// Extract prefix from CIDR (e.g., "10.0.0.0/24" -> "24")
		parts := strings.Split(iface.CIDR, "/")
		if len(parts) == 2 {
			prefix := parts[1]
			fullAddress := fmt.Sprintf("%s/%s", iface.Address, prefix)

			ethernetConfig["dhcp4"] = false
			ethernetConfig["addresses"] = []string{fullAddress}
			if iface.MTU > 0 {
				ethernetConfig["mtu"] = iface.MTU
			}
		} else {
			a.logger.WithFields(logrus.Fields{
				"address": iface.Address,
				"cidr":    iface.CIDR,
			}).Warn("Invalid CIDR format, skipping IP configuration")
		}
	}

	config := map[string]interface{}{
		"network": map[string]interface{}{
			"version": 2,
			"ethernets": map[string]interface{}{
				interfaceName: ethernetConfig,
			},
		},
	}

	return config
}

// extractInterfaceIndex extracts the index from interface name
func extractInterfaceIndex(name string) int {
	// multinic0 -> 0, multinic1 -> 1 etc
	if strings.HasPrefix(name, "multinic") {
		indexStr := strings.TrimPrefix(name, "multinic")
		if index, err := strconv.Atoi(indexStr); err == nil {
			return index
		}
	}
	return 0
}
