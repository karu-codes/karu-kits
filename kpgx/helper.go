package kpgx

import (
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func ToUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{
		Valid: id != uuid.Nil,
		Bytes: id,
	}
}

func ToUUIDPtr(id *uuid.UUID) pgtype.UUID {
	if id == nil {
		return pgtype.UUID{Valid: false}
	}
	return ToUUID(*id)
}

func ToTimestamp(t time.Time) pgtype.Timestamp {
	return pgtype.Timestamp{
		Valid: !t.IsZero(),
		Time:  t,
	}
}

func ToTimestampPtr(t *time.Time) pgtype.Timestamp {
	if t == nil {
		return pgtype.Timestamp{Valid: false}
	}
	return ToTimestamp(*t)
}

func ToDate(t time.Time) pgtype.Date {
	return pgtype.Date{
		Valid: !t.IsZero(),
		Time:  t,
	}
}

func ToDatePtr(t *time.Time) pgtype.Date {
	if t == nil {
		return pgtype.Date{Valid: false}
	}
	return ToDate(*t)
}

func ToText(s string) pgtype.Text {
	return pgtype.Text{
		Valid:  true,
		String: s,
	}
}

func ToTextPtr(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{Valid: false}
	}
	return ToText(*s)
}

func ToInt4(i int32) pgtype.Int4 {
	return pgtype.Int4{
		Valid: true,
		Int32: i,
	}
}

func ToInt4Ptr(i *int32) pgtype.Int4 {
	if i == nil {
		return pgtype.Int4{Valid: false}
	}
	return ToInt4(*i)
}

func ToInt8(i int64) pgtype.Int8 {
	return pgtype.Int8{
		Valid: true,
		Int64: i,
	}
}

func ToInt8Ptr(i *int64) pgtype.Int8 {
	if i == nil {
		return pgtype.Int8{Valid: false}
	}
	return ToInt8(*i)
}

func ToBool(b bool) pgtype.Bool {
	return pgtype.Bool{
		Valid: true,
		Bool:  b,
	}
}

func ToBoolPtr(b *bool) pgtype.Bool {
	if b == nil {
		return pgtype.Bool{Valid: false}
	}
	return ToBool(*b)
}

func ToFloat8(f float64) pgtype.Float8 {
	return pgtype.Float8{
		Valid:   true,
		Float64: f,
	}
}

func ToFloat8Ptr(f *float64) pgtype.Float8 {
	if f == nil {
		return pgtype.Float8{Valid: false}
	}
	return ToFloat8(*f)
}

func ToTimestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{
		Valid: !t.IsZero(),
		Time:  t,
	}
}

func ToTimestamptzPtr(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{Valid: false}
	}
	return ToTimestamptz(*t)
}
