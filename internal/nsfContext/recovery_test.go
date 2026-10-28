package nsfContext_test

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	loggerutil "github.com/Alonza0314/logger-go/v2/util"
	"nsf/internal/db"
	"nsf/internal/nsfContext"
	"nsf/logger"
)

func TestRecoveryAndPartialDeregistration(t *testing.T) {
	previous := db.NSF_DB_PATH
	db.NSF_DB_PATH = filepath.Join(t.TempDir(), "recovery.db")
	defer func() { db.NSF_DB_PATH = previous }()
	log := logger.NewNsfLogger(loggerutil.LogLevelString("error"))
	newContext := func() *nsfContext.NsfContext {
		ctx := nsfContext.NewNsfContext(&nsfContext.NsfContextIE{DbType: "bbolt", NsfLogger: &log})
		if ctx == nil {
			t.Fatal("create context")
		}
		return ctx
	}
	ctx := newContext()
	if err := ctx.RegisterServices("NF1", "127.0.0.1", 8080, []string{"shared", "one"}); err != nil {
		t.Fatal(err)
	}
	if err := ctx.RegisterServices("NF2", "127.0.0.2", 8081, []string{"shared", "two"}); err != nil {
		t.Fatal(err)
	}
	ctx.Release()
	ctx = newContext()
	if nf, addr, port, err := ctx.DiscoverService("one"); err != nil || nf != "NF1" || addr != "127.0.0.1" || port != 8080 {
		t.Fatalf("recovery: %s %s %d %v", nf, addr, port, err)
	}
	if err := ctx.DeregisterServices("NF1", []string{"shared"}); err != nil {
		t.Fatal(err)
	}
	if nf, _, _, err := ctx.DiscoverService("shared"); err != nil || nf != "NF2" {
		t.Fatalf("other provider lost: %s %v", nf, err)
	}
	if _, _, _, err := ctx.DiscoverService("one"); err != nil {
		t.Fatal("partial deregistration removed remaining service", err)
	}
	if err := ctx.DeregisterServices("NF1", []string{"one"}); err != nil {
		t.Fatal(err)
	}
	ctx.Release()
	ctx = newContext()
	defer ctx.Release()
	if _, _, _, err := ctx.DiscoverService("one"); err == nil {
		t.Fatal("deleted service returned after restart")
	}
	if _, _, _, err := ctx.DiscoverService("missing"); err == nil {
		t.Fatal("missing service must return an error")
	}
	if nf, _, _, err := ctx.DiscoverService("two"); err != nil || nf != "NF2" {
		t.Fatalf("unrelated registration lost: %s %v", nf, err)
	}
}

func TestConcurrentRegistration(t *testing.T) {
	previous := db.NSF_DB_PATH
	db.NSF_DB_PATH = filepath.Join(t.TempDir(), "concurrent.db")
	defer func() { db.NSF_DB_PATH = previous }()
	log := logger.NewNsfLogger(loggerutil.LogLevelString("error"))
	ctx := nsfContext.NewNsfContext(&nsfContext.NsfContextIE{DbType: "bbolt", NsfLogger: &log})
	if ctx == nil {
		t.Fatal("create context")
	}
	defer ctx.Release()
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			service := fmt.Sprintf("service-%d", i)
			if err := ctx.RegisterServices("NF", "127.0.0.1", 8080, []string{service}); err != nil {
				t.Error(err)
			}
			if _, _, _, err := ctx.DiscoverService(service); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	for i := 0; i < 20; i++ {
		if _, _, _, err := ctx.DiscoverService(fmt.Sprintf("service-%d", i)); err != nil {
			t.Fatal("registration lost", err)
		}
	}
}
