package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"testing"

	types "github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/threefoldtech/substrate-client"
	client "github.com/threefoldtech/terraform-provider-grid/internal/node"
	"github.com/threefoldtech/terraform-provider-grid/pkg/deployer"
	"github.com/threefoldtech/terraform-provider-grid/pkg/workloads"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

const Words = "actress baby exhaust blind forget vintage express torch luxury symbol weird eight"

func TestValidateBadAccount(t *testing.T) {
	ctrl := gomock.NewController(t)

	// Assert that Bar() is invoked.
	defer ctrl.Finish()

	m := NewMockSubstrateClient(ctrl)

	identity, err := substrate.NewIdentityFromEd25519Phrase(Words)
	assert.NoError(t, err)
	m.
		EXPECT().
		GetAccount(gomock.Eq(identity)).
		Return(types.AccountInfo{}, errors.New("bad account"))
	gw := GatewayFQDNDeployer{
		APIClient: &apiClient{
			identity: identity,
		},
	}
	err = gw.Validate(context.TODO(), m)
	assert.Error(t, err)
}
func TestValidateEnoughMoneyNodeNotReachable(t *testing.T) {
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	sub := NewMockSubstrateClient(ctrl)
	cl := NewMockClient(ctrl)
	pool := NewMockNodeClientCollection(ctrl)
	identity, err := substrate.NewIdentityFromEd25519Phrase(Words)
	assert.NoError(t, err)
	sub.
		EXPECT().
		GetAccount(gomock.Eq(identity)).
		Return(types.AccountInfo{
			Data: struct {
				Free       types.U128
				Reserved   types.U128
				MiscFrozen types.U128
				FreeFrozen types.U128
			}{
				Free: types.NewU128(*big.NewInt(30000)),
			},
		}, nil)
	cl.
		EXPECT().
		Call(
			gomock.Any(),
			uint32(10),
			"zos.network.interfaces",
			nil,
			gomock.Any(),
		).
		Return(errors.New("couldn't reach node"))
	pool.
		EXPECT().
		GetNodeClient(
			gomock.Any(),
			uint32(11),
		).
		Return(client.NewNodeClient(10, cl), nil)

	gw := GatewayFQDNDeployer{
		APIClient: &apiClient{
			identity: identity,
		},
		ncPool: pool,
		Node:   11,
	}
	err = gw.Validate(context.TODO(), sub)
	assert.Error(t, err)
}

func TestValidateEnoughMoneyNodeReachable(t *testing.T) {
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	sub := NewMockSubstrateClient(ctrl)
	cl := NewMockClient(ctrl)
	pool := NewMockNodeClientCollection(ctrl)
	identity, err := substrate.NewIdentityFromEd25519Phrase(Words)
	assert.NoError(t, err)
	sub.
		EXPECT().
		GetAccount(gomock.Eq(identity)).
		Return(types.AccountInfo{
			Data: struct {
				Free       types.U128
				Reserved   types.U128
				MiscFrozen types.U128
				FreeFrozen types.U128
			}{
				Free: types.NewU128(*big.NewInt(30000)),
			},
		}, nil)
	cl.
		EXPECT().
		Call(
			gomock.Any(),
			uint32(10),
			"zos.network.interfaces",
			nil,
			gomock.Any(),
		).
		Return(nil)
	pool.
		EXPECT().
		GetNodeClient(
			gomock.Any(),
			uint32(11),
		).
		Return(client.NewNodeClient(10, cl), nil)

	gw := GatewayFQDNDeployer{
		APIClient: &apiClient{
			identity: identity,
		},
		ncPool: pool,
		Node:   11,
	}
	err = gw.Validate(context.TODO(), sub)
	assert.NoError(t, err)
}

func TestGenerateDeployment(t *testing.T) {
	g := workloads.GatewayFQDNProxy{
		Name:           "name",
		TLSPassthrough: false,
		Backends:       []zos.Backend{"a", "b"},
		FQDN:           "name.com",
	}
	gw := GatewayFQDNDeployer{
		APIClient: &apiClient{
			twin_id: 11,
		},
		Node: 10,
		Gw:   g,
	}
	dls, err := gw.GenerateVersionlessDeployments(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, dls, map[uint32]gridtypes.Deployment{
		10: {
			Version: 0,
			TwinID:  11,
			Workloads: []gridtypes.Workload{
				{
					Version: 0,
					Type:    zos.GatewayFQDNProxyType,
					Name:    gridtypes.Name(g.Name),
					Data: gridtypes.MustMarshal(zos.GatewayFQDNProxy{
						TLSPassthrough: g.TLSPassthrough,
						Backends:       g.Backends,
						FQDN:           gw.Gw.FQDN,
					}),
				},
			},
			SignatureRequirement: gridtypes.SignatureRequirement{
				WeightRequired: 1,
				Requests: []gridtypes.SignatureRequest{
					{
						TwinID: 11,
						Weight: 1,
					},
				},
			},
		},
	})
}

func TestDeploy(t *testing.T) {
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	identity, err := substrate.NewIdentityFromEd25519Phrase(Words)
	assert.NoError(t, err)
	deployer := deployer.NewMockDeployer(ctrl)
	sub := NewMockSubstrateClient(ctrl)
	gw := GatewayFQDNDeployer{
		APIClient: &apiClient{
			identity: identity,
			twin_id:  11,
		},
		Node: 10,
		Gw: workloads.GatewayFQDNProxy{
			Name:           "name",
			TLSPassthrough: false,
			Backends:       []zos.Backend{"https://1.1.1.1", "http://2.2.2.2"},
			FQDN:           "name.com",
		},
		deployer: deployer,
	}
	dls, err := gw.GenerateVersionlessDeployments(context.Background())
	assert.NoError(t, err)
	deployer.EXPECT().Deploy(
		gomock.Any(),
		sub,
		nil,
		dls,
	).Return(map[uint32]uint64{10: 100}, nil)
	err = gw.Deploy(context.Background(), sub)
	assert.NoError(t, err)
	assert.Equal(t, gw.NodeDeploymentID, map[uint32]uint64{uint32(10): uint64(100)})
}

func TestUpdate(t *testing.T) {
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	identity, err := substrate.NewIdentityFromEd25519Phrase(Words)
	assert.NoError(t, err)
	deployer := deployer.NewMockDeployer(ctrl)
	sub := NewMockSubstrateClient(ctrl)
	gw := GatewayFQDNDeployer{
		APIClient: &apiClient{
			identity: identity,
			twin_id:  11,
		},
		Node: 10,
		Gw: workloads.GatewayFQDNProxy{
			Name:           "name",
			TLSPassthrough: false,
			Backends:       []zos.Backend{"https://1.1.1.1", "http://2.2.2.2"},
			FQDN:           "name.com",
		},
		deployer:         deployer,
		NodeDeploymentID: map[uint32]uint64{10: 100},
	}
	dls, err := gw.GenerateVersionlessDeployments(context.Background())
	assert.NoError(t, err)
	deployer.EXPECT().Deploy(
		gomock.Any(),
		sub,
		map[uint32]uint64{10: 100},
		dls,
	).Return(map[uint32]uint64{uint32(10): uint64(100)}, nil)
	err = gw.Deploy(context.Background(), sub)
	assert.NoError(t, err)
	assert.Equal(t, gw.NodeDeploymentID, map[uint32]uint64{uint32(10): uint64(100)})
}
func TestUpdateFailed(t *testing.T) {
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	identity, err := substrate.NewIdentityFromEd25519Phrase(Words)
	assert.NoError(t, err)
	deployer := deployer.NewMockDeployer(ctrl)
	sub := NewMockSubstrateClient(ctrl)
	gw := GatewayFQDNDeployer{
		APIClient: &apiClient{
			identity: identity,
			twin_id:  11,
		},
		Node: 10,
		Gw: workloads.GatewayFQDNProxy{
			Name:           "name",
			TLSPassthrough: false,
			Backends:       []zos.Backend{"https://1.1.1.1", "http://2.2.2.2"},
			FQDN:           "name.com",
		},
		deployer:         deployer,
		NodeDeploymentID: map[uint32]uint64{10: 100},
	}
	dls, err := gw.GenerateVersionlessDeployments(context.Background())
	assert.NoError(t, err)
	deployer.EXPECT().Deploy(
		gomock.Any(),
		sub,
		map[uint32]uint64{10: 100},
		dls,
	).Return(map[uint32]uint64{10: 100}, errors.New("error"))
	err = gw.Deploy(context.Background(), sub)
	assert.Error(t, err)
	assert.Equal(t, gw.NodeDeploymentID, map[uint32]uint64{uint32(10): uint64(100)})
}

func TestCancel(t *testing.T) {
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	identity, err := substrate.NewIdentityFromEd25519Phrase(Words)
	assert.NoError(t, err)
	deployer := deployer.NewMockDeployer(ctrl)
	sub := NewMockSubstrateClient(ctrl)
	gw := GatewayFQDNDeployer{
		APIClient: &apiClient{
			identity: identity,
			twin_id:  11,
		},
		Node: 10,
		Gw: workloads.GatewayFQDNProxy{
			Name:           "name",
			TLSPassthrough: false,
			Backends:       []zos.Backend{"https://1.1.1.1", "http://2.2.2.2"},
			FQDN:           "name.com",
		},
		deployer:         deployer,
		NodeDeploymentID: map[uint32]uint64{10: 100},
	}
	deployer.EXPECT().Deploy(
		gomock.Any(),
		sub,
		map[uint32]uint64{10: 100},
		map[uint32]gridtypes.Deployment{},
	).Return(map[uint32]uint64{}, nil)
	err = gw.Cancel(context.Background(), sub)
	assert.NoError(t, err)
	assert.Equal(t, gw.NodeDeploymentID, map[uint32]uint64{})
}

func TestCancelFailed(t *testing.T) {
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	identity, err := substrate.NewIdentityFromEd25519Phrase(Words)
	assert.NoError(t, err)
	deployer := deployer.NewMockDeployer(ctrl)
	sub := NewMockSubstrateClient(ctrl)
	gw := GatewayFQDNDeployer{
		APIClient: &apiClient{
			identity: identity,
			twin_id:  11,
		},
		Node: 10,
		Gw: workloads.GatewayFQDNProxy{
			Name:           "name",
			TLSPassthrough: false,
			Backends:       []zos.Backend{"https://1.1.1.1", "http://2.2.2.2"},
			FQDN:           "name.com",
		},
		deployer:         deployer,
		NodeDeploymentID: map[uint32]uint64{10: 100},
	}
	deployer.EXPECT().Deploy(
		gomock.Any(),
		sub,
		map[uint32]uint64{10: 100},
		map[uint32]gridtypes.Deployment{},
	).Return(map[uint32]uint64{10: 100}, errors.New("error"))
	err = gw.Cancel(context.Background(), sub)
	assert.Error(t, err)
	assert.Equal(t, gw.NodeDeploymentID, map[uint32]uint64{10: 100})
}

func TestSyncContracts(t *testing.T) {
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	identity, err := substrate.NewIdentityFromEd25519Phrase(Words)
	assert.NoError(t, err)
	sub := NewMockSubstrateClient(ctrl)
	gw := GatewayFQDNDeployer{
		ID: "123",
		APIClient: &apiClient{
			identity: identity,
			twin_id:  11,
		},
		Node: 10,
		Gw: workloads.GatewayFQDNProxy{
			Name:           "name",
			TLSPassthrough: false,
			Backends:       []zos.Backend{"https://1.1.1.1", "http://2.2.2.2"},
			FQDN:           "name.com",
		},
		NodeDeploymentID: map[uint32]uint64{10: 100},
	}
	sub.EXPECT().GetContract(
		uint64(100),
	).Return(&substrate.Contract{
		State: substrate.ContractState{
			IsCreated: true,
		},
	}, nil)
	err = gw.syncContracts(context.Background(), sub)
	assert.NoError(t, err)
	assert.Equal(t, gw.NodeDeploymentID, map[uint32]uint64{10: 100})
	assert.Equal(t, gw.ID, "123")
}

func TestSyncDeletedContracts(t *testing.T) {
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	identity, err := substrate.NewIdentityFromEd25519Phrase(Words)
	assert.NoError(t, err)
	sub := NewMockSubstrateClient(ctrl)
	gw := GatewayFQDNDeployer{
		ID: "123",
		APIClient: &apiClient{
			identity: identity,
			twin_id:  11,
		},
		Node: 10,
		Gw: workloads.GatewayFQDNProxy{
			Name:           "name",
			TLSPassthrough: false,
			Backends:       []zos.Backend{"https://1.1.1.1", "http://2.2.2.2"},
			FQDN:           "name.com",
		},
		NodeDeploymentID: map[uint32]uint64{10: 100},
	}
	sub.EXPECT().GetContract(
		uint64(100),
	).Return(&substrate.Contract{
		State: substrate.ContractState{
			IsCreated: false,
		},
	}, nil)
	err = gw.syncContracts(context.Background(), sub)
	assert.NoError(t, err)
	assert.Equal(t, gw.NodeDeploymentID, map[uint32]uint64{})
	assert.Equal(t, gw.ID, "")
}

func TestSyncContractsFailure(t *testing.T) {
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	identity, err := substrate.NewIdentityFromEd25519Phrase(Words)
	assert.NoError(t, err)
	sub := NewMockSubstrateClient(ctrl)
	gw := GatewayFQDNDeployer{
		ID: "123",
		APIClient: &apiClient{
			identity: identity,
			twin_id:  11,
		},
		Node: 10,
		Gw: workloads.GatewayFQDNProxy{
			Name:           "name",
			TLSPassthrough: false,
			Backends:       []zos.Backend{"https://1.1.1.1", "http://2.2.2.2"},
			FQDN:           "name.com",
		},
		NodeDeploymentID: map[uint32]uint64{10: 100},
	}
	sub.EXPECT().GetContract(
		uint64(100),
	).Return(nil, errors.New("123"))
	err = gw.syncContracts(context.Background(), sub)
	assert.Error(t, err)
	assert.Equal(t, gw.NodeDeploymentID, map[uint32]uint64{10: 100})
	assert.Equal(t, gw.ID, "123")
}

func TestSyncFailureInContract(t *testing.T) {
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	identity, err := substrate.NewIdentityFromEd25519Phrase(Words)
	deployer := deployer.NewMockDeployer(ctrl)
	assert.NoError(t, err)
	sub := NewMockSubstrateClient(ctrl)
	gw := GatewayFQDNDeployer{
		ID: "123",
		APIClient: &apiClient{
			identity: identity,
			twin_id:  11,
		},
		Node: 10,
		Gw: workloads.GatewayFQDNProxy{
			Name:           "name",
			TLSPassthrough: false,
			Backends:       []zos.Backend{"https://1.1.1.1", "http://2.2.2.2"},
			FQDN:           "name.com",
		},
		NodeDeploymentID: map[uint32]uint64{10: 100},
		deployer:         deployer,
	}
	sub.EXPECT().GetContract(
		uint64(100),
	).Return(nil, errors.New("123"))
	err = gw.sync(context.Background(), sub)
	assert.Error(t, err)
	assert.Equal(t, gw.NodeDeploymentID, map[uint32]uint64{10: 100})
	assert.Equal(t, gw.ID, "123")
}

func TestSync(t *testing.T) {
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	identity, err := substrate.NewIdentityFromEd25519Phrase(Words)
	deployer := deployer.NewMockDeployer(ctrl)
	pool := NewMockNodeClientCollection(ctrl)
	cl := NewMockClient(ctrl)
	assert.NoError(t, err)
	sub := NewMockSubstrateClient(ctrl)
	gw := GatewayFQDNDeployer{
		ID: "123",
		APIClient: &apiClient{
			identity: identity,
			twin_id:  11,
		},
		Node: 10,
		Gw: workloads.GatewayFQDNProxy{
			Name:           "name",
			TLSPassthrough: false,
			Backends:       []zos.Backend{"https://1.1.1.1", "http://2.2.2.2"},
			FQDN:           "name.com",
		},
		NodeDeploymentID: map[uint32]uint64{10: 100},
		deployer:         deployer,
		ncPool:           pool,
	}
	dls, err := gw.GenerateVersionlessDeployments(context.Background())
	assert.NoError(t, err)
	dl := dls[10]
	dl.Workloads[0].Result.State = gridtypes.StateOk
	dl.Workloads[0].Result.Data, err = json.Marshal(zos.GatewayFQDNResult{})
	assert.NoError(t, err)
	sub.EXPECT().GetContract(
		uint64(100),
	).Return(&substrate.Contract{
		State: substrate.ContractState{
			IsCreated: true,
		},
	}, nil)
	pool.EXPECT().
		GetNodeClient(sub, uint32(10)).
		Return(client.NewNodeClient(12, cl), nil)
	cl.EXPECT().
		Call(gomock.Any(), uint32(12), "zos.deployment.get", gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, twin uint32, fn string, data interface{}, result interface{}) error {
			// TODO: check argument has correct deployment id
			*result.(*gridtypes.Deployment) = dl
			fmt.Printf("%+v", dl)
			return nil
		})
	gw.Gw.FQDN = "123"
	err = gw.sync(context.Background(), sub)
	assert.NoError(t, err)
	assert.Equal(t, gw.NodeDeploymentID, map[uint32]uint64{10: 100})
	assert.Equal(t, gw.ID, "123")
	assert.Equal(t, gw.Gw.FQDN, "name.com")
}

func TestSyncDeletedWorkload(t *testing.T) {
	ctrl := gomock.NewController(t)

	defer ctrl.Finish()

	identity, err := substrate.NewIdentityFromEd25519Phrase(Words)
	deployer := deployer.NewMockDeployer(ctrl)
	pool := NewMockNodeClientCollection(ctrl)
	cl := NewMockClient(ctrl)
	assert.NoError(t, err)
	sub := NewMockSubstrateClient(ctrl)
	gw := GatewayFQDNDeployer{
		ID: "123",
		APIClient: &apiClient{
			identity: identity,
			twin_id:  11,
		},
		Node: 10,
		Gw: workloads.GatewayFQDNProxy{
			Name:           "name",
			TLSPassthrough: false,
			Backends:       []zos.Backend{"https://1.1.1.1", "http://2.2.2.2"},
			FQDN:           "name.com",
		},
		NodeDeploymentID: map[uint32]uint64{10: 100},
		deployer:         deployer,
		ncPool:           pool,
	}
	dls, err := gw.GenerateVersionlessDeployments(context.Background())
	assert.NoError(t, err)
	dl := dls[10]
	// state is deleted
	sub.EXPECT().GetContract(
		uint64(100),
	).Return(&substrate.Contract{
		State: substrate.ContractState{
			IsCreated: true,
		},
	}, nil)
	pool.EXPECT().
		GetNodeClient(sub, uint32(10)).
		Return(client.NewNodeClient(12, cl), nil)
	cl.EXPECT().
		Call(gomock.Any(), uint32(12), "zos.deployment.get", gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, twin uint32, fn string, data interface{}, result interface{}) error {
			// TODO: check argument has correct deployment id
			*result.(*gridtypes.Deployment) = dl
			fmt.Printf("%+v", dl)
			return nil
		})
	gw.Gw.FQDN = "123"
	err = gw.sync(context.Background(), sub)
	assert.NoError(t, err)
	assert.Equal(t, gw.NodeDeploymentID, map[uint32]uint64{10: 100})
	assert.Equal(t, gw.ID, "123")
	assert.Equal(t, gw.Gw, workloads.GatewayFQDNProxy{})
}
