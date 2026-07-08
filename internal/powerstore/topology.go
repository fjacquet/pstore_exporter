package powerstore

import (
	"github.com/dell/gopowerstore"
)

// Topology is one array's inventory plus lookup indices used to resolve metric labels.
type Topology struct {
	Cluster      gopowerstore.Cluster
	Appliances   []gopowerstore.ApplianceInstance
	Volumes      []gopowerstore.Volume
	VolumeGroups []gopowerstore.VolumeGroup
	NASServers   []gopowerstore.NAS
	FileSystems  []gopowerstore.FileSystem
	FCPorts      []gopowerstore.FcPort
	EthPorts     []gopowerstore.EthPort

	// Alerts is the per-cycle list of array alerts. Unlike the inventory slices
	// above it is dynamic state (not used for label resolution), so it is set
	// directly by GetTopology after construction rather than via NewTopology.
	Alerts []gopowerstore.Alert

	applianceName       map[string]string
	applianceServiceTag map[string]string
	vgName              map[string]string
	volumeVGID          map[string]string
	volumeName          map[string]string
	volumeApplianceID   map[string]string
	nasName             map[string]string
}

// NewTopology builds a Topology and its lookup indices from the inventory slices
// fetched for a single PowerStore array.
func NewTopology(
	cluster gopowerstore.Cluster,
	appliances []gopowerstore.ApplianceInstance,
	volumes []gopowerstore.Volume,
	vgs []gopowerstore.VolumeGroup,
	nas []gopowerstore.NAS,
	fs []gopowerstore.FileSystem,
	fc []gopowerstore.FcPort,
	eth []gopowerstore.EthPort,
) *Topology {
	t := &Topology{
		Cluster:      cluster,
		Appliances:   appliances,
		Volumes:      volumes,
		VolumeGroups: vgs,
		NASServers:   nas,
		FileSystems:  fs,
		FCPorts:      fc,
		EthPorts:     eth,

		applianceName:       make(map[string]string),
		applianceServiceTag: make(map[string]string),
		vgName:              make(map[string]string),
		volumeVGID:          make(map[string]string),
		volumeName:          make(map[string]string),
		volumeApplianceID:   make(map[string]string),
		nasName:             make(map[string]string),
	}
	for _, a := range appliances {
		t.applianceName[a.ID] = a.Name
		t.applianceServiceTag[a.ID] = a.ServiceTag
	}
	for _, v := range volumes {
		t.volumeName[v.ID] = v.Name
		t.volumeApplianceID[v.ID] = v.ApplianceID
	}
	for _, g := range vgs {
		t.vgName[g.ID] = g.Name
		// VolumeGroup.Volumes ([]gopowerstore.Volume) lists member volumes.
		for _, v := range g.Volumes {
			t.volumeVGID[v.ID] = g.ID
		}
	}
	// Volume.VolumeGroup ([]gopowerstore.VolumeGroup) carries the volume's group
	// membership from the volume side; index it too so VG resolution works whether
	// the membership was populated on the volume or the volume-group fetch.
	for _, v := range volumes {
		for _, g := range v.VolumeGroup {
			if g.ID == "" {
				continue
			}
			t.volumeVGID[v.ID] = g.ID
			if _, ok := t.vgName[g.ID]; !ok {
				t.vgName[g.ID] = g.Name
			}
		}
	}
	for _, n := range nas {
		t.nasName[n.ID] = n.Name
	}
	return t
}

// ClusterID returns the PowerStore cluster's unique id as a string.
func (t *Topology) ClusterID() string { return t.Cluster.ID }

// ApplianceName resolves an appliance id to its name (empty if unknown).
func (t *Topology) ApplianceName(id string) string { return t.applianceName[id] }

// VolumeGroupOf returns (vgID, vgName) for a volume id (empty if none).
func (t *Topology) VolumeGroupOf(volID string) (string, string) {
	vgID := t.volumeVGID[volID]
	return vgID, t.vgName[vgID]
}

// NASName resolves a NAS server id to its name (empty if unknown).
func (t *Topology) NASName(id string) string { return t.nasName[id] }

// ApplianceServiceTag resolves an appliance id to its service tag (empty if unknown).
func (t *Topology) ApplianceServiceTag(id string) string { return t.applianceServiceTag[id] }

// VolumeInfo returns the name and appliance ID for a volume id, plus whether the
// id was found in the inventory index. A false `known` means the id was absent
// (e.g. a snapshot/clone present in the bulk CSV but filtered out of the volume
// inventory); callers fall back to the id for the name and may count the miss.
func (t *Topology) VolumeInfo(id string) (name, applianceID string, known bool) {
	name, known = t.volumeName[id]
	return name, t.volumeApplianceID[id], known
}
