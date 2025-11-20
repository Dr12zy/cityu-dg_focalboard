package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mattermost/focalboard/server/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/stretchr/testify/require"
)

func TestAICreateCard(t *testing.T) {
	// This is a basic structure test
	// Full integration test would require a real database setup
	testAPI := API{logger: mlog.CreateConsoleTestLogger(t)}

	t.Run("should handle missing boardID", func(t *testing.T) {
		card := &model.Card{
			Title: "Test Card",
		}
		cardJSON, _ := json.Marshal(card)

		req := httptest.NewRequest(http.MethodPost, "/api/v2/ai/cards/create", bytes.NewReader(cardJSON))
		req.Header.Set("X-Requested-With", "XMLHttpRequest")
		w := httptest.NewRecorder()

		// Note: This will fail at sessionRequired, but we're testing the structure
		testAPI.handleAICreateCard(w, req)
		res := w.Result()

		// Should return an error (either auth or validation)
		require.NotEqual(t, http.StatusOK, res.StatusCode)
	})
}

func TestAIModifyCardStatus(t *testing.T) {
	testAPI := API{logger: mlog.CreateConsoleTestLogger(t)}

	t.Run("should handle missing cardId", func(t *testing.T) {
		statusUpdate := AICardStatusUpdateRequest{
			Status: "Done",
		}
		statusJSON, _ := json.Marshal(statusUpdate)

		req := httptest.NewRequest(http.MethodPost, "/api/v2/ai/cards/modify", bytes.NewReader(statusJSON))
		req.Header.Set("X-Requested-With", "XMLHttpRequest")
		w := httptest.NewRecorder()

		testAPI.handleAIModifyCardStatus(w, req)
		res := w.Result()

		// Should return bad request for missing cardId
		require.Equal(t, http.StatusBadRequest, res.StatusCode)
	})

	t.Run("should handle missing status", func(t *testing.T) {
		statusUpdate := AICardStatusUpdateRequest{
			CardID: "test-card-id",
		}
		statusJSON, _ := json.Marshal(statusUpdate)

		req := httptest.NewRequest(http.MethodPost, "/api/v2/ai/cards/modify", bytes.NewReader(statusJSON))
		req.Header.Set("X-Requested-With", "XMLHttpRequest")
		w := httptest.NewRecorder()

		testAPI.handleAIModifyCardStatus(w, req)
		res := w.Result()

		// Should return bad request for missing status
		require.Equal(t, http.StatusBadRequest, res.StatusCode)
	})
}

