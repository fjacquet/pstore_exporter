package powerstore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dell/gopowerstore"
	"github.com/dell/gopowerstore/api"
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
	// trace enables response-body logging (--trace) on the raw bulk HTTP path.
	// The typed gopowerstore path cannot be traced: the SDK builds its
	// *http.Client internally with no transport-injection seam (v1.22.0).
	trace bool
	// maxConcurrency caps how many typed API requests the per-entity fan-outs
	// (replication, FS/VG perf, appliance enumeration) issue at once, to bound
	// the load this exporter puts on the array. Always >= 1.
	maxConcurrency int
}

// defaultMaxConcurrency mirrors models.defaultMaxConcurrency; it is the safety
// floor applied when an ArrayClient is constructed with a non-positive cap.
const defaultMaxConcurrency = 16

// Compile-time assertion that ArrayClient satisfies Client.
var _ Client = (*ArrayClient)(nil)

// NewArrayClient constructs an ArrayClient from an array configuration.
// maxConcurrency is the resolved per-array fan-out cap (a non-positive value is
// clamped to defaultMaxConcurrency). trace enables raw bulk-API response-body
// logging (see tracingRoundTripper).
func NewArrayClient(cfg models.ArrayConfig, maxConcurrency int, trace bool) (*ArrayClient, error) {
	if maxConcurrency < 1 {
		maxConcurrency = defaultMaxConcurrency
	}
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
		name:           cfg.Name,
		interval:       gopowerstore.MetricsIntervalEnum(cfg.MetricsInterval()),
		gp:             gp,
		endpoint:       cfg.Endpoint,
		username:       cfg.Username,
		password:       cfg.Password,
		insecure:       cfg.InsecureSkipVerify,
		trace:          trace,
		maxConcurrency: maxConcurrency,
	}, nil
}

// Name returns the configured array name.
func (c *ArrayClient) Name() string { return c.name }

// alertPageSize bounds each alert page request. PowerStore's REST API caps an
// unbounded query at a small server-default page, so we request fixed-size pages
// explicitly and advance until exhausted (see collectActiveAlerts).
const alertPageSize = 1000

// collectActiveAlerts pages through the array's ACTIVE alerts and returns them
// all. gopowerstore's GetAlerts issues a single request and, given empty opts,
// sets no page limit — so the PowerStore server returns only its default first
// page in arbitrary id order and the rest are silently dropped. That made
// powerstore_alert_active under-count on arrays with many alerts (an active
// alert beyond the first page read as zero). We instead filter server-side to
// state=ACTIVE (keeping the result set small) and drive pagination explicitly:
// request fixed-size pages and stop on the first short page. get is injected
// (c.gp.GetAlerts in production) so the pagination logic is unit-testable.
func collectActiveAlerts(ctx context.Context, get func(context.Context, gopowerstore.GetAlertsOpts) (*gopowerstore.GetAlertsResponse, error), pageSize int) ([]gopowerstore.Alert, error) {
	var all []gopowerstore.Alert
	for start := 0; ; start += pageSize {
		resp, err := get(ctx, gopowerstore.GetAlertsOpts{
			RequestPagination: gopowerstore.RequestPagination{PageSize: pageSize, StartIndex: start},
			Queries:           map[string]string{"state": "eq.ACTIVE"},
		})
		if err != nil {
			return nil, err
		}
		if resp == nil {
			return all, nil
		}
		page := resp.Alerts
		all = append(all, page...)
		if len(page) < pageSize {
			return all, nil
		}
	}
}

// isNotFound reports whether err is a gopowerstore 404 (the resource does not
// exist) — an expected, benign condition we skip silently rather than warn
// about. Real failures (server errors, request timeouts, context cancellation)
// are NOT not-found, so callers still surface them as warnings.
func isNotFound(err error) bool {
	var apiErr gopowerstore.APIError
	if errors.As(err, &apiErr) {
		return apiErr.NotFound()
	}
	return false
}

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
		a, err := collectActiveAlerts(gctx, c.gp.GetAlerts, alertPageSize)
		if err != nil {
			logging.LogWarn(fmt.Sprintf("array %q: get alerts: %v", c.name, err))
			return nil
		}
		alerts = a
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
	g2, gctx2 := errgroup.WithContext(ctx)
	g2.SetLimit(c.maxConcurrency)
	for i, id := range ids {
		g2.Go(func() error {
			a, err := c.gp.GetAppliance(gctx2, id)
			if err != nil {
				logging.LogWarn(fmt.Sprintf("array %q: get appliance %q: %v", c.name, id, err))
				return nil
			}
			results[i] = a // distinct index per goroutine — no lock needed
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
	replicated := make([]string, 0, len(topo.Volumes))
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

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(c.maxConcurrency)
	for i, volID := range replicated {
		g.Go(func() error {
			var r resReplication
			if sess, err := c.gp.GetReplicationSessionByLocalResourceID(gctx, volID); err != nil {
				// A volume can carry a protection policy yet have no replication
				// session (e.g. a snapshot-only policy), which the SDK reports as
				// a 404 with an empty message. Skip it silently — that empty-reason
				// warning was pure noise; only real failures warrant a warning.
				if !isNotFound(err) {
					logging.LogWarn(fmt.Sprintf("array %q: replication session for volume %s: %v", c.name, volID, err))
				}
			} else if sess.ID != "" {
				r.session = sess
				r.hasSess = true
			}
			if rate, err := c.gp.VolumeMirrorTransferRate(gctx, volID); err != nil {
				// Likewise, a non-replicated volume has no mirror transfer rate
				// (404). Timeouts and other real errors are not not-found, so they
				// still surface as warnings.
				if !isNotFound(err) {
					logging.LogWarn(fmt.Sprintf("array %q: mirror transfer rate for volume %s: %v", c.name, volID, err))
				}
			} else {
				r.transfer = deriveReplicationTransfer(c.name, topo, volID, "volume", rate)
			}
			results[i] = r // distinct index per goroutine — no lock needed
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
	return parallelSamples(ctx, topo.FileSystems, c.maxConcurrency, func(ctx context.Context, fs gopowerstore.FileSystem) []Sample {
		resp, err := c.gp.PerformanceMetricsByFileSystem(ctx, fs.ID, c.interval)
		if err != nil {
			logging.LogWarn(fmt.Sprintf("array %q: file system %s perf failed: %v", c.name, fs.ID, err))
			return nil
		}
		return deriveFileSystemPerf(c.name, topo, fs, resp)
	})
}

// volumeGroupPerf collects live per-volume-group performance via the typed
// PerformanceMetricsByVg method, one call per volume group, failures logged and
// skipped. Called from both export paths for parity.
func (c *ArrayClient) volumeGroupPerf(ctx context.Context, topo *Topology) []Sample {
	return parallelSamples(ctx, topo.VolumeGroups, c.maxConcurrency, func(ctx context.Context, vg gopowerstore.VolumeGroup) []Sample {
		resp, err := c.gp.PerformanceMetricsByVg(ctx, vg.ID, c.interval)
		if err != nil {
			logging.LogWarn(fmt.Sprintf("array %q: volume group %s perf failed: %v", c.name, vg.ID, err))
			return nil
		}
		return deriveVolumeGroupPerf(c.name, topo, vg, resp)
	})
}

// parallelSamples fans fn out across items and concatenates the per-item samples
// in item order. At most limit invocations run concurrently, bounding the load
// on the array. Each goroutine writes its own slot, so the collection needs no
// locking. Per-item failures are the callback's concern (return nil to
// contribute nothing).
func parallelSamples[T any](ctx context.Context, items []T, limit int, fn func(context.Context, T) []Sample) []Sample {
	if len(items) == 0 {
		return nil
	}
	per := make([][]Sample, len(items))
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(limit)
	for i, item := range items {
		g.Go(func() error {
			per[i] = fn(gctx, item)
			return nil
		})
	}
	_ = g.Wait()

	var samples []Sample
	for _, s := range per {
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

// driveMetrics emits per-drive lifecycle state and wear. PowerStore exposes no
// typed list-drives method, so drives are enumerated through the generic API
// (the sanctioned fallback per ADR-0009): a single GET on the hardware resource
// filtered to type=Drive, which returns each drive's extra_details.drive_wear_level
// in one call (no per-drive metrics requests). Called from both export paths.
func (c *ArrayClient) driveMetrics(ctx context.Context, topo *Topology) []Sample {
	drives, err := c.enumerateDrives(ctx)
	if err != nil {
		logging.LogWarn(fmt.Sprintf("array %q: enumerate drives: %v", c.name, err))
		return nil
	}
	logging.LogDebug(fmt.Sprintf("array %q: enumerated %d drives", c.name, len(drives)))
	return deriveDrives(c.name, topo, drives)
}

// enumerateDrives lists drive hardware via the generic API, paginating defensively.
func (c *ArrayClient) enumerateDrives(ctx context.Context) ([]driveInfo, error) {
	const pageSize = 2000
	var all []driveInfo
	for offset := 0; ; offset += pageSize {
		qp := c.gp.APIClient().QueryParams().
			RawArg("type", "eq.Drive").
			Select("id", "name", "appliance_id", "lifecycle_state", "extra_details").
			Order("id").
			Limit(pageSize).
			Offset(offset)

		var page []driveInfo
		_, err := c.gp.APIClient().Query(ctx, api.RequestConfig{
			Method:      "GET",
			Endpoint:    "hardware",
			QueryParams: qp,
		}, &page)
		if err != nil {
			return nil, err
		}
		all = append(all, page...)
		if len(page) < pageSize {
			return all, nil
		}
	}
}

// witnessMetrics fetches the Metro witness service and derives its state series.
// A 404 means the array predates the witness feature (PowerStoreOS < 3.6) or has
// no witness configured — that is benign and silently yields no samples. Any
// other error is logged and degraded to no samples (powerstore_up stays 1).
func (c *ArrayClient) witnessMetrics(ctx context.Context, topo *Topology) []Sample {
	witnesses, err := c.enumerateWitnesses(ctx)
	if err != nil {
		if !isNotFound(err) {
			logging.LogWarn(fmt.Sprintf("array %q: enumerate witnesses: %v", c.name, err))
		}
		return nil
	}
	return deriveWitness(c.name, topo, witnesses)
}

// enumerateWitnesses lists Metro witness services via the generic API, paginating
// defensively. The witness list is tiny (typically one), but the pattern mirrors
// enumerateDrives for consistency. gopowerstore has no typed witness method, so
// the resource is read with the generic Query escape hatch (see ADR-0009/0015).
func (c *ArrayClient) enumerateWitnesses(ctx context.Context) ([]witnessInfo, error) {
	const pageSize = 2000
	var all []witnessInfo
	for offset := 0; ; offset += pageSize {
		qp := c.gp.APIClient().QueryParams().
			Select("id", "name", "state", "connections").
			Order("id").
			Limit(pageSize).
			Offset(offset)

		var page []witnessInfo
		_, err := c.gp.APIClient().Query(ctx, api.RequestConfig{
			Method:      "GET",
			Endpoint:    "witness",
			QueryParams: qp,
		}, &page)
		if err != nil {
			return nil, err
		}
		all = append(all, page...)
		if len(page) < pageSize {
			return all, nil
		}
	}
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
	samples = append(samples, c.commonMetrics(ctx, topo)...)
	return samples, nil
}

// commonMetrics returns the metrics emitted identically on BOTH the bulk and
// per-entity export paths. Centralizing them here keeps the two paths at metric
// parity structurally rather than by convention — add a shared metric here once
// and both paths pick it up. deriveFileSystemCapacity, derivePortLinkStatus, and
// deriveAlerts are topology-derived (no extra API call here — alerts were fetched
// in GetTopology); the rest issue their own typed/generic calls.
func (c *ArrayClient) commonMetrics(ctx context.Context, topo *Topology) []Sample {
	var samples []Sample
	samples = append(samples, deriveFileSystemCapacity(c.name, topo)...)
	samples = append(samples, derivePortLinkStatus(c.name, topo)...)
	samples = append(samples, deriveAlerts(c.name, topo)...)
	samples = append(samples, c.replicationMetrics(ctx, topo)...)
	samples = append(samples, c.fileSystemPerf(ctx, topo)...)
	samples = append(samples, c.volumeGroupPerf(ctx, topo)...)
	samples = append(samples, c.clusterSpace(ctx, topo)...)
	samples = append(samples, c.driveMetrics(ctx, topo)...)
	samples = append(samples, c.witnessMetrics(ctx, topo)...)
	return samples
}

// Close releases client resources. gopowerstore has no explicit close, so this
// is a no-op kept for interface symmetry and future cleanup.
func (c *ArrayClient) Close() error { return nil }
