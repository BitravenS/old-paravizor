package cli

import (
	"context"
	"fmt"

	"github.com/bitravens/paravizor/v1/internal/config"
	"github.com/bitravens/paravizor/v1/internal/project"
	"github.com/bitravens/paravizor/v1/internal/store"
)

func openProjectStore(ctx context.Context, dir string) (string, *store.Store, error) {
	projectDir, err := resolveRunProjectDir(dir)
	if err != nil {
		return "", nil, err
	}
	if _, err := project.LoadProject(projectDir); err != nil {
		return "", nil, fmt.Errorf("load project from %q: %w", projectDir, err)
	}

	cfg := config.LoadConfig(projectDir)
	dbCfg := store.DBConfig{}
	if cfg.DBConfig != nil {
		dbCfg = *cfg.DBConfig
	}

	st, err := store.Open(ctx, project.DBPath(projectDir), dbCfg)
	if err != nil {
		return "", nil, fmt.Errorf("open project database %q: %w", project.DBPath(projectDir), err)
	}
	return projectDir, st, nil
}
