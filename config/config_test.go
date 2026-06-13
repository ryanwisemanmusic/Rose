package config

import "testing"

func TestApplyLoadedDefaultsMigratesOldDefaultModel(t *testing.T) {
	cfg := &Config{ActiveModel: oldDefaultModel}
	changed := cfg.applyLoadedDefaults(false)

	if !changed {
		t.Fatal("expected config to change")
	}
	if cfg.ActiveModel != DefaultActiveModel {
		t.Fatalf("ActiveModel = %q, want %q", cfg.ActiveModel, DefaultActiveModel)
	}
	if !cfg.DefaultModelMigrated {
		t.Fatal("DefaultModelMigrated = false, want true")
	}
}

func TestApplyLoadedDefaultsKeepsExplicitModelAfterMigration(t *testing.T) {
	cfg := &Config{ActiveModel: oldDefaultModel, DefaultModelMigrated: true}
	changed := cfg.applyLoadedDefaults(true)

	if cfg.ActiveModel != oldDefaultModel {
		t.Fatalf("ActiveModel = %q, want explicit %q", cfg.ActiveModel, oldDefaultModel)
	}
	if !changed {
		t.Fatal("other missing defaults should still be filled")
	}
}
