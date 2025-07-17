#!/bin/bash
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
BINARY_NAME="multinic-agent"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/multinic-agent"
SYSTEMD_DIR="/etc/systemd/system"
SERVICE_NAME="multinic-agent.service"

# Functions
print_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

check_root() {
    if [[ $EUID -ne 0 ]]; then
        print_error "This script must be run as root"
        exit 1
    fi
}

stop_service() {
    print_info "Stopping multinic-agent service..."
    
    if systemctl is-active --quiet ${SERVICE_NAME}; then
        systemctl stop ${SERVICE_NAME}
        print_info "Service stopped"
    else
        print_info "Service is not running"
    fi
}

disable_service() {
    print_info "Disabling multinic-agent service..."
    
    if systemctl is-enabled --quiet ${SERVICE_NAME} 2>/dev/null; then
        systemctl disable ${SERVICE_NAME}
        print_info "Service disabled"
    else
        print_info "Service is not enabled"
    fi
}

remove_systemd_service() {
    print_info "Removing systemd service..."
    
    if [[ -f ${SYSTEMD_DIR}/${SERVICE_NAME} ]]; then
        rm -f ${SYSTEMD_DIR}/${SERVICE_NAME}
        systemctl daemon-reload
        print_info "Systemd service removed"
    else
        print_info "Systemd service file not found"
    fi
}

remove_binary() {
    print_info "Removing binary..."
    
    if [[ -f ${INSTALL_DIR}/${BINARY_NAME} ]]; then
        rm -f ${INSTALL_DIR}/${BINARY_NAME}
        print_info "Binary removed"
    else
        print_info "Binary not found"
    fi
}

remove_config() {
    print_info "Removing configuration..."
    
    if [[ -d ${CONFIG_DIR} ]]; then
        print_warning "Configuration directory ${CONFIG_DIR} found"
        read -p "Do you want to remove configuration files? (y/N) " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            rm -rf ${CONFIG_DIR}
            print_info "Configuration removed"
        else
            print_info "Configuration kept"
        fi
    else
        print_info "Configuration directory not found"
    fi
}

cleanup_netplan_files() {
    print_info "Checking for orphaned netplan files..."
    
    local netplan_dir="/etc/netplan"
    local count=0
    
    if [[ -d ${netplan_dir} ]]; then
        for file in ${netplan_dir}/9*-multinic*.yaml; do
            if [[ -f "$file" ]]; then
                ((count++))
            fi
        done
        
        if [[ $count -gt 0 ]]; then
            print_warning "Found $count multinic netplan files"
            read -p "Do you want to remove these files? (y/N) " -n 1 -r
            echo
            if [[ $REPLY =~ ^[Yy]$ ]]; then
                rm -f ${netplan_dir}/9*-multinic*.yaml
                netplan apply
                print_info "Netplan files removed and configuration applied"
            else
                print_info "Netplan files kept"
            fi
        fi
    fi
}

cleanup_ifcfg_files() {
    print_info "Checking for orphaned ifcfg files..."
    
    local ifcfg_dir="/etc/sysconfig/network-scripts"
    local count=0
    
    if [[ -d ${ifcfg_dir} ]]; then
        for file in ${ifcfg_dir}/ifcfg-multinic*; do
            if [[ -f "$file" ]]; then
                ((count++))
            fi
        done
        
        if [[ $count -gt 0 ]]; then
            print_warning "Found $count multinic ifcfg files"
            read -p "Do you want to remove these files? (y/N) " -n 1 -r
            echo
            if [[ $REPLY =~ ^[Yy]$ ]]; then
                rm -f ${ifcfg_dir}/ifcfg-multinic*
                systemctl restart NetworkManager
                print_info "ifcfg files removed and NetworkManager restarted"
            else
                print_info "ifcfg files kept"
            fi
        fi
    fi
}

# Main uninstallation flow
main() {
    print_info "Starting MultiNIC Agent daemon uninstallation..."
    
    check_root
    
    # Stop and disable service
    stop_service
    disable_service
    
    # Remove components
    remove_systemd_service
    remove_binary
    remove_config
    
    # Cleanup network files
    cleanup_netplan_files
    cleanup_ifcfg_files
    
    print_info "Uninstallation completed successfully!"
}

# Run main function
main "$@"