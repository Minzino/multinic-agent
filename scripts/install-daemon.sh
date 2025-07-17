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

detect_os() {
    if [[ -f /etc/os-release ]]; then
        . /etc/os-release
        OS=$ID
        OS_VERSION=$VERSION_ID
    else
        print_error "Cannot detect OS. /etc/os-release not found."
        exit 1
    fi
}

check_dependencies() {
    local deps=("systemctl")
    
    for dep in "${deps[@]}"; do
        if ! command -v $dep &> /dev/null; then
            print_error "$dep is required but not installed."
            exit 1
        fi
    done
}

build_binary() {
    print_info "Building multinic-agent binary..."
    
    if ! command -v go &> /dev/null; then
        print_error "Go is not installed. Please install Go 1.21 or later."
        exit 1
    fi
    
    # Build the binary
    go build -o ${BINARY_NAME} ./cmd/agent
    
    if [[ ! -f ${BINARY_NAME} ]]; then
        print_error "Failed to build binary"
        exit 1
    fi
    
    print_info "Binary built successfully"
}

install_binary() {
    print_info "Installing binary to ${INSTALL_DIR}..."
    
    # Create install directory if it doesn't exist
    mkdir -p ${INSTALL_DIR}
    
    # Copy binary
    cp ${BINARY_NAME} ${INSTALL_DIR}/
    chmod +x ${INSTALL_DIR}/${BINARY_NAME}
    
    print_info "Binary installed successfully"
}

setup_config() {
    print_info "Setting up configuration..."
    
    # Create config directory
    mkdir -p ${CONFIG_DIR}
    
    # Copy config template if config doesn't exist
    if [[ ! -f ${CONFIG_DIR}/config.env ]]; then
        cp deployments/systemd/config.env.template ${CONFIG_DIR}/config.env
        chmod 600 ${CONFIG_DIR}/config.env
        print_warning "Please edit ${CONFIG_DIR}/config.env with your database credentials"
    else
        print_info "Config file already exists, skipping..."
    fi
}

install_systemd_service() {
    print_info "Installing systemd service..."
    
    # Copy service file
    cp deployments/systemd/${SERVICE_NAME} ${SYSTEMD_DIR}/
    
    # Reload systemd
    systemctl daemon-reload
    
    print_info "Systemd service installed successfully"
}

enable_service() {
    print_info "Enabling multinic-agent service..."
    
    systemctl enable ${SERVICE_NAME}
    
    print_info "Service enabled successfully"
}

start_service() {
    print_info "Starting multinic-agent service..."
    
    systemctl start ${SERVICE_NAME}
    
    # Check if service started successfully
    if systemctl is-active --quiet ${SERVICE_NAME}; then
        print_info "Service started successfully"
    else
        print_error "Failed to start service. Check logs with: journalctl -u ${SERVICE_NAME}"
        exit 1
    fi
}

# Main installation flow
main() {
    print_info "Starting MultiNIC Agent daemon installation..."
    
    check_root
    detect_os
    check_dependencies
    
    print_info "Detected OS: ${OS} ${OS_VERSION}"
    
    # Build and install
    build_binary
    install_binary
    setup_config
    install_systemd_service
    enable_service
    
    print_info "Installation completed successfully!"
    print_info ""
    print_info "Next steps:"
    print_info "1. Edit configuration: sudo nano ${CONFIG_DIR}/config.env"
    print_info "2. Start the service: sudo systemctl start ${SERVICE_NAME}"
    print_info "3. Check status: sudo systemctl status ${SERVICE_NAME}"
    print_info "4. View logs: sudo journalctl -u ${SERVICE_NAME} -f"
}

# Run main function
main "$@"