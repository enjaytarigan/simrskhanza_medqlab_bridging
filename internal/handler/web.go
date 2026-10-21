package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"lis-khanza-mapper/internal/model"
	"lis-khanza-mapper/internal/repository"
)

type pageData struct {
	Title         string
	Flash         string
	Error         string
	Standalone    bool
	Stats         model.DashboardStats
	Panels        []model.Panel
	Tests         []model.LisTest
	Test          *model.LisTest
	Templates     []model.Template
	Mappings      []model.Mapping
	BridgingLogs  model.BridgingLogListResult
	BridgingLog   *model.BridgingLog
	Bulk          bulkPageData
	CatalogImport *model.CatalogImportResult
	CatalogHeader string
	CatalogForm   *model.CatalogRow
	Query         map[string]string
	PrevPage      int
	NextPage      int
	HasNext       bool
}

type bulkPageData struct {
	LisTestsPK uint64
}

func (s *Server) view(w http.ResponseWriter, name string, data pageData) {
	data.Standalone = s.cfg.Standalone()
	render(w, name, data)
}

func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	stats, _ := repository.Dashboard(r.Context(), s.db, s.simrs)
	s.view(w, "dashboard.html", pageData{Title: "Dashboard", Stats: stats})
}

func (s *Server) lisTestList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	status := r.URL.Query().Get("status")
	if status == "" {
		status = ""
	}
	list, err := s.lisTests.List(r.Context(), q, status)
	if err != nil {
		s.view(w, "lis_tests.html", pageData{Title: "LIS Tests", Error: err.Error()})
		return
	}
	s.view(w, "lis_tests.html", pageData{
		Title: "LIS Tests",
		Tests: list,
		Query: map[string]string{"q": q, "status": status},
		Flash: r.URL.Query().Get("flash"),
	})
}

func (s *Server) lisTestNewForm(w http.ResponseWriter, r *http.Request) {
	s.view(w, "lis_test_form.html", pageData{Title: "New LIS Test"})
}

func (s *Server) lisTestCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	t := &model.LisTest{
		LisTestID: strings.TrimSpace(r.FormValue("lis_test_id")),
		TestName:  strings.TrimSpace(r.FormValue("test_name")),
		LocalCode: strings.TrimSpace(r.FormValue("local_code")),
		Status:    "aktif",
	}
	if err := s.lisTests.Create(r.Context(), t); err != nil {
		s.view(w, "lis_test_form.html", pageData{Title: "New LIS Test", Test: t, Error: err.Error()})
		return
	}
	http.Redirect(w, r, "/lis-tests?flash=created", http.StatusSeeOther)
}

func (s *Server) lisTestEditForm(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	t, err := s.lisTests.GetByID(r.Context(), id)
	if err != nil || t == nil {
		http.NotFound(w, r)
		return
	}
	maps, _ := s.mappings.ListByLisTestPK(r.Context(), id)
	s.view(w, "lis_test_edit.html", pageData{Title: "Edit LIS Test", Test: t, Mappings: maps, Flash: r.URL.Query().Get("flash")})
}

func (s *Server) lisTestUpdate(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	t, err := s.lisTests.GetByID(r.Context(), id)
	if err != nil || t == nil {
		http.NotFound(w, r)
		return
	}
	_ = r.ParseForm()
	t.TestName = strings.TrimSpace(r.FormValue("test_name"))
	t.LocalCode = strings.TrimSpace(r.FormValue("local_code"))
	t.Status = r.FormValue("status")
	if t.Status == "" {
		t.Status = "aktif"
	}
	if err := s.lisTests.Update(r.Context(), t); err != nil {
		s.view(w, "lis_test_edit.html", pageData{Title: "Edit LIS Test", Test: t, Error: err.Error()})
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/lis-tests/%d?flash=updated", id), http.StatusSeeOther)
}

func (s *Server) lisTestDelete(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	_ = s.lisTests.Delete(r.Context(), id)
	http.Redirect(w, r, "/lis-tests?flash=deleted", http.StatusSeeOther)
}

func (s *Server) templateList(w http.ResponseWriter, r *http.Request) {
	kd := r.URL.Query().Get("kd_jenis_prw")
	q := r.URL.Query().Get("q")
	lisPK, _ := strconv.ParseUint(r.URL.Query().Get("lis_tests_pk"), 10, 64)
	panels, _ := s.simrs.ListPanels(r.Context(), "", "")
	templates, err := s.simrs.ListTemplates(r.Context(), kd, q, lisPK, 500)
	if err != nil {
		s.view(w, "templates.html", pageData{Title: "SIMRS Templates", Error: err.Error(), Panels: panels})
		return
	}
	s.view(w, "templates.html", pageData{
		Title:     "SIMRS Templates",
		Panels:    panels,
		Templates: templates,
		Query:     map[string]string{"kd_jenis_prw": kd, "q": q, "lis_tests_pk": r.URL.Query().Get("lis_tests_pk")},
	})
}

func (s *Server) panelList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	kategori := r.URL.Query().Get("kategori")
	if kategori == "" {
		kategori = "PK"
	}
	if kategori == "all" {
		kategori = ""
	}
	panels, err := s.simrs.ListPanels(r.Context(), q, kategori)
	katQuery := r.URL.Query().Get("kategori")
	if katQuery == "" {
		katQuery = "PK"
	}
	if err != nil {
		s.view(w, "panels.html", pageData{
			Title: "SIMRS Panels",
			Error: err.Error(),
			Query: map[string]string{"q": q, "kategori": katQuery},
		})
		return
	}
	s.view(w, "panels.html", pageData{
		Title:  "SIMRS Panels",
		Panels: panels,
		Query:  map[string]string{"q": q, "kategori": katQuery},
	})
}

func (s *Server) bulkMapForm(w http.ResponseWriter, r *http.Request) {
	tests, _ := s.lisTests.List(r.Context(), "", "aktif")
	lisPK, _ := strconv.ParseUint(r.URL.Query().Get("lis_tests_pk"), 10, 64)
	templates, err := s.simrs.ListTemplates(r.Context(), "", "", 0, 10000)
	if err != nil {
		s.view(w, "bulk_map.html", pageData{Title: "Bulk Map", Error: err.Error(), Tests: tests})
		return
	}
	s.view(w, "bulk_map.html", pageData{
		Title:     "Bulk Map",
		Tests:     tests,
		Templates: templates,
		Bulk:      bulkPageData{LisTestsPK: lisPK},
		Flash:     r.URL.Query().Get("flash"),
		Error:     r.URL.Query().Get("error"),
	})
}

func (s *Server) bulkMapSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	lisPK, _ := strconv.ParseUint(r.FormValue("lis_tests_pk"), 10, 64)
	var idTemplates []int
	for _, v := range r.Form["id_template"] {
		id, err := strconv.Atoi(v)
		if err == nil {
			idTemplates = append(idTemplates, id)
		}
	}
	redirectBack := func(errMsg string) {
		vals := url.Values{}
		if lisPK > 0 {
			vals.Set("lis_tests_pk", strconv.FormatUint(lisPK, 10))
		}
		if errMsg != "" {
			vals.Set("error", errMsg)
		}
		http.Redirect(w, r, "/map/bulk?"+vals.Encode(), http.StatusSeeOther)
	}
	if len(idTemplates) == 0 {
		redirectBack("Select at least one SIMRS template")
		return
	}
	if lisPK == 0 {
		redirectBack("Select a LIS test to map")
		return
	}
	res, err := s.mappings.BulkUpsert(r.Context(), lisPK, idTemplates, "web")
	if err != nil {
		redirectBack(err.Error())
		return
	}
	flash := fmt.Sprintf("Saved: %d created, %d updated, %d skipped", res.Created, res.Updated, res.Skipped)
	vals := url.Values{}
	vals.Set("flash", flash)
	vals.Set("lis_tests_pk", strconv.FormatUint(lisPK, 10))
	http.Redirect(w, r, "/map/bulk?"+vals.Encode(), http.StatusSeeOther)
}

func (s *Server) mappingList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	lisPK, _ := strconv.ParseUint(q.Get("lis_tests_pk"), 10, 64)
	idTpl, _ := strconv.Atoi(q.Get("id_template"))
	list, err := s.mappings.List(r.Context(), lisPK, idTpl, q.Get("kd_jenis_prw"), q.Get("status"))
	if err != nil {
		s.view(w, "mappings.html", pageData{Title: "Mappings", Error: err.Error()})
		return
	}
	tests, _ := s.lisTests.List(r.Context(), "", "aktif")
	s.view(w, "mappings.html", pageData{
		Title:    "Mappings",
		Mappings: list,
		Tests:    tests,
		Query:    map[string]string{"lis_tests_pk": q.Get("lis_tests_pk"), "id_template": q.Get("id_template"), "kd_jenis_prw": q.Get("kd_jenis_prw"), "status": q.Get("status")},
		Flash:    q.Get("flash"),
	})
}

func (s *Server) mappingDelete(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	_ = s.mappings.Deactivate(r.Context(), id)
	http.Redirect(w, r, "/mappings?flash=deactivated", http.StatusSeeOther)
}

func (s *Server) bridgingLogList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	status := r.URL.Query().Get("status")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	const limit = 50
	result, err := s.bridgingLogs.List(r.Context(), q, status, page, limit)
	if err != nil {
		s.view(w, "bridging_logs.html", pageData{Title: "Log Bridging", Error: err.Error()})
		return
	}
	hasNext := page*limit < result.Total
	s.view(w, "bridging_logs.html", pageData{
		Title:        "Log Bridging",
		BridgingLogs: result,
		Query:        map[string]string{"q": q, "status": status},
		PrevPage:     page - 1,
		NextPage:     page + 1,
		HasNext:      hasNext,
	})
}

func (s *Server) bridgingLogDetail(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	row, err := s.bridgingLogs.GetByID(r.Context(), id)
	if err != nil {
		s.view(w, "bridging_log_detail.html", pageData{Title: "Log Bridging", Error: err.Error()})
		return
	}
	if row == nil {
		http.NotFound(w, r)
		return
	}
	payload := row.PayloadJSON
	if payload != "" {
		var pretty any
		if json.Unmarshal([]byte(payload), &pretty) == nil {
			if b, err := json.MarshalIndent(pretty, "", "  "); err == nil {
				payload = string(b)
				row.PayloadJSON = payload
			}
		}
	}
	s.view(w, "bridging_log_detail.html", pageData{
		Title:       fmt.Sprintf("Log Bridging #%d", row.ID),
		BridgingLog: row,
	})
}
