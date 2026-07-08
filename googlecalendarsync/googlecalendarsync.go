// Package googlecalendarsync this handles all the logic for syncing to Google
package googlecalendarsync

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lufraser/gotaskmanager/models"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

type LocalTask struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	Description   string    `json:"description"`
	StartTime     time.Time `json:"start_time"`
	EndTime       time.Time `json:"end_time"`
	GoogleEventID string    `json:"google_event_id"`
	UpdateAt      time.Time `json:"update_at"`
}

// func main() {
// 	ctx := context.Background()
//
// 	// read the google api credentials
// 	b, err := os.ReadFile("credentials.json")
// 	if err != nil {
// 		fmt.Errorf("unable to read the credentials file: %v", err)
// 	}
//
// 	// request read write access to calendar
// 	config, err := google.ConfigFromJSON(b, calendar.CalendarScope)
// 	if err != nil {
// 		fmt.Errorf("Unable to parse client secret file: %v", err)
// 	}
//
// 	client := getClient(config)
//
// 	srv, err := calendar.NewService(ctx, option.WithHTTPClient(client))
// 	if err != nil {
// 		fmt.Errorf("Unable to retrieve Calendar client: %v", err)
// 	}
// }

// --- OAUTH2 UTILITIES FOR TOKEN MANAGEMENT ---
func getClient(config *oauth2.Config) *http.Client {
	tokFile := "token.json"
	tok, err := tokenFromFile(tokFile)
	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(tokFile, tok)
	}
	return config.Client(context.Background(), tok)
}

func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		fmt.Errorf("Unable to read authorization code %v", err)
	}

	tok, err := config.Exchange(context.Background(), authCode)
	if err != nil {
		fmt.Errorf("Unable to retrieve token from web %v", err)
	}
	return tok
}

func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, nil
}

func saveToken(path string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		fmt.Errorf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

func getCalendarService() (*calendar.Service, error) {
	b, err := os.ReadFile("../credentials.json")
	if err != nil {
		return nil, fmt.Errorf("unable to read credentials.json: %v", err)
	}
	config, err := google.ConfigFromJSON(b, calendar.CalendarScope)
	if err != nil {
		return nil, fmt.Errorf("unable to parse client secret: %v", err)
	}
	client := getClient(config)
	return calendar.NewService(context.Background(), option.WithHTTPClient(client))
}

func taskToAllDayEvent(task models.Task) *calendar.Event {
	if task.DoDate == nil {
		return nil
	}
	dateStr := task.DoDate.Format("2006-01-02")
	return &calendar.Event{
		Summary: task.Description,
		Start:   &calendar.EventDateTime{Date: dateStr},
		End:     &calendar.EventDateTime{Date: dateStr},
	}
}

func hasConflict(localUpdated, googleUpdated, lastSynced time.Time) bool {
	return localUpdated.After(lastSynced) && googleUpdated.After(lastSynced)
}

func SyncTask(db *sqlx.DB, task models.Task) error {
	if task.DoDate == nil || task.TaskID == nil {
		return nil
	}

	srv, err := getCalendarService()
	if err != nil {
		return err
	}

	eventID, lastSynced, err := models.GetGoogleEventID(db, *task.TaskID)
	event := taskToAllDayEvent(task)
	if event == nil {
		return nil
	}

	if eventID != "" {
		// existing event – check for conflict
		existing, err := srv.Events.Get("primary", eventID).Do()
		if err == nil && hasConflict(
			*task.UpdatedAt,
			parseGoogleTime(existing.Updated),
			lastSynced,
		) {
			return fmt.Errorf("conflict detected")
		}
		_, err = srv.Events.Update("primary", eventID, event).Do()
		if err != nil {
			return err
		}
		return models.UpdateLastSyncedAt(db, *task.TaskID)
	}

	// new event
	created, err := srv.Events.Insert("primary", event).Do()
	if err != nil {
		return err
	}
	return models.SaveGoogleEventMapping(db, *task.TaskID, created.Id)
}

func DeleteTask(db *sqlx.DB, taskID int64) error {
	eventID, _, err := models.GetGoogleEventID(db, taskID)
	if err != nil || eventID == "" {
		return nil
	}

	srv, err := getCalendarService()
	if err != nil {
		return err
	}
	if err := srv.Events.Delete("primary", eventID).Do(); err != nil {
		return err
	}
	return models.DeleteGoogleEventMapping(db, taskID)
}

func parseGoogleTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}
