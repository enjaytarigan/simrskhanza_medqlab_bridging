package medqlab

import (
	"encoding/json"
	"testing"
	"time"
)

func TestResolveVerifyNIPLatestWins(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		t.Fatal(err)
	}
	s := NewService(nil, "ENV_NIP", loc)

	nip, source := s.resolveVerifyNIP([]mappedRow{
		{Leaf: LeafResult{
			VerifiedAt:       "2026-10-06T08:16:08.370Z",
			IdEmployeeVerify: "OLD001",
		}},
		{Leaf: LeafResult{
			VerifiedAt:       "2026-10-06T08:16:08.412Z",
			IdEmployeeVerify: "LAB001",
		}},
		{Leaf: LeafResult{
			VerifiedAt:       "2026-10-06T08:16:08.380Z",
			IdEmployeeVerify: "MID001",
		}},
	})
	if nip != "LAB001" || source != "idEmployeeVerify" {
		t.Fatalf("want LAB001 from idEmployeeVerify, got nip=%q source=%q", nip, source)
	}
}

func TestResolveVerifyNIPFallsBackToEnv(t *testing.T) {
	loc := time.UTC
	s := NewService(nil, "ENV_NIP", loc)

	nip, source := s.resolveVerifyNIP([]mappedRow{
		{Leaf: LeafResult{VerifiedAt: "2026-10-06T08:16:08.412Z"}},
		{Leaf: LeafResult{IdEmployeeVerify: "LAB001"}}, // no verifiedAt → ignored for primary pick
	})
	if nip != "ENV_NIP" || source != "env" {
		t.Fatalf("want ENV_NIP from env, got nip=%q source=%q", nip, source)
	}

	sEmpty := NewService(nil, "", loc)
	nip2, source2 := sEmpty.resolveVerifyNIP(nil)
	if nip2 != "" || source2 != "" {
		t.Fatalf("want empty when no verify and no env, got nip=%q source=%q", nip2, source2)
	}
}

func TestMapLeafCopiesIdEmployeeVerify(t *testing.T) {
	val := "12.4"
	verified := "2026-10-06T08:16:08.412Z"
	emp := "LAB001"
	leaf := mapLeaf(Examination{
		TestID:           json.Number("58"),
		Position:         "1.22.8",
		TestName:         "Hemoglobin",
		ExamValue:        &val,
		VerifiedAt:       &verified,
		IdEmployeeVerify: &emp,
	})
	if leaf == nil {
		t.Fatal("expected leaf")
	}
	if leaf.VerifiedAt != verified || leaf.IdEmployeeVerify != "LAB001" {
		t.Fatalf("verify fields: %+v", leaf)
	}
}
