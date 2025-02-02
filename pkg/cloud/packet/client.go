/*
Copyright 2020 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package packet

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"text/template"

	"github.com/packethost/packngo"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
	infrastructurev1alpha3 "sigs.k8s.io/cluster-api-provider-packet/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-packet/pkg/cloud/packet/scope"
)

const (
	apiTokenVarName = "PACKET_API_KEY"
	clientName      = "CAPP-v1alpha3"
	ipxeOS          = "custom_ipxe"
)

var (
	ErrControlPlanEndpointNotFound = errors.New("control plane not found")
	ErrInvalidRequest              = errors.New("invalid request")
)

type PacketClient struct {
	*packngo.Client
}

// NewClient creates a new Client for the given Packet credentials
func NewClient(packetAPIKey string) *PacketClient {
	token := strings.TrimSpace(packetAPIKey)

	if token != "" {
		return &PacketClient{packngo.NewClientWithAuth(clientName, token, nil)}
	}

	return nil
}

func GetClient() (*PacketClient, error) {
	token := os.Getenv(apiTokenVarName)
	if token == "" {
		return nil, fmt.Errorf("env var %s is required", apiTokenVarName)
	}
	return NewClient(token), nil
}

func (p *PacketClient) GetDevice(deviceID string) (*packngo.Device, error) {
	dev, _, err := p.Client.Devices.Get(deviceID, nil)
	return dev, err
}

type CreateDeviceRequest struct {
	ExtraTags            []string
	MachineScope         *scope.MachineScope
	ControlPlaneEndpoint string
}

func (p *PacketClient) NewDevice(req CreateDeviceRequest) (*packngo.Device, error) {
	if req.MachineScope.PacketMachine.Spec.IPXEUrl != "" {
		// Error if pxe url and OS conflict
		if req.MachineScope.PacketMachine.Spec.OS != ipxeOS {
			return nil, fmt.Errorf("os should be set to custom_pxe when using pxe urls: %w", ErrInvalidRequest)
		}
	}

	userDataRaw, err := req.MachineScope.GetRawBootstrapData()
	if err != nil {
		return nil, errors.Wrap(err, "impossible to retrieve bootstrap data from secret")
	}

	stringWriter := &strings.Builder{}
	userData := string(userDataRaw)
	userDataValues := map[string]interface{}{
		"kubernetesVersion": pointer.StringPtrDerefOr(req.MachineScope.Machine.Spec.Version, ""),
	}

	tags := append(req.MachineScope.PacketMachine.Spec.Tags, req.ExtraTags...)

	tmpl, err := template.New("user-data").Parse(userData)
	if err != nil {
		return nil, fmt.Errorf("error parsing userdata template: %v", err)
	}

	if req.MachineScope.IsControlPlane() {
		// control plane machines should get the API key injected
		userDataValues["apiKey"] = p.Client.APIKey

		if req.ControlPlaneEndpoint != "" {
			userDataValues["controlPlaneEndpoint"] = req.ControlPlaneEndpoint
		}

		tags = append(tags, infrastructurev1alpha3.ControlPlaneTag)
	} else {
		tags = append(tags, infrastructurev1alpha3.WorkerTag)
	}

	if err := tmpl.Execute(stringWriter, userDataValues); err != nil {
		return nil, fmt.Errorf("error executing userdata template: %v", err)
	}

	userData = stringWriter.String()

	// Allow to override the facility for each PacketMachineTemplate
	var facility = req.MachineScope.PacketCluster.Spec.Facility
	if req.MachineScope.PacketMachine.Spec.Facility != "" {
		facility = req.MachineScope.PacketMachine.Spec.Facility
	}

	serverCreateOpts := &packngo.DeviceCreateRequest{
		Hostname:      req.MachineScope.Name(),
		ProjectID:     req.MachineScope.PacketCluster.Spec.ProjectID,
		Facility:      []string{facility},
		BillingCycle:  req.MachineScope.PacketMachine.Spec.BillingCycle,
		Plan:          req.MachineScope.PacketMachine.Spec.MachineType,
		OS:            req.MachineScope.PacketMachine.Spec.OS,
		IPXEScriptURL: req.MachineScope.PacketMachine.Spec.IPXEUrl,
		Tags:          tags,
		UserData:      userData,
	}

	reservationIDs := strings.Split(req.MachineScope.PacketMachine.Spec.HardwareReservationID, ",")

	// If there are no reservationIDs to process, go ahead and return early
	if len(reservationIDs) == 0 {
		dev, _, err := p.Client.Devices.Create(serverCreateOpts)
		return dev, err
	}

	// Do a naive loop through the list of reservationIDs, continuing if we hit any error
	// TODO: if we can determine how to differentiate a failure based on the reservation
	// being in use vs other errors, then we can make this a bit smarter in the future.
	var lastErr error

	for _, resID := range reservationIDs {
		serverCreateOpts.HardwareReservationID = resID
		dev, _, err := p.Client.Devices.Create(serverCreateOpts)
		if err != nil {
			lastErr = err
			continue
		}

		return dev, nil
	}

	return nil, lastErr
}

func (p *PacketClient) GetDeviceAddresses(device *packngo.Device) ([]corev1.NodeAddress, error) {
	addrs := make([]corev1.NodeAddress, 0)
	for _, addr := range device.Network {
		addrType := corev1.NodeInternalIP
		if addr.IpAddressCommon.Public {
			addrType = corev1.NodeExternalIP
		}
		a := corev1.NodeAddress{
			Type:    addrType,
			Address: addr.Address,
		}
		addrs = append(addrs, a)
	}
	return addrs, nil
}

func (p *PacketClient) GetDeviceByTags(project string, tags []string) (*packngo.Device, error) {
	devices, _, err := p.Devices.List(project, nil)
	if err != nil {
		return nil, fmt.Errorf("Error retrieving devices: %v", err)
	}
	// returns the first one that matches all of the tags
	for _, device := range devices {
		if ItemsInList(device.Tags, tags) {
			return &device, nil
		}
	}
	return nil, nil
}

// CreateIP reserves an IP via Packet API. The request fails straight if no IP are available for the specified project.
// This prevent the cluster to become ready.
func (p *PacketClient) CreateIP(namespace, clusterName, projectID, facility string) (net.IP, error) {
	req := packngo.IPReservationRequest{
		Type:                   packngo.PublicIPv4,
		Quantity:               1,
		Facility:               &facility,
		FailOnApprovalRequired: true,
		Tags:                   []string{generateElasticIPIdentifier(clusterName)},
	}

	r, resp, err := p.ProjectIPs.Request(projectID, &req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusUnprocessableEntity {
		return nil, fmt.Errorf("Could not create an Elastic IP due to quota limits on the account. Please contact Packet support.")
	}

	ip := net.ParseIP(r.Address)
	if ip == nil {
		return nil, fmt.Errorf("impossible to parse IP: %s. IP not valid.", r.Address)
	}
	return ip, nil
}

func (p *PacketClient) GetIPByClusterIdentifier(namespace, name, projectID string) (packngo.IPAddressReservation, error) {
	var err error
	var reservedIP packngo.IPAddressReservation

	listOpts := &packngo.ListOptions{}
	reservedIPs, _, err := p.ProjectIPs.List(projectID, listOpts)
	if err != nil {
		return reservedIP, err
	}
	for _, reservedIP := range reservedIPs {
		for _, v := range reservedIP.Tags {
			if v == generateElasticIPIdentifier(name) {
				return reservedIP, nil
			}
		}
	}
	return reservedIP, ErrControlPlanEndpointNotFound
}

func generateElasticIPIdentifier(name string) string {
	return fmt.Sprintf("cluster-api-provider-packet:cluster-id:%s", name)
}
