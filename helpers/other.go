package helpers

import (
	"math"
	"time"
)

// GetISTTime returns the current time in IST timezone
func GetISTTime() time.Time {
	istLocation, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		// Fallback to UTC if IST cannot be loaded
		return time.Now().UTC().Add(5*time.Hour + 30*time.Minute)
	}
	return time.Now().In(istLocation)
}

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
		return GetISTTime().Format("02 Jan 2006 03:04 PM")
	}

	istLocation, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		// Fallback: approximate IST by adding 5.5 hours to the given time
		return t.Add(5*time.Hour + 30*time.Minute).Format("02 Jan 2006 03:04 PM")
	}

	return t.In(istLocation).Format("02 Jan 2006 03:04 PM")
}

func FormatDateDDMMYYYY(t *time.Time) string {
	if t == nil {
		return GetISTTime().Format("02 Jan 2006")
	}

	istLocation, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		// Fallback: approximate IST by adding 5.5 hours to the given time
		return t.Add(5*time.Hour + 30*time.Minute).Format("02 Jan 2006")
	}

	return t.In(istLocation).Format("02 Jan 2006")
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
	return math.Round(*cm*0.393701*100) / 100
}
