package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"lis-khanza-mapper/internal/model"
	"lis-khanza-mapper/internal/repository"
)

func (s *Server) apiListLisTests(w http.ResponseWriter, r *http.Request) {
	list, err := s.lisTests.List(r.Context(), r.URL.Query().Get("q"), r.URL.Query().Get("status"))
	if err != nil {
		writeAPI(w, http.StatusInternalServerError, nil, err.Error())
		return
	}
	writeAPI(w, http.StatusOK, list, "")
}

func (s *Server) apiCreateLisTest(w http.ResponseWriter, r *http.Request) {
	var body struct {
		LisTestID string `json:"lis_test_id"`
		TestName  string `json:"test_name"`
		LocalCode string `json:"local_code"`
		Status    string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeAPI(w, http.StatusBadRequest, nil, "invalid json")
		return
	}
	t := &model.LisTest{LisTestID: body.LisTestID, TestName: body.TestName, LocalCode: body.LocalCode, Status: "aktif"}
	if body.Status != "" {
		t.Status = body.Status
	}
	if t.LisTestID == "" || t.TestName == "" {
		writeAPI(w, http.StatusBadRequest, nil, "lis_test_id and test_name required")
		return
	}
	if err := s.lisTests.Create(r.Context(), t); err != nil {
		writeAPI(w, http.StatusBadRequest, nil, err.Error())
		return
	}
	writeAPI(w, http.StatusCreated, t, "")
}

func (s *Server) apiGetLisTest(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	t, err := s.lisTests.GetByID(r.Context(), id)
	if err != nil {
		writeAPI(w, http.StatusInternalServerError, nil, err.Error())
		return
	}
	if t == nil {
		writeAPI(w, http.StatusNotFound, nil, "not found")
		return
	}
	writeAPI(w, http.StatusOK, t, "")
}

func (s *Server) apiUpdateLisTest(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	t, err := s.lisTests.GetByID(r.Context(), id)
	if err != nil || t == nil {
		writeAPI(w, http.StatusNotFound, nil, "not found")
		return
	}
	var body struct {
		TestName  string `json:"test_name"`
		LocalCode string `json:"local_code"`
		Status    string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeAPI(w, http.StatusBadRequest, nil, "invalid json")
		return
	}
	t.TestName = body.TestName
	t.LocalCode = body.LocalCode
	if body.Status != "" {
		t.Status = body.Status
	}
	if err := s.lisTests.Update(r.Context(), t); err != nil {
		writeAPI(w, http.StatusInternalServerError, nil, err.Error())
		return
	}
	writeAPI(w, http.StatusOK, t, "")
}

func (s *Server) apiDeleteLisTest(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err := s.lisTests.Delete(r.Context(), id); err != nil {
		writeAPI(w, http.StatusInternalServerError, nil, err.Error())
		return
	}
	writeAPI(w, http.StatusOK, nil, "")
}

func (s *Server) apiListPanels(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	kategori := r.URL.Query().Get("kategori")
	list, err := s.simrs.ListPanels(r.Context(), q, kategori)
	if err != nil {
		writeAPI(w, http.StatusInternalServerError, nil, err.Error())
		return
	}
	writeAPI(w, http.StatusOK, list, "")
}

func (s *Server) apiListTemplates(w http.ResponseWriter, r *http.Request) {
	kd := r.URL.Query().Get("kd_jenis_prw")
	q := r.URL.Query().Get("q")
	lisPK, _ := strconv.ParseUint(r.URL.Query().Get("lis_tests_pk"), 10, 64)
	list, err := s.simrs.ListTemplates(r.Context(), kd, q, lisPK, 500)
	if err != nil {
		writeAPI(w, http.StatusInternalServerError, nil, err.Error())
		return
	}
	writeAPI(w, http.StatusOK, list, "")
}

func (s *Server) apiListMappings(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	lisPK, _ := strconv.ParseUint(q.Get("lis_tests_pk"), 10, 64)
	if lisPK == 0 && q.Get("lis_test_id") != "" {
		var err error
		lisPK, err = repository.FilterLisTestsPKByLisTestID(r.Context(), s.db, q.Get("lis_test_id"))
		if err != nil {
			writeAPI(w, http.StatusInternalServerError, nil, err.Error())
			return
		}
	}
	idTpl, _ := strconv.Atoi(q.Get("id_template"))
	list, err := s.mappings.List(r.Context(), lisPK, idTpl, q.Get("kd_jenis_prw"), q.Get("status"))
	if err != nil {
		writeAPI(w, http.StatusInternalServerError, nil, err.Error())
		return
	}
	writeAPI(w, http.StatusOK, list, "")
}

func (s *Server) apiBulkMap(w http.ResponseWriter, r *http.Request) {
	var body struct {
		LisTestsPK  uint64 `json:"lis_tests_pk"`
		IDTemplates []int  `json:"id_templates"`
		CreatedBy   string `json:"created_by"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeAPI(w, http.StatusBadRequest, nil, "invalid json")
		return
	}
	if body.LisTestsPK == 0 || len(body.IDTemplates) == 0 {
		writeAPI(w, http.StatusBadRequest, nil, "lis_tests_pk and id_templates required")
		return
	}
	by := body.CreatedBy
	if by == "" {
		by = "api"
	}
	res, err := s.mappings.BulkUpsert(r.Context(), body.LisTestsPK, body.IDTemplates, by)
	if err != nil {
		writeAPI(w, http.StatusBadRequest, nil, err.Error())
		return
	}
	writeAPI(w, http.StatusOK, res, "")
}

func (s *Server) apiDeleteMapping(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if r.URL.Query().Get("hard") == "1" {
		_ = s.mappings.Delete(r.Context(), id)
	} else {
		_ = s.mappings.Deactivate(r.Context(), id)
	}
	writeAPI(w, http.StatusOK, nil, "")
}
