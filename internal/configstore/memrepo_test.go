// SPDX-License-Identifier: AGPL-3.0-only
package configstore_test

import (
	"testing"

	"github.com/MagicalWig34653/db-manager/internal/configstore"
	"github.com/MagicalWig34653/db-manager/internal/configstore/repotest"
)

func TestMemoryRepoConformance(t *testing.T) {
	repotest.Run(t, func(t *testing.T) *configstore.Service {
		return configstore.NewService(configstore.NewMemoryRepo())
	})
}

func TestUnavailableMapsTo503(t *testing.T) {
	repo := configstore.NewMemoryRepo()
	service := configstore.NewService(repo)
	repo.Down = true
	_, err := service.Get(nil, "o", "bookmarks", "0123456789abcdef0123456789abcdef")
	var public *configstore.Error
	if !asPublic(err, &public) || public.Status != 503 || public.Code != "state_unavailable" {
		t.Fatalf("outage mapped to %v", err)
	}
}

func asPublic(err error, target **configstore.Error) bool {
	p, ok := err.(*configstore.Error)
	*target = p
	return ok
}
