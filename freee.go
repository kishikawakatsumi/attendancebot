package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"golang.org/x/oauth2"
	"io/ioutil"
	"net/http"
	"time"
)

type User struct {
	SlackUserID    string       `json:"slack_user_id"`
	SlackChannelID string       `json:"slack_channel_id"`
	EmployeeID     string       `json:"emp_id"`
	Token          oauth2.Token `json:"token"`
}

func AuthConfig() oauth2.Config {
	config := oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://secure.freee.co.jp/oauth/authorize",
			TokenURL: "https://api.freee.co.jp/oauth/token",
		},
		RedirectURL: "urn:ietf:wg:oauth:2.0:oob",
	}
	return config
}

func AuthCodeURL() string {
	config := AuthConfig()
	return config.AuthCodeURL("")
}

func Token(code string) (*oauth2.Token, error) {
	config := AuthConfig()
	token, err := config.Exchange(context.Background(), code)
	if err != nil {
		return nil, err
	}
	return token, nil
}

func findUser(userID string) (*User, error) {
	data, err := ioutil.ReadFile(fmt.Sprintf("users/%s", userID))
	if err != nil {
		return nil, fmt.Errorf("failed to find user [%s]: %s", userID, err)
	}

	var user User
	if err := json.Unmarshal(data, &user); err != nil {
		return nil, err
	}

	return &user, nil
}

func httpClient(user *User) (*http.Client, error) {
	config := AuthConfig()
	var client *http.Client
	if user.Token.AccessToken != "" {
		client = config.Client(context.Background(), &user.Token)
	} else {
		admin, err := findUser("admin")
		if err != nil {
			return nil, err
		}
		client = config.Client(context.Background(), &admin.Token)
	}
	return client, nil
}

func PunchIn(userID string) error {
	user, err := findUser(userID)
	if err != nil {
		return fmt.Errorf("cannot find the user '%s': %s", userID, err)
	}

	client, err := httpClient(user)
	if err != nil {
		return err
	}

	location := time.FixedZone("Asia/Tokyo", 9*60*60)
	now := time.Now().In(location)
	endpoint := fmt.Sprintf("https://api.freee.co.jp/hr/api/v1/employees/%s/work_records/%s", user.EmployeeID, now.Format("2006-01-02"))

	jsonStr := `{"break_records":[],"clock_in_at":"` + now.Format(time.RFC3339) + `","clock_out_at":"` + now.Add(9 * time.Hour).Format(time.RFC3339) + `"}`
	request, err := http.NewRequest("PUT", endpoint, bytes.NewBuffer([]byte(jsonStr)))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := client.Do(request)
	if err != nil {
		return err
	}

	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body '%s': %s", userID, err)
	}

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to request:\n\tstatus code: %d\n\tresponse: %s", response.StatusCode, string(data))
	}

	return nil
}

func PunchOut(userID string) error {
	user, err := findUser(userID)
	if err != nil {
		return err
	}

	client, err := httpClient(user)
	if err != nil {
		return err
	}

	location := time.FixedZone("Asia/Tokyo", 9*60*60)
	now := time.Now().In(location)
	endpoint := fmt.Sprintf("https://api.freee.co.jp/hr/api/v1/employees/%s/work_records/%s", user.EmployeeID, now.Format("2006-01-02"))

	response, err := client.Get(endpoint)
	if err != nil {
		return err
	}
	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return err
	}
	var resObj map[string]interface{}
	if err := json.Unmarshal(data, &resObj); err != nil {
		return err
	}

	clockInAt := resObj["clock_in_at"]
	var inTime string
	if clockInAt == nil {
		inTime = now.Add(-1 * time.Minute).Format(time.RFC3339)
	} else {
		inTime = clockInAt.(string)
		inDate, _ := time.Parse(time.RFC3339, inTime)
		if inDate.Unix() > now.Unix() {
			inTime = now.Add(-9 * time.Hour).Format(time.RFC3339)
		}
	}

	jsonStr := `{"break_records":[],"clock_in_at":"` + inTime + `","clock_out_at":"` + now.Format(time.RFC3339) + `"}`
	request, err := http.NewRequest("PUT", endpoint, bytes.NewBuffer([]byte(jsonStr)))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")

	response, err = client.Do(request)
	if err != nil {
		return err
	}

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to request: %d", response.StatusCode)
	}

	return nil
}

func PunchLeave(userID string) error {
	user, err := findUser(userID)
	if err != nil {
		return err
	}

	client, err := httpClient(user)
	if err != nil {
		return err
	}

	location := time.FixedZone("Asia/Tokyo", 9*60*60)
	now := time.Now().In(location)
	endpoint := fmt.Sprintf("https://api.freee.co.jp/hr/api/v1/employees/%s/work_records/%s", user.EmployeeID, now.Format("2006-01-02"))

	jsonStr := `{"is_absence":true}`
	request, err := http.NewRequest("PUT", endpoint, bytes.NewBuffer([]byte(jsonStr)))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := client.Do(request)
	if err != nil {
		return err
	}

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to request: %d", response.StatusCode)
	}

	return nil
}
