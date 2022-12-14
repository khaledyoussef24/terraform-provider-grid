---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "grid_scheduler Resource - terraform-provider-grid"
subcategory: ""
description: |-
  Resource to dynamically assign resource requests to nodes.
---

# grid_scheduler (Resource)

Resource to dynamically assign resource requests to nodes.



<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `requests` (Block List, Min: 1) List of node assignment requests (see [below for nested schema](#nestedblock--requests))

### Read-Only

- `id` (String) The ID of this resource.
- `nodes` (Map of Number) Mapping from the request name to the node id

<a id="nestedblock--requests"></a>
### Nested Schema for `requests`

Required:

- `name` (String) used as a key in the `nodes` dict to be used as a reference

Optional:

- `certified` (Boolean) Pick only certified nodes (Not implemented)
- `cru` (Number) Number of VCPUs
- `domain` (Boolean) Pick only nodes with public config containing domain
- `farm` (String) Farm name
- `hru` (Number) Disk HDD size in MBs
- `ipv4` (Boolean) Pick only nodes with public config containing ipv4
- `mru` (Number) Memory size in MBs
- `sru` (Number) Disk SSD size in MBs


