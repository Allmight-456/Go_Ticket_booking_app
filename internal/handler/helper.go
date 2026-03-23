package handler

import (
	"encoding/json"
	"net/http"
)

// render writes a JSON response with the given status code.
func render(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// renderError writes a structured JSON error response.
func renderError(w http.ResponseWriter, status int, msg string) {
	render(w, status, map[string]string{"error": msg})
}

// decode deserialises the request body into v and returns a bool indicating success.
// On failure it writes a 400 response automatically.
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		renderError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return false
	}
	return true
}
