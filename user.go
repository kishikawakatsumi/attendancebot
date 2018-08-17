package main

import (
	"encoding/json"
	"fmt"
	"golang.org/x/oauth2"
	"io/ioutil"
	"time"
)

type User struct {
	SlackUserID    string       `json:"slack_user_id"`
	SlackChannelID string       `json:"slack_channel_id"`
	EmployeeID     string       `json:"emp_id"`
	Reminder       Reminder     `json:"reminder"`
	LastUsed       time.Time    `json:"last_used"`
	Token          oauth2.Token `json:"token"`
}

type Reminder struct {
	Enabled bool      `json:"enabled"`
	AM      time.Time `json:"am"`
	PM      time.Time `json:"pm"`
}

func FindUser(userID string) (*User, error) {
	data, err := ioutil.ReadFile(fmt.Sprintf("users/%s", userID))
	if err != nil {
		return nil, fmt.Errorf("failed to find user [%s]: %s", userID, err)
	}

	am, _ := time.Parse("1504", "0900")
	pm, _ := time.Parse("1504", "1700")
	user := User{
		Reminder: Reminder{
			Enabled: true,
			AM:      am,
			PM:      pm,
		},
	}
	if err := json.Unmarshal(data, &user); err != nil {
		return nil, err
	}

	return &user, nil
}

func (u *User) Save() error {
	text, err := json.Marshal(*u)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(fmt.Sprintf("users/%s", u.SlackUserID), text, 0644)
	if err != nil {
		return err
	}

	return nil
}
