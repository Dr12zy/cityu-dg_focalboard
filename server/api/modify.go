package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/mattermost/focalboard/server/model"
	"github.com/mattermost/focalboard/server/services/audit"

	"github.com/mattermost/mattermost/server/public/shared/mlog"
)

// AICardStatusUpdateRequest represents the request body for updating card status
type AICardStatusUpdateRequest struct {
	CardID string `json:"cardId"`
	Status string `json:"status"`
}

func (a *API) registerAIModifyCardRoutes(r *mux.Router) {
	// AI Card Status Modification API
	r.HandleFunc("/ai/cards/modify", a.sessionRequired(a.handleAIModifyCardStatus)).Methods("POST")
}

func (a *API) handleAIModifyCardStatus(w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /ai/cards/modify aiModifyCardStatus
	//
	// Modifies the status of a card for AI system.
	//
	// ---
	// produces:
	// - application/json
	// parameters:
	// - name: Body
	//   in: body
	//   description: the card status update request
	//   required: true
	//   schema:
	//     type: object
	//     required:
	//       - cardId
	//       - status
	//     properties:
	//       cardId:
	//         type: string
	//         description: The ID of the card to update
	//       status:
	//         type: string
	//         description: The new status value
	// - name: disable_notify
	//   in: query
	//   description: Disables notifications (for bulk data patching)
	//   required: false
	//   type: bool
	// security:
	// - BearerAuth: []
	// responses:
	//   '200':
	//     description: success
	//     schema:
	//       $ref: '#/definitions/Card'
	//   default:
	//     description: internal error
	//     schema:
	//       "$ref": "#/definitions/ErrorResponse"

	userID := getUserID(r)

	val := r.URL.Query().Get("disable_notify")
	disableNotify := val == True

	requestBody, err := io.ReadAll(r.Body)
	if err != nil {
		a.errorResponse(w, r, err)
		return
	}

	var statusUpdate AICardStatusUpdateRequest
	if err = json.Unmarshal(requestBody, &statusUpdate); err != nil {
		a.errorResponse(w, r, model.NewErrBadRequest(err.Error()))
		return
	}

	if statusUpdate.CardID == "" {
		a.errorResponse(w, r, model.NewErrBadRequest("cardId is required"))
		return
	}

	if statusUpdate.Status == "" {
		a.errorResponse(w, r, model.NewErrBadRequest("status is required"))
		return
	}

	card, err := a.app.GetCardByID(statusUpdate.CardID)
	if err != nil {
		message := fmt.Sprintf("could not fetch card %s: %s", statusUpdate.CardID, err)
		a.errorResponse(w, r, model.NewErrBadRequest(message))
		return
	}

	if !a.permissions.HasPermissionToBoard(userID, card.BoardID, model.PermissionManageBoardCards) {
		a.errorResponse(w, r, model.NewErrPermission("access denied to modify card"))
		return
	}

	// Get the board to find the status property ID
	board, err := a.app.GetBoard(card.BoardID)
	if err != nil {
		a.errorResponse(w, r, model.NewErrBadRequest(fmt.Sprintf("could not fetch board %s: %s", card.BoardID, err)))
		return
	}

	// Find the status property ID from board's cardProperties
	var statusPropertyID string
	if board.CardProperties != nil {
		for _, prop := range board.CardProperties {
			if name, ok := prop["name"].(string); ok && name == "Status" {
				if id, ok := prop["id"].(string); ok {
					statusPropertyID = id
					break
				}
			}
		}
	}

	if statusPropertyID == "" {
		a.errorResponse(w, r, model.NewErrBadRequest("status property not found in board"))
		return
	}

	// Create a CardPatch to update the status property
	patch := &model.CardPatch{
		UpdatedProperties: make(map[string]any),
	}
	patch.UpdatedProperties[statusPropertyID] = statusUpdate.Status

	auditRec := a.makeAuditRecord(r, "aiModifyCardStatus", audit.Fail)
	defer a.audit.LogRecord(audit.LevelModify, auditRec)
	auditRec.AddMeta("boardID", card.BoardID)
	auditRec.AddMeta("cardID", card.ID)
	auditRec.AddMeta("status", statusUpdate.Status)

	// patch card
	cardPatched, err := a.app.PatchCard(patch, card.ID, userID, disableNotify)
	if err != nil {
		a.errorResponse(w, r, err)
		return
	}

	a.logger.Debug("AIModifyCardStatus",
		mlog.String("boardID", cardPatched.BoardID),
		mlog.String("cardID", cardPatched.ID),
		mlog.String("userID", userID),
		mlog.String("status", statusUpdate.Status),
	)

	data, err := json.Marshal(cardPatched)
	if err != nil {
		a.errorResponse(w, r, err)
		return
	}

	// response
	jsonBytesResponse(w, http.StatusOK, data)

	auditRec.Success()
}

