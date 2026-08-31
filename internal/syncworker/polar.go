package syncworker

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

const polarBaseURL = "https://www.polaraccesslink.com/v3"

// BuildActivityURL constructs the Polar daily activity endpoint URL.
func BuildActivityURL(date string) string {
	return fmt.Sprintf("%s/users/activities?date=%s", polarBaseURL, url.QueryEscape(date))
}

// ActivitySummary holds parsed active calorie data from Polar.
type ActivitySummary struct {
	ActiveCalories float64 `json:"active-calories"`
	Date           string  `json:"date"`
}

// ParseActivityResponse extracts active calories from Polar API JSON.
// Polar returns an array of activity summaries for the requested date.
func ParseActivityResponse(body []byte) (float64, error) {
	body = bytesTrimSpace(body)
	if len(body) == 0 {
		return 0, fmt.Errorf("empty polar response")
	}

	var summaries []ActivitySummary
	if err := json.Unmarshal(body, &summaries); err == nil {
		return sumActiveCalories(summaries), nil
	}

	var single ActivitySummary
	if err := json.Unmarshal(body, &single); err == nil && single.ActiveCalories > 0 {
		return single.ActiveCalories, nil
	}

	var wrapped struct {
		ActiveCalories float64 `json:"active_calories"`
	}
	if err := json.Unmarshal(body, &wrapped); err == nil && wrapped.ActiveCalories > 0 {
		return wrapped.ActiveCalories, nil
	}

	return 0, fmt.Errorf("unable to parse polar activity response")
}

func sumActiveCalories(summaries []ActivitySummary) float64 {
	var total float64
	for _, s := range summaries {
		total += s.ActiveCalories
	}
	return total
}

func bytesTrimSpace(b []byte) []byte {
	return []byte(strings.TrimSpace(string(b)))
}
