package storage

import (
	"context"
	"fmt"

	"github.com/example/fru-tracker/internal/storage/ent"
	"github.com/example/fru-tracker/internal/storage/ent/label"
	entresource "github.com/example/fru-tracker/internal/storage/ent/resource"

	v1 "github.com/example/fru-tracker/apis/example.fabrica.dev/v1"
)

// ensureEntClient verifies the ent client has been initialized
func ensureEntClient() {
	if entClient == nil {
		panic("ent client not initialized: call SetEntClient in main.go before using storage")
	}
}

// QueryResources returns a generic query builder for a given kind
func QueryResources(ctx context.Context, kind string) *ent.ResourceQuery {
	ensureEntClient()
	return entClient.Resource.Query().
		Where(entresource.KindEQ(kind))
}

// QueryResourcesByLabels queries resources by kind and exact-match labels
func QueryResourcesByLabels(ctx context.Context, kind string, labels map[string]string) (*ent.ResourceQuery, error) {
	ensureEntClient()
	q := entClient.Resource.Query().Where(entresource.KindEQ(kind))
	for k, v := range labels {
		q = q.Where(entresource.HasLabelsWith(
			label.KeyEQ(k),
			label.ValueEQ(v),
		))
	}
	return q, nil
}

// Querydevices returns a query builder for devices
func Querydevices(ctx context.Context) *ent.ResourceQuery {
	return QueryResources(ctx, "Device")
}

// GetDeviceByUID loads a single Device by UID
func GetDeviceByUID(ctx context.Context, uid string) (*v1.Device, error) {
	ensureEntClient()
	r, err := entClient.Resource.Query().
		Where(entresource.UIDEQ(uid), entresource.KindEQ("Device")).
		WithLabels().
		WithAnnotations().
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to load Device %s: %w", uid, err)
	}
	v, err := FromEntResource(ctx, r)
	if err != nil {
		return nil, err
	}
	return v.(*v1.Device), nil
}

// ListdevicesByLabels returns devices matching all provided labels
func ListdevicesByLabels(ctx context.Context, labels map[string]string) ([]*v1.Device, error) {
	q, err := QueryResourcesByLabels(ctx, "Device", labels)
	if err != nil {
		return nil, err
	}
	rs, err := q.WithLabels().WithAnnotations().All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*v1.Device, 0, len(rs))
	for _, r := range rs {
		v, err := FromEntResource(ctx, r)
		if err != nil {
			continue
		}
		out = append(out, v.(*v1.Device))
	}
	return out, nil
}

// Querydiscoverysnapshots returns a query builder for discoverysnapshots
func Querydiscoverysnapshots(ctx context.Context) *ent.ResourceQuery {
	return QueryResources(ctx, "DiscoverySnapshot")
}

// GetDiscoverySnapshotByUID loads a single DiscoverySnapshot by UID
func GetDiscoverySnapshotByUID(ctx context.Context, uid string) (*v1.DiscoverySnapshot, error) {
	ensureEntClient()
	r, err := entClient.Resource.Query().
		Where(entresource.UIDEQ(uid), entresource.KindEQ("DiscoverySnapshot")).
		WithLabels().
		WithAnnotations().
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to load DiscoverySnapshot %s: %w", uid, err)
	}
	v, err := FromEntResource(ctx, r)
	if err != nil {
		return nil, err
	}
	return v.(*v1.DiscoverySnapshot), nil
}

// ListdiscoverysnapshotsByLabels returns discoverysnapshots matching all provided labels
func ListdiscoverysnapshotsByLabels(ctx context.Context, labels map[string]string) ([]*v1.DiscoverySnapshot, error) {
	q, err := QueryResourcesByLabels(ctx, "DiscoverySnapshot", labels)
	if err != nil {
		return nil, err
	}
	rs, err := q.WithLabels().WithAnnotations().All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*v1.DiscoverySnapshot, 0, len(rs))
	for _, r := range rs {
		v, err := FromEntResource(ctx, r)
		if err != nil {
			continue
		}
		out = append(out, v.(*v1.DiscoverySnapshot))
	}
	return out, nil
}
