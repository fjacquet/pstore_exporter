package powerstore

import (
	"context"
	"fmt"

	"github.com/fjacquet/pstore_exporter/internal/logging"
)

// PerEntityMetrics collects performance + space metrics via the typed gopowerstore
// client, plus file-system capacity and port link-status from inventory. A failed
// sub-query is logged and skipped (graceful degradation).
func (c *ArrayClient) PerEntityMetrics(ctx context.Context, topo *Topology) ([]Sample, error) {
	var samples []Sample

	for _, a := range topo.Appliances {
		if perf, err := c.gp.PerformanceMetricsByAppliance(ctx, a.ID, c.interval); err != nil {
			logging.LogWarn(fmt.Sprintf("array %q: appliance %s perf failed: %v", c.name, a.ID, err))
		} else {
			samples = append(samples, deriveAppliancePerf(c.name, topo, perf)...)
		}
		if space, err := c.gp.SpaceMetricsByAppliance(ctx, a.ID, c.interval); err != nil {
			logging.LogWarn(fmt.Sprintf("array %q: appliance %s space failed: %v", c.name, a.ID, err))
		} else {
			samples = append(samples, deriveApplianceSpace(c.name, topo, space)...)
		}
	}

	for _, v := range topo.Volumes {
		if perf, err := c.gp.PerformanceMetricsByVolume(ctx, v.ID, c.interval); err != nil {
			logging.LogWarn(fmt.Sprintf("array %q: volume %s perf failed: %v", c.name, v.ID, err))
		} else {
			samples = append(samples, deriveVolumePerf(c.name, topo, perf)...)
		}
	}

	samples = append(samples, deriveFileSystemCapacity(c.name, topo)...)
	samples = append(samples, derivePortLinkStatus(c.name, topo)...)
	samples = append(samples, deriveAlerts(c.name, topo)...)
	samples = append(samples, c.replicationMetrics(ctx, topo)...)
	samples = append(samples, c.fileSystemPerf(ctx, topo)...)
	samples = append(samples, c.volumeGroupPerf(ctx, topo)...)
	samples = append(samples, c.clusterSpace(ctx, topo)...)
	samples = append(samples, c.driveMetrics(ctx, topo)...)

	return samples, nil
}
