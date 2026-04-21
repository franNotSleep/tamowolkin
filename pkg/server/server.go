package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/frannotsleep/tamowolkin/pkg/queue"
)

const (
	maxWebhookBodyBytes = 1 << 20
)

type Server struct {
	port          int
	webhookSecret string
	linearEmail   string
	queue         *queue.Queue
}

func NewServer(port int, webhookSecret string, queue *queue.Queue, linearEmail string) *Server {
	return &Server{port: port, webhookSecret: webhookSecret, queue: queue, linearEmail: linearEmail}
}

func (srv *Server) StartServer() error {
	addr := fmt.Sprintf(":%d", srv.port)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /webhook", srv.linearWebhookHandler())

	return http.ListenAndServe(addr, mux)
}

func (srv *Server) linearWebhookHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		req.Body = http.MaxBytesReader(w, req.Body, maxWebhookBodyBytes)
		body, err := io.ReadAll(req.Body)
		if err != nil {
			log.Printf("linear webhook: read body from %s: %v", req.RemoteAddr, err)
			http.Error(w, "failed to read body", http.StatusBadRequest)
			return
		}

		signature := req.Header.Get("Linear-Signature")
		if signature == "" || !srv.verifyLinearSignature(body, signature, srv.webhookSecret) {
			log.Printf("linear webhook: invalid signature from %s (header present=%t)", req.RemoteAddr, signature != "")
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}

		var evt LinearWebhook
		if err := json.Unmarshal(body, &evt); err != nil {
			log.Printf("linear webhook: decode envelope from %s: %v", req.RemoteAddr, err)
			http.Error(w, "invalid payload", http.StatusBadRequest)
			return
		}

		switch evt.Type {
		case "Issue":
			var issue LinearIssue
			if err := json.Unmarshal(evt.Data, &issue); err != nil {
				log.Printf("linear webhook: decode Issue data (id=%s): %v", evt.WebhookID, err)
				http.Error(w, "invalid issue payload", http.StatusBadRequest)
				return
			}

			if issue.Assignee == nil ||  issue.Assignee.Email != srv.linearEmail {
				w.WriteHeader(http.StatusOK)
				return
			}

			srv.queue.Enqueue(issue.Identifier, issue.Title, issue.Description, issue.BranchName)
		default:
			log.Printf("linear webhook: unhandled type=%s action=%s", evt.Type, evt.Action)
		}

		w.WriteHeader(http.StatusOK)
	}
}

func (srv *Server) verifyLinearSignature(body []byte, signature, secret string) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return subtle.ConstantTimeCompare([]byte(expected), []byte(signature)) == 1
}
