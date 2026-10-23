package workerapi

import "encoding/json"

type ChangeRunStateMessage struct {
	NewState RunState `json:"new_state"`
	Note     *string  `json:"note,omitempty"`
}

type PostIntentResultMessage struct {
	ResponseType string          `json:"response_type"`
	ResponseBody json.RawMessage `json:"response_body"`
}
