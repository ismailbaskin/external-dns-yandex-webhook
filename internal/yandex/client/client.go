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

package client

import (
	"context"
	"fmt"
	"os"

	"github.com/yandex-cloud/go-genproto/yandex/cloud/dns/v1"
	ycsdk "github.com/yandex-cloud/go-sdk"
	"github.com/yandex-cloud/go-sdk/iamkey"
)

type YandexDNSClient interface {
	ListZones(ctx context.Context) ([]Zone, error)
	ListRecordSets(ctx context.Context, zoneID string) ([]RecordSet, error)
	UpsertRecordSets(ctx context.Context, req UpsertRequest) error
}

type RecordSet struct {
	Name string
	Type string
	TTL  int64
	Data []string
}

type UpsertRequest struct {
	DnsZoneID    string
	Deletions    []RecordSet
	Replacements []RecordSet
	Merges       []RecordSet
}

type Zone struct {
	ID        string
	Name      string
	IsPrivate bool
}

type YandexClient struct {
	sdk      *ycsdk.SDK
	folderID string
}

func NewYandexClient(folderID string, authKeyFile string) (*YandexClient, error) {
	if authKeyFile == "" {
		return nil, fmt.Errorf("auth-key-file must be set")
	}

	saBytes, err := os.ReadFile(authKeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read service account key file: %v", err)
	}

	key := &iamkey.Key{}
	if err := key.UnmarshalJSON(saBytes); err != nil {
		return nil, fmt.Errorf("failed to parse service account key: %v", err)
	}

	credentials, err := ycsdk.ServiceAccountKey(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create credentials: %v", err)
	}

	sdk, err := ycsdk.Build(context.Background(), ycsdk.Config{
		Credentials: credentials,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Yandex Cloud SDK: %v", err)
	}

	return &YandexClient{
		sdk:      sdk,
		folderID: folderID,
	}, nil
}

func (c *YandexClient) ListZones(ctx context.Context) ([]Zone, error) {
	var zones []Zone
	pageToken := ""

	for {
		req := &dns.ListDnsZonesRequest{
			FolderId:  c.folderID,
			PageToken: pageToken,
		}

		resp, err := c.sdk.DNS().DnsZone().List(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("failed to list zones: %v", err)
		}

		for _, zone := range resp.DnsZones {
			zones = append(zones, Zone{
				ID:        zone.Id,
				Name:      zone.Zone,
				IsPrivate: zone.PrivateVisibility != nil,
			})
		}

		if resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}

	return zones, nil
}

func (c *YandexClient) ListRecordSets(ctx context.Context, zoneID string) ([]RecordSet, error) {
	req := &dns.ListDnsZoneRecordSetsRequest{
		DnsZoneId: zoneID,
	}

	resp, err := c.sdk.DNS().DnsZone().ListRecordSets(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to list record sets: %v", err)
	}

	recordSets := make([]RecordSet, len(resp.RecordSets))
	for i, recordSet := range resp.RecordSets {
		recordSets[i] = RecordSet{
			Name: recordSet.Name,
			Type: recordSet.Type,
			TTL:  recordSet.Ttl,
			Data: recordSet.Data,
		}
	}

	return recordSets, nil
}

func (c *YandexClient) UpsertRecordSets(ctx context.Context, req UpsertRequest) error {
	var deletions []*dns.RecordSet
	var replacements []*dns.RecordSet
	var merges []*dns.RecordSet

	for _, d := range req.Deletions {
		deletions = append(deletions, &dns.RecordSet{
			Name: d.Name,
			Type: d.Type,
			Ttl:  d.TTL,
			Data: d.Data,
		})
	}

	for _, r := range req.Replacements {
		replacements = append(replacements, &dns.RecordSet{
			Name: r.Name,
			Type: r.Type,
			Ttl:  r.TTL,
			Data: r.Data,
		})
	}

	for _, m := range req.Merges {
		merges = append(merges, &dns.RecordSet{
			Name: m.Name,
			Type: m.Type,
			Ttl:  m.TTL,
			Data: m.Data,
		})
	}

	upsertReq := &dns.UpsertRecordSetsRequest{
		DnsZoneId:    req.DnsZoneID,
		Deletions:    deletions,
		Replacements: replacements,
		Merges:       merges,
	}

	_, err := c.sdk.DNS().DnsZone().UpsertRecordSets(ctx, upsertReq)
	if err != nil {
		return fmt.Errorf("failed to upsert record sets: %v", err)
	}

	return nil
}
