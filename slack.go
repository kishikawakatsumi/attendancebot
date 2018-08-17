package main

import (
	"fmt"
	"strings"

	"io/ioutil"
	"os"
	"time"
	"unicode/utf8"

	"encoding/json"
	"github.com/nlopes/slack"
)

const (
	actionIn     = "in"
	actionOut    = "out"
	actionLeave  = "leave"
	actionCancel = "cancel"

	callbackID  = "punch"
	helpMessage = "```\n" +
		`Usage:
	Integration:
		auth
		add [emp_id]

	Deintegration
		remove

	Check In:
		in
		in now
		in 0930

	Check Out:
		out
		out now
		out 1810

	Off:
		leave
		off

	Reminder:
		reminder set 0900 1700
		reminder off
` + "```"
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
				s.respond(ev.Channel, fmt.Sprintf(":warning: %s", err))
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
		fields := strings.Fields(ev.Msg.Text)

		var employeeID string
		code := ""
		if len(fields) == 2 {
			employeeID = fields[1]
		} else if len(fields) == 3 {
			employeeID = fields[1]
			code = fields[2]
			if utf8.RuneCountInString(code) != 64 {
				return s.respond(ev.Channel, ":warning: Invalid authorization code.")
			}
		} else {
			return s.respond(ev.Channel, ":warning: Invalid parameters.")
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

		err := user.Save()
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
		fields := strings.Fields(ev.Msg.Text)
		if len(fields) != 3 {
			return s.respond(ev.Channel, ":warning: Invalid parameters.")
		}

		code := fields[2]
		if utf8.RuneCountInString(code) != 64 {
			return s.respond(ev.Channel, ":warning: Invalid authorization code.")
		}

		token, err := Token(code)
		if err != nil {
			return err
		}

		user := User{
			SlackUserID:    "admin",
			SlackChannelID: ev.Channel,
			EmployeeID:     "",
			Token:          *token,
		}

		err = user.Save()
		if err != nil {
			return err
		}

		return s.respond(ev.Channel, ":ok: Saved the admin access token successfully.")
	}
	if isDirectMessageChannel && ev.Msg.Text == "admin stat" {
		admin, err := FindUser("admin")
		if err != nil {
			return err
		}

		if ev.Channel != admin.SlackChannelID {
			return s.respond(ev.Channel, ":warning: `stat` command requires admin privileges.")
		}

		fileInfo, err := ioutil.ReadDir("users")
		if err != nil {
			return err
		}

		stats := []string{}
		stats = append(stats, "Emoloyee ID  Reminder  Last Used")
		stats = append(stats, "-----------  --------  ----------------")

		for _, file := range fileInfo {
			userID := file.Name()
			if userID == "admin" {
				continue
			}
			user, err := FindUser(userID)
			if err != nil {
				continue
			}

			var reminder string
			if user.Reminder.Enabled {
				reminder = "ON"
			} else {
				reminder = "OFF"
			}
			stats = append(stats, fmt.Sprintf("%-11s  %-8s  %-16s", user.EmployeeID, reminder, user.LastUsed.Format("2006/01/02 15:04")))
		}

		return s.respond(ev.Channel, fmt.Sprintf("```\n%s\n```", strings.Join(stats, "\n")))
	}
	if isDirectMessageChannel && (ev.Msg.Text == "in" || ev.Msg.Text == "out") {
		if _, _, err := s.client.PostMessage(ev.Channel, "", checkInOptions()); err != nil {
			return fmt.Errorf("failed to post message: %s", err)
		}
		return nil
	}
	if isDirectMessageChannel && (strings.HasPrefix(ev.Msg.Text, "in") || strings.HasPrefix(ev.Msg.Text, "out")) {
		fields := strings.Fields(ev.Msg.Text)
		if len(fields) != 2 {
			return s.respond(ev.Channel, ":warning: Invalid parameters.")
		}

		var clock time.Time
		timeParam := fields[1]
		if timeParam == "now" {
			clock = time.Now()
		} else {
			t, err := time.Parse("1504", timeParam)
			if err != nil {
				return err
			}
			now := time.Now()
			tokyoTime := time.FixedZone("Asia/Tokyo", 9*60*60)
			clock = time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, tokyoTime)
		}

		if fields[0] == "in" {
			responseText := fmt.Sprintf(":ok: You have punched in at *%s*.", clock.Format("2006/01/02 15:04"))
			err := PunchInAt(ev.Msg.User, clock)
			if err != nil {
				return err
			}
			return s.respond(ev.Channel, responseText)
		} else {
			responseText := fmt.Sprintf(":ok: You have punched out at *%s*.", clock.Format("2006/01/02 15:04"))
			err := PunchOutAt(ev.Msg.User, clock)
			if err != nil {
				return err
			}
			return s.respond(ev.Channel, responseText)
		}
	}
	if isDirectMessageChannel && (ev.Msg.Text == "leave" || ev.Msg.Text == "off") {
		responseText := ":ok: You are off today. Enjoy :tada:"
		err := PunchLeave(ev.Msg.User)
		if err != nil {
			return err
		}
		return s.respond(ev.Channel, responseText)
	}
	if isDirectMessageChannel && strings.HasPrefix(ev.Msg.Text, "report") {
		go func() {
			fields := strings.Fields(ev.Msg.Text)
			if len(fields) > 3 {
				s.respond(ev.Channel, ":warning: Invalid parameters.")
				return
			}

			jsonFormat := false
			onlyIncomplete := false
			if len(fields) == 2 {
				if fields[1] == "-json" {
					jsonFormat = true
				} else {
					s.respond(ev.Channel, ":warning: Invalid parameters.")
					return
				}
			}
			if len(fields) == 3 {
				if fields[1] == "-json" {
					jsonFormat = true
				} else if fields[1] == "-incomplete" {
					onlyIncomplete = true
				} else {
					s.respond(ev.Channel, ":warning: Invalid parameters.")
					return
				}
				if fields[2] == "-json" {
					jsonFormat = true
				} else if fields[2] == "-incomplete" {
					onlyIncomplete = true
				} else {
					s.respond(ev.Channel, ":warning: Invalid parameters.")
					return
				}
			}

			s.respond(ev.Channel, ":hourglass: Creating timesheet report ...")

			records, err := Report(ev.Msg.User)
			if err != nil {
				s.respond(ev.Channel, fmt.Sprintf(":warning: %s", err))
				sugar.Errorf("%s", err)
				return
			}

			if jsonFormat {
				if onlyIncomplete {
					incompleteRecords := []map[string]interface{}{}
					for _, record := range records {
						if record["in"] == nil && record["out"] == nil && !record["off"].(bool) {
							incompleteRecords = append(incompleteRecords, record)
						}
					}
					records = incompleteRecords
				}

				byte, err := json.MarshalIndent(records, "", "  ")
				if err != nil {
					s.respond(ev.Channel, fmt.Sprintf(":warning: %s", err))
					sugar.Errorf("%s", err)
					return
				}
				s.respond(ev.Channel, string(byte))
			} else {
				results := []string{}
				results = append(results, fmt.Sprintf("Date        In     Out    Off"))
				results = append(results, fmt.Sprintf("----------  -----  -----  ---"))
				for _, record := range records {
					date, _ := time.Parse("2006-01-02", record["date"].(string))
					in := record["in"]
					if in == nil {
						in = "     "
					} else {
						inTime, _ := time.Parse(time.RFC3339, in.(string))
						in = inTime.Format("15:04")
					}
					out := record["out"]
					if out == nil {
						out = "     "
					} else {
						outTime, _ := time.Parse(time.RFC3339, out.(string))
						out = outTime.Format("15:04")
					}
					var off string
					if record["off"].(bool) {
						off = " * "
					} else {
						off = "   "
					}
					results = append(results, fmt.Sprintf("%s  %s  %s  %s", date.Format("2006/01/02"), in, out, off))
				}

				s.respond(ev.Channel, fmt.Sprintf("```\n%s\n```", strings.Join(results, "\n")))
			}
		}()
		return nil
	}
	if isDirectMessageChannel && strings.HasPrefix(ev.Msg.Text, "update") {
		go func() {
			data := strings.Replace(ev.Msg.Text, "update", "", 1)
			var records []map[string]interface{}
			if err := json.Unmarshal([]byte(data), &records); err != nil {
				s.respond(ev.Channel, fmt.Sprintf(":warning: %s", err))
				return
			}

			s.respond(ev.Channel, ":hourglass: Start bulk update ...")
			err := BulkUpdate(ev.User, records)
			if err != nil {
				s.respond(ev.Channel, fmt.Sprintf(":warning: %s", err))
				return
			}

			s.respond(ev.Channel, ":ok: Bulk update finished successfully.")
		}()
		return nil
	}
	if isDirectMessageChannel && strings.HasPrefix(ev.Msg.Text, "reminder set") {
		fields := strings.Fields(ev.Msg.Text)
		if len(fields) != 4 {
			return s.respond(ev.Channel, ":warning: Invalid parameters.")
		}

		user, err := FindUser(ev.User)
		if err != nil {
			return err
		}

		am, err := time.Parse("1504", fields[2])
		if err != nil {
			return err
		}
		pm, err := time.Parse("1504", fields[3])
		if err != nil {
			return err
		}

		user.Reminder.Enabled = true
		user.Reminder.AM = am
		user.Reminder.PM = pm
		err = user.Save()
		if err != nil {
			return err
		}

		return s.respond(ev.Channel, fmt.Sprintf(":ok: The reminders have been set to *%s*/*%s*", am.Format("15:04"), pm.Format("15:04")))
	}
	if isDirectMessageChannel && ev.Msg.Text == "reminder off" {
		responseText := ":ok: The reminders have been turned off."
		user, err := FindUser(ev.User)
		if err != nil {
			return err
		}

		user.Reminder.Enabled = false
		err = user.Save()
		if err != nil {
			return err
		}
		return s.respond(ev.Channel, responseText)
	}

	if ev.Msg.Text == "ping" {
		return s.respond(ev.Channel, "pong")
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
				Name:  actionIn,
				Text:  "Punch in",
				Type:  "button",
				Style: "primary",
			},
			{
				Name:  actionOut,
				Text:  "Punch out",
				Type:  "button",
				Style: "primary",
			},
			{
				Name:  actionLeave,
				Text:  "Leave",
				Type:  "button",
				Style: "danger",
			},
			{
				Name: actionCancel,
				Text: "Cancel",
				Type: "button",
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

			fileInfo, err := ioutil.ReadDir("users")
			if err != nil {
				return err
			}

			for _, file := range fileInfo {
				userID := file.Name()
				if userID == "admin" {
					continue
				}
				user, err := FindUser(userID)
				if err != nil {
					continue
				}
				if !user.Reminder.Enabled {
					continue
				}
				reminder := user.Reminder
				if (now.Hour() != reminder.AM.Hour() || now.Minute() != reminder.AM.Minute()) && (now.Hour() != reminder.PM.Hour() || now.Minute() != reminder.PM.Minute()) {
					continue
				}
				if _, _, err := s.client.PostMessage(user.SlackChannelID, "", checkInOptions()); err != nil {
					return fmt.Errorf("failed to post message: %s", err)
				}
			}
		}
	}
}
