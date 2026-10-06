package medqlab

import (
	"sort"
	"strconv"
	"strings"
)

// FlattenLeaves walks the examination tree and keeps only leaf nodes
// (no children) that have a result value and a position — same rules as
// ApiMEDQLAB.flattenExaminations / mapExaminationToHasil.
func FlattenLeaves(exams []Examination) []LeafResult {
	var out []LeafResult
	var walk func([]Examination)
	walk = func(nodes []Examination) {
		for _, n := range nodes {
			if len(n.Children) > 0 {
				walk(n.Children)
				continue
			}
			if leaf := mapLeaf(n); leaf != nil {
				out = append(out, *leaf)
			}
		}
	}
	walk(exams)
	return out
}

func mapLeaf(exam Examination) *LeafResult {
	nilai := getExamValue(exam)
	if nilai == "" {
		return nil
	}
	pos := strings.TrimSpace(exam.Position)
	if pos == "" {
		return nil
	}
	ket := strings.TrimSpace(strPtr(exam.ExamValueFlag))
	validated := strings.TrimSpace(strPtr(exam.ValidatedAt))
	verified := strings.TrimSpace(strPtr(exam.VerifiedAt))
	if validated == "" {
		validated = verified
	}
	return &LeafResult{
		LisTestID:        strings.TrimSpace(exam.TestID.String()),
		LocalCode:        strings.TrimSpace(strPtr(exam.LocalCode)),
		TestName:         strings.TrimSpace(exam.TestName),
		Position:         pos,
		Nilai:            nilai,
		Keterangan:       ket,
		NilaiRujukan:     strings.TrimSpace(strPtr(exam.NormalValueText)),
		ValidatedAt:      validated,
		VerifiedAt:       verified,
		IdEmployeeVerify: strings.TrimSpace(strPtr(exam.IdEmployeeVerify)),
	}
}

func getExamValue(exam Examination) string {
	for _, p := range []*string{exam.ExamValue, exam.OriginalResult, exam.ResultInterpretation} {
		if v := strings.TrimSpace(strPtr(p)); v != "" {
			return v
		}
	}
	return ""
}

// SortByPosition sorts leaves by hierarchical numeric position (1.2 before 1.10).
func SortByPosition(leaves []LeafResult) {
	sort.SliceStable(leaves, func(i, j int) bool {
		return comparePosition(leaves[i].Position, leaves[j].Position) < 0
	})
}

func comparePosition(a, b string) int {
	as := strings.Split(a, ".")
	bs := strings.Split(b, ".")
	n := len(as)
	if len(bs) > n {
		n = len(bs)
	}
	for i := 0; i < n; i++ {
		ai := positionSegment(as, i)
		bi := positionSegment(bs, i)
		if ai != bi {
			return ai - bi
		}
	}
	return 0
}

func positionSegment(parts []string, i int) int {
	if i >= len(parts) {
		return 0
	}
	v, err := strconv.Atoi(parts[i])
	if err != nil {
		return 0
	}
	return v
}
