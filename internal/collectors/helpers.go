package collectors

import (
	"errors"
	"fmt"
	"time"
	
	"lead-scout/internal/core"
)

func stringField(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return fmt.Sprintf("%.0f", x)
	case int:
		return fmt.Sprint(x)
	case int64:
		return fmt.Sprint(x)
	default:
		return fmt.Sprint(x)
	}
}

func firstString(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func unwrapList(payload any) []any {
	switch x := payload.(type) {
	case []any:
		return x
	case map[string]any:
		for _, key := range []string{"jobs", "data", "results"} {
			if rows, ok := x[key].([]any); ok {
				return rows
			}
		}
	}
	return nil
}

func errStatus(status string) error {
	if status == "" {
		return errors.New("unexpected http status")
	}
	return errors.New(status)
}

func filterByCurrentMonth(items []core.RawItem) []core.RawItem {
	now := time.Now()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	
	filtered := make([]core.RawItem, 0, len(items))
	for _, item := range items {
		if item.PublishedAt == nil || item.PublishedAt.After(startOfMonth) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}
