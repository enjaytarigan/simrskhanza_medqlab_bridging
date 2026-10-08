package medqlab

import (
	"strings"
	"testing"
)

func TestUpsertDetailUpdateSQLRefreshesTariff(t *testing.T) {
	for _, col := range []string{
		"nilai=?",
		"nilai_rujukan=?",
		"keterangan=?",
		"bagian_rs=?",
		"bhp=?",
		"bagian_perujuk=?",
		"bagian_dokter=?",
		"bagian_laborat=?",
		"kso=?",
		"menejemen=?",
		"biaya_item=?",
	} {
		if !strings.Contains(upsertDetailUpdateSQL, col) {
			t.Errorf("upsertDetailUpdateSQL missing %s", col)
		}
	}
}
