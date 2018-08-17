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

const apiBase = "https://api.freee.co.jp/hr"

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

func RefreshToken(config oauth2.Config, token oauth2.Token) (*oauth2.Token, error) {
	tokenSource := config.TokenSource(context.Background(), &token)
	newToken, err := tokenSource.Token()
	if err != nil {
		return nil, err
	}
	return newToken, nil
}

func httpClient(user *User) (*http.Client, error) {
	config := AuthConfig()
	var client *http.Client
	if user.Token.AccessToken != "" {
		token, err := RefreshToken(config, user.Token)
		if err != nil {
			return nil, err
		}
		if token.AccessToken != user.Token.AccessToken {
			user.Token = *token
			user.Save()
		}

		client = config.Client(context.Background(), &user.Token)
	} else {
		admin, err := FindUser("admin")
		if err != nil {
			return nil, err
		}

		token, err := RefreshToken(config, admin.Token)
		if err != nil {
			return nil, err
		}
		if token.AccessToken != admin.Token.AccessToken {
			admin.Token = *token
			admin.Save()
		}

		client = config.Client(context.Background(), &admin.Token)
	}
	return client, nil
}

func PunchIn(userID string) error {
	location := time.FixedZone("Asia/Tokyo", 9*60*60)
	now := time.Now().In(location)
	return PunchInAt(userID, now)
}

func PunchInAt(userID string, inTime time.Time) error {
	user, err := FindUser(userID)
	if err != nil {
		return fmt.Errorf("cannot find the user '%s': %s", userID, err)
	}

	client, err := httpClient(user)
	if err != nil {
		return err
	}

	location := time.FixedZone("Asia/Tokyo", 9*60*60)
	clockIn := inTime.In(location)
	endpoint := fmt.Sprintf("%s/api/v1/employees/%s/work_records/%s", apiBase, user.EmployeeID, clockIn.Format("2006-01-02"))

	jsonStr := `{"break_records":[],"clock_in_at":"` + clockIn.Format(time.RFC3339) + `","clock_out_at":"` + clockIn.Add(9*time.Hour).Format(time.RFC3339) + `"}`
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
		return fmt.Errorf("failed to read response body: %s", err)
	}
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to request:\n\tstatus code: %d\n\tresponse: %s", response.StatusCode, string(data))
	}

	user.LastUsed = time.Now()
	user.Save()

	return nil
}

func PunchOut(userID string) error {
	location := time.FixedZone("Asia/Tokyo", 9*60*60)
	now := time.Now().In(location)
	return PunchOutAt(userID, now)
}

func PunchOutAt(userID string, outTime time.Time) error {
	user, err := FindUser(userID)
	if err != nil {
		return err
	}

	client, err := httpClient(user)
	if err != nil {
		return err
	}

	location := time.FixedZone("Asia/Tokyo", 9*60*60)
	clockOut := outTime.In(location)
	endpoint := fmt.Sprintf("%s/api/v1/employees/%s/work_records/%s", apiBase, user.EmployeeID, clockOut.Format("2006-01-02"))

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
		inTime = clockOut.Add(-1 * time.Minute).Format(time.RFC3339)
	} else {
		inTime = clockInAt.(string)
		inDate, _ := time.Parse(time.RFC3339, inTime)
		if inDate.Unix() > clockOut.Unix() {
			inTime = clockOut.Add(-9 * time.Hour).Format(time.RFC3339)
		}
	}

	jsonStr := `{"break_records":[],"clock_in_at":"` + inTime + `","clock_out_at":"` + clockOut.Format(time.RFC3339) + `"}`
	request, err := http.NewRequest("PUT", endpoint, bytes.NewBuffer([]byte(jsonStr)))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")

	response, err = client.Do(request)
	if err != nil {
		return err
	}

	data, err = ioutil.ReadAll(response.Body)
	if err != nil {
		return err
	}
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to request:\n\tstatus code: %d\n\tresponse: %s", response.StatusCode, string(data))
	}

	user.LastUsed = time.Now()
	user.Save()

	return nil
}

func PunchLeave(userID string) error {
	user, err := FindUser(userID)
	if err != nil {
		return err
	}

	client, err := httpClient(user)
	if err != nil {
		return err
	}

	location := time.FixedZone("Asia/Tokyo", 9*60*60)
	now := time.Now().In(location)
	endpoint := fmt.Sprintf("%s/api/v1/employees/%s/work_records/%s", apiBase, user.EmployeeID, now.Format("2006-01-02"))

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

	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return err
	}
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to request:\n\tstatus code: %d\n\tresponse: %s", response.StatusCode, string(data))
	}

	user.LastUsed = time.Now()
	user.Save()

	return nil
}
