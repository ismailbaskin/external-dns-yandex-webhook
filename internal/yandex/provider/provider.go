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
	"strings"

	log "github.com/sirupsen/logrus"

	"external-dns-yandex-webhook/internal/yandex/client"

	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/plan"
	"sigs.k8s.io/external-dns/provider"
)

// yandex provider type
type yandexProvider struct {
	provider.BaseProvider
	client client.YandexDNSClient

	// only consider hosted zones managing domains ending in this suffix
	domainFilter endpoint.DomainFilter
	dryRun       bool
}

// NewYandexProvider initializes a new Yandex Cloud DNS based Provider
func NewYandexProvider(domainFilter endpoint.DomainFilter, dryRun bool, client client.YandexDNSClient) provider.Provider {
	return &yandexProvider{
		client:       client,
		domainFilter: domainFilter,
		dryRun:       dryRun,
	}
}

// normalizeDNSName converts a DNS name to a canonical form, so that we can use string equality
// it: removes space, converts to lower case, ensures there is a trailing dot
func normalizeDNSName(dnsName string) string {
	s := strings.TrimSpace(strings.ToLower(dnsName))
	if !strings.HasSuffix(s, ".") {
		s += "."
	}
	return s
}

// getZones returns a map of zone name to zone ID
func (p yandexProvider) getZones(ctx context.Context) (map[string]string, error) {
	result := make(map[string]string)

	zones, err := p.client.ListZones(ctx)
	if err != nil {
		return nil, err
	}

	for _, zone := range zones {
		if !p.domainFilter.Match(zone.Name) {
			continue
		}
		result[zone.Name] = zone.ID
	}

	return result, nil
}

func (p yandexProvider) getHostZoneID(hostname string, managedZones map[string]string) (string, error) {
	longestZoneLength := 0
	resultID := ""

	hostname = normalizeDNSName(hostname)

	for zoneName, zoneID := range managedZones {
		normalizedZoneName := normalizeDNSName(zoneName)
		if strings.HasSuffix(hostname, normalizedZoneName) {
			if len(zoneName) > longestZoneLength {
				longestZoneLength = len(zoneName)
				resultID = zoneID
			}
		}
	}

	if resultID == "" {
		return "", fmt.Errorf("no matching zone found for %s", hostname)
	}

	return resultID, nil
}

func (p yandexProvider) Records(ctx context.Context) ([]*endpoint.Endpoint, error) {
	zones, err := p.getZones(ctx)
	if err != nil {
		return nil, err
	}

	var endpoints []*endpoint.Endpoint

	for zoneName, zoneID := range zones {
		recordSets, err := p.client.ListRecordSets(ctx, zoneID)
		if err != nil {
			return nil, err
		}

		for _, recordSet := range recordSets {
			if recordSet.Type == "SOA" || recordSet.Type == "NS" {
				continue
			}

			name := recordSet.Name
			if !strings.HasSuffix(name, zoneName) {
				name = name + "." + zoneName
			}

			name = normalizeDNSName(name)

			ep := endpoint.NewEndpointWithTTL(name, recordSet.Type, endpoint.TTL(recordSet.TTL), recordSet.Data...)

			endpoints = append(endpoints, ep)
		}
	}

	return endpoints, nil
}

type recordSet struct {
	dnsName     string
	recordType  string
	zoneID      string
	recordSetID string
	names       map[string]bool
}

func addEndpoint(ep *endpoint.Endpoint, recordSets map[string]*recordSet, oldEndpoints []*endpoint.Endpoint, delete bool) {
	key := fmt.Sprintf("%s-%s", ep.DNSName, ep.RecordType)
	if _, ok := recordSets[key]; !ok {
		recordSets[key] = &recordSet{
			dnsName:    ep.DNSName,
			recordType: ep.RecordType,
			names:      make(map[string]bool),
		}
	}

	rs := recordSets[key]

	if !delete {
		for _, target := range ep.Targets {
			rs.names[target] = true
		}
	}

}

func (p yandexProvider) ApplyChanges(ctx context.Context, changes *plan.Changes) error {
	zones, err := p.getZones(ctx)
	if err != nil {
		return err
	}

	recordSets := make(map[string]*recordSet)

	for _, ep := range changes.Create {
		addEndpoint(ep, recordSets, nil, false)
	}

	oldEndpoints, err := p.Records(ctx)
	if err != nil {
		return err
	}

	for _, ep := range changes.UpdateNew {
		addEndpoint(ep, recordSets, oldEndpoints, false)
	}

	for _, ep := range changes.Delete {
		addEndpoint(ep, recordSets, oldEndpoints, true)
	}

	for _, rs := range recordSets {
		err := p.upsertRecordSet(ctx, rs, zones)
		if err != nil {
			return err
		}
	}

	return nil
}

func (p yandexProvider) upsertRecordSet(ctx context.Context, rs *recordSet, managedZones map[string]string) error {
	if len(rs.names) == 0 {
		if p.dryRun {
			log.Infof("Would delete record set %s in zone %s", rs.dnsName, rs.zoneID)
			return nil
		}

		req := client.UpsertRequest{
			DnsZoneID: rs.zoneID,
			Deletions: []client.RecordSet{{
				Name: normalizeDNSName(rs.dnsName),
				Type: rs.recordType,
				TTL:  300,
			}},
		}

		return p.client.UpsertRecordSets(ctx, req)
	}

	if rs.zoneID == "" {
		var err error
		rs.zoneID, err = p.getHostZoneID(rs.dnsName, managedZones)
		if err != nil {
			return err
		}
	}

	var records []string
	for name := range rs.names {
		records = append(records, name)
	}

	if p.dryRun {
		operation := "update"
		if rs.recordSetID == "" {
			operation = "create"
		}
		log.Infof("Would %s record set %s with type %s in zone %s",
			operation,
			rs.dnsName,
			rs.recordType,
			rs.zoneID)
		return nil
	}

	req := client.UpsertRequest{
		DnsZoneID: rs.zoneID,
		Replacements: []client.RecordSet{{
			Name: normalizeDNSName(rs.dnsName),
			Type: rs.recordType,
			TTL:  300,
			Data: records,
		}},
	}

	return p.client.UpsertRecordSets(ctx, req)
}
