package handler

import (
	"encoding/json"
	"net/http"

	"lis-khanza-mapper/internal/model"
	"lis-khanza-mapper/internal/resolve"
)

func (s *Server) resolveMappings(w http.ResponseWriter, r *http.Request) {
	var body resolve.Request
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"status": false, "code": 400, "message": "invalid json: " + err.Error(),
		})
		return
	}
	if len(body.Tests) > 500 {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"status": false, "code": 400, "message": "maximum 500 tests per request",
		})
		return
	}

	seen := make([]model.LisTest, 0, len(body.Tests))
	var testIDs, localCodes []string
	for _, item := range body.Tests {
		id := item.TestID.String()
		code := item.LocalCode.String()
		if id != "" {
			testIDs = append(testIDs, id)
			seen = append(seen, model.LisTest{
				LisTestID: id,
				LocalCode: code,
				TestName:  item.TestName.String(),
			})
		}
		if code != "" {
			localCodes = append(localCodes, code)
		}
	}
	if err := s.lisTests.UpsertSeen(r.Context(), seen); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"status": false, "code": 500, "message": err.Error(),
		})
		return
	}

	cands, err := s.mappings.ActiveCandidates(r.Context(), testIDs, localCodes)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"status": false, "code": 500, "message": err.Error(),
		})
		return
	}
	known, err := s.lisTests.ActiveIDs(r.Context(), testIDs)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"status": false, "code": 500, "message": err.Error(),
		})
		return
	}
	knownMap := map[string]bool{}
	for id := range known {
		knownMap[id] = true
	}

	results := make([]resolve.Result, 0, len(body.Tests))
	for _, item := range body.Tests {
		results = append(results, resolve.Pick(item, cands, knownMap))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": true,
		"code":   200,
		"data":   map[string]any{"results": results},
	})
}
