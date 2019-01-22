package admission

import (
	"net/http"
)

// handleHealthz handles requests to the "/healthz" endpoint by responding with "200 OK".
func handleHealthz(res http.ResponseWriter, _ *http.Request) {
	res.WriteHeader(http.StatusOK)
}
