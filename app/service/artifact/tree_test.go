package artifact

import (
	"context"
	"testing"

	model "github.com/easysoft/gitfox-community/app/model/artifact"
	artifactStore "github.com/easysoft/gitfox-community/app/store/artifact"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRefreshTreeKeepsFileNodesAsGroups(t *testing.T) {
	db := newRefreshTreeTestDB(t)
	store := artifactStore.NewStore(db)
	svc := &Service{db: db, artStore: store}
	ctx := context.Background()

	if err := svc.refreshTree(ctx, string(model.ArtifactFileFormat), store); err != nil {
		t.Fatalf("refresh file tree: %v", err)
	}

	assertTreeNodeTarget(t, db, "/runtime/version", model.ArtifactFileFormat, model.ArtifactTreeNodeTypeGroup, 11)
	assertTreeNodeTarget(t, db, "/runtime/version/1.0.2", model.ArtifactFileFormat, model.ArtifactTreeNodeTypeGroup, 12)

	// 全量刷新时 raw 仍必须使用 package/version 节点，避免守卫误伤其它格式。
	if err := svc.refreshTree(ctx, string(model.ArtifactAllFormat), store); err != nil {
		t.Fatalf("refresh all trees: %v", err)
	}

	assertTreeNodeTarget(t, db, "/runtime/version", model.ArtifactFileFormat, model.ArtifactTreeNodeTypeGroup, 11)
	assertTreeNodeTarget(t, db, "/runtime/version/1.0.2", model.ArtifactFileFormat, model.ArtifactTreeNodeTypeGroup, 12)
	assertTreeNodeTarget(t, db, "/rawroot/rawpkg", model.ArtifactRawFormat, model.ArtifactTreeNodeTypePackage, 201)
	assertTreeNodeTarget(t, db, "/rawroot/rawpkg/v1", model.ArtifactRawFormat, model.ArtifactTreeNodeTypeVersion, 501)
}

func assertTreeNodeTarget(
	t *testing.T,
	db *gorm.DB,
	path string,
	format model.ArtifactFormat,
	wantType model.ArtifactTreeNodeType,
	wantTarget int64,
) {
	t.Helper()

	var got struct {
		LinkTable  string `gorm:"column:linkTable"`
		LinkRecord int64  `gorm:"column:linkRecord"`
	}
	if err := db.Table("ops_artifact_tree_nodes").
		Select("linkTable, linkRecord").
		Where("artifactLibID = ? AND nodePath = ? AND type = ?", 1, path, format).
		Take(&got).Error; err != nil {
		t.Fatalf("query tree node %s: %v", path, err)
	}
	if got.LinkTable != string(wantType) || got.LinkRecord != wantTarget {
		t.Fatalf(
			"tree node %s = %s:%d, want %s:%d",
			path,
			got.LinkTable,
			got.LinkRecord,
			wantType,
			wantTarget,
		)
	}
}

func newRefreshTreeTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })

	schemas := []string{
		`CREATE TABLE ops_artifact_groups (
			id INTEGER PRIMARY KEY,
			parentID INTEGER NOT NULL DEFAULT 0,
			artifactLibID INTEGER NOT NULL DEFAULT 0,
			name TEXT NOT NULL DEFAULT '',
			type TEXT NOT NULL DEFAULT '',
			createdBy TEXT NOT NULL DEFAULT '',
			createdDate DATETIME,
			editedBy TEXT NOT NULL DEFAULT '',
			editedDate DATETIME,
			deleted INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE ops_artifact_packages (
			id INTEGER PRIMARY KEY,
			artifactLibID INTEGER NOT NULL DEFAULT 0,
			groupID INTEGER NOT NULL DEFAULT 0,
			namespace TEXT NOT NULL DEFAULT '',
			name TEXT NOT NULL DEFAULT '',
			type TEXT NOT NULL DEFAULT '',
			createdBy TEXT NOT NULL DEFAULT '',
			createdDate DATETIME,
			editedBy TEXT NOT NULL DEFAULT '',
			editedDate DATETIME,
			deleted INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE ops_artifact_versions (
			id INTEGER PRIMARY KEY,
			artifactLibID INTEGER NOT NULL DEFAULT 0,
			packageID INTEGER NOT NULL DEFAULT 0,
			version TEXT NOT NULL DEFAULT '',
			createdBy TEXT NOT NULL DEFAULT '',
			createdDate DATETIME,
			editedBy TEXT NOT NULL DEFAULT '',
			editedDate DATETIME,
			deleted INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE ops_artifact_tree_nodes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			artifactLibID INTEGER NOT NULL DEFAULT 0,
			parentID INTEGER NOT NULL DEFAULT 0,
			nodeName TEXT NOT NULL DEFAULT '',
			nodePath TEXT NOT NULL DEFAULT '',
			linkTable TEXT NOT NULL DEFAULT '',
			linkRecord INTEGER NOT NULL DEFAULT 0,
			type TEXT NOT NULL DEFAULT ''
		)`,
	}
	for _, schema := range schemas {
		if err := db.Exec(schema).Error; err != nil {
			t.Fatalf("create test schema: %v", err)
		}
	}

	seed := []string{
		`INSERT INTO ops_artifact_groups
			(id, parentID, artifactLibID, name, type, deleted)
		 VALUES
			(10, 0, 1, 'runtime', 'file', 0),
			(11, 10, 1, 'version', 'file', 0),
			(12, 11, 1, '1.0.2', 'file', 0),
			(20, 0, 1, 'rawroot', 'raw', 0)`,
		`INSERT INTO ops_artifact_packages
			(id, artifactLibID, groupID, namespace, name, type, deleted)
		 VALUES
			(101, 1, 10, 'runtime', 'version', 'file', 0),
			(201, 1, 20, '', 'rawpkg', 'raw', 0)`,
		`INSERT INTO ops_artifact_versions
			(id, artifactLibID, packageID, version, deleted)
		 VALUES
			(409, 1, 101, '1.0.2', 0),
			(501, 1, 201, 'v1', 0)`,
		`INSERT INTO ops_artifact_tree_nodes
			(id, artifactLibID, parentID, nodeName, nodePath, linkTable, linkRecord, type)
		 VALUES
			(1, 1, 0, 'file', '/', 'format', 0, 'file'),
			(2, 1, 1, 'runtime', '/runtime', 'group', 10, 'file'),
			(3, 1, 2, 'version', '/runtime/version', 'package', 101, 'file'),
			(4, 1, 3, '1.0.2', '/runtime/version/1.0.2', 'version', 409, 'file'),
			(5, 1, 0, 'raw', '/', 'format', 0, 'raw'),
			(6, 1, 5, 'rawroot', '/rawroot', 'group', 20, 'raw')`,
	}
	for _, statement := range seed {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("seed test data: %v", err)
		}
	}

	return db
}
