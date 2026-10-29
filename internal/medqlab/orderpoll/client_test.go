package orderpoll

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSaveOrderSuccess(t *testing.T) {
	var gotXTime string
	var gotBody SaveOrderBody
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/saveOrder" {
			t.Errorf("path=%s", r.URL.Path)
		}
		if r.Header.Get("X-cons") != "cons" || r.Header.Get("X-sign") != "secret" {
			t.Errorf("headers cons=%s sign=%s", r.Header.Get("X-cons"), r.Header.Get("X-sign"))
		}
		gotXTime = r.Header.Get("X-Time")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"metaData":{"code":200,"message":"ok"},"response":{"no_laboratorium":"LAB001"}}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "cons", "secret")
	body := SaveOrderBody{
		NoOrder:       "2602250001",
		NoPendaftaran: "2026/02/25/000001",
		ICD10:         []string{},
		Order:         []OrderItem{{IDTest: "1001", NamaTest: "HB"}},
	}
	labNo, err := c.SaveOrder(context.Background(), body, false)
	if err != nil {
		t.Fatal(err)
	}
	if labNo != "LAB001" {
		t.Fatalf("labNo=%q", labNo)
	}
	if gotXTime != "" {
		t.Fatalf("ralan must not send X-Time, got %q", gotXTime)
	}
	if gotBody.NoOrder != "2602250001" {
		t.Fatalf("posted no_order=%q", gotBody.NoOrder)
	}
}

func TestSaveOrderRanapSendsXTime(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Time") == "" {
			t.Error("ranap missing X-Time")
		}
		_, _ = w.Write([]byte(`{"metaData":{"code":"200"},"response":{"noLaboratorium":"L2"}}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "c", "s")
	labNo, err := c.SaveOrder(context.Background(), SaveOrderBody{ICD10: []string{}}, true)
	if err != nil {
		t.Fatal(err)
	}
	if labNo != "L2" {
		t.Fatalf("labNo=%q", labNo)
	}
}

func TestSaveOrderRejectsNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"metaData":{"code":400,"message":"test unmapped"}}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "c", "s")
	_, err := c.SaveOrder(context.Background(), SaveOrderBody{ICD10: []string{}}, false)
	if err == nil || err.Error() != "test unmapped" {
		t.Fatalf("err=%v", err)
	}
}

func TestCodeIs200(t *testing.T) {
	if !codeIs200(float64(200)) || !codeIs200("200") {
		t.Fatal("200 should succeed")
	}
	if codeIs200(float64(400)) || codeIs200("400") || codeIs200(nil) {
		t.Fatal("non-200 should fail")
	}
}
