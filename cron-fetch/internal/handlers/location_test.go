package handlers

import (
	"testing"

	"github.com/kv13b/Crontfetch/internal/models"
)

func TestFilterNearby(t *testing.T) {
	ptr := func(f float64) *float64 { return &f }
	companies := []models.Company{
		{Name: "far", Lat: ptr(28.6139), Lng: ptr(77.2090)},    // Delhi
		{Name: "close", Lat: ptr(12.9756), Lng: ptr(77.6066)},  // MG Road, ~1.4 km
		{Name: "medium", Lat: ptr(12.9352), Lng: ptr(77.6245)}, // Koramangala, ~5 km
		{Name: "no location"},
	}

	got := filterNearby(companies, 12.9716, 77.5946, 10)

	if len(got) != 2 || got[0].Name != "close" || got[1].Name != "medium" {
		names := []string{}
		for _, c := range got {
			names = append(names, c.Name)
		}
		t.Fatalf("filterNearby() = %v, want [close medium]", names)
	}
	if got[0].DistanceKm <= 0 || got[0].DistanceKm >= got[1].DistanceKm {
		t.Errorf("distances not sorted ascending: %.2f, %.2f", got[0].DistanceKm, got[1].DistanceKm)
	}
}
