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

	applianceName map[string]string
	vgName        map[string]string
	volumeVGID    map[string]string
	nasName       map[string]string
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

		applianceName: make(map[string]string),
		vgName:        make(map[string]string),
		volumeVGID:    make(map[string]string),
		nasName:       make(map[string]string),
	}
	for _, a := range appliances {
		t.applianceName[a.ID] = a.Name
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
