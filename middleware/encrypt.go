// Package middleware provides Gin middleware functions.
package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"syncspace-backend/pkg/crypto"
	"syncspace-backend/pkg/utils"
	"syncspace-backend/service/data"
	"syncspace-backend/service/session"
	"syncspace-backend/constants"
)

// encryptedPayload is the wire format for all encrypted requests and responses.
//
//	{ "data": "<base64(nonce || ciphertext || gcm-tag)>" }
type encryptedPayload struct {
	Data string `json:"data"`
}

// responseCapture wraps gin.ResponseWriter so we can intercept the bytes
// written by the controller and encrypt them before flushing to the client.
type responseCapture struct {
	gin.ResponseWriter
	body       *bytes.Buffer
	statusCode int
}

func (rc *responseCapture) Write(b []byte) (int, error) {
	return rc.body.Write(b)
}

func (rc *responseCapture) WriteHeader(code int) {
	rc.statusCode = code
}

// Status returns the captured HTTP status code (defaults to 200 if never set).
func (rc *responseCapture) Status() int {
	if rc.statusCode == 0 {
		return http.StatusOK
	}
	return rc.statusCode
}

// Encryption returns a Gin middleware that:
//  1. Looks up the session key for the requesting user (via X-User-ID header).
//  2. Decrypts the incoming request body from {"data":"<base64>"} to plaintext JSON.
//  3. After the controller writes its response, encrypts that response back to
//     {"data":"<base64>"} before it reaches the client.
//
// When JWT auth middleware is added, replace the X-User-ID header lookup with
// a context value set by the JWT middleware.
func Encryption(store *session.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetHeader("X-User-ID")
		if userID == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, data.Fail("missing X-User-ID header"))
			return
		}

		key, ok := store.GetKey(userID)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, data.Fail("no active session — please log in"))
			return
		}

		// ── Decrypt request body ──────────────────────────────────────────────
		var req encryptedPayload
		if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, data.Fail("request must be {\"data\":\"<base64>\"}"))
			return
		}
		if req.Data == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, data.Fail("encrypted data field is empty"))
			return
		}

		plaintext, err := crypto.Decrypt(key, req.Data)
		if err != nil {
			utils.Error(constants.LogTagCrypto, "request decryption failed", err)
			c.AbortWithStatusJSON(http.StatusBadRequest, data.Fail("decryption failed"))
			return
		}

		// Replace the request body with the decrypted plaintext so controllers
		// can bind it normally via c.ShouldBindJSON / json.NewDecoder.
		c.Request.Body = io.NopCloser(bytes.NewReader(plaintext))
		c.Request.ContentLength = int64(len(plaintext))

		// ── Capture response ──────────────────────────────────────────────────
		capture := &responseCapture{
			ResponseWriter: c.Writer,
			body:           &bytes.Buffer{},
		}
		c.Writer = capture

		c.Next()

		// ── Encrypt response ──────────────────────────────────────────────────
		encrypted, err := crypto.Encrypt(key, capture.body.Bytes())
		if err != nil {
			utils.Error(constants.LogTagCrypto, "response encryption failed", err)
			// Write error directly to the underlying writer — we can't use
			// c.JSON here because the writer is still the capture wrapper.
			capture.ResponseWriter.Header().Set("Content-Type", "application/json")
			capture.ResponseWriter.WriteHeader(http.StatusInternalServerError)
			errBody, _ := json.Marshal(data.Fail("response encryption failed"))
			capture.ResponseWriter.Write(errBody) //nolint:errcheck
			return
		}

		resp, _ := json.Marshal(encryptedPayload{Data: encrypted})
		capture.ResponseWriter.Header().Set("Content-Type", "application/json")
		capture.ResponseWriter.WriteHeader(capture.Status())
		capture.ResponseWriter.Write(resp) //nolint:errcheck
	}
}
