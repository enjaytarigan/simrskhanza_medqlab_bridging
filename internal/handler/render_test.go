package handler

import "testing"

func TestFormatMoney(t *testing.T) {
	cases := map[float64]string{
		0:       "0",
		40000:   "40.000",
		120000:  "120.000",
		1160000: "1.160.000",
		100:     "100",
		-5000:   "-5.000",
	}
	for in, want := range cases {
		if got := formatMoney(in); got != want {
			t.Fatalf("formatMoney(%v)=%q want %q", in, got, want)
		}
	}
}

func TestInitTemplates(t *testing.T) {
	if err := initTemplates(); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"templates.html", "bulk_map.html", "panels.html", "catalog.html", "nav", "tom_select_assets"} {
		if baseTpl.Lookup(name) == nil {
			t.Fatalf("missing template %q", name)
		}
	}
}
