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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"external-dns-yandex-webhook/internal/yandex/client"

	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/plan"
)

func TestYandexProvider_Records(t *testing.T) {
	tests := []struct {
		name           string
		zones          []client.Zone
		recordSets     map[string][]client.RecordSet
		expectedResult []*endpoint.Endpoint
		expectError    bool
		zonesErr       error
		recordsErr     error
	}{
		{
			name: "successful record listing",
			zones: []client.Zone{
				{
					ID:   "zone-1",
					Name: "zone-1.ext-dns-test-2.yc.zalan.do",
				},
			},
			recordSets: map[string][]client.RecordSet{
				"zone-1": {
					{
						Name: "list-test",
						Type: "A",
						TTL:  300,
						Data: []string{"1.2.3.4"},
					},
					{
						Name: "list-test-alias",
						Type: "CNAME",
						TTL:  300,
						Data: []string{"foo.elb.amazonaws.com"},
					},
					{
						Name: "ignored-soa",
						Type: "SOA",
						TTL:  300,
						Data: []string{"ns1.example.com"},
					},
					{
						Name: "ignored-ns",
						Type: "NS",
						TTL:  300,
						Data: []string{"ns1.example.com"},
					},
				},
			},
			expectedResult: []*endpoint.Endpoint{
				{
					DNSName:    "list-test.zone-1.ext-dns-test-2.yc.zalan.do",
					RecordType: "A",
					Targets:    endpoint.Targets{"1.2.3.4"},
					RecordTTL:  300,
					Labels:     map[string]string{},
				},
				{
					DNSName:    "list-test-alias.zone-1.ext-dns-test-2.yc.zalan.do",
					RecordType: "CNAME",
					Targets:    endpoint.Targets{"foo.elb.amazonaws.com"},
					RecordTTL:  300,
					Labels:     map[string]string{},
				},
			},
			expectError: false,
		},
		{
			name:        "zone list error",
			zones:       []client.Zone{},
			zonesErr:    fmt.Errorf("failed to list zones"),
			expectError: true,
		},
		{
			name: "record list error",
			zones: []client.Zone{
				{
					ID:   "zone-1",
					Name: "zone-1.ext-dns-test-2.yc.zalan.do",
				},
			},
			recordsErr:  fmt.Errorf("failed to list records"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := NewMockYandexClient()
			mockClient.SetZones(tt.zones)
			for zoneID, records := range tt.recordSets {
				mockClient.SetRecordSets(zoneID, records)
			}
			mockClient.ListZonesFunc = func(ctx context.Context) ([]client.Zone, error) {
				return tt.zones, tt.zonesErr
			}
			mockClient.ListRecordSetsFunc = func(ctx context.Context, zoneID string) ([]client.RecordSet, error) {
				return tt.recordSets[zoneID], tt.recordsErr
			}

			provider := NewYandexProvider(endpoint.NewDomainFilter([]string{"ext-dns-test-2.yc.zalan.do."}), false, mockClient)

			result, err := provider.Records(context.Background())

			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				validateEndpoints(t, result, tt.expectedResult)
			}
		})
	}
}

func TestYandexProvider_ApplyChanges(t *testing.T) {
	tests := []struct {
		name        string
		zones       []client.Zone
		changes     *plan.Changes
		expectError bool
		dryRun      bool
	}{
		{
			name: "successful record changes",
			zones: []client.Zone{
				{
					ID:   "zone-1",
					Name: "zone-1.ext-dns-test-2.yc.zalan.do",
				},
				{
					ID:   "zone-2",
					Name: "zone-2.ext-dns-test-2.yc.zalan.do",
				},
			},
			changes: &plan.Changes{
				Create: []*endpoint.Endpoint{
					{
						DNSName:    "create-test.zone-1.ext-dns-test-2.yc.zalan.do",
						RecordType: "A",
						Targets:    endpoint.Targets{"8.8.8.8"},
						RecordTTL:  300,
					},
					{
						DNSName:    "create-test-cname.zone-1.ext-dns-test-2.yc.zalan.do",
						RecordType: "CNAME",
						Targets:    endpoint.Targets{"foo.elb.amazonaws.com"},
						RecordTTL:  300,
					},
				},
				UpdateNew: []*endpoint.Endpoint{
					{
						DNSName:    "update-test.zone-1.ext-dns-test-2.yc.zalan.do",
						RecordType: "A",
						Targets:    endpoint.Targets{"1.2.3.4"},
						RecordTTL:  300,
						Labels:     map[string]string{},
					},
					{
						DNSName:    "update-test-cname.zone-1.ext-dns-test-2.yc.zalan.do",
						RecordType: "CNAME",
						Targets:    endpoint.Targets{"baz.elb.amazonaws.com"},
						RecordTTL:  300,
						Labels:     map[string]string{},
					},
				},
				UpdateOld: []*endpoint.Endpoint{
					{
						DNSName:    "update-test.zone-1.ext-dns-test-2.yc.zalan.do",
						RecordType: "A",
						Targets:    endpoint.Targets{"8.8.8.8"},
					},
					{
						DNSName:    "update-test-cname.zone-1.ext-dns-test-2.yc.zalan.do",
						RecordType: "CNAME",
						Targets:    endpoint.Targets{"bar.elb.amazonaws.com"},
					},
				},
				Delete: []*endpoint.Endpoint{
					{
						DNSName:    "delete-test.zone-1.ext-dns-test-2.yc.zalan.do",
						RecordType: "A",
						Targets:    endpoint.Targets{"8.8.8.8"},
						Labels:     map[string]string{},
					},
					{
						DNSName:    "delete-test-cname.zone-1.ext-dns-test-2.yc.zalan.do",
						RecordType: "CNAME",
						Targets:    endpoint.Targets{"qux.elb.amazonaws.com"},
						Labels:     map[string]string{},
					},
				},
			},
			expectError: false,
		},
		{
			name:        "empty changes",
			zones:       []client.Zone{},
			changes:     &plan.Changes{},
			expectError: false,
		},
		{
			name: "dry run mode",
			zones: []client.Zone{
				{
					ID:   "zone-1",
					Name: "zone-1.ext-dns-test-2.yc.zalan.do",
				},
			},
			changes: &plan.Changes{
				Delete: []*endpoint.Endpoint{
					{
						DNSName:    "delete-test.zone-1.ext-dns-test-2.yc.zalan.do",
						RecordType: "A",
						Targets:    endpoint.Targets{"8.8.8.8"},
						Labels:     map[string]string{},
					},
				},
			},
			dryRun:      true,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := NewMockYandexClient()
			mockClient.SetZones(tt.zones)

			provider := NewYandexProvider(endpoint.NewDomainFilter([]string{"ext-dns-test-2.yc.zalan.do."}), tt.dryRun, mockClient)

			err := provider.ApplyChanges(context.Background(), tt.changes)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				upsertCalls := mockClient.GetUpsertCalls()
				if !tt.dryRun && len(tt.changes.Create)+len(tt.changes.UpdateNew)+len(tt.changes.Delete) > 0 {
					assert.NotEmpty(t, upsertCalls)
				}
			}
		})
	}
}

func TestNormalizeDNSName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "already normalized",
			input:    "test.example.com.",
			expected: "test.example.com.",
		},
		{
			name:     "missing trailing dot",
			input:    "test.example.com",
			expected: "test.example.com.",
		},
		{
			name:     "with spaces",
			input:    " test.example.com ",
			expected: "test.example.com.",
		},
		{
			name:     "mixed case",
			input:    "Test.Example.Com",
			expected: "test.example.com.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeDNSName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func validateEndpoints(t *testing.T, endpoints []*endpoint.Endpoint, expected []*endpoint.Endpoint) {
	assert.Equal(t, len(expected), len(endpoints))

	for i := range endpoints {
		assert.Equal(t, expected[i].DNSName, endpoints[i].DNSName)
		assert.Equal(t, expected[i].RecordType, endpoints[i].RecordType)
		assert.Equal(t, expected[i].Targets, endpoints[i].Targets)
		assert.Equal(t, expected[i].RecordTTL, endpoints[i].RecordTTL)
		if expected[i].Labels != nil {
			assert.Equal(t, expected[i].Labels, endpoints[i].Labels)
		}
	}
}

func TestYandexProvider_AdjustEndpoints(t *testing.T) {
	tests := []struct {
		name           string
		endpoints      []*endpoint.Endpoint
		expectedResult []*endpoint.Endpoint
		expectError    bool
	}{
		{
			name: "adjust endpoints",
			endpoints: []*endpoint.Endpoint{
				{
					DNSName:    "test.example.com",
					RecordType: "A",
					Targets:    endpoint.Targets{"192.0.2.1"},
					RecordTTL:  300,
				},
			},
			expectedResult: []*endpoint.Endpoint{
				{
					DNSName:    "test.example.com",
					RecordType: "A",
					Targets:    endpoint.Targets{"192.0.2.1"},
					RecordTTL:  300,
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewYandexProvider(endpoint.NewDomainFilter([]string{}), false, NewMockYandexClient())

			result, err := provider.AdjustEndpoints(tt.endpoints)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedResult, result)
			}
		})
	}
}

func TestYandexProvider_GetZones(t *testing.T) {
	tests := []struct {
		name          string
		zones         []client.Zone
		domainFilter  []string
		expectedZones map[string]string
		zonesErr      error
		expectError   bool
	}{
		{
			name: "successful zone filter",
			zones: []client.Zone{
				{
					ID:   "zone-1",
					Name: "zone-1.ext-dns-test-2.yc.zalan.do",
				},
				{
					ID:   "zone-2",
					Name: "zone-2.another-domain.com",
				},
			},
			domainFilter: []string{"ext-dns-test-2.yc.zalan.do."},
			expectedZones: map[string]string{
				"zone-1.ext-dns-test-2.yc.zalan.do": "zone-1",
			},
			expectError: false,
		},
		{
			name: "list all zones",
			zones: []client.Zone{
				{
					ID:   "zone-1",
					Name: "zone-1.example.com",
				},
				{
					ID:   "zone-2",
					Name: "zone-2.example.com",
				},
			},
			domainFilter: []string{},
			expectedZones: map[string]string{
				"zone-1.example.com": "zone-1",
				"zone-2.example.com": "zone-2",
			},
			expectError: false,
		},
		{
			name:         "zone list error",
			zones:        []client.Zone{},
			domainFilter: []string{},
			zonesErr:     fmt.Errorf("failed to list zones"),
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := NewMockYandexClient()
			mockClient.SetZones(tt.zones)
			mockClient.ListZonesFunc = func(ctx context.Context) ([]client.Zone, error) {
				return tt.zones, tt.zonesErr
			}

			provider := NewYandexProvider(endpoint.NewDomainFilter(tt.domainFilter), false, mockClient)
			zones, err := provider.(*yandexProvider).getZones(context.Background())

			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedZones, zones)
			}
		})
	}
}

func TestYandexProvider_GetHostZoneID(t *testing.T) {
	tests := []struct {
		name         string
		hostname     string
		managedZones map[string]string
		expectedID   string
		expectError  bool
	}{
		{
			name:     "find matching zone",
			hostname: "test.zone-1.ext-dns-test-2.yc.zalan.do",
			managedZones: map[string]string{
				"zone-1.ext-dns-test-2.yc.zalan.do": "zone-1",
				"zone-2.ext-dns-test-2.yc.zalan.do": "zone-2",
			},
			expectedID:  "zone-1",
			expectError: false,
		},
		{
			name:     "find longest matching zone",
			hostname: "test.sub.zone-1.ext-dns-test-2.yc.zalan.do",
			managedZones: map[string]string{
				"zone-1.ext-dns-test-2.yc.zalan.do":     "zone-1",
				"sub.zone-1.ext-dns-test-2.yc.zalan.do": "sub-zone-1",
			},
			expectedID:  "sub-zone-1",
			expectError: false,
		},
		{
			name:     "no matching zone found",
			hostname: "test.unknown-zone.com",
			managedZones: map[string]string{
				"zone-1.ext-dns-test-2.yc.zalan.do": "zone-1",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &yandexProvider{}
			zoneID, err := provider.getHostZoneID(tt.hostname, tt.managedZones)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedID, zoneID)
			}
		})
	}
}

func TestYandexProvider_ApplyChanges_Errors(t *testing.T) {
	tests := []struct {
		name        string
		zones       []client.Zone
		changes     *plan.Changes
		zonesErr    error
		upsertErr   error
		expectError bool
	}{
		{
			name: "zone list error",
			zones: []client.Zone{
				{
					ID:   "zone-1",
					Name: "zone-1.ext-dns-test-2.yc.zalan.do",
				},
			},
			changes: &plan.Changes{
				Create: []*endpoint.Endpoint{
					{
						DNSName:    "test.zone-1.ext-dns-test-2.yc.zalan.do",
						RecordType: "A",
						Targets:    endpoint.Targets{"192.0.2.1"},
					},
				},
			},
			zonesErr:    fmt.Errorf("failed to list zones"),
			expectError: true,
		},
		{
			name: "upsert error",
			zones: []client.Zone{
				{
					ID:   "zone-1",
					Name: "zone-1.ext-dns-test-2.yc.zalan.do",
				},
			},
			changes: &plan.Changes{
				Create: []*endpoint.Endpoint{
					{
						DNSName:    "test.zone-1.ext-dns-test-2.yc.zalan.do",
						RecordType: "A",
						Targets:    endpoint.Targets{"192.0.2.1"},
					},
				},
			},
			upsertErr:   fmt.Errorf("failed to upsert records"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := NewMockYandexClient()
			mockClient.SetZones(tt.zones)
			mockClient.ListZonesFunc = func(ctx context.Context) ([]client.Zone, error) {
				return tt.zones, tt.zonesErr
			}
			mockClient.UpsertRecordSetsFunc = func(ctx context.Context, req client.UpsertRequest) error {
				return tt.upsertErr
			}

			provider := NewYandexProvider(endpoint.NewDomainFilter([]string{"ext-dns-test-2.yc.zalan.do."}), false, mockClient)

			err := provider.ApplyChanges(context.Background(), tt.changes)
			assert.Error(t, err)
		})
	}
}
