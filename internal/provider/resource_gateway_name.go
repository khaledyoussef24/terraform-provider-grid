package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pkg/errors"
	"github.com/threefoldtech/zos/pkg/gridtypes"
	"github.com/threefoldtech/zos/pkg/gridtypes/zos"
)

func resourceGatewayNameProxy() *schema.Resource {
	return &schema.Resource{
		// This description is used by the documentation generator and the language server.
		Description: "Resource for deploying gateway domains.",

		CreateContext: resourceGatewayNameCreate,
		ReadContext:   resourceGatewayNameRead,
		UpdateContext: resourceGatewayNameUpdate,
		DeleteContext: resourceGatewayNameDelete,

		Schema: map[string]*schema.Schema{
			"name": {
				Description: "resource name",
				Type:        schema.TypeString,
				Required:    true,
			},
			"description": {
				Description: "Description field",
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "",
			},
			"node": {
				Description: "The gateway's node id",
				Type:        schema.TypeInt,
				Required:    true,
			},
			"fqdn": {
				Description: "The fully quallified domain name of the deployed workload.",
				Type:        schema.TypeString,
				Computed:    true,
			},
			"tls_passthrough": {
				Description: "true to pass the tls as is to the backends.",
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
			},
			"backends": {
				Description: "The backends of the gateway proxy",
				Type:        schema.TypeList,
				Required:    true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"node_deployment_id": {
				Type:     schema.TypeMap,
				Computed: true,
				Elem:     &schema.Schema{Type: schema.TypeInt},
			},
		},
	}
}

type GatewayNameDeployer struct {
	Name           string
	Description    string
	Node           uint32
	TLSPassthrough bool
	Backends       []zos.Backend

	FQDN             string
	NodeDeploymentID map[uint32]uint64

	APIClient *apiClient
	ncPool    *NodeClientPool
}

func NewGatewayNameDeployer(ctx context.Context, d *schema.ResourceData, apiClient *apiClient) (GatewayNameDeployer, error) {
	backendsIf := d.Get("backends").([]interface{})
	backends := make([]zos.Backend, len(backendsIf))
	for idx, n := range backendsIf {
		backends[idx] = zos.Backend(n.(string))
	}
	nodeDeploymentIDIf := d.Get("node_deployment_id").(map[string]interface{})
	nodeDeploymentID := make(map[uint32]uint64)
	for node, id := range nodeDeploymentIDIf {
		nodeInt, err := strconv.ParseUint(node, 10, 32)
		if err != nil {
			return GatewayNameDeployer{}, errors.Wrap(err, "couldn't parse node id")
		}
		deploymentID := uint64(id.(int))
		nodeDeploymentID[uint32(nodeInt)] = deploymentID
	}

	deployer := GatewayNameDeployer{
		Name:             d.Get("name").(string),
		Description:      d.Get("description").(string),
		Node:             uint32(d.Get("node").(int)),
		Backends:         backends,
		FQDN:             d.Get("fqdn").(string),
		TLSPassthrough:   d.Get("tls_passthrough").(bool),
		NodeDeploymentID: nodeDeploymentID,
		APIClient:        apiClient,
		ncPool:           NewNodeClient(apiClient.sub, apiClient.rmb),
	}
	return deployer, nil
}

func (k *GatewayNameDeployer) ValidateCreate(ctx context.Context) error {
	return isNodesUp(ctx, []uint32{k.Node}, k.ncPool)
}

func (k *GatewayNameDeployer) ValidateUpdate(ctx context.Context) error {
	nodes := make([]uint32, 0)
	nodes = append(nodes, k.Node)
	for node := range k.NodeDeploymentID {
		nodes = append(nodes, node)
	}
	return isNodesUp(ctx, nodes, k.ncPool)
}

func (k *GatewayNameDeployer) ValidateRead(ctx context.Context) error {
	nodes := make([]uint32, 0)
	for node := range k.NodeDeploymentID {
		nodes = append(nodes, node)
	}
	return isNodesUp(ctx, nodes, k.ncPool)
}

func (k *GatewayNameDeployer) ValidateDelete(ctx context.Context) error {
	return nil
}

func (k *GatewayNameDeployer) storeState(d *schema.ResourceData) {

	nodeDeploymentID := make(map[string]interface{})
	for node, id := range k.NodeDeploymentID {
		nodeDeploymentID[fmt.Sprintf("%d", node)] = int(id)
	}

	d.Set("node", k.Node)
	d.Set("tls_passthrough", k.TLSPassthrough)
	d.Set("backends", k.Backends)
	d.Set("fqdn", k.FQDN)
	d.Set("node_deployment_id", nodeDeploymentID)
}
func (k *GatewayNameDeployer) GenerateVersionlessDeployments(ctx context.Context) (map[uint32]gridtypes.Deployment, error) {
	deployments := make(map[uint32]gridtypes.Deployment)
	workload := gridtypes.Workload{
		Version:     0,
		Type:        zos.GatewayNameProxyType,
		Description: k.Description,
		Name:        gridtypes.Name(k.Name),
		Data: gridtypes.MustMarshal(zos.GatewayNameProxy{
			Name:           k.Name,
			TLSPassthrough: k.TLSPassthrough,
			Backends:       k.Backends,
		}),
	}

	deployment := gridtypes.Deployment{
		Version: Version,
		TwinID:  k.APIClient.twin_id, //LocalTwin,
		// this contract id must match the one on substrate
		Workloads: []gridtypes.Workload{
			workload,
		},
		SignatureRequirement: gridtypes.SignatureRequirement{
			WeightRequired: 1,
			Requests: []gridtypes.SignatureRequest{
				{
					TwinID: k.APIClient.twin_id,
					Weight: 1,
				},
			},
		},
	}
	deployments[k.Node] = deployment
	return deployments, nil
}

func (k *GatewayNameDeployer) GetOldDeployments(ctx context.Context) (map[uint32]gridtypes.Deployment, error) {
	return getDeploymentObjects(ctx, k.NodeDeploymentID, k.ncPool)
}

func (k *GatewayNameDeployer) Deploy(ctx context.Context) error {
	newDeployments, err := k.GenerateVersionlessDeployments(ctx)
	if err != nil {
		return errors.Wrap(err, "couldn't generate deployments data")
	}
	oldDeployments, err := k.GetOldDeployments(ctx)
	if err != nil {
		return errors.Wrap(err, "couldn't get old deployments data")
	}
	currentDeployments, err := deployDeployments(ctx, oldDeployments, newDeployments, k.ncPool, k.APIClient, true)
	if err := k.updateState(ctx, currentDeployments); err != nil {
		log.Printf("error updating state: %s\n", err)
	}
	return err
}
func (k *GatewayNameDeployer) updateState(ctx context.Context, currentDeploymentIDs map[uint32]uint64) error {
	k.NodeDeploymentID = currentDeploymentIDs
	dls, err := getDeploymentObjects(ctx, currentDeploymentIDs, k.ncPool)
	if err != nil {
		return errors.Wrap(err, "couldn't get deployment objects")
	}
	dl, ok := dls[k.Node]
	if !ok {
		k.FQDN = ""
	} else {
		var result zos.GatewayProxyResult
		if err := json.Unmarshal(dl.Workloads[0].Result.Data, &result); err != nil {
			return errors.Wrap(err, "error unmarshalling json")
		}
		k.FQDN = result.FQDN
	}
	return nil
}

func (k *GatewayNameDeployer) updateFromRemote(ctx context.Context) error {
	return k.updateState(ctx, k.NodeDeploymentID)
}

func (k *GatewayNameDeployer) Cancel(ctx context.Context) error {
	newDeployments := make(map[uint32]gridtypes.Deployment)
	oldDeployments := make(map[uint32]gridtypes.Deployment)
	for node, deploymentID := range k.NodeDeploymentID {
		oldDeployments[node] = gridtypes.Deployment{
			ContractID: deploymentID,
		}
	}

	currentDeployments, err := deployDeployments(ctx, oldDeployments, newDeployments, k.ncPool, k.APIClient, false)
	if err := k.updateState(ctx, currentDeployments); err != nil {
		log.Printf("error updating state: %s\n", err)
	}
	return err
}

func resourceGatewayNameCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {

	var diags diag.Diagnostics
	apiClient := meta.(*apiClient)
	rmbctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go startRmb(rmbctx, apiClient.substrate_url, int(apiClient.twin_id))
	deployer, err := NewGatewayNameDeployer(ctx, d, apiClient)
	if err != nil {
		return diag.FromErr(errors.Wrap(err, "couldn't load deployer data"))
	}
	if err := deployer.ValidateCreate(ctx); err != nil {
		diags = append(diags, diag.Diagnostic{
			Severity: diag.Error,
			Summary:  "Error happened while doing initial check (check https://github.com/threefoldtech/terraform-provider-grid/blob/development/TROUBLESHOOTING.md)",
			Detail:   err.Error(),
		})
		return diags
	}
	err = deployer.Deploy(ctx)
	if err != nil {
		if len(deployer.NodeDeploymentID) != 0 {
			// failed to deploy and failed to revert, store the current state locally
			diags = diag.FromErr(err)
		} else {
			return diag.FromErr(err)
		}
	}
	deployer.storeState(d)
	d.SetId(uuid.New().String())
	return diags
}

func resourceGatewayNameUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {

	var diags diag.Diagnostics
	apiClient := meta.(*apiClient)
	rmbctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go startRmb(rmbctx, apiClient.substrate_url, int(apiClient.twin_id))
	deployer, err := NewGatewayNameDeployer(ctx, d, apiClient)
	if err != nil {
		return diag.FromErr(errors.Wrap(err, "couldn't load deployer data"))
	}

	if err := deployer.ValidateUpdate(ctx); err != nil {
		diags = append(diags, diag.Diagnostic{
			Severity: diag.Error,
			Summary:  "Error happened while doing initial check (check https://github.com/threefoldtech/terraform-provider-grid/blob/development/TROUBLESHOOTING.md)",
			Detail:   err.Error(),
		})
		return diags
	}

	err = deployer.Deploy(ctx)
	if err != nil {
		diags = diag.FromErr(err)
	}
	deployer.storeState(d)
	return diags
}

func resourceGatewayNameRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {

	var diags diag.Diagnostics
	apiClient := meta.(*apiClient)
	rmbctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go startRmb(rmbctx, apiClient.substrate_url, int(apiClient.twin_id))
	deployer, err := NewGatewayNameDeployer(ctx, d, apiClient)
	if err != nil {
		return diag.FromErr(errors.Wrap(err, "couldn't load deployer data"))
	}

	if err := deployer.ValidateRead(ctx); err != nil {
		diags = append(diags, diag.Diagnostic{
			Severity: diag.Error,
			Summary:  "Error happened while doing initial check (check https://github.com/threefoldtech/terraform-provider-grid/blob/development/TROUBLESHOOTING.md)",
			Detail:   err.Error(),
		})
		return diags
	}
	err = deployer.updateFromRemote(ctx)
	log.Printf("read updateFromRemote err: %s\n", err)
	if err != nil {
		return diag.FromErr(err)
	}
	deployer.storeState(d)
	return diags
}

func resourceGatewayNameDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	apiClient := meta.(*apiClient)
	rmbctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go startRmb(rmbctx, apiClient.substrate_url, int(apiClient.twin_id))
	deployer, err := NewGatewayNameDeployer(ctx, d, apiClient)
	if err != nil {
		return diag.FromErr(errors.Wrap(err, "couldn't load deployer data"))
	}
	err = deployer.Cancel(ctx)
	if err != nil {
		diags = diag.FromErr(err)
	}
	if err == nil {
		d.SetId("")
	} else {
		deployer.storeState(d)
	}
	return diags
}