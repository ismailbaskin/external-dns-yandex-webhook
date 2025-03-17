/*
Copyright 2025 YIVA BULUT.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package provider

import (
	"context"
	"sync"

	"external-dns-yandex-webhook/internal/yandex/client"
)

type MockYandexClient struct {
	mu sync.RWMutex

	zones       []client.Zone
	recordSets  map[string][]client.RecordSet
	upsertCalls []client.UpsertRequest

	ListZonesFunc        func(ctx context.Context) ([]client.Zone, error)
	ListRecordSetsFunc   func(ctx context.Context, zoneID string) ([]client.RecordSet, error)
	UpsertRecordSetsFunc func(ctx context.Context, req client.UpsertRequest) error
}

func NewMockYandexClient() *MockYandexClient {
	return &MockYandexClient{
		recordSets:  make(map[string][]client.RecordSet),
		upsertCalls: make([]client.UpsertRequest, 0),
	}
}

func (m *MockYandexClient) ListZones(ctx context.Context) ([]client.Zone, error) {
	if m.ListZonesFunc != nil {
		return m.ListZonesFunc(ctx)
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.zones, nil
}

func (m *MockYandexClient) ListRecordSets(ctx context.Context, zoneID string) ([]client.RecordSet, error) {
	if m.ListRecordSetsFunc != nil {
		return m.ListRecordSetsFunc(ctx, zoneID)
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.recordSets[zoneID], nil
}

func (m *MockYandexClient) UpsertRecordSets(ctx context.Context, req client.UpsertRequest) error {
	if m.UpsertRecordSetsFunc != nil {
		return m.UpsertRecordSetsFunc(ctx, req)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.upsertCalls = append(m.upsertCalls, req)
	return nil
}

func (m *MockYandexClient) SetZones(zones []client.Zone) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.zones = zones
}

func (m *MockYandexClient) SetRecordSets(zoneID string, records []client.RecordSet) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.recordSets[zoneID] = records
}

func (m *MockYandexClient) GetUpsertCalls() []client.UpsertRequest {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.upsertCalls
}
