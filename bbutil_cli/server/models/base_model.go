package models

import (
	"database/sql"
)

type Base struct {
	ID         uint32        `gorm:"primaryKey"`
	Status     sql.NullInt64 `gorm:"comment:1 or 0"`
	CreatorId  sql.NullInt64
	ModifierId sql.NullInt64
	CreateTime sql.NullTime
	ModifyTime sql.NullTime
}
