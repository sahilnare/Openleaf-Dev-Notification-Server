package helpers

import "time"

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

func DerefFloatPointer(f *float64) float64 {
	if f == nil {
		return 0.0
	}
	return *f
}

func DerefTimePointer(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}

func FormatDateDDMMYYYYHHMM(t *time.Time) string {
	if t == nil {
		return time.Now().Format("02 Jan 2006 15:04")
	}
	return t.Format("02 Jan 2006 15:04")
}

func FormatDateDDMMYYYY(t *time.Time) string {
	if t == nil {
		return time.Now().Format("02 Jan 2006")
	}
	return t.Format("02 Jan 2006")
}
