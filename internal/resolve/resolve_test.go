package resolve

import (
	"encoding/json"
	"testing"
)

func cand(testID, local string, idTpl int, kd, name string) Candidate {
	return Candidate{LisTestID: testID, LocalCode: local, IDTemplate: idTpl, KdJenisPrw: kd, Pemeriksaan: name}
}

func TestPickUnmapped(t *testing.T) {
	got := Pick(Item{TestID: "99", LocalCode: "HB"}, nil, nil)
	if got.Status != StatusUnmapped {
		t.Fatalf("status=%s", got.Status)
	}
}

func TestPickSingleMapping(t *testing.T) {
	all := []Candidate{cand("123", "HB", 1001, "PK-01", "Hemoglobin")}
	got := Pick(Item{TestID: "123"}, all, map[string]bool{"123": true})
	if got.Status != StatusMapped || got.IDTemplate != 1001 {
		t.Fatalf("got %+v", got)
	}
}

func TestPickKnownTestIDNoMappingDoesNotFallback(t *testing.T) {
	all := []Candidate{cand("other", "HB", 1001, "PK-01", "Hemoglobin")}
	got := Pick(Item{TestID: "123", LocalCode: "HB"}, all, map[string]bool{"123": true})
	if got.Status != StatusUnmapped {
		t.Fatalf("status=%s", got.Status)
	}
}

func TestPickLocalCodeFallbackUnique(t *testing.T) {
	all := []Candidate{cand("123", "HB", 1001, "PK-01", "Hemoglobin")}
	got := Pick(Item{TestID: "999", LocalCode: "HB"}, all, nil)
	if got.Status != StatusMapped || got.IDTemplate != 1001 || got.TestID != "999" {
		t.Fatalf("got %+v", got)
	}
}

func TestPickLocalCodeNotUnique(t *testing.T) {
	all := []Candidate{
		cand("1", "HB", 1001, "PK-01", "Hb A"),
		cand("2", "HB", 1002, "PK-02", "Hb B"),
	}
	got := Pick(Item{TestID: "999", LocalCode: "HB"}, all, nil)
	if got.Status != StatusAmbiguous {
		t.Fatalf("status=%s", got.Status)
	}
}

func TestPickSeveralUsesOrderIDTemplate(t *testing.T) {
	all := []Candidate{
		cand("123", "NA", 11, "EL-A", "Natrium"),
		cand("123", "NA", 22, "EL-B", "Natrium alt"),
	}
	got := Pick(Item{TestID: "123", OrderIDs: []FlexString{"22"}}, all, map[string]bool{"123": true})
	if got.Status != StatusMapped || got.IDTemplate != 22 {
		t.Fatalf("got %+v", got)
	}
}

func TestPickSeveralUsesOrderKdJenisPrw(t *testing.T) {
	all := []Candidate{
		cand("123", "NA", 11, "EL-A", "Natrium"),
		cand("123", "NA", 22, "EL-B", "Natrium alt"),
	}
	got := Pick(Item{TestID: "123", OrderIDs: []FlexString{"EL-A"}}, all, map[string]bool{"123": true})
	if got.Status != StatusMapped || got.IDTemplate != 11 {
		t.Fatalf("got %+v", got)
	}
}

func TestPickSeveralNoOrderIDsAmbiguous(t *testing.T) {
	all := []Candidate{
		cand("123", "NA", 11, "EL-A", "Natrium"),
		cand("123", "NA", 22, "EL-B", "Natrium alt"),
	}
	got := Pick(Item{TestID: "123"}, all, map[string]bool{"123": true})
	if got.Status != StatusAmbiguous {
		t.Fatalf("status=%s", got.Status)
	}
}

func TestPickSeveralOrderIDsStillAmbiguous(t *testing.T) {
	all := []Candidate{
		cand("123", "NA", 11, "EL-A", "Natrium"),
		cand("123", "NA", 22, "EL-B", "Natrium alt"),
	}
	got := Pick(Item{TestID: "123", OrderIDs: []FlexString{"11", "22"}}, all, map[string]bool{"123": true})
	if got.Status != StatusAmbiguous {
		t.Fatalf("status=%s", got.Status)
	}
}

func TestItemNumericJSON(t *testing.T) {
	var item Item
	if err := json.Unmarshal([]byte(`{"test_id":12345,"local_code":"HB","order_ids":[1001]}`), &item); err != nil {
		t.Fatal(err)
	}
	if item.TestID.String() != "12345" || item.LocalCode.String() != "HB" {
		t.Fatalf("got %+v", item)
	}
	if len(item.OrderIDs) != 1 || item.OrderIDs[0].String() != "1001" {
		t.Fatalf("order_ids=%v", item.OrderIDs)
	}
}

func TestItemTestNameJSON(t *testing.T) {
	var item Item
	raw := `{"test_id":"12345","local_code":"HB","test_name":"Hemoglobin","order_ids":["1001"]}`
	if err := json.Unmarshal([]byte(raw), &item); err != nil {
		t.Fatal(err)
	}
	if item.TestName.String() != "Hemoglobin" {
		t.Fatalf("test_name=%q", item.TestName)
	}
}
