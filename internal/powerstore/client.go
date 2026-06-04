package powerstore

import (
	"context"
	"fmt"
	"time"

	"github.com/dell/gopowerstore"

	"github.com/fjacquet/pstore_exporter/internal/logging"
	"github.com/fjacquet/pstore_exporter/internal/models"
)

// defaultTimeout bounds each API request to a live array.
const defaultTimeout = 60 * time.Second

// ArrayClient is the per-array PowerStore API client. It wraps a gopowerstore
// client and satisfies the Client interface. The endpoint/credentials are also
// retained so the bulk-CSV path can issue raw authenticated HTTP requests to the
// latest_five_min_metrics endpoints, which return a gzipped tar (not JSON) and so
// cannot go through the typed, JSON-decoding gopowerstore client.
type ArrayClient struct {
	name            string
	interval        gopowerstore.MetricsIntervalEnum
	softwareVersion string // cached PowerStoreOS version; "" until detected
	gp              gopowerstore.Client

	endpoint string // configured API endpoint, e.g. https://10.0.0.1/api/rest
	username string
	password string
	// insecure mirrors cfg.InsecureSkipVerify. PowerStore arrays commonly use
	// self-signed certificates, so disabling verification is an operator-chosen,
	// per-array setting (logged on startup) rather than a hardcoded default.
	insecure bool
}

// Compile-time assertion that ArrayClient satisfies Client.
var _ Client = (*ArrayClient)(nil)

// NewArrayClient constructs an ArrayClient from an array configuration.
func NewArrayClient(cfg models.ArrayConfig) (*ArrayClient, error) {
	if cfg.InsecureSkipVerify {
		logging.LogWarn(fmt.Sprintf("array %q: InsecureSkipVerify is enabled; TLS certificate verification is disabled", cfg.Name))
	}

	opts := gopowerstore.NewClientOptions()
	opts.SetInsecure(cfg.InsecureSkipVerify)
	opts.SetDefaultTimeout(defaultTimeout)

	gp, err := gopowerstore.NewClientWithArgs(cfg.Endpoint, cfg.Username, cfg.Password, opts)
	if err != nil {
		return nil, fmt.Errorf("array %q: create gopowerstore client: %w", cfg.Name, err)
	}

	return &ArrayClient{
		name:     cfg.Name,
		interval: gopowerstore.MetricsIntervalEnum(cfg.MetricsInterval()),
		gp:       gp,
		endpoint: cfg.Endpoint,
		username: cfg.Username,
		password: cfg.Password,
		insecure: cfg.InsecureSkipVerify,
	}, nil
}

// Name returns the configured array name.
func (c *ArrayClient) Name() string { return c.name }

// GetTopology fetches the array inventory and builds lookup indices. A failure to
// reach the cluster is fatal (the array is down); failures on the optional
// inventory lists (volumes, NAS, file systems, ports) are tolerated so a
// block-only or file-only array still produces a usable topology.
func (c *ArrayClient) GetTopology(ctx context.Context) (*Topology, error) {
	cluster, err := c.gp.GetCluster(ctx)
	if err != nil {
		return nil, fmt.Errorf("array %q: get cluster: %w", c.name, err)
	}

	volumes, err := c.gp.GetVolumes(ctx)
	if err != nil {
		logging.LogWarn(fmt.Sprintf("array %q: get volumes: %v", c.name, err))
		volumes = nil
	}
	vgs, err := c.gp.GetVolumeGroups(ctx)
	if err != nil {
		logging.LogWarn(fmt.Sprintf("array %q: get volume groups: %v", c.name, err))
		vgs = nil
	}
	nas, err := c.gp.GetNASServers(ctx)
	if err != nil {
		logging.LogWarn(fmt.Sprintf("array %q: get NAS servers: %v", c.name, err))
		nas = nil
	}
	fs, err := c.gp.ListFS(ctx)
	if err != nil {
		logging.LogWarn(fmt.Sprintf("array %q: list file systems: %v", c.name, err))
		fs = nil
	}
	fc, err := c.gp.GetFCPorts(ctx)
	if err != nil {
		logging.LogWarn(fmt.Sprintf("array %q: get FC ports: %v", c.name, err))
		fc = nil
	}
	eth, err := c.gp.GetEthPorts(ctx)
	if err != nil {
		logging.LogWarn(fmt.Sprintf("array %q: get Ethernet ports: %v", c.name, err))
		eth = nil
	}

	appliances := c.enumerateAppliances(ctx, volumes, fc, eth)

	return NewTopology(cluster, appliances, volumes, vgs, nas, fs, fc, eth), nil
}

// enumerateAppliances resolves the distinct appliances referenced by the
// inventory. PowerStore exposes no list-appliances method, so we collect distinct
// ApplianceIDs from volumes and ports and fetch each one individually.
func (c *ArrayClient) enumerateAppliances(
	ctx context.Context,
	volumes []gopowerstore.Volume,
	fc []gopowerstore.FcPort,
	eth []gopowerstore.EthPort,
) []gopowerstore.ApplianceInstance {
	seen := make(map[string]struct{})
	addID := func(id string) {
		if id != "" {
			seen[id] = struct{}{}
		}
	}
	for i := range volumes {
		addID(volumes[i].ApplianceID)
	}
	for i := range fc {
		addID(fc[i].ApplianceID)
	}
	for i := range eth {
		addID(eth[i].ApplianceID)
	}

	appliances := make([]gopowerstore.ApplianceInstance, 0, len(seen))
	for id := range seen {
		a, err := c.gp.GetAppliance(ctx, id)
		if err != nil {
			logging.LogWarn(fmt.Sprintf("array %q: get appliance %q: %v", c.name, id, err))
			continue
		}
		appliances = append(appliances, a)
	}
	return appliances
}

// BulkCapable reports whether the array supports the bulk CSV metrics API
// (introduced in PowerStoreOS 4.1). The version is detected via the typed
// GetSoftwareMajorMinorVersion method and cached. On any detection error it
// returns false, which routes the collector to the always-available per-entity
// path.
func (c *ArrayClient) BulkCapable(ctx context.Context, _ *Topology) bool {
	if c.softwareVersion == "" {
		v, err := c.gp.GetSoftwareMajorMinorVersion(ctx)
		if err != nil {
			logging.LogWarn(fmt.Sprintf("array %q: detect software version: %v", c.name, err))
			return false
		}
		// GetSoftwareMajorMinorVersion returns e.g. 4.1 as a float; render it
		// back to a dotted version string for the version comparator.
		c.softwareVersion = fmt.Sprintf("%.1f.0.0", v)
	}
	return bulkCapableFromVersion(c.softwareVersion)
}

// BulkMetrics enables the bulk five-minute metrics export, downloads the gzipped
// tar of CSVs via a raw authenticated HTTP client, parses it, and derives samples
// with metric names and label keys identical to the per-entity path. Any failure
// returns an error so the collector falls back to the per-entity path.
func (c *ArrayClient) BulkMetrics(ctx context.Context, topo *Topology) ([]Sample, error) {
	archive, err := c.downloadBulkArchive(ctx)
	if err != nil {
		return nil, err
	}
	files, err := parseBulkArchive(archive)
	if err != nil {
		return nil, err
	}
	var samples []Sample
	samples = append(samples, deriveBulkAppliancePerf(c.name, topo, files["performance_metrics_by_appliance.csv"])...)
	samples = append(samples, deriveBulkApplianceSpace(c.name, topo, files["space_metrics_by_appliance.csv"])...)
	samples = append(samples, deriveBulkVolumePerf(c.name, topo, files["performance_metrics_by_volume.csv"])...)
	// File-system capacity and port link status are inventory-derived (no API
	// call), so emit them on the bulk path too for parity with per-entity.
	samples = append(samples, deriveFileSystemCapacity(c.name, topo)...)
	samples = append(samples, derivePortLinkStatus(c.name, topo)...)
	return samples, nil
}

// Close releases client resources. gopowerstore has no explicit close, so this
// is a no-op kept for interface symmetry and future cleanup.
func (c *ArrayClient) Close() error { return nil }
