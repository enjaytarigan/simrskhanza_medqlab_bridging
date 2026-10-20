package resolve

import (
	"encoding/json"
	"strconv"
	"strings"
)

const (
	StatusMapped    = "mapped"
	StatusUnmapped  = "unmapped"
	StatusAmbiguous = "ambiguous"
)

type FlexString string

func (s FlexString) String() string {
	return strings.TrimSpace(string(s))
}

func (s *FlexString) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		*s = ""
		return nil
	}
	var str string
	if err := json.Unmarshal(b, &str); err == nil {
		*s = FlexString(str)
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(b, &n); err == nil {
		*s = FlexString(n.String())
		return nil
	}
	return json.Unmarshal(b, (*string)(s))
}

type Request struct {
	Tests []Item `json:"tests"`
}

type Item struct {
	TestID    FlexString   `json:"test_id"`
	LocalCode FlexString   `json:"local_code"`
	TestName  FlexString   `json:"test_name"`
	OrderIDs  []FlexString `json:"order_ids"`
}

type Result struct {
	TestID      string `json:"test_id"`
	IDTemplate  int    `json:"id_template"`
	KdJenisPrw  string `json:"kd_jenis_prw"`
	Pemeriksaan string `json:"pemeriksaan"`
	Status      string `json:"status"`
}

type Candidate struct {
	LisTestID   string
	LocalCode   string
	IDTemplate  int
	KdJenisPrw  string
	Pemeriksaan string
}

func Pick(item Item, all []Candidate, knownTestIDs map[string]bool) Result {
	testID := item.TestID.String()
	out := Result{TestID: testID, Status: StatusUnmapped}

	cands := filterByTestID(all, testID)
	if len(cands) == 0 {
		if testID != "" && knownTestIDs[testID] {
			return out
		}
		local := item.LocalCode.String()
		if local == "" {
			return out
		}
		byCode := filterByLocalCode(all, local)
		ids := uniqueLisTestIDs(byCode)
		if len(ids) == 0 {
			return out
		}
		if len(ids) > 1 {
			out.Status = StatusAmbiguous
			return out
		}
		cands = byCode
	}
	if len(cands) == 0 {
		return out
	}
	if len(cands) == 1 {
		return mapped(testID, cands[0])
	}

	filtered := filterByOrderIDs(cands, item.OrderIDs)
	if len(filtered) == 1 {
		return mapped(testID, filtered[0])
	}
	out.Status = StatusAmbiguous
	return out
}

func mapped(testID string, c Candidate) Result {
	return Result{
		TestID:      testID,
		IDTemplate:  c.IDTemplate,
		KdJenisPrw:  c.KdJenisPrw,
		Pemeriksaan: c.Pemeriksaan,
		Status:      StatusMapped,
	}
}

func filterByTestID(all []Candidate, testID string) []Candidate {
	if testID == "" {
		return nil
	}
	var out []Candidate
	for _, c := range all {
		if c.LisTestID == testID {
			out = append(out, c)
		}
	}
	return out
}

func filterByLocalCode(all []Candidate, local string) []Candidate {
	var out []Candidate
	for _, c := range all {
		if strings.EqualFold(strings.TrimSpace(c.LocalCode), local) {
			out = append(out, c)
		}
	}
	return out
}

func uniqueLisTestIDs(cands []Candidate) []string {
	seen := map[string]struct{}{}
	var ids []string
	for _, c := range cands {
		if _, ok := seen[c.LisTestID]; ok {
			continue
		}
		seen[c.LisTestID] = struct{}{}
		ids = append(ids, c.LisTestID)
	}
	return ids
}

func filterByOrderIDs(cands []Candidate, orderIDs []FlexString) []Candidate {
	if len(orderIDs) == 0 {
		return nil
	}
	want := map[string]struct{}{}
	for _, id := range orderIDs {
		v := id.String()
		if v == "" {
			continue
		}
		want[v] = struct{}{}
	}
	if len(want) == 0 {
		return nil
	}
	var out []Candidate
	for _, c := range cands {
		tpl := strconv.Itoa(c.IDTemplate)
		if _, ok := want[tpl]; ok {
			out = append(out, c)
			continue
		}
		if _, ok := want[strings.TrimSpace(c.KdJenisPrw)]; ok {
			out = append(out, c)
		}
	}
	return out
}
