package migrations

import (
	"github.com/BOPR/types"
	"github.com/jinzhu/gorm"
)

func init() {
	m := &Migration{
		ID: "1582859979",
		Up: func(db *gorm.DB) error {
			if !db.HasTable(&types.Tx{}) {
				db.CreateTable(&types.Tx{})
			}
			if !db.HasTable(&types.UserAccount{}) {
				db.CreateTable(&types.UserAccount{})
			}
			if !db.HasTable(&types.Batch{}) {
				db.CreateTable(&types.Batch{})
			}
			if !db.HasTable(&types.Params{}) {
				db.CreateTable(&types.Params{})
			}
			if !db.HasTable(&types.ListenerLog{}) {
				db.CreateTable(&types.ListenerLog{})
			}
			if !db.HasTable(&types.Token{}) {
				db.CreateTable(&types.Token{})
			}

			return nil
		},
		Down: func(db *gorm.DB) error {
			db.DropTableIfExists(&types.Tx{})
			db.DropTableIfExists(&types.Batch{})
			db.DropTableIfExists(&types.UserAccount{})
			return nil
		},
	}

	// add migration to list
	addMigration(m)
}
