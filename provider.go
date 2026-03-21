// Package netnod implements a DNS record management client compatible
// with the libdns interfaces for the Netnod Primary DNS API.
package netnod

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/libdns/libdns"
)

// Provider facilitates DNS record manipulation with the Netnod Primary DNS API.
type Provider struct {
	// APIToken is the authentication token for the Netnod API.
	APIToken string `json:"api_token,omitempty"`

	// BaseURL optionally overrides the default API base URL.
	// Useful for testing.
	BaseURL string `json:"base_url,omitempty"`

	// HTTPClient optionally overrides the default HTTP client.
	HTTPClient *http.Client `json:"-"`

	mu sync.Mutex
}

// GetRecords lists all the records in the zone.
func (p *Provider) GetRecords(ctx context.Context, zone string) ([]libdns.Record, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	zoneID := normalizeZone(zone)
	apiZone, err := p.getZone(ctx, zoneID)
	if err != nil {
		return nil, err
	}

	return fromAPIRRsets(apiZone.RRsets, zone), nil
}

// AppendRecords adds records to the zone. It returns the records that were added.
func (p *Provider) AppendRecords(ctx context.Context, zone string, recs []libdns.Record) ([]libdns.Record, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	zoneID := normalizeZone(zone)
	rrsets := toAPIRRsets(recs, zone, "EXTEND")

	if err := p.patchZone(ctx, zoneID, rrsets); err != nil {
		return nil, err
	}

	return recs, nil
}

// SetRecords sets the records in the zone, fully replacing any existing records
// with the same name and type. It returns the records that were set.
func (p *Provider) SetRecords(ctx context.Context, zone string, recs []libdns.Record) ([]libdns.Record, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	zoneID := normalizeZone(zone)
	rrsets := toAPIRRsets(recs, zone, "REPLACE")

	if err := p.patchZone(ctx, zoneID, rrsets); err != nil {
		return nil, err
	}

	return recs, nil
}

// DeleteRecords deletes the records from the zone. It returns the records that were deleted.
func (p *Provider) DeleteRecords(ctx context.Context, zone string, recs []libdns.Record) ([]libdns.Record, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	zoneID := normalizeZone(zone)
	rrsets := toAPIRRsets(recs, zone, "PRUNE")

	if err := p.patchZone(ctx, zoneID, rrsets); err != nil {
		return nil, err
	}

	return recs, nil
}

// ListZones lists the zones available in the account.
func (p *Provider) ListZones(ctx context.Context) ([]libdns.Zone, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	apiZones, err := p.listAllZones(ctx)
	if err != nil {
		return nil, err
	}

	zones := make([]libdns.Zone, len(apiZones))
	for i, z := range apiZones {
		zones[i] = libdns.Zone{Name: z.Name}
	}
	return zones, nil
}

// normalizeZone ensures the zone name has a trailing dot.
func normalizeZone(zone string) string {
	if zone == "" {
		return zone
	}
	if zone[len(zone)-1] != '.' {
		return zone + "."
	}
	return zone
}

// rrsetKey groups records by name and type.
type rrsetKey struct {
	name string
	typ  string
}

// toAPIRRsets converts libdns records to Netnod API rrsets grouped by (name, type).
func toAPIRRsets(recs []libdns.Record, zone string, changeType string) []apiRRset {
	grouped := make(map[rrsetKey]*apiRRset)
	order := make([]rrsetKey, 0)

	for _, rec := range recs {
		rr := rec.RR()
		fqdn := libdns.AbsoluteName(rr.Name, zone)
		key := rrsetKey{name: fqdn, typ: rr.Type}

		if _, exists := grouped[key]; !exists {
			ttl := int(rr.TTL / time.Second)
			var ttlPtr *int
			if rr.TTL > 0 {
				ttlPtr = &ttl
			}
			grouped[key] = &apiRRset{
				Name:       fqdn,
				Type:       rr.Type,
				TTL:        ttlPtr,
				ChangeType: changeType,
			}
			order = append(order, key)
		}

		if rr.Data != "" {
			content := rr.Data
			if rr.Type == "TXT" {
				content = quoteTXT(content)
			}
			grouped[key].Records = append(grouped[key].Records, apiRecord{
				Content: content,
			})
		}
	}

	result := make([]apiRRset, 0, len(grouped))
	for _, key := range order {
		result = append(result, *grouped[key])
	}
	return result
}

// fromAPIRRsets converts Netnod API rrsets to libdns records.
func fromAPIRRsets(rrsets []apiRRset, zone string) []libdns.Record {
	var records []libdns.Record

	for _, rrset := range rrsets {
		relName := libdns.RelativeName(rrset.Name, zone)

		var ttl time.Duration
		if rrset.TTL != nil {
			ttl = time.Duration(*rrset.TTL) * time.Second
		}

		for _, rec := range rrset.Records {
			content := rec.Content
			if rrset.Type == "TXT" {
				content = unquoteTXT(content)
			}
			rr := libdns.RR{
				Name: relName,
				TTL:  ttl,
				Type: rrset.Type,
				Data: content,
			}

			parsed, err := rr.Parse()
			if err != nil {
				records = append(records, rr)
				continue
			}
			records = append(records, parsed)
		}
	}

	return records
}

// quoteTXT wraps a TXT value in quotes as required by the Netnod API (PowerDNS format).
func quoteTXT(s string) string {
	return `"` + s + `"`
}

// unquoteTXT strips surrounding quotes from a TXT value returned by the Netnod API.
func unquoteTXT(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

// Interface guards
var (
	_ libdns.RecordGetter   = (*Provider)(nil)
	_ libdns.RecordAppender = (*Provider)(nil)
	_ libdns.RecordSetter   = (*Provider)(nil)
	_ libdns.RecordDeleter  = (*Provider)(nil)
	_ libdns.ZoneLister     = (*Provider)(nil)
)
