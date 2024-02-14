package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gorilla/mux"
)

type APIServer struct {
	r          *mux.Router
	serv       *http.Server
	kratosAddr string
}

func NewAPIServer(port int, kratosAddr string) *APIServer {
	r := mux.NewRouter()
	serv := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: r,
	}
	return &APIServer{r, serv, kratosAddr}
}

func (s *APIServer) Start() error {
	s.r.Path("/identities").Methods(http.MethodPost).HandlerFunc(s.identityCreate)
	return s.serv.ListenAndServe()
}

const identityCreateTmpl = `
{
  "credentials": {
    "password": {
      "config": {
        "password": "%s"
      }
    }
  },
  "schema_id": "user",
  "state": "active",
  "traits": {
    "username": "%s"
  }
}
`

type identityCreateReq struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

func (s *APIServer) identityCreate(w http.ResponseWriter, r *http.Request) {
	var req identityCreateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "request can not be parsed", http.StatusBadRequest)
		return
	}
	var buf bytes.Buffer
	fmt.Fprintf(&buf, identityCreateTmpl, req.Password, req.Username)
	resp, err := http.Post(s.identitiesEndpoint(), "application/json", &buf)
	if err != nil {
		http.Error(w, "failed", http.StatusInternalServerError)
		return
	} else if resp.StatusCode != http.StatusCreated {
		var buf bytes.Buffer
		if _, err := io.Copy(&buf, resp.Body); err != nil {
			http.Error(w, "failed to copy response body", http.StatusInternalServerError)
		} else {
			http.Error(w, buf.String(), resp.StatusCode)
		}
	}
}

func (s *APIServer) identitiesEndpoint() string {
	return fmt.Sprintf("%s/admin/identities", s.kratosAddr)
}
