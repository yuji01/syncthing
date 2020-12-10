// Copyright (C) 2016 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package model

import (
	"context"
	"testing"
	"time"

	"github.com/syncthing/syncthing/lib/protocol"
)

// TestIgnoreDeleteUnignore checks that the deletion of an ignored file is not
// propagated upon un-ignoring.
// https://github.com/syncthing/syncthing/issues/6038
func TestIgnoreDeleteUnignore(t *testing.T) {
	w, fcfg := tmpDefaultWrapper()
	m := setupModel(w)
	fss := fcfg.Filesystem()
	tmpDir := fss.URI()
	defer cleanupModelAndRemoveDir(m, tmpDir)

	folderIgnoresAlwaysReload(m, fcfg)

	fc := addFakeConn(m, device1)
	fc.folder = "default"
	fc.mut.Lock()
	fc.mut.Unlock()

	file := "foobar"
	contents := []byte("test file contents\n")

	basicCheck := func(fs []protocol.FileInfo) {
		t.Helper()
		if len(fs) != 1 {
			t.Fatal("expected a single index entry, got", len(fs))
		} else if fs[0].Name != file {
			t.Fatalf("expected a index entry for %v, got one for %v", file, fs[0].Name)
		}
	}

	done := make(chan struct{})
	fc.mut.Lock()
	fc.indexFn = func(_ context.Context, folder string, fs []protocol.FileInfo) {
		basicCheck(fs)
		close(done)
	}
	fc.mut.Unlock()

	if err := writeFile(fss, file, contents, 0644); err != nil {
		panic(err)
	}
	m.ScanFolders()

	select {
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out before index was received")
	case <-done:
	}

	done = make(chan struct{})
	fc.mut.Lock()
	fc.indexFn = func(_ context.Context, folder string, fs []protocol.FileInfo) {
		basicCheck(fs)
		f := fs[0]
		if !f.IsInvalid() {
			t.Errorf("Received non-invalid index update")
		}
		close(done)
	}
	fc.mut.Unlock()

	if err := m.SetIgnores("default", []string{"foobar"}); err != nil {
		panic(err)
	}

	select {
	case <-time.After(5 * time.Second):
		t.Fatal("timed out before receiving index update")
	case <-done:
	}

	done = make(chan struct{})
	fc.mut.Lock()
	fc.indexFn = func(_ context.Context, folder string, fs []protocol.FileInfo) {
		basicCheck(fs)
		f := fs[0]
		if f.IsInvalid() {
			t.Errorf("Received invalid index update")
		}
		if !f.Version.Equal(protocol.Vector{}) && f.Deleted {
			t.Error("Received deleted index entry with non-empty version")
		}
		l.Infoln(f)
		close(done)
	}
	fc.mut.Unlock()

	if err := fss.Remove(file); err != nil {
		t.Fatal(err)
	}
	if err := m.SetIgnores("default", []string{}); err != nil {
		panic(err)
	}

	select {
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out before index was received")
	case <-done:
	}
}
