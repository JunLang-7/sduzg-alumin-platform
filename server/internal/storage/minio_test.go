package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLocalFilePathRejectsTraversal(t *testing.T) {
	client := &Client{driver: driverLocal, localPath: t.TempDir()}

	for _, objectKey := range []string{"../outside.txt", "../../outside.txt", "/absolute.txt"} {
		if _, err := client.localFilePath(objectKey); err == nil {
			t.Fatalf("expected %q to be rejected", objectKey)
		}
	}
}

func TestLocalFilePathStaysWithinRoot(t *testing.T) {
	root := t.TempDir()
	client := &Client{driver: driverLocal, localPath: root}

	target, err := client.localFilePath("alumni/1/record.pdf")
	if err != nil {
		t.Fatalf("expected valid object key, got %v", err)
	}

	relative, err := filepath.Rel(root, target)
	if err != nil {
		t.Fatalf("resolve relative path: %v", err)
	}
	if relative != filepath.Join("alumni", "1", "record.pdf") {
		t.Fatalf("unexpected relative path %q", relative)
	}
}

func TestDeleteLocalFileIgnoresMissingObject(t *testing.T) {
	client := &Client{driver: driverLocal, localPath: t.TempDir()}

	if err := client.DeleteFile(t.Context(), "alumni/1/missing.pdf"); err != nil {
		t.Fatalf("expected missing file deletion to succeed, got %v", err)
	}
}

func TestDeleteByPrefixRemovesOnlyTargetDirectory(t *testing.T) {
	root := t.TempDir()
	client := &Client{driver: driverLocal, localPath: root}
	target := filepath.Join(root, "alumni", "1")
	other := filepath.Join(root, "alumni", "2", "keep.txt")

	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(other), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "remove.txt"), []byte("remove"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(other, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := client.DeleteByPrefix(t.Context(), "alumni/1"); err != nil {
		t.Fatalf("delete prefix: %v", err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("expected target directory to be removed, got %v", err)
	}
	if _, err := os.Stat(other); err != nil {
		t.Fatalf("expected unrelated file to remain, got %v", err)
	}
}
