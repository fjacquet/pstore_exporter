package powerstore

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/dell/gopowerstore"
	"golang.org/x/sync/errgroup"

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

	var (
		volumes []gopowerstore.Volume
		vgs     []gopowerstore.VolumeGroup
		nas     []gopowerstore.NAS
		fs      []gopowerstore.FileSystem
		fc      []gopowerstore.FcPort
		eth     []gopowerstore.EthPort
		alerts  []gopowerstore.Alert
	)

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		v, err := c.gp.GetVolumes(gctx)
		if err != nil {
			logging.LogWarn(fmt.Sprintf("array %q: get volumes: %v", c.name, err))
			return nil
		}
		volumes = v
		return nil
	})
	g.Go(func() error {
		v, err := c.gp.GetVolumeGroups(gctx)
		if err != nil {
			logging.LogWarn(fmt.Sprintf("array %q: get volume groups: %v", c.name, err))
			return nil
		}
		vgs = v
		return nil
	})
	g.Go(func() error {
		v, err := c.gp.GetNASServers(gctx)
		if err != nil {
			logging.LogWarn(fmt.Sprintf("array %q: get NAS servers: %v", c.name, err))
			return nil
		}
		nas = v
		return nil
	})
	g.Go(func() error {
		v, err := c.gp.ListFS(gctx)
		if err != nil {
			logging.LogWarn(fmt.Sprintf("array %q: list file systems: %v", c.name, err))
			return nil
		}
		fs = v
		return nil
	})
	g.Go(func() error {
		v, err := c.gp.GetFCPorts(gctx)
		if err != nil {
			logging.LogWarn(fmt.Sprintf("array %q: get FC ports: %v", c.name, err))
			return nil
		}
		fc = v
		return nil
	})
	g.Go(func() error {
		v, err := c.gp.GetEthPorts(gctx)
		if err != nil {
			logging.LogWarn(fmt.Sprintf("array %q: get Ethernet ports: %v", c.name, err))
			return nil
		}
		eth = v
		return nil
	})
	g.Go(func() error {
		resp, err := c.gp.GetAlerts(gctx, gopowerstore.GetAlertsOpts{})
		if err != nil {
			logging.LogWarn(fmt.Sprintf("array %q: get alerts: %v", c.name, err))
			return nil
		}
		if resp != nil {
			alerts = resp.Alerts
		}
		return nil
	})
	// errgroup always returns nil here because each goroutine swallows errors.
	_ = g.Wait()

	appliances := c.enumerateAppliances(ctx, volumes, fc, eth)

	topo := NewTopology(cluster, appliances, volumes, vgs, nas, fs, fc, eth)
	topo.Alerts = alerts
	return topo, nil
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

	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}

	results := make([]gopowerstore.ApplianceInstance, len(ids))
	var mu sync.Mutex
	g2, gctx2 := errgroup.WithContext(ctx)
	for i, id := range ids {
		i, id := i, id // capture loop variables
		g2.Go(func() error {
			a, err := c.gp.GetAppliance(gctx2, id)
			if err != nil {
				logging.LogWarn(fmt.Sprintf("array %q: get appliance %q: %v", c.name, id, err))
				return nil
			}
			mu.Lock()
			results[i] = a
			mu.Unlock()
			return nil
		})
	}
	_ = g2.Wait()

	appliances := make([]gopowerstore.ApplianceInstance, 0, len(ids))
	for _, a := range results {
		if a.ID != "" {
			appliances = append(appliances, a)
		}
	}
	return appliances
}

// replicationMetrics collects replication session state, RPO, and transfer metrics
// via typed gopowerstore methods. It is library-first: RPO comes from the
// cluster-wide GetReplicationRules; sessions and transfer rates are enumerated
// from the volumes that carry a protection policy (PowerStore exposes no
// list-replication-sessions method), one typed call per replicated volume.
// Per-call failures are logged and skipped (graceful degradation). It is called
// from BOTH export paths so the bulk and per-entity outputs stay at parity.
func (c *ArrayClient) replicationMetrics(ctx context.Context, topo *Topology) []Sample {
	var samples []Sample

	if rules, err := c.gp.GetReplicationRules(ctx); err != nil {
		logging.LogWarn(fmt.Sprintf("array %q: get replication rules: %v", c.name, err))
	} else {
		samples = append(samples, deriveReplicationRules(c.name, topo, rules)...)
	}

	// Candidate resources: volumes with a protection policy may have a
	// replication session. Volumes without one are skipped to avoid a flood of
	// not-found lookups.
	var replicated []string
	for _, v := range topo.Volumes {
		if v.ProtectionPolicyID != "" {
			replicated = append(replicated, v.ID)
		}
	}
	if len(replicated) == 0 {
		return samples
	}

	type resReplication struct {
		session  gopowerstore.ReplicationSession
		hasSess  bool
		transfer []Sample
	}
	results := make([]resReplication, len(replicated))

	var mu sync.Mutex
	g, gctx := errgroup.WithContext(ctx)
	for i, volID := range replicated {
		i, volID := i, volID
		g.Go(func() error {
			var r resReplication
			if sess, err := c.gp.GetReplicationSessionByLocalResourceID(gctx, volID); err != nil {
				logging.LogWarn(fmt.Sprintf("array %q: replication session for volume %s: %v", c.name, volID, err))
			} else if sess.ID != "" {
				r.session = sess
				r.hasSess = true
			}
			if rate, err := c.gp.VolumeMirrorTransferRate(gctx, volID); err != nil {
				logging.LogWarn(fmt.Sprintf("array %q: mirror transfer rate for volume %s: %v", c.name, volID, err))
			} else {
				r.transfer = deriveReplicationTransfer(c.name, topo, volID, "volume", rate)
			}
			mu.Lock()
			results[i] = r
			mu.Unlock()
			return nil
		})
	}
	_ = g.Wait()

	var sessions []gopowerstore.ReplicationSession
	for _, r := range results {
		if r.hasSess {
			sessions = append(sessions, r.session)
		}
		samples = append(samples, r.transfer...)
	}
	samples = append(samples, deriveReplicationSessions(c.name, topo, sessions)...)
	return samples
}

// fileSystemPerf collects live per-file-system performance via the typed
// PerformanceMetricsByFileSystem method (available in gopowerstore v1.22.0;
// see ADR-0009). One typed call per file system, failures logged and skipped.
// Called from BOTH export paths so bulk and per-entity stay at parity; it
// complements the inventory-derived file-system capacity metrics.
func (c *ArrayClient) fileSystemPerf(ctx context.Context, topo *Topology) []Sample {
	if len(topo.FileSystems) == 0 {
		return nil
	}
	perFS := make([][]Sample, len(topo.FileSystems))
	var mu sync.Mutex
	g, gctx := errgroup.WithContext(ctx)
	for i, fs := range topo.FileSystems {
		i, fs := i, fs
		g.Go(func() error {
			resp, err := c.gp.PerformanceMetricsByFileSystem(gctx, fs.ID, c.interval)
			if err != nil {
				logging.LogWarn(fmt.Sprintf("array %q: file system %s perf failed: %v", c.name, fs.ID, err))
				return nil
			}
			s := deriveFileSystemPerf(c.name, topo, fs, resp)
			mu.Lock()
			perFS[i] = s
			mu.Unlock()
			return nil
		})
	}
	_ = g.Wait()

	var samples []Sample
	for _, s := range perFS {
		samples = append(samples, s...)
	}
	return samples
}

// volumeGroupPerf collects live per-volume-group performance via the typed
// PerformanceMetricsByVg method, one call per volume group, failures logged and
// skipped. Called from both export paths for parity.
func (c *ArrayClient) volumeGroupPerf(ctx context.Context, topo *Topology) []Sample {
	if len(topo.VolumeGroups) == 0 {
		return nil
	}
	perVG := make([][]Sample, len(topo.VolumeGroups))
	var mu sync.Mutex
	g, gctx := errgroup.WithContext(ctx)
	for i, vg := range topo.VolumeGroups {
		i, vg := i, vg
		g.Go(func() error {
			resp, err := c.gp.PerformanceMetricsByVg(gctx, vg.ID, c.interval)
			if err != nil {
				logging.LogWarn(fmt.Sprintf("array %q: volume group %s perf failed: %v", c.name, vg.ID, err))
				return nil
			}
			s := deriveVolumeGroupPerf(c.name, topo, vg, resp)
			mu.Lock()
			perVG[i] = s
			mu.Unlock()
			return nil
		})
	}
	_ = g.Wait()

	var samples []Sample
	for _, s := range perVG {
		samples = append(samples, s...)
	}
	return samples
}

// clusterSpace collects cluster-wide space metrics via the typed
// SpaceMetricsByCluster method (one call). Used for capacity forecasting. Called
// from both export paths for parity. A failure is logged and yields no samples.
func (c *ArrayClient) clusterSpace(ctx context.Context, topo *Topology) []Sample {
	resp, err := c.gp.SpaceMetricsByCluster(ctx, topo.ClusterID(), c.interval)
	if err != nil {
		logging.LogWarn(fmt.Sprintf("array %q: cluster space failed: %v", c.name, err))
		return nil
	}
	return deriveClusterSpace(c.name, topo, resp)
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
	// File-system capacity, port link status, and alerts are topology-derived (no
	// extra API call here — alerts were fetched in GetTopology), so emit them on
	// the bulk path too for parity with per-entity.
	samples = append(samples, deriveFileSystemCapacity(c.name, topo)...)
	samples = append(samples, derivePortLinkStatus(c.name, topo)...)
	samples = append(samples, deriveAlerts(c.name, topo)...)
	samples = append(samples, c.replicationMetrics(ctx, topo)...)
	samples = append(samples, c.fileSystemPerf(ctx, topo)...)
	samples = append(samples, c.volumeGroupPerf(ctx, topo)...)
	samples = append(samples, c.clusterSpace(ctx, topo)...)
	return samples, nil
}

// Close releases client resources. gopowerstore has no explicit close, so this
// is a no-op kept for interface symmetry and future cleanup.
func (c *ArrayClient) Close() error { return nil }
