package vos

import "time"

type NullableTime struct {
	Time  *time.Time
	Valid bool
}

func NewNullableTime(t time.Time) NullableTime {
	return NullableTime{Time: &t, Valid: true}
}
