package kpgx

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestToUUID(t *testing.T) {
	id := uuid.New()
	pgUUID := ToUUID(id)
	if !pgUUID.Valid {
		t.Error("Expected valid UUID")
	}
	if pgUUID.Bytes != id {
		t.Errorf("Expected UUID bytes %v, got %v", id, pgUUID.Bytes)
	}

	nilUUID := ToUUID(uuid.Nil)
	if nilUUID.Valid {
		t.Error("Expected invalid UUID for nil UUID")
	}
}

func TestToUUIDPtr(t *testing.T) {
	id := uuid.New()
	pgUUID := ToUUIDPtr(&id)
	if !pgUUID.Valid {
		t.Error("Expected valid UUID")
	}
	if pgUUID.Bytes != id {
		t.Errorf("Expected UUID bytes %v, got %v", id, pgUUID.Bytes)
	}

	pgUUIDNil := ToUUIDPtr(nil)
	if pgUUIDNil.Valid {
		t.Error("Expected invalid UUID for nil pointer")
	}
}

func TestToTimestamp(t *testing.T) {
	now := time.Now()
	pgTs := ToTimestamp(now)
	if !pgTs.Valid {
		t.Error("Expected valid timestamp")
	}
	if !pgTs.Time.Equal(now) {
		t.Errorf("Expected timestamp %v, got %v", now, pgTs.Time)
	}

	zeroTs := ToTimestamp(time.Time{})
	if zeroTs.Valid {
		t.Error("Expected invalid timestamp for zero time")
	}
}

func TestToTimestampPtr(t *testing.T) {
	now := time.Now()
	pgTs := ToTimestampPtr(&now)
	if !pgTs.Valid {
		t.Error("Expected valid timestamp")
	}
	if !pgTs.Time.Equal(now) {
		t.Errorf("Expected timestamp %v, got %v", now, pgTs.Time)
	}

	pgTsNil := ToTimestampPtr(nil)
	if pgTsNil.Valid {
		t.Error("Expected invalid timestamp for nil pointer")
	}
}

func TestToDate(t *testing.T) {
	now := time.Now()
	pgDate := ToDate(now)
	if !pgDate.Valid {
		t.Error("Expected valid date")
	}
	if !pgDate.Time.Equal(now) {
		t.Errorf("Expected date %v, got %v", now, pgDate.Time)
	}

	zeroDate := ToDate(time.Time{})
	if zeroDate.Valid {
		t.Error("Expected invalid date for zero time")
	}
}

func TestToDatePtr(t *testing.T) {
	now := time.Now()
	pgDate := ToDatePtr(&now)
	if !pgDate.Valid {
		t.Error("Expected valid date")
	}
	if !pgDate.Time.Equal(now) {
		t.Errorf("Expected date %v, got %v", now, pgDate.Time)
	}

	pgDateNil := ToDatePtr(nil)
	if pgDateNil.Valid {
		t.Error("Expected invalid date for nil pointer")
	}
}

func TestToText(t *testing.T) {
	s := "hello"
	pgText := ToText(s)
	if !pgText.Valid {
		t.Error("Expected valid text")
	}
	if pgText.String != s {
		t.Errorf("Expected text %v, got %v", s, pgText.String)
	}
}

func TestToTextPtr(t *testing.T) {
	s := "hello"
	pgText := ToTextPtr(&s)
	if !pgText.Valid {
		t.Error("Expected valid text")
	}
	if pgText.String != s {
		t.Errorf("Expected text %v, got %v", s, pgText.String)
	}

	pgTextNil := ToTextPtr(nil)
	if pgTextNil.Valid {
		t.Error("Expected invalid text for nil pointer")
	}
}

func TestToInt4(t *testing.T) {
	i := int32(123)
	pgInt4 := ToInt4(i)
	if !pgInt4.Valid {
		t.Error("Expected valid int4")
	}
	if pgInt4.Int32 != i {
		t.Errorf("Expected int4 %v, got %v", i, pgInt4.Int32)
	}
}

func TestToInt4Ptr(t *testing.T) {
	i := int32(123)
	pgInt4 := ToInt4Ptr(&i)
	if !pgInt4.Valid {
		t.Error("Expected valid int4")
	}
	if pgInt4.Int32 != i {
		t.Errorf("Expected int4 %v, got %v", i, pgInt4.Int32)
	}

	pgInt4Nil := ToInt4Ptr(nil)
	if pgInt4Nil.Valid {
		t.Error("Expected invalid int4 for nil pointer")
	}
}

func TestToInt8(t *testing.T) {
	i := int64(123)
	pgInt8 := ToInt8(i)
	if !pgInt8.Valid {
		t.Error("Expected valid int8")
	}
	if pgInt8.Int64 != i {
		t.Errorf("Expected int8 %v, got %v", i, pgInt8.Int64)
	}
}

func TestToInt8Ptr(t *testing.T) {
	i := int64(123)
	pgInt8 := ToInt8Ptr(&i)
	if !pgInt8.Valid {
		t.Error("Expected valid int8")
	}
	if pgInt8.Int64 != i {
		t.Errorf("Expected int8 %v, got %v", i, pgInt8.Int64)
	}

	pgInt8Nil := ToInt8Ptr(nil)
	if pgInt8Nil.Valid {
		t.Error("Expected invalid int8 for nil pointer")
	}
}

func TestToBool(t *testing.T) {
	b := true
	pgBool := ToBool(b)
	if !pgBool.Valid {
		t.Error("Expected valid bool")
	}
	if pgBool.Bool != b {
		t.Errorf("Expected bool %v, got %v", b, pgBool.Bool)
	}
}

func TestToBoolPtr(t *testing.T) {
	b := true
	pgBool := ToBoolPtr(&b)
	if !pgBool.Valid {
		t.Error("Expected valid bool")
	}
	if pgBool.Bool != b {
		t.Errorf("Expected bool %v, got %v", b, pgBool.Bool)
	}

	pgBoolNil := ToBoolPtr(nil)
	if pgBoolNil.Valid {
		t.Error("Expected invalid bool for nil pointer")
	}
}

func TestToFloat8(t *testing.T) {
	f := 123.456
	pgFloat8 := ToFloat8(f)
	if !pgFloat8.Valid {
		t.Error("Expected valid float8")
	}
	if pgFloat8.Float64 != f {
		t.Errorf("Expected float8 %v, got %v", f, pgFloat8.Float64)
	}
}

func TestToFloat8Ptr(t *testing.T) {
	f := 123.456
	pgFloat8 := ToFloat8Ptr(&f)
	if !pgFloat8.Valid {
		t.Error("Expected valid float8")
	}
	if pgFloat8.Float64 != f {
		t.Errorf("Expected float8 %v, got %v", f, pgFloat8.Float64)
	}

	pgFloat8Nil := ToFloat8Ptr(nil)
	if pgFloat8Nil.Valid {
		t.Error("Expected invalid float8 for nil pointer")
	}
}

func TestToTimestamptz(t *testing.T) {
	now := time.Now()
	pgTsz := ToTimestamptz(now)
	if !pgTsz.Valid {
		t.Error("Expected valid timestamptz")
	}
	if !pgTsz.Time.Equal(now) {
		t.Errorf("Expected timestamptz %v, got %v", now, pgTsz.Time)
	}

	zeroTsz := ToTimestamptz(time.Time{})
	if zeroTsz.Valid {
		t.Error("Expected invalid timestamptz for zero time")
	}
}

func TestToTimestamptzPtr(t *testing.T) {
	now := time.Now()
	pgTsz := ToTimestamptzPtr(&now)
	if !pgTsz.Valid {
		t.Error("Expected valid timestamptz")
	}
	if !pgTsz.Time.Equal(now) {
		t.Errorf("Expected timestamptz %v, got %v", now, pgTsz.Time)
	}

	pgTszNil := ToTimestamptzPtr(nil)
	if pgTszNil.Valid {
		t.Error("Expected invalid timestamptz for nil pointer")
	}
}
