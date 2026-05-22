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
		if v == nil {
			return 0.0
		}
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

// FormatDateBlankIfNil renders an IST date (02 Jan 2006), empty when nil.
func FormatDateBlankIfNil(t *time.Time) string {
	if t == nil {
		return ""
	}
	istLocation, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		return t.Add(5*time.Hour + 30*time.Minute).Format("02 Jan 2006")
	}
	return t.In(istLocation).Format("02 Jan 2006")
}

// FormatTimeBlankIfNil renders an IST time (03:04 PM), empty when nil.
func FormatTimeBlankIfNil(t *time.Time) string {
	if t == nil {
		return ""
	}
	istLocation, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		return t.Add(5*time.Hour + 30*time.Minute).Format("03:04 PM")
	}
	return t.In(istLocation).Format("03:04 PM")
}

func RoundFloat(f any, precision int) float64 {
	if f == nil {
		return 0.0
	}
	switch v := f.(type) {
	case *float64:
		if v == nil {
			return 0.0
		}
		return math.Round(*v*float64(precision)) / float64(precision)
	case float64:
		return math.Round(float64(v)*float64(precision)) / float64(precision)
	case int:
		return math.Round(float64(v)*float64(precision)) / float64(precision)
	case int64:
		return math.Round(float64(v)*float64(precision)) / float64(precision)
	case float32:
		return math.Round(float64(v)*float64(precision)) / float64(precision)
	case int32:
		return math.Round(float64(v)*float64(precision)) / float64(precision)
	case int16:
		return math.Round(float64(v)*float64(precision)) / float64(precision)
	case int8:
		return math.Round(float64(v)*float64(precision)) / float64(precision)
	case uint:
		return math.Round(float64(v)*float64(precision)) / float64(precision)
	case uint64:
		return float64(v)
	case uint32:
		return math.Round(float64(v)*float64(precision)) / float64(precision)
	case uint16:
		return math.Round(float64(v)*float64(precision)) / float64(precision)
	case uint8:
		return math.Round(float64(v)*float64(precision)) / float64(precision)
	}
	return 0.0
}

func CmToInch(cm *float64) float64 {
	if cm == nil {
		return 0.00
	}
	return math.Round(*cm*0.393701*100) / 100
}

// AdjustNotificationTimeToSkipSunday adjusts the notification time to skip Sundays
// If the sendAt time falls on Sunday, it moves it back to Saturday
// This ensures notifications for Sunday and Monday appointments are both sent on Saturday
func AdjustNotificationTimeToSkipSunday(sendAt time.Time, skipSunday bool) time.Time {
	// Check if the send time falls on Sunday
	if skipSunday && sendAt.Weekday() == time.Sunday {
		// Move back one day to Saturday
		sendAt = sendAt.AddDate(0, 0, -1)
		LogInfo("Notification time adjusted to skip Sunday", map[string]interface{}{
			"original_day":  "Sunday",
			"adjusted_day":  "Saturday",
			"adjusted_time": sendAt.Format("02 Jan 2006 03:04 PM"),
		})
	}
	return sendAt
}
