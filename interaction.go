package main

import (
	"encoding/json"
	"github.com/nlopes/slack"
	"io/ioutil"
	"net/http"
	"net/url"
	"fmt"
)

type interactionHandler struct {
	slackClient       *slack.Client
	verificationToken string
}

func (h interactionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		sugar.Errorf("Invalid method: %s", r.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		sugar.Errorf("Failed to read request body: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	jsonStr, err := url.QueryUnescape(string(buf)[8:])
	if err != nil {
		sugar.Errorf("Failed to un-escape request body: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var message slack.AttachmentActionCallback
	if err := json.Unmarshal([]byte(jsonStr), &message); err != nil {
		sugar.Errorf("Failed to decode json message from slack: %s", jsonStr)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if message.Token != h.verificationToken {
		sugar.Errorf("Invalid token: %s", message.Token)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	action := message.Actions[0]
	switch action.Name {
	case actionIn:
		title := ":ok: You have punched in for today."
		err := PunchIn(message.User.ID)
		if err != nil {
			title = fmt.Sprintf(":warning: Error occurred: %s", err)
			sugar.Errorf("error occurred: %s", err)
		}
		responseMessage(w, message.OriginalMessage, title, "")
		return
	case actionOut:
		title := ":ok: You have punched out for today."
		err := PunchOut(message.User.ID)
		if err != nil {
			title = fmt.Sprintf(":warning: Error occurred: %s", err)
			sugar.Errorf("error occurred: %s", err)
		}
		responseMessage(w, message.OriginalMessage, title, "")
		return
	case actionLeave:
		title := ":ok: You are off today. Enjoy :tada:"
		err := PunchLeave(message.User.ID)
		if err != nil {
			title = fmt.Sprintf(":warning: Error occurred: %s", err)
			sugar.Errorf("error occurred: %s", err)
		}
		responseMessage(w, message.OriginalMessage, title, "")
		return
	case actionCancel:
		responseMessage(w, message.OriginalMessage, "Operation canceled.", "")
	default:
		sugar.Errorf("Invalid action was submitted: %s", action.Name)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func responseMessage(w http.ResponseWriter, original slack.Message, title, value string) {
	original.Attachments[0].Actions = []slack.AttachmentAction{}
	original.Attachments[0].Fields = []slack.AttachmentField{
		{
			Title: title,
			Value: value,
			Short: false,
		},
	}

	w.Header().Add("Content-type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(&original)
}

func responseAction(w http.ResponseWriter, original slack.Message, text string, actions []slack.AttachmentAction) {
	original.Attachments[0].Text = text
	original.Attachments[0].Actions = actions

	w.Header().Add("Content-type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(&original)
}

func responseError(w http.ResponseWriter, original slack.Message, title, value string) {
	original.Attachments[0].Actions = []slack.AttachmentAction{}
	original.Attachments[0].Fields = []slack.AttachmentField{
		{
			Title: title,
			Value: value,
			Short: false,
		},
	}

	w.Header().Add("Content-type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	json.NewEncoder(w).Encode(&original)
}
