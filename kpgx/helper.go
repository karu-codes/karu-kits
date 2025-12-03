package kpgx

import (
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func UUIDToPgTypeUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{
		Valid: true,
		Bytes: id,
	}
}

func TimeToPgTypeTimestamp(t time.Time) pgtype.Timestamp {
	return pgtype.Timestamp{
		Valid: !t.IsZero(),
		Time:  t,
	}
}
