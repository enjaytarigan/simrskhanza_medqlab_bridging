package handler

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"time"

	"lis-khanza-mapper/internal/medqlab"
)

const maxWebhookBody = 16 << 20 // 16 MiB

func (s *Server) medqlabHasil(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Standalone() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"status":  false,
			"code":    503,
			"message": "hasil push is disabled in standalone mode",
		})
		return
	}
	start := time.Now()
	body, err := io.ReadAll(io.LimitReader(r.Body, maxWebhookBody))
	if err != nil {
		log.Printf("[medqlab-webhook] read body failed: %v", err)
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"status": false, "code": 400, "message": "cannot read body",
		})
		return
	}
	defer r.Body.Close()

	log.Printf("[medqlab-webhook] POST /api/v1/medqlab/hasil received bytes=%d remote=%s", len(body), r.RemoteAddr)

	var env medqlab.Envelope
	if err := json.Unmarshal(body, &env); err != nil {
		log.Printf("[medqlab-webhook] invalid json: %v", err)
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"status": false, "code": 400, "message": "invalid json: " + err.Error(),
		})
		return
	}
	if env.Response == nil {
		log.Printf("[medqlab-webhook] missing response object")
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"status": false, "code": 400, "message": "missing response object",
		})
		return
	}

	metaCode := 0
	metaMsg := ""
	if env.MetaData != nil {
		metaCode = env.MetaData.Code
		metaMsg = env.MetaData.Message
	}
	log.Printf("[medqlab-webhook] payload metaData.code=%d message=%q noOrder=%q noLaboratorium=%q visitNumber=%q examinations=%d",
		metaCode, metaMsg,
		env.Response.NoOrder, env.Response.NoLaboratorium,
		medqlabVisitHint(env.Response),
		len(env.Response.Examinations),
	)

	result, err := s.medqlab.ProcessEnvelope(r.Context(), &env, body)
	if err != nil {
		code, msg := mapMedQLabError(err)
		log.Printf("[medqlab-webhook] FAILED status=%d message=%q duration=%s err=%v",
			code, msg, time.Since(start).Round(time.Millisecond), err)
		writeJSON(w, code, map[string]any{
			"status": false, "code": code, "message": msg,
		})
		return
	}

	log.Printf("[medqlab-webhook] OK inbox_id=%d medqlab_order=%q no_order=%q no_rawat=%q mapped=%d unmapped=%d panels=%d details=%d tgl=%s jam=%s duration=%s",
		result.InboxID, result.MedQLabOrder, result.NoOrder, result.NoRawat,
		result.Mapped, result.Unmapped, result.PanelsWritten, result.DetailWritten,
		result.TglPeriksa, result.JamPeriksa, time.Since(start).Round(time.Millisecond),
	)

	writeJSON(w, http.StatusOK, map[string]any{
		"status":  true,
		"code":    200,
		"message": "Hasil laboratorium berhasil diposting ke SIMRS",
		"data":    result,
	})
}

func medqlabVisitHint(resp *medqlab.Response) string {
	if resp == nil {
		return ""
	}
	if resp.Demographics != nil {
		if v := resp.Demographics.VisitNumber; v != "" {
			return v
		}
		if v := resp.Demographics.VisitNumberSnake; v != "" {
			return v
		}
	}
	return resp.NoPendaftaran
}

func mapMedQLabError(err error) (int, string) {
	switch {
	case errors.Is(err, medqlab.ErrOrderNotFound):
		return http.StatusNotFound, err.Error()
	case errors.Is(err, medqlab.ErrNoRawatRequired):
		return http.StatusBadRequest, err.Error()
	case errors.Is(err, medqlab.ErrRegistrationClosed):
		return http.StatusConflict, err.Error()
	case errors.Is(err, medqlab.ErrNoMappedResults):
		return http.StatusUnprocessableEntity, err.Error()
	case errors.Is(err, medqlab.ErrNoValidatedAt):
		return http.StatusUnprocessableEntity, err.Error()
	case errors.Is(err, medqlab.ErrNIPRequired):
		return http.StatusServiceUnavailable, err.Error()
	default:
		return http.StatusInternalServerError, err.Error()
	}
}
