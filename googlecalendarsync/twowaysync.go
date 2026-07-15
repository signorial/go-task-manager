package googlecalendarsync

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lufraser/gotaskmanager/models"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
	_ "modernc.org/sqlite" // Pure Go SQLite driver (CGO-free)
)

const (
	dbFile          = "calendar_sync.db"
	credentialsFile = "credentials.json"
	tokenFile       = "token.json"
)

// Event maps directly to your database schema using struct tags
type Event struct {
	ID               string `db:"id"`
	Summary          string `db:"summary"`
	Description      string `db:"description"`
	StartTime        string `db:"start_time"`
	EndTime          string `db:"end_time"`
	UpdatedAt        string `db:"updated_at"`
	UpdateTasksDB    bool   `db:"update_tasks_db"`
	UpdateCalendar   bool   `db:"update_calendar"`
	Deleted          bool   `db:"deleted"`
	FK_tasks_task_id int64  `db:"FK_tasks_task_id"`
}

func TwoWaySync(db *sqlx.DB) error {
	ctx := context.Background()

	// 2. Auth Google Client
	client, err := getClient(ctx, credentialsFile)
	if err != nil {
		log.Fatalf("OAuth setup failed: %v", err)
	}

	srv, err := calendar.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		log.Fatalf("Calendar service setup failed: %v", err)
	}

	// 3. Two-Way Sync Workflow
	fmt.Println("➡️ Pushing local SQLite modifications to Google Calendar...")
	if err := pushLocalChanges(db, srv); err != nil {
		log.Printf("⚠️ Error pushing local changes: %v", err)
	}

	fmt.Println("⬅️ Pulling incremental changes from Google Calendar...")
	if err := pullRemoteChanges(db, srv); err != nil {
		log.Fatalf("❌ Error pulling remote changes: %v", err)
	}

	fmt.Println("✅ Synchronization round-trip complete.")
	return nil
}

// func initDatabase(db *sqlx.DB) error {
// 	queries := []string{
// 		`CREATE TABLE IF NOT EXISTS sync_meta (
// 			key TEXT PRIMARY KEY,
// 			value TEXT
// 		);`,
// 		`CREATE TABLE IF NOT EXISTS events (
// 			id 								TEXT PRIMARY KEY,
// 			summary 					TEXT,
// 			description 			TEXT,
// 			start_time 				TEXT,
// 			end_time 					TEXT,
// 			updated_at 				TEXT,
// 			update_tasks_db 		INTEGER DEFAULT 0,
// 			update_calendar 		INTEGER DEFAULT 0,
// 			deleted					 	INTEGER DEFAULT 0,
// 			FK_tasks_task_id	INTEGER
// 		  FOREIGN KEY (FK_tasks_task_id)
// 			REFERENCES tasks(task_id)
// 		);`,
// 	}
// 	for _, q := range queries {
// 		if _, err := db.Exec(q); err != nil {
// 			return err
// 		}
// 	}
// 	return nil
// }

// --- GOOGLE TO SQLITE (PULL VIA SYNC TOKEN) ---
func pullRemoteChanges(db *sqlx.DB, srv *calendar.Service) error {
	var syncToken string
	err := db.Get(&syncToken, "SELECT value FROM sync_meta WHERE key = 'sync_token'")
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	req := srv.Events.List("primary").ShowDeleted(true).SingleEvents(true)
	if syncToken != "" {
		req.SyncToken(syncToken)
	} else {
		req.TimeMin(time.Now().AddDate(0, 0, -30).Format(time.RFC3339))
	}

	for {
		events, err := req.Do()
		if err != nil {
			var gErr *googleapi.Error
			if errors.As(err, &gErr) && gErr.Code == http.StatusGone {
				fmt.Println("Sync token expired. Resetting baseline...")
				_, _ = db.Exec("DELETE FROM sync_meta WHERE key = 'sync_token'")
				return pullRemoteChanges(db, srv)
			}
			return err
		}

		tx, err := db.Beginx()
		if err != nil {
			return err
		}

		for _, item := range events.Items {
			var deleted int64
			if item.Status == "cancelled" {
				deleted = 1
			} else {
				deleted = 0
			}

			start := item.Start.DateTime
			if start == "" {
				start = item.Start.Date
			}
			end := item.End.DateTime
			if end == "" {
				end = item.End.Date
			}

			_, err = tx.Exec(`
					INSERT INTO events (id, summary, description, start_time, end_time, updated_at, update_tasks_db,update_calendar, deleted)
					VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
					ON CONFLICT(id) DO UPDATE SET
						summary=excluded.summary,
						description=excluded.description,
						start_time=excluded.start_time,
						end_time=excluded.end_time,
						updated_at=excluded.updated_at,
						update_tasks_db=excluded.update_tasks_db,
						update_calendar=excluded.update_calendar,
						deleted=excluded.deleted
				`, item.Id, item.Summary, item.Description, start, end, item.Updated, 1, 0, deleted)
			if err != nil {
				_ = tx.Rollback()
				return err
			}
		}

		if err := tx.Commit(); err != nil {
			return err
		}

		if events.NextPageToken != "" {
			req.PageToken(events.NextPageToken)
		} else {
			if events.NextSyncToken != "" {
				_, err = db.Exec(`INSERT INTO sync_meta (key, value) VALUES ('sync_token', ?) 
					ON CONFLICT(key) DO UPDATE SET value=excluded.value`, events.NextSyncToken)
				if err != nil {
					return err
				}
			}
			break
		}
	}
	return nil
}

// ---  SQLITE EVENTS to TASKS ---
func updatetaskswithevents(db *sqlx.DB) error {
	var localEvents []Event
	var task models.Task

	// sqlx automatically maps database fields to struct attributes
	err := db.Select(&localEvents, "SELECT * FROM events WHERE UpdateTasksDB = 1")
	if err != nil {
		return err
	}

	for _, ev := range localEvents {
		if ev.Deleted {
			_, _ = db.Exec("UPDATE tasks SET deleted = 1  WHERE task_id = ?", ev.FK_tasks_task_id) // no longer deleting items
			continue
		}

		task, err = models.DBGetTask(db, ev.FK_tasks_task_id)
		evTask := convertEventToTask(ev, task)
		log.Printf("evtask %v", evTask)

		// 	_, err = tx.Exec(`
		// 		INSERT INTO tasks (id, summary, description, start_time, end_time, updated_at, UpdateTasksDB,UpdateCalendar, deleted,FK_tasks_task_id)
		// 		VALUES (?, ?, ?, ?, ?, ?, 0, 0)
		// 		ON CONFLICT(id) DO UPDATE SET
		// 			summary=excluded.summary,
		// 			description=excluded.description,
		// 			start_time=excluded.start_time,
		// 			end_time=excluded.end_time,
		// 			updated_at=excluded.updated_at,
		// 			UpdateTasksDB=1,
		// 			UpdateCalendar=0,
		// 			deleted=0
		// 	`, item.Id, item.Summary, item.Description, start, end, item.Updated)
		// }

		// func DBAddTask(db *sqlx.DB, task Task) (int64, error) {
		// 	query := `INSERT INTO tasks (
		//                 description, status, created_at, updated_at, priority,
		//                 assignee_id, do_date, final_due_date, start_time, end_time,
		//                 completed_at, estimated_hours, progress, parent_task_id,deleted
		//             ) VALUES (
		//                 :description, :status, :created_at, :updated_at, :priority,
		//                 :assignee_id, :do_date, :final_due_date, :start_time, :end_time,
		//               	:completed_at, :estimated_hours, :progress, :parent_task_id,:deleted
		//             )`
		//
	}

	return nil
}

// convert task to event
func convertEventToTask(e Event, t models.Task) (models.Task, error) {
	t.Description = e.Description
	// status:
	// created_at:
	var err error

	dt, err := time.Parse(time.RFC3339, e.UpdatedAt)
	if err != nil {
		log.Printf("failed to convert string date to time %s, %v", e.UpdatedAt, err)
	} else {
		t.UpdatedAt = &dt
	}
	// priority:
	// assignee_id:
	// do_date:
	dt, err = time.Parse(time.RFC3339, e.EndTime)
	if err != nil {
		log.Printf("failed to convert string date to time %s, %v", e.EndTime, err)
	} else {
		t.FinalDueDate = &dt
	}
	dt, err = time.Parse(time.RFC3339, e.StartTime)
	if err != nil {
		log.Printf("failed to convert string date to time %s, %v", e.StartTime, err)
	} else {
		t.StartTime = &dt
	}
	dt, err = time.Parse(time.RFC3339, e.EndTime)
	if err != nil {
		log.Printf("failed to convert string date to time %s, %v", e.EndTime, err)
	} else {
		t.EndTime = &dt
	}
	// completed_at:
	// estimated_hours:
	// progress:
	// parent_task_id:
	t.Deleted = e.Deleted

	return t, err
}

// --- SQLITE TO GOOGLE (PUSH VIA UpdateCalendar/DELETED FLAGS) ---

func pushLocalChanges(db *sqlx.DB, srv *calendar.Service) error {
	var localEvents []Event
	// sqlx automatically maps database fields to struct attributes
	err := db.Select(&localEvents, "SELECT * FROM events WHERE UpdateCalendar = 1")
	if err != nil {
		return err
	}

	for _, ev := range localEvents {
		if ev.Deleted {
			err = srv.Events.Delete("primary", ev.ID).Do()
			if err != nil && !isNotFoundError(err) {
				log.Printf("Failed to push deletion for event %s: %v", ev.ID, err)
				continue
			}
			//	_, _ = db.Exec("DELETE FROM events WHERE id = ?", ev.ID) //no longer deleting items
			continue
		}

		gEvent := &calendar.Event{
			Summary:     ev.Summary,
			Description: ev.Description,
			Start:       &calendar.EventDateTime{DateTime: ev.StartTime},
			End:         &calendar.EventDateTime{DateTime: ev.EndTime},
		}

		var apiErr error
		if isLocalOnlyID(ev.ID) {
			gEvent.Id = ""
			res, err := srv.Events.Insert("primary", gEvent).Do()
			if err == nil {
				_, _ = db.Exec("UPDATE events SET id = ?, UpdateCalendar = 0, updated_at = ? WHERE id = ?", res.Id, res.Updated, ev.ID)
			}
			apiErr = err
		} else {
			res, err := srv.Events.Update("primary", ev.ID, gEvent).Do()
			if err == nil {
				_, _ = db.Exec("UPDATE events SET UpdateCalendar = 0, updated_at = ? WHERE id = ?", res.Updated, ev.ID)
			}
			apiErr = err
		}

		if apiErr != nil {
			log.Printf("Failed to update remote event %s: %v", ev.ID, apiErr)
		}
	}
	return nil
}

// --- HELPER UTILITIES ---

func isNotFoundError(err error) bool {
	var gErr *googleapi.Error
	return errors.As(err, &gErr) && gErr.Code == http.StatusNotFound
}

func isLocalOnlyID(id string) bool {
	return len(id) >= 6 && id[:6] == "local-"
}

func getClient(ctx context.Context, credentialsFile string) (*http.Client, error) {
	b, err := os.ReadFile(credentialsFile)
	if err != nil {
		return nil, err
	}
	config, err := google.ConfigFromJSON(b, calendar.CalendarScope)
	if err != nil {
		return nil, err
	}
	tok, err := tokenFromFile(tokenFile)
	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(tokenFile, tok)
	}
	return config.Client(ctx, tok), nil
}

func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to browser: \n%v\nType code: ", authURL)
	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		log.Fatalf("Code read error: %v", err)
	}
	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		log.Fatalf("Exchange error: %v", err)
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
	return tok, err
}

func saveToken(path string, token *oauth2.Token) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		log.Fatalf("Cache error: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}
