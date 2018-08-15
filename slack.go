package main

import (
	"fmt"
	"strings"

	"encoding/json"
	"github.com/nlopes/slack"
	"io/ioutil"
	"os"
	"time"
	"unicode/utf8"
)

const (
	actionIn     = "in"
	actionOut    = "out"
	actionLeave  = "leave"
	actionCancel = "cancel"

	callbackID  = "punch"
	helpMessage = "```\nUsage:\n\tIntegration:\n\t\tauth\n\t\tadd [emp_id]\n\n\tDeintegration\n\t\tremove\n\n\tCheck In:\n\t\tin\n\t\tin now\n\t\tin 0930\n\n\tCheck Out:\n\t\tout\n\t\tout now\n\t\tout 1810\n\n\tOff:\n\t\tleave\n\t\toff\n\n\tDebug:\n\t\tme```"
)

type SlackListener struct {
	client *slack.Client
	botID  string
}

func (s *SlackListener) ListenAndResponse() {
	rtm := s.client.NewRTM()
	go rtm.ManageConnection()

	for msg := range rtm.IncomingEvents {
		switch ev := msg.Data.(type) {
		case *slack.MessageEvent:
			if err := s.handleMessageEvent(ev); err != nil {
				s.respond(ev.Channel, fmt.Sprintf("%s", err))
				sugar.Errorf("Failed to handle message: %s", err)
			}
		}
	}
}

func (s *SlackListener) handleMessageEvent(ev *slack.MessageEvent) error {
	if ev.Msg.SubType == "bot_message" {
		return nil
	}

	isDirectMessageChannel := strings.HasPrefix(ev.Msg.Channel, "D")
	if isDirectMessageChannel && ev.Msg.Text == "auth" {
		authURL := AuthCodeURL()
		return s.respond(ev.Channel, fmt.Sprintf("Please open the following URL in your browser:\n%s", authURL))
	}
	if isDirectMessageChannel && (strings.HasPrefix(ev.Msg.Text, "register") || strings.HasPrefix(ev.Msg.Text, "add")) {
		split := strings.Fields(ev.Msg.Text)

		var employeeID string
		code := ""
		if len(split) == 2 {
			employeeID = split[1]
		} else if len(split) == 3 {
			employeeID = split[1]
			code = split[2]
			if utf8.RuneCountInString(code) != 64 {
				return s.respond(ev.Channel, "Invalid authorization code.")
			}
		} else {
			return s.respond(ev.Channel, "Invalid parameters.")
		}

		var user User
		if code != "" {
			token, err := Token(code)
			if err != nil {
				return err
			}
			user = User{
				SlackUserID:    ev.Msg.User,
				SlackChannelID: ev.Channel,
				EmployeeID:     employeeID,
				Token:          *token,
			}
		} else {
			user = User{
				SlackUserID:    ev.Msg.User,
				SlackChannelID: ev.Channel,
				EmployeeID:     employeeID,
			}
		}

		text, err := json.Marshal(user)
		if err != nil {
			return err
		}

		err = ioutil.WriteFile(fmt.Sprintf("users/%s", ev.Msg.User), text, 0644)
		if err != nil {
			return err
		}

		if code != "" {
			return s.respond(ev.Channel, ":ok: Saved your access token successfully.")
		} else {
			return s.respond(ev.Channel, ":ok: Saved your employee ID successfully.")
		}
	}
	if isDirectMessageChannel && (ev.Msg.Text == "unregister" || ev.Msg.Text == "remove") {
		err := os.Remove(fmt.Sprintf("users/%s", ev.Msg.User))
		if err != nil {
			s.respond(ev.Channel, fmt.Sprintf(":warning: Failed to remove '%s'.", ev.User))
			return err
		}

		return s.respond(ev.Channel, fmt.Sprintf(":ok: '%s' was removed successfully.", ev.User))
	}
	if isDirectMessageChannel && (strings.HasPrefix(ev.Msg.Text, "admin register") || strings.HasPrefix(ev.Msg.Text, "admin add")) {
		split := strings.Fields(ev.Msg.Text)
		if len(split) != 3 {
			return s.respond(ev.Channel, "Invalid parameters.")
		}

		code := split[2]
		if utf8.RuneCountInString(code) != 64 {
			return s.respond(ev.Channel, "Invalid authorization code.")
		}

		token, err := Token(code)
		if err != nil {
			return err
		}

		user := User{
			SlackUserID:    ev.Msg.User,
			SlackChannelID: ev.Channel,
			EmployeeID:     "admin",
			Token:          *token,
		}

		text, err := json.Marshal(user)
		if err != nil {
			return err
		}

		err = ioutil.WriteFile(fmt.Sprintf("users/%s", "admin"), text, 0644)
		if err != nil {
			return err
		}

		return s.respond(ev.Channel, ":ok: Saved the admin access token successfully.")
	}
	if isDirectMessageChannel && (ev.Msg.Text == "in" || ev.Msg.Text == "out") {
		if _, _, err := s.client.PostMessage(ev.Channel, "", checkInOptions()); err != nil {
			return fmt.Errorf("failed to post message: %s", err)
		}
		return nil
	}
	if isDirectMessageChannel && (strings.HasPrefix(ev.Msg.Text, "in") || strings.HasPrefix(ev.Msg.Text, "out")) {
		split := strings.Fields(ev.Msg.Text)
		if len(split) != 2 {
			return s.respond(ev.Channel, "Invalid parameters.")
		}

		var clock time.Time
		timeParam := split[1]
		if timeParam == "now" {
			clock = time.Now()
		} else {
			t, err := time.Parse("1504", timeParam)
			if err != nil {
				return s.respond(ev.Channel, "Invalid parameters.")
			}
			now := time.Now()
			tokyoTime := time.FixedZone("Asia/Tokyo", 9*60*60)
			clock = time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, tokyoTime)
		}

		if split[0] == "in" {
			responseText := ":ok: You have punched in for today."
			err := PunchInAt(ev.Msg.User, clock)
			if err != nil {
				responseText = fmt.Sprintf(":warning: Error occurred: %s", err)
				sugar.Errorf("error occurred: %s", err)
			}
			return s.respond(ev.Channel, responseText)
		} else {
			responseText := ":ok: You have punched out for today."
			err := PunchOutAt(ev.Msg.User, clock)
			if err != nil {
				responseText = fmt.Sprintf(":warning: Error occurred: %s", err)
				sugar.Errorf("error occurred: %s", err)
			}
			return s.respond(ev.Channel, responseText)
		}
	}
	if isDirectMessageChannel && (ev.Msg.Text == "leave" || ev.Msg.Text == "off") {
		responseText := ":ok: You are off today. Enjoy :tada:"
		err := PunchLeave(ev.Msg.User)
		if err != nil {
			responseText = fmt.Sprintf(":warning: Error occurred: %s", err)
			sugar.Errorf("error occurred: %s", err)
		}
		return s.respond(ev.Channel, responseText)
	}

	if ev.Msg.Text == "ping" {
		return s.respond(ev.Channel, "pong")
	}
	if ev.Msg.Text == "me" {
		text, err := Me(ev.Msg.User)
		if err != nil {
			text = fmt.Sprintf(":warning: Error occurred: %s", err)
			sugar.Errorf("error occurred: %s", err)
		}
		return s.respond(ev.Channel, text)
	}
	if ev.Msg.Text == "help" {
		return s.respond(ev.Channel, helpMessage)
	}

	return nil
}

func (s *SlackListener) respond(channel string, text string) error {
	_, _, err := s.client.PostMessage(channel, text, slack.NewPostMessageParameters())
	return err
}

func checkInOptions() slack.PostMessageParameters {
	attachment := slack.Attachment{
		Text:       time.Now().Format("2006/01/02 15:04"),
		CallbackID: callbackID,
		Actions: []slack.AttachmentAction{
			{
				Name: actionIn,
				Text: "Punch in",
				Type: "button",
				Style: "primary",
			},
			{
				Name: actionOut,
				Text: "Punch out",
				Type: "button",
				Style: "primary",
			},
			{
				Name:  actionLeave,
				Text:  "Leave",
				Type:  "button",
				Style: "danger",
			},
			{
				Name:  actionCancel,
				Text:  "Cancel",
				Type:  "button",
			},
		},
	}
	parameters := slack.PostMessageParameters{
		Attachments: []slack.Attachment{
			attachment,
		},
	}
	return parameters
}

func (s *SlackListener) sendReminderMessage() error {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			location := time.FixedZone("Asia/Tokyo", 9*60*60)
			now := time.Now().In(location)
			if (now.Hour() == 9 && now.Minute() == 0) || (now.Hour() == 17 && now.Minute() == 0) {
				fileInfo, err := ioutil.ReadDir("users")
				if err != nil {
					return err
				}

				for _, file := range fileInfo {
					userID := file.Name()
					if userID == "admin" {
						continue
					}
					user, err := findUser(userID)
					if err != nil {
						continue
					}
					parameters := checkInOptions()

					if _, _, err := s.client.PostMessage(user.SlackChannelID, "", parameters); err != nil {
						return fmt.Errorf("failed to post message: %s", err)
					}
				}
			}
		}
	}
}
