package cli

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDownloadFileRefusesExistingDestination(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "facture.pdf")
	if err := os.WriteFile(dest, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := downloadFile(context.Background(), "http://127.0.0.1:1", dest, false)
	if err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("erreur = %v, attendu refus avec --force", err)
	}
	if got, err := os.ReadFile(dest); err != nil || string(got) != "original" {
		t.Fatalf("destination modifiée : %q, err=%v", got, err)
	}
}

func TestDownloadFilePreservesExistingDestinationOnFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "indisponible", http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	dir := t.TempDir()
	dest := filepath.Join(dir, "facture.pdf")
	if err := os.WriteFile(dest, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := downloadFile(context.Background(), srv.URL, dest, true); err == nil {
		t.Fatal("téléchargement en erreur accepté")
	}
	if got, err := os.ReadFile(dest); err != nil || string(got) != "original" {
		t.Fatalf("destination modifiée : %q, err=%v", got, err)
	}
	if matches, err := filepath.Glob(filepath.Join(dir, ".yorkhost-invoice-*.tmp")); err != nil || len(matches) != 0 {
		t.Fatalf("fichiers temporaires restants : %v, err=%v", matches, err)
	}
}

func TestDownloadFileReplacesOnlyAfterSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		fmt.Fprint(w, "%PDF-1.7\nnew")
	}))
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "facture.pdf")
	if err := os.WriteFile(dest, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := downloadFile(context.Background(), srv.URL, dest, true); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(dest); err != nil || string(got) != "%PDF-1.7\nnew" {
		t.Fatalf("destination = %q, err=%v", got, err)
	}
}

func TestDownloadFileCreatesNewDestination(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "%PDF-1.7\nnew")
	}))
	defer srv.Close()

	dir := t.TempDir()
	dest := filepath.Join(dir, "facture.pdf")
	if err := downloadFile(context.Background(), srv.URL, dest, false); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(dest); err != nil || string(got) != "%PDF-1.7\nnew" {
		t.Fatalf("destination = %q, err=%v", got, err)
	}
	if matches, err := filepath.Glob(filepath.Join(dir, ".yorkhost-invoice-*.tmp")); err != nil || len(matches) != 0 {
		t.Fatalf("fichiers temporaires restants : %v, err=%v", matches, err)
	}
}
