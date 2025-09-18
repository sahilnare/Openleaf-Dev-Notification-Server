package helpers

import (
	"math"
	"time"
)

func DerefStringPointer(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func DerefIntPointer(i *int) int {
	if i == nil {
		return 0
	}
	return *i
}

func DerefFloatPointer(f any) float64 {
	if f == nil {
		return 0.0
	}
	switch v := f.(type) {
	case *float64:
		return *v
	case float64:
		return v
	}
	return 0.0
}

func DerefTimePointer(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}

func FormatDateDDMMYYYYHHMM(t *time.Time) string {
	if t == nil {
		return time.Now().Format("02 Jan 2006 03:04 PM")
	}
	return t.Format("02 Jan 2006 03:04 PM")
}

func FormatDateDDMMYYYY(t *time.Time) string {
	if t == nil {
		return time.Now().Format("02 Jan 2006")
	}
	return t.Format("02 Jan 2006")
}

func RoundFloat(f any) float64 {
	if f == nil {
		return 0.00
	}
	switch v := f.(type) {
	case *float64:
		return math.Round(*v*100) / 100
	case float64:
		return math.Round(float64(v)*100) / 100
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case float32:
		return float64(v)
	case int32:
		return float64(v)
	case int16:
		return float64(v)
	case int8:
		return float64(v)
	case uint:
		return float64(v)
	case uint64:
		return float64(v)
	case uint32:
		return float64(v)
	case uint16:
		return float64(v)
	case uint8:
		return float64(v)
	}
	return 0.00
}

func CmToInch(cm *float64) float64 {
	if cm == nil {
		return 0.00
	}
	return math.Round(*cm * 0.393701 * 100) / 100
}

