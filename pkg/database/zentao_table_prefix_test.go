package database

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

type prefixedUser struct {
	ID uint `gorm:"primaryKey"`
}

func (prefixedUser) TableName() string {
	return ZentaoTablePrefix + "user"
}

func restoreZentaoTablePrefix(t *testing.T) {
	t.Helper()
	previous := ZentaoTablePrefix
	t.Cleanup(func() {
		ZentaoTablePrefix = previous
	})
}

func TestSetZentaoTablePrefix(t *testing.T) {
	restoreZentaoTablePrefix(t)

	if err := SetZentaoTablePrefix(" tenant_ "); err != nil {
		t.Fatalf("SetZentaoTablePrefix() error = %v", err)
	}
	if got := ZentaoTablePrefix + "user"; got != "tenant_user" {
		t.Fatalf("ZentaoTablePrefix+user = %q, want %q", got, "tenant_user")
	}
	if err := SetZentaoTablePrefix(""); err == nil {
		t.Fatal("SetZentaoTablePrefix() should reject an empty prefix")
	}
	if got := ZentaoTablePrefix + "notify"; got != "tenant_notify" {
		t.Fatalf("ZentaoTablePrefix+notify = %q, want %q", got, "tenant_notify")
	}
}

func TestSetZentaoTablePrefixRejectsInvalidPrefix(t *testing.T) {
	restoreZentaoTablePrefix(t)

	if err := SetZentaoTablePrefix("tenant_"); err != nil {
		t.Fatalf("SetZentaoTablePrefix() error = %v", err)
	}

	if err := SetZentaoTablePrefix("tenant-"); err == nil {
		t.Fatal("SetZentaoTablePrefix() should reject an invalid prefix")
	}
	if got := ZentaoTablePrefix; got != "tenant_" {
		t.Fatalf("ZentaoTablePrefix = %q, want %q", got, "tenant_")
	}
}

func TestZentaoTableWithGORM(t *testing.T) {
	restoreZentaoTablePrefix(t)

	if err := SetZentaoTablePrefix("tenant_"); err != nil {
		t.Fatalf("SetZentaoTablePrefix() error = %v", err)
	}

	db, err := gorm.Open(sqlite.Open("file:naming_test?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			TablePrefix:   "ops_",
			SingularTable: true,
		},
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	t.Cleanup(func() {
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	if err := db.AutoMigrate(&prefixedUser{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}
	if !db.Migrator().HasTable("tenant_user") {
		t.Fatal("expected table tenant_user to be created")
	}
	if db.Migrator().HasTable("ops_tenant_user") {
		t.Fatal("did not expect Gitfox naming strategy to prefix the ZenTao table")
	}
}

func TestOpsNamingStrategyIsUnchanged(t *testing.T) {
	strategy := schema.NamingStrategy{
		TablePrefix:   "ops_",
		SingularTable: true,
	}
	if got := strategy.TableName("Queue"); got != "ops_queue" {
		t.Fatalf("ops TableName(Queue) = %q, want %q", got, "ops_queue")
	}
}
