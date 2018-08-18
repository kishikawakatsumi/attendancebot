package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/dustin/go-humanize"
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
	now := now()
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

	clockIn := inTime.In(JST())
	endpoint := fmt.Sprintf("%s/api/v1/employees/%s/work_records/%s", apiBase, user.EmployeeID, clockIn.Format("2006-01-02"))

	parameters := `{"break_records":[],"clock_in_at":"` + clockIn.Format(time.RFC3339) + `","clock_out_at":"` + clockIn.Add(9*time.Hour).Format(time.RFC3339) + `","is_absence":false}`
	_, err = DoPut(client, endpoint, parameters)
	if err != nil {
		return err
	}

	user.LastUsed = time.Now()
	user.Save()

	return nil
}

func PunchOut(userID string) error {
	now := now()
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

	clockOut := outTime.In(JST())
	endpoint := fmt.Sprintf("%s/api/v1/employees/%s/work_records/%s", apiBase, user.EmployeeID, clockOut.Format("2006-01-02"))

	record, err := DoGet(client, endpoint)
	if err != nil {
		return err
	}

	clockInAt := record["clock_in_at"]
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

	parameters := `{"break_records":[],"clock_in_at":"` + inTime + `","clock_out_at":"` + clockOut.Format(time.RFC3339) + `","is_absence":false}`
	_, err = DoPut(client, endpoint, parameters)
	if err != nil {
		return err
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

	now := now()
	endpoint := fmt.Sprintf("%s/api/v1/employees/%s/work_records/%s", apiBase, user.EmployeeID, now.Format("2006-01-02"))

	parameters := `{"is_absence":true}`
	_, err = DoPut(client, endpoint, parameters)
	if err != nil {
		return err
	}

	user.LastUsed = time.Now()
	user.Save()

	return nil
}

func Report(userID string) ([]map[string]interface{}, error) {
	user, err := FindUser(userID)
	if err != nil {
		return nil, err
	}

	client, err := httpClient(user)
	if err != nil {
		return nil, err
	}

	records := []map[string]interface{}{}
	now := now()
	start, err := time.Parse("2006-1-2", fmt.Sprintf("%d-%d-1", now.Year(), now.Month()))
	if err != nil {
		return nil, err
	}
	for d := start; d.Day() <= now.Day(); d = d.AddDate(0, 0, 1) {
		endpoint := fmt.Sprintf("%s/api/v1/employees/%s/work_records/%s", apiBase, user.EmployeeID, d.Format("2006-01-02"))
		record, err := DoGet(client, endpoint)
		if err != nil {
			return nil, err
		}
		if record["day_pattern"] != "normal_day" {
			continue
		}

		records = append(records, map[string]interface{}{"date": record["date"], "in": record["clock_in_at"], "out": record["clock_out_at"], "off": record["is_absence"]})
	}

	return records, nil
}

func BulkUpdate(userID string, records []map[string]interface{}) error {
	user, err := FindUser(userID)
	if err != nil {
		return err
	}

	client, err := httpClient(user)
	if err != nil {
		return err
	}

	for i, record := range records {
		var date string
		if record["date"] != nil {
			date = record["date"].(string)
		} else {
			return fmt.Errorf("an error occurred while processing the %s record", humanize.Ordinal(i+1))
		}
		in := ""
		if record["in"] != nil {
			in = record["in"].(string)
		}
		out := ""
		if record["out"] != nil {
			out = record["out"].(string)
		}
		off := false
		if record["off"] != nil {
			off = record["off"].(bool)
		}

		var dateTime time.Time
		dateTime, err = time.Parse("2006-01-02", date)
		if err != nil {
			return fmt.Errorf("an error occurred while processing the %s record", humanize.Ordinal(i+1))
		}

		var inTime time.Time
		var outTime time.Time
		if !off {
			inTime, err = time.Parse(time.RFC3339, in)
			if err != nil {
				inTime, err = time.Parse("15:04", in)
				if err != nil {
					inTime, err = time.Parse("1504", in)
					if err != nil {
						return fmt.Errorf("an error occurred while processing the %s record", humanize.Ordinal(i+1))
					}
				}
				inTime = time.Date(dateTime.Year(), dateTime.Month(), dateTime.Day(), inTime.Hour(), inTime.Minute(), 0, 0, JST())
			}

			outTime, err = time.Parse(time.RFC3339, out)
			if err != nil {
				outTime, err = time.Parse("15:04", out)
				if err != nil {
					outTime, err = time.Parse("1504", out)
					if err != nil {
						return fmt.Errorf("an error occurred while processing the %s record", humanize.Ordinal(i))
					}
				}
				outTime = time.Date(dateTime.Year(), dateTime.Month(), dateTime.Day(), outTime.Hour(), outTime.Minute(), 0, 0, JST())
			}
		}

		endpoint := fmt.Sprintf("%s/api/v1/employees/%s/work_records/%s", apiBase, user.EmployeeID, dateTime.Format("2006-01-02"))
		var jsonStr string
		if off {
			jsonStr = `{"is_absence":true}`
		} else {
			jsonStr = `{"break_records":[],"clock_in_at":"` + inTime.Format(time.RFC3339) + `","clock_out_at":"` + outTime.Format(time.RFC3339) + `","is_absence":false}`
		}
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
	}

	user.LastUsed = time.Now()
	user.Save()

	return nil
}

func IsNormalDay(userID string) bool {
	user, err := FindUser(userID)
	if err != nil {
		return false
	}

	client, err := httpClient(user)
	if err != nil {
		return false
	}

	now := now()
	endpoint := fmt.Sprintf("%s/api/v1/employees/%s/work_records/%s", apiBase, user.EmployeeID, now.Format("2006-01-02"))

	record, err := DoGet(client, endpoint)
	if err != nil {
		return false
	}

	return record["day_pattern"] == "normal_day"
}

func DoGet(client *http.Client, endpoint string) (map[string]interface{}, error) {
	response, err := client.Get(endpoint)
	if err != nil {
		return nil, err
	}
	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	var record map[string]interface{}
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, err
	}

	return record, nil
}

func DoPut(client *http.Client, endpoint string, parameters string) (*http.Response, error) {
	request, err := http.NewRequest("PUT", endpoint, bytes.NewBuffer([]byte(parameters)))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}

	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %s", err)
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to request:\n\tstatus code: %d\n\tresponse: %s", response.StatusCode, string(data))
	}

	return response, nil
}

func now() time.Time {
	return time.Now().In(JST())
}

func JST() *time.Location {
	return time.FixedZone("Asia/Tokyo", 9*60*60)
}