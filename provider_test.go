package netnod

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/libdns/libdns"
)

var (
	envToken = ""
	envZone  = ""
)

func setupTest(t *testing.T) (*Provider, string) {
	t.Helper()

	envToken = os.Getenv("NETNOD_API_TOKEN")
	envZone = os.Getenv("NETNOD_TEST_ZONE")

	if envToken == "" || envZone == "" {
		t.Skip("NETNOD_API_TOKEN and NETNOD_TEST_ZONE must be set to run integration tests")
	}

	provider := &Provider{
		APIToken: envToken,
	}

	return provider, envZone
}

func TestIntegration(t *testing.T) {
	provider, zone := setupTest(t)
	ctx := context.Background()

	testName := fmt.Sprintf("_libdns-test-%d", time.Now().UnixNano())

	// ListZones
	t.Run("ListZones", func(t *testing.T) {
		zones, err := provider.ListZones(ctx)
		if err != nil {
			t.Fatalf("ListZones: %v", err)
		}

		found := false
		for _, z := range zones {
			if z.Name == zone {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("test zone %q not found in zone list", zone)
		}
	})

	// AppendRecords
	t.Run("AppendRecords", func(t *testing.T) {
		recs := []libdns.Record{
			libdns.TXT{
				Name: testName,
				TTL:  60 * time.Second,
				Text: "libdns-test-value",
			},
		}

		result, err := provider.AppendRecords(ctx, zone, recs)
		if err != nil {
			t.Fatalf("AppendRecords: %v", err)
		}
		if len(result) != 1 {
			t.Fatalf("expected 1 record, got %d", len(result))
		}
	})

	// GetRecords — verify append
	t.Run("GetRecords_after_append", func(t *testing.T) {
		recs, err := provider.GetRecords(ctx, zone)
		if err != nil {
			t.Fatalf("GetRecords: %v", err)
		}

		found := false
		for _, rec := range recs {
			rr := rec.RR()
			if rr.Name == testName && rr.Type == "TXT" && rr.Data == "libdns-test-value" {
				found = true
				break
			}
		}
		if !found {
			t.Fatal("appended TXT record not found")
		}
	})

	// SetRecords — update value
	t.Run("SetRecords", func(t *testing.T) {
		recs := []libdns.Record{
			libdns.TXT{
				Name: testName,
				TTL:  120 * time.Second,
				Text: "libdns-test-updated",
			},
		}

		result, err := provider.SetRecords(ctx, zone, recs)
		if err != nil {
			t.Fatalf("SetRecords: %v", err)
		}
		if len(result) != 1 {
			t.Fatalf("expected 1 record, got %d", len(result))
		}
	})

	// GetRecords — verify set
	t.Run("GetRecords_after_set", func(t *testing.T) {
		recs, err := provider.GetRecords(ctx, zone)
		if err != nil {
			t.Fatalf("GetRecords: %v", err)
		}

		found := false
		for _, rec := range recs {
			rr := rec.RR()
			if rr.Name == testName && rr.Type == "TXT" && rr.Data == "libdns-test-updated" {
				found = true
				break
			}
		}
		if !found {
			t.Fatal("updated TXT record not found")
		}

		// Verify old value is gone
		for _, rec := range recs {
			rr := rec.RR()
			if rr.Name == testName && rr.Type == "TXT" && rr.Data == "libdns-test-value" {
				t.Fatal("old TXT record value should have been replaced")
			}
		}
	})

	// DeleteRecords
	t.Run("DeleteRecords", func(t *testing.T) {
		recs := []libdns.Record{
			libdns.TXT{
				Name: testName,
				TTL:  120 * time.Second,
				Text: "libdns-test-updated",
			},
		}

		result, err := provider.DeleteRecords(ctx, zone, recs)
		if err != nil {
			t.Fatalf("DeleteRecords: %v", err)
		}
		if len(result) != 1 {
			t.Fatalf("expected 1 record, got %d", len(result))
		}
	})

	// GetRecords — verify delete
	t.Run("GetRecords_after_delete", func(t *testing.T) {
		recs, err := provider.GetRecords(ctx, zone)
		if err != nil {
			t.Fatalf("GetRecords: %v", err)
		}

		for _, rec := range recs {
			rr := rec.RR()
			if rr.Name == testName && rr.Type == "TXT" {
				t.Fatal("deleted TXT record should not be present")
			}
		}
	})
}
