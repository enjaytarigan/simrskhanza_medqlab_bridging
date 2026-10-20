package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"lis-khanza-mapper/internal/catalog"
	"lis-khanza-mapper/internal/model"
)

const maxCatalogBody = 4 << 20

func catalogPage(extra pageData) pageData {
	extra.Title = "SIMRS Catalog"
	extra.CatalogHeader = catalog.HeaderLine()
	return extra
}

func (s *Server) catalogForm(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.Standalone() {
		http.Redirect(w, r, "/?flash=catalog-standalone-only", http.StatusSeeOther)
		return
	}
	s.view(w, "catalog.html", catalogPage(pageData{Flash: r.URL.Query().Get("flash")}))
}

func (s *Server) catalogImport(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.Standalone() {
		http.Error(w, "catalog import is only available in standalone mode", http.StatusBadRequest)
		return
	}
	if err := r.ParseMultipartForm(maxCatalogBody); err != nil {
		s.view(w, "catalog.html", catalogPage(pageData{Error: err.Error()}))
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		s.view(w, "catalog.html", catalogPage(pageData{Error: "CSV file is required"}))
		return
	}
	defer file.Close()
	rows, err := catalog.ParseCSV(file)
	if err != nil {
		s.view(w, "catalog.html", catalogPage(pageData{Error: err.Error()}))
		return
	}
	result, err := s.simrs.UpsertCatalog(r.Context(), rows)
	if err != nil {
		s.view(w, "catalog.html", catalogPage(pageData{Error: err.Error()}))
		return
	}
	s.view(w, "catalog.html", catalogPage(pageData{CatalogImport: &result}))
}

func (s *Server) catalogTemplateCreate(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.Standalone() {
		http.Error(w, "catalog import is only available in standalone mode", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	row, err := catalogRowFromForm(r)
	if err != nil {
		s.view(w, "catalog.html", catalogPage(pageData{Error: err.Error(), CatalogForm: &row}))
		return
	}
	result, err := s.simrs.UpsertCatalog(r.Context(), []model.CatalogRow{row})
	if err != nil {
		s.view(w, "catalog.html", catalogPage(pageData{Error: err.Error(), CatalogForm: &row}))
		return
	}
	flash := fmt.Sprintf("Saved template %d", row.IDTemplate)
	if result.Skipped > 0 {
		flash = "Skipped: " + strings.Join(result.Errors, "; ")
	}
	http.Redirect(w, r, "/catalog?flash="+url.QueryEscape(flash), http.StatusSeeOther)
}

func (s *Server) apiImportCatalog(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.Standalone() {
		writeAPI(w, http.StatusBadRequest, nil, "catalog import is only available in standalone mode")
		return
	}
	rows, err := readCatalogRows(r)
	if err != nil {
		writeAPI(w, http.StatusBadRequest, nil, err.Error())
		return
	}
	result, err := s.simrs.UpsertCatalog(r.Context(), rows)
	if err != nil {
		writeAPI(w, http.StatusBadRequest, nil, err.Error())
		return
	}
	writeAPI(w, http.StatusOK, result, "")
}

func (s *Server) apiCreateCatalogTemplate(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.Standalone() {
		writeAPI(w, http.StatusBadRequest, nil, "catalog import is only available in standalone mode")
		return
	}
	var body model.CatalogRow
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeAPI(w, http.StatusBadRequest, nil, "invalid json")
		return
	}
	result, err := s.simrs.UpsertCatalog(r.Context(), []model.CatalogRow{body})
	if err != nil {
		writeAPI(w, http.StatusBadRequest, nil, err.Error())
		return
	}
	writeAPI(w, http.StatusOK, result, "")
}

func readCatalogRows(r *http.Request) ([]model.CatalogRow, error) {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxCatalogBody))
	if err != nil {
		return nil, err
	}
	ct := strings.ToLower(r.Header.Get("Content-Type"))
	trim := bytes.TrimSpace(body)
	if strings.Contains(ct, "json") || (len(trim) > 0 && trim[0] == '{') {
		var wrap struct {
			Rows []model.CatalogRow `json:"rows"`
		}
		if err := json.Unmarshal(trim, &wrap); err != nil {
			return nil, fmt.Errorf("invalid json")
		}
		return wrap.Rows, nil
	}
	return catalog.ParseCSV(bytes.NewReader(trim))
}

func catalogRowFromForm(r *http.Request) (model.CatalogRow, error) {
	urut, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("urut")))
	biaya, _ := strconv.ParseFloat(strings.TrimSpace(r.FormValue("biaya_item")), 64)
	total, _ := strconv.ParseFloat(strings.TrimSpace(r.FormValue("total_byr")), 64)
	row := model.CatalogRow{
		Pemeriksaan: strings.TrimSpace(r.FormValue("pemeriksaan")),
		Satuan:      strings.TrimSpace(r.FormValue("satuan")),
		Urut:        urut,
		KdJenisPrw:  strings.TrimSpace(r.FormValue("kd_jenis_prw")),
		NmPerawatan: strings.TrimSpace(r.FormValue("nm_perawatan")),
		BiayaItem:   biaya,
		Kategori:    strings.TrimSpace(r.FormValue("kategori")),
		TotalByr:    total,
		Status:      strings.TrimSpace(r.FormValue("status")),
	}
	idTpl, err := strconv.Atoi(strings.TrimSpace(r.FormValue("id_template")))
	if err != nil {
		return row, fmt.Errorf("template ID must be an integer")
	}
	row.IDTemplate = idTpl
	return row, nil
}
