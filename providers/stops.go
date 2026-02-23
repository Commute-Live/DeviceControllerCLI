package providers

import "math/rand"

// Stop represents a single transit stop assignment.
type Stop struct {
    Provider  string
    ProviderID string
    Line      string
    StopID    string
    Direction string
}

// curated stop lists per provider
var stopsByProvider = map[string][]Stop{
    "mta": {
        {Provider: "mta", ProviderID: "mta-subway", Line: "A", StopID: "A19N", Direction: ""},
        {Provider: "mta", ProviderID: "mta-subway", Line: "A", StopID: "A19S", Direction: ""},
        {Provider: "mta", ProviderID: "mta-subway", Line: "L", StopID: "L03N", Direction: ""},
        {Provider: "mta", ProviderID: "mta-subway", Line: "L", StopID: "L03S", Direction: ""},
        {Provider: "mta", ProviderID: "mta-subway", Line: "1", StopID: "127N", Direction: ""},
        {Provider: "mta", ProviderID: "mta-subway", Line: "1", StopID: "127S", Direction: ""},
        {Provider: "mta", ProviderID: "mta-subway", Line: "N", StopID: "N02N", Direction: ""},
        {Provider: "mta", ProviderID: "mta-subway", Line: "N", StopID: "N02S", Direction: ""},
    },
    "cta": {
        {Provider: "cta", ProviderID: "cta-subway", Line: "Red", StopID: "40900", Direction: "N"},
        {Provider: "cta", ProviderID: "cta-subway", Line: "Red", StopID: "40900", Direction: "S"},
        {Provider: "cta", ProviderID: "cta-subway", Line: "Blue", StopID: "40380", Direction: "S"},
        {Provider: "cta", ProviderID: "cta-subway", Line: "Blue", StopID: "40380", Direction: "N"},
        {Provider: "cta", ProviderID: "cta-subway", Line: "Brn", StopID: "40730", Direction: "N"},
        {Provider: "cta", ProviderID: "cta-subway", Line: "Brn", StopID: "40730", Direction: "S"},
        {Provider: "cta", ProviderID: "cta-subway", Line: "G", StopID: "40280", Direction: "S"},
        {Provider: "cta", ProviderID: "cta-subway", Line: "G", StopID: "40280", Direction: "N"},
    },
    "mbta": {
        {Provider: "mbta", ProviderID: "mbta", Line: "Red", StopID: "place-pktrm", Direction: "1"},
        {Provider: "mbta", ProviderID: "mbta", Line: "Red", StopID: "place-pktrm", Direction: "0"},
        {Provider: "mbta", ProviderID: "mbta", Line: "Orange", StopID: "place-dwnxg", Direction: "0"},
        {Provider: "mbta", ProviderID: "mbta", Line: "Orange", StopID: "place-dwnxg", Direction: "1"},
        {Provider: "mbta", ProviderID: "mbta", Line: "Green-B", StopID: "place-kenmore", Direction: "0"},
        {Provider: "mbta", ProviderID: "mbta", Line: "Green-B", StopID: "place-kenmore", Direction: "1"},
    },
    "septa": {
        {Provider: "septa", ProviderID: "septa-rail", Line: "NHSL", StopID: "PENN_CENTER", Direction: "N"},
        {Provider: "septa", ProviderID: "septa-rail", Line: "NHSL", StopID: "PENN_CENTER", Direction: "S"},
        {Provider: "septa", ProviderID: "septa-rail", Line: "PAOLI", StopID: "30TH_STREET", Direction: "E"},
        {Provider: "septa", ProviderID: "septa-rail", Line: "PAOLI", StopID: "30TH_STREET", Direction: "W"},
    },
}

// PickStop returns a random stop for the given provider key (e.g. "cta", "mta").
func PickStop(provider string) (Stop, bool) {
    stops, ok := stopsByProvider[provider]
    if !ok || len(stops) == 0 {
        return Stop{}, false
    }
    return stops[rand.Intn(len(stops))], true
}

// ValidProviders returns the list of known provider keys.
func ValidProviders() []string {
    keys := make([]string, 0, len(stopsByProvider))
    for k := range stopsByProvider {
        keys = append(keys, k)
    }
    return keys
}

// AssignProviders returns a slice of provider keys of length n, distributed
// according to the given percentage map (must sum to 100).
func AssignProviders(n int, dist map[string]int) []string {
    result := make([]string, 0, n)
    // Build a weighted list
    var weighted []string
    for provider, pct := range dist {
        count := (pct * n) / 100
        for i := 0; i < count; i++ {
            weighted = append(weighted, provider)
        }
    }
    // Fill any remainder due to integer division
    for len(weighted) < n {
        for provider := range dist {
            weighted = append(weighted, provider)
            if len(weighted) >= n {
                break
            }
        }
    }
    // Shuffle and trim
    rand.Shuffle(len(weighted), func(i, j int) { weighted[i], weighted[j] = weighted[j], weighted[i] })
    result = append(result, weighted[:n]...)
    return result
}
