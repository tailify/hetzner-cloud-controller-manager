/*
Copyright 2018 Hetzner Cloud GmbH.

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

package hcloud

import (
	"context"
	"os"
	"strconv"

	"github.com/hetznercloud/hcloud-go/hcloud"
	"github.com/identw/hetzner-cloud-controller-manager/internal/hcops"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	cloudprovider "k8s.io/cloud-provider"
)

type instances struct {
	client commonClient
}

func newInstances(client commonClient) *instances {
	return &instances{client}
}

func (i *instances) NodeAddressesByProviderID(ctx context.Context, providerID string) ([]v1.NodeAddress, error) {
	id, err := hcops.ProviderIDToServerID(providerID)
	if err != nil {
		return nil, err
	}

	server, err := getServerByID(ctx, i.client, id)
	if err != nil {
		return nil, err
	}
	adresses, err := i.nodeAddresses(ctx, server)
	if err != nil {
		return nil, err
	}
	return adresses, nil
}

func (i *instances) NodeAddresses(ctx context.Context, nodeName types.NodeName) ([]v1.NodeAddress, error) {
	server, err := getServerByName(ctx, i.client, string(nodeName))
	if err != nil {
		return nil, err
	}
	adresses, err := i.nodeAddresses(ctx, server)
	if err != nil {
		return nil, err
	}
	return adresses, nil
}

func (i *instances) ExternalID(ctx context.Context, nodeName types.NodeName) (string, error) {
	return i.InstanceID(ctx, nodeName)
}

func (i *instances) InstanceID(ctx context.Context, nodeName types.NodeName) (string, error) {
	server, err := getServerByName(ctx, i.client, string(nodeName))
	if err != nil {
		return "", err
	}
	return strconv.Itoa(server.ID), nil
}

func (i *instances) InstanceType(ctx context.Context, nodeName types.NodeName) (string, error) {
	server, err := getServerByName(ctx, i.client, string(nodeName))
	if err != nil {
		return "", err
	}
	return server.ServerType.Name, nil
}

func (i *instances) InstanceTypeByProviderID(ctx context.Context, providerID string) (string, error) {
	id, err := hcops.ProviderIDToServerID(providerID)
	if err != nil {
		return "", err
	}

	server, err := getServerByID(ctx, i.client, id)
	if err != nil {
		return "", err
	}
	return server.ServerType.Name, nil
}

func (i *instances) AddSSHKeyToAllInstances(ctx context.Context, user string, keyData []byte) error {
	return cloudprovider.NotImplemented
}

func (i *instances) CurrentNodeName(ctx context.Context, hostname string) (types.NodeName, error) {
	return types.NodeName(hostname), nil
}

func (i instances) InstanceExistsByProviderID(ctx context.Context, providerID string) (exists bool, err error) {
	var id int
	id, err = hcops.ProviderIDToServerID(providerID)
	if err != nil {
		return false, err
	}

	if id == hcops.ExcludeServer.ID {
		return true, nil
	}

	var server *hcloud.Server
	server, _, err = i.client.Hcloud.Server.GetByID(ctx, id)
	if server == nil {
		server, err = hrobotGetServerByID(id)
	}

	if err != nil {
		return false, err
	}

	exists = server != nil

	return exists, nil
}

func (i instances) InstanceExists(ctx context.Context, node *v1.Node) (exists bool, err error) {
	var id int
	id, err = hcops.ProviderIDToServerID(node.Spec.ProviderID)
	if err != nil {
		return
	}

	if id == hcops.ExcludeServer.ID {
		return true, nil
	}

	var server *hcloud.Server
	server, _, err = i.client.Hcloud.Server.GetByID(ctx, id)
	if server == nil {
		server, err = hrobotGetServerByID(id)
	}

	if err != nil {
		return
	}

	exists = server != nil

	return
}

func (i instances) InstanceShutdownByProviderID(ctx context.Context, providerID string) (isOff bool, err error) {
	var id int
	id, err = hcops.ProviderIDToServerID(providerID)
	if err != nil {
		return
	}

	if id == hcops.ExcludeServer.ID {
		return false, nil
	}

	var server *hcloud.Server
	server, _, err = i.client.Hcloud.Server.GetByID(ctx, id)
	if server == nil {
		server, err = hrobotGetServerByID(id)
	}

	if err != nil {
		return
	}

	isOff = server != nil && server.Status == hcloud.ServerStatusOff
	return
}

func (i instances) InstanceShutdown(ctx context.Context, node *v1.Node) (isOff bool, err error) {
	var id int

	id, err = hcops.ProviderIDToServerID(node.Spec.ProviderID)
	if err != nil {
		return
	}

	if id == hcops.ExcludeServer.ID {
		return false, nil
	}

	var server *hcloud.Server
	server, _, err = i.client.Hcloud.Server.GetByID(ctx, id)
	if server == nil {
		server, err = hrobotGetServerByID(id)
	}

	if err != nil {
		return
	}

	isOff = server != nil && server.Status == hcloud.ServerStatusOff
	return
}

func (i *instances) findInventoryAddresses(ctx context.Context, name string) []v1.NodeAddress {
	inventory := i.client.Inventory
	var addresses []v1.NodeAddress
	host, ok := inventory.Hosts[name]
	if ok {
		for _, varName := range i.client.InventoryVars {
			addr, ok := host.Vars[varName]
			if ok {
				addresses = append(
					addresses,
					v1.NodeAddress{Type: v1.NodeInternalIP, Address: addr},
				)
			}
		}
	}
	return addresses
}

func (i *instances) nodeAddresses(ctx context.Context, server *hcloud.Server) ([]v1.NodeAddress, error) {
	var addresses []v1.NodeAddress
	addresses = append(
		addresses,
		v1.NodeAddress{Type: v1.NodeHostName, Address: server.Name},
		v1.NodeAddress{Type: v1.NodeExternalIP, Address: server.PublicNet.IPv4.IP.String()},
	)
	n := os.Getenv(hcloudNetworkENVVar)
	if len(n) > 0 {
		network, _, _ := i.client.Hcloud.Network.Get(ctx, n)
		if network != nil {
			for _, privateNet := range server.PrivateNet {
				if privateNet.Network.ID == network.ID {
					addresses = append(
						addresses,
						v1.NodeAddress{Type: v1.NodeInternalIP, Address: privateNet.IP.String()},
					)
				}
			}

		}
	}

	// Lookup Ansible inventory for private IP addresses
	inventory := i.client.Inventory
	if inventory != nil {
		privateAddrs := i.findInventoryAddresses(ctx, server.Name)
		addresses = append(addresses, privateAddrs...)
	}
	return addresses, nil
}
