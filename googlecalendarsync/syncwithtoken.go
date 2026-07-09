package googlecalendarsync

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jmoiron/sqlx"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
)

const oneYearAgo = -365 * 24 * time.Hour

// getCalendarService creates a Calendar service using ../credentials.json and token.json
func getCalendarService() (*calendar.Service, error) {
	b, err := os.ReadFile("../credentials.json")
	if err != nil {
		return nil, fmt.Errorf("unable to read credentials.json: %w", err)
	}
	config, err := google.ConfigFromJSON(b, calendar.CalendarScope)
	if err != nil {
		return nil, fmt.Errorf("unable to parse client secret: %w", err)
	}
	client := getClient(config)
	return calendar.NewService(context.Background(), option.WithHTTPClient(client))
}

// SyncWithToken performs a full or incremental sync of the primary calendar
// into google_calendar_events. Cancelled events are tombstoned (deleted=1).
func SyncWithToken(db *sqlx.DB) error {
	srv, err := getCalendarService()
	if err != nil {
		return fmt.Errorf("calendar service: %w", err)
	}

	syncToken := loadSyncToken(db)

	var request *calendar.EventsListCall
	if syncToken == "" {
		fmt.Println("Performing full sync (no sync token).")
		request = srv.Events.List("primary").
			TimeMin(time.Now().Add(oneYearAgo).Format(time.RFC3339))
	} else {
		fmt.Println("Performing incremental sync.")
		request = srv.Events.List("primary").SyncToken(syncToken)
	}

	var nextSyncToken string
	pageToken := ""
	for {
		if pageToken != "" {
			request = request.PageToken(pageToken)
		}

		events, err := request.Do()
		if err != nil {
			if gerr, ok := err.(*googleapi.Error); ok && gerr.Code == 410 {
				fmt.Println("Sync token invalid (410). Clearing state and full resync.")
				if err := clearSyncState(db); err != nil {
					return err
				}
				return SyncWithToken(db)
			}
			return fmt.Errorf("events list: %w", err)
		}

		for _, ev := range events.Items {
			if err := syncOneEvent(db, ev); err != nil {
				log.Printf("sync event %s failed: %v", ev.Id, err)
			}
		}

		pageToken = events.NextPageToken
		nextSyncToken = events.NextSyncToken
		if pageToken == "" {
			break
		}
	}

	if nextSyncToken != "" {
		saveSyncToken(db, nextSyncToken)
	}
	fmt.Println("Google Calendar sync complete.")
	return nil
}

func syncOneEvent(db *sqlx.DB, ev *calendar.Event) error {
	if ev.Status == "cancelled" {
		_, err := db.Exec(`UPDATE google_calendar_events SET deleted=1, synced_at=CURRENT_TIMESTAMP WHERE google_event_id=?`, ev.Id)
		if err != nil {
			return err
		}
		fmt.Printf("Tombstoned event: %s\n", ev.Id)
		return nil
	}

	start := ""
	if ev.Start != nil {
		start = ev.Start.DateTime
		if start == "" {
			start = ev.Start.Date
		}
	}
	end := ""
	if ev.End != nil {
		end = ev.End.DateTime
		if end == "" {
			end = ev.End.Date
		}
	}

	_, err := db.NamedExec(`
		INSERT OR REPLACE INTO google_calendar_events
		(google_event_id, summary, description, start_date, end_date, status, updated, raw_json, deleted, synced_at)
		VALUES
		(:id, :summary, :description, :start, :end, :status, :updated, :raw, 0, CURRENT_TIMESTAMP)`,
		map[string]any{
			"id":          ev.Id,
			"summary":     ev.Summary,
			"description": ev.Description,
			"start":       start,
			"end":         end,
			"status":      ev.Status,
			"updated":     ev.Updated,
			"raw":         "", // set to json.Marshal(ev) if full payload desired
		})
	return err
}

func loadSyncToken(db *sqlx.DB) string {
	var token string
	_ = db.Get(&token, `SELECT sync_token FROM google_sync_state WHERE id=1`)
	return token
}

func saveSyncToken(db *sqlx.DB, token string) {
	_, _ = db.Exec(`INSERT OR REPLACE INTO google_sync_state (id, sync_token, last_sync_at) VALUES (1, ?, CURRENT_TIMESTAMP)`, token)
}

func clearSyncState(db *sqlx.DB) error {
	_, err := db.Exec(`DELETE FROM google_sync_state WHERE id=1`)
	if err != nil {
		return err
	}
	_, err = db.Exec(`DELETE FROM google_calendar_events`)
	return err
}
