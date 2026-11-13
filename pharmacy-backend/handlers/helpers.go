package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5/pgconn"
)

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	writeJSON(w, status, data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{
		"error": message,
	})
}

func parseID(r *http.Request) (int, error) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		return 0, errors.New("invalid ID")
	}
	return id, nil
}

func decodeJSON(r *http.Request, destination interface{}) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(destination); err != nil {
		return err
	}

	var extra interface{}
	if err := decoder.Decode(&extra); err != io.EOF {
		return errors.New("invalid JSON body")
	}

	return nil
}

func writeDBError(w http.ResponseWriter, err error) {
	var pgErr *pgconn.PgError

	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23503":
			writeError(w, http.StatusConflict,
				"Related record does not exist or is still in use")
			return
		case "23505":
			writeError(w, http.StatusConflict,
				"Duplicate data")
			return
		case "23514", "23502":
			writeError(w, http.StatusBadRequest,
				"Data violates database constraints")
			return
		}
	}

	writeError(w, http.StatusInternalServerError, "Database error")
}
